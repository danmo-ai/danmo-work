"""prompt-pack: write a disk pack, keep gate stdout short (path + size + write target)."""
from __future__ import annotations

from pathlib import Path

from .common import Report, write_book_text
from .outline import unit_prose_rel

PACK_DIR = ".pack"
# write pack must fit one read_file (~50k chars); hard cap with honest stdout bytes
PACK_WRITE_MAX_BYTES = 45000

STAGE_WRITE = "write"
STAGE_OUTLINE = "outline"
STAGE_FINALIZE = "finalize"
STAGES = (STAGE_WRITE, STAGE_OUTLINE, STAGE_FINALIZE)


def _utf8_len(text: str) -> int:
    return len(text.encode("utf-8"))


def _clip_context_blocks(lines: list[str], max_bytes: int) -> list[str]:
    """If CONTEXT exceeds budget, drop fat blocks: beats → debts → genre → prev tail."""
    body = "\n".join(lines)
    if _utf8_len(body) <= max_bytes:
        return lines

    def block_ranges(src: list[str], preds: list) -> list[tuple[int, int]]:
        """Inclusive start, exclusive end for each matching top-level `- ` block."""
        ranges: list[tuple[int, int]] = []
        i = 0
        n = len(src)
        while i < n:
            ln = src[i]
            if ln.startswith("- ") and any(p(ln) for p in preds):
                j = i + 1
                while j < n and not src[j].startswith("- "):
                    j += 1
                ranges.append((i, j))
                i = j
                continue
            i += 1
        return ranges

    def shrink_beat_rows(src: list[str]) -> list[str]:
        out = list(src)
        for i, ln in enumerate(out):
            if "章级beat" in ln or ln.strip() == "场面序:":
                # keep header + first 2 data rows; drop rest
                j = i + 1
                kept = 0
                while j < len(out) and not out[j].startswith("- "):
                    if out[j].startswith("    ch") or out[j].startswith("    S") or out[j].startswith("    第"):
                        kept += 1
                        if kept > 2:
                            out[j] = "    …（beat truncated for pack budget）"
                            # drop following beat rows until next top-level
                            k = j + 1
                            while k < len(out) and not out[k].startswith("- "):
                                if out[k].startswith("    "):
                                    out[k] = ""
                                k += 1
                            break
                    j += 1
                break
        return [x for x in out if x != ""]

    def replace_block(src: list[str], start: int, end: int, stub: str) -> list[str]:
        return src[:start] + [stub] + src[end:]

    cur = list(lines)
    # 1) shrink chapter beats / 场面序
    cur = shrink_beat_rows(cur)
    if _utf8_len("\n".join(cur)) <= max_bytes:
        return cur

    # 2) open debts
    for a, b in reversed(block_ranges(cur, [lambda s: s.startswith("- 开放债务")])):
        cur = replace_block(cur, a, b, "- 开放债务: （truncated for pack budget）")
        if _utf8_len("\n".join(cur)) <= max_bytes:
            return cur

    # 3) genre bullets / articles
    for a, b in reversed(
        block_ranges(
            cur,
            [
                lambda s: "题材写时约束" in s or "子类写时约束" in s,
                lambda s: "题材专有文" in s or "子类专有文" in s,
            ],
        )
    ):
        label = cur[a].split("（", 1)[0].rstrip(":")
        cur = replace_block(cur, a, b, f"{label}: （truncated for pack budget）")
        if _utf8_len("\n".join(cur)) <= max_bytes:
            return cur

    # 4) prev prose tail
    for a, b in reversed(block_ranges(cur, [lambda s: s.startswith("- 上章文末")])):
        cur = replace_block(cur, a, b, "- 上章文末: （truncated for pack budget）")
        if _utf8_len("\n".join(cur)) <= max_bytes:
            return cur

    # hard tail clip
    joined = "\n".join(cur)
    raw = joined.encode("utf-8")
    if len(raw) <= max_bytes:
        return cur
    cut = raw[: max_bytes - 40].decode("utf-8", errors="ignore").rstrip()
    return cut.splitlines() + ["…（pack hard-truncated for budget）"]


def pack_filename(stage: str, unit_id: str = "", volume: str = "") -> str:
    st = (stage or "").strip().lower()
    if st == STAGE_OUTLINE:
        return f"outline-{volume or 'vNN'}.md"
    if st == STAGE_FINALIZE:
        return f"finalize-{unit_id or 'unit'}.md"
    return f"write-{unit_id or 'unit'}.md"


def pack_project_rel(book_root: Path, name: str) -> str:
    """Path relative to project cwd (novel/<book-id>/.pack/…), else book-relative."""
    if book_root.parent.name == "novel":
        return f"novel/{book_root.name}/{PACK_DIR}/{name}"
    return f"{PACK_DIR}/{name}"


def _section_map(r: Report) -> dict[str, list[str]]:
    return {str(t).upper(): list(body or []) for t, body in r.extra_sections}


def _hits_need_work(body: list[str]) -> bool:
    lines = [ln.strip() for ln in body if ln.strip()]
    if not lines:
        return False
    if len(lines) == 1 and lines[0].rstrip(".") in ("None", "none"):
        return False
    return True


def _length_needs_expand(body: list[str]) -> bool:
    return any("expand_needed: yes" in ln for ln in body)


def _commit_is_clean(body: list[str]) -> bool:
    text = "\n".join(body)
    return "账本已齐" in text


def write_hint(stage: str, unit_id: str = "", volume: str = "", r: Report | None = None) -> str:
    st = (stage or "").strip().lower()
    if st == STAGE_OUTLINE:
        vol = volume or "vNN"
        return f"outline/units/{vol}-U#.yaml (this batch ≤4 proposed)"
    if st == STAGE_FINALIZE and unit_id and r is not None:
        sm = _section_map(r)
        need_prose = _length_needs_expand(sm.get("LENGTH") or []) or _hits_need_work(sm.get("HITS") or [])
        need_ledger = not _commit_is_clean(sm.get("COMMIT") or [])
        bits: list[str] = []
        if need_prose:
            bits.append(unit_prose_rel(unit_id))
        if need_ledger:
            bits.append("COMMIT skeletons")
        return "; ".join(bits) if bits else "(none — three checks then seal-commit)"
    if unit_id:
        return unit_prose_rel(unit_id)
    return "(see pack)"


def _do_line(stage: str, r: Report) -> str:
    st = (stage or "").strip().lower()
    if st == STAGE_WRITE:
        return (
            "do: read PACK only (genre bullets in CONTEXT; if truncated parallel offset) "
            "→ one write units/vNN-U#.md → seal-write"
        )
    if st == STAGE_OUTLINE:
        return (
            "do: read PACK only (if truncated parallel offset) "
            "→ fill ≤4 proposed YAML → lint-outline (retry once if FAIL)"
        )
    sm = _section_map(r)
    need_prose = _length_needs_expand(sm.get("LENGTH") or []) or _hits_need_work(sm.get("HITS") or [])
    need_ledger = not _commit_is_clean(sm.get("COMMIT") or [])
    if need_prose or need_ledger:
        return (
            "do: read PACK (if truncated parallel offset); patch listed HITS/LENGTH/COMMIT gaps; "
            "then 3× exec_shell parallel check-length + check-deslop + check-commit; "
            "seal-commit if all PASS"
        )
    return (
        "do: read PACK (if truncated parallel offset); no HITS/LENGTH/COMMIT gaps → "
        "3× exec_shell parallel check-length + check-deslop + check-commit → seal-commit"
    )


def materialize_pack(
    r: Report,
    book_root: Path,
    stage: str,
    *,
    unit_id: str = "",
    volume: str = "",
) -> str:
    """Move CONTEXT / extra sections / counts onto disk; stdout keeps PACK pointer.

    Writes even on FAIL so repair can read the pack. Returns project-relative path.
    Write stage CONTEXT is clipped to PACK_WRITE_MAX_BYTES (honest stdout bytes).
    """
    has_body = bool(r.context_lines or r.extra_sections or r.counts)
    if not has_body:
        return ""
    st = (stage or "").strip().lower()
    ctx_lines = list(r.context_lines or [])
    if st == STAGE_WRITE and ctx_lines:
        # Reserve room for ## CONTEXT header + other sections
        other_parts: list[str] = []
        for title, body in r.extra_sections:
            other_parts.append(f"## {title}\n\n" + "\n".join(body if body else ["None."]))
        if r.counts:
            count_lines = [f"{k}: {v}" for k, v in sorted(r.counts.items())]
            other_parts.append("## COUNTS\n\n" + "\n".join(count_lines))
        other = ("\n\n".join(other_parts) + "\n\n") if other_parts else ""
        header = "## CONTEXT\n\n"
        overhead = _utf8_len(header) + _utf8_len(other) + 1
        budget = max(2000, PACK_WRITE_MAX_BYTES - overhead)
        ctx_lines = _clip_context_blocks(ctx_lines, budget)
    parts: list[str] = []
    if ctx_lines:
        parts.append("## CONTEXT\n\n" + "\n".join(ctx_lines))
    for title, body in r.extra_sections:
        parts.append(f"## {title}\n\n" + "\n".join(body if body else ["None."]))
    if r.counts:
        count_lines = [f"{k}: {v}" for k, v in sorted(r.counts.items())]
        parts.append("## COUNTS\n\n" + "\n".join(count_lines))
    name = pack_filename(stage, unit_id, volume)
    rel = pack_project_rel(book_root, name)
    dest = book_root / PACK_DIR / name
    dest.parent.mkdir(parents=True, exist_ok=True)
    text = "\n\n".join(parts).rstrip() + "\n"
    if st == STAGE_WRITE and _utf8_len(text) > PACK_WRITE_MAX_BYTES:
        # last-resort hard cut of whole file (keep header)
        raw = text.encode("utf-8")[: PACK_WRITE_MAX_BYTES - 60]
        text = raw.decode("utf-8", errors="ignore").rstrip() + "\n…（pack hard-truncated for budget）\n"
    write_book_text(dest, text)
    n_lines = text.count("\n") if text.endswith("\n") else text.count("\n") + 1
    n_bytes = _utf8_len(text)
    hint = write_hint(stage, unit_id, volume, r)
    do = _do_line(stage, r)
    r.context_lines = []
    r.extra_sections = []
    r.counts = {}
    r.section(
        "PACK",
        [
            f"file: {rel}",
            f"lines: {n_lines}",
            f"bytes: {n_bytes}",
            f"write: {hint}",
            do,
        ],
    )
    return rel
