"""Continuity ledger: facts.md cursor/snapshot/open loops, per-volume chapter summaries,
locked terms, and the postcommit check."""
from __future__ import annotations

import re
from pathlib import Path

from .common import (
    Report,
    _as_int,
    file_exists,
    load_yaml_map,
    nonempty_list,
    read_book_text,
    read_text,
    rune_count,
    volume_number,
)
from .outline import (
    chapter_range_of,
    load_unit,
    state_delta_who,
    unit_outline_rel,
    unit_prose_rel,
)

MAX_OPEN_DEBTS = 5
OPEN_STATUS = re.compile(r"(?i)\|\s*open\s*\|")
ADVANCED_STATUS = re.compile(r"(?i)\|\s*advanced\s*\|")
SUMMARY_KEYS = ("事件", "状态变化", "伏笔", "钩子", "下章指向")
FS_ID_RE = re.compile(r"FS-\d+", re.I)
CHAPTER_SUMMARY_HEAD_RE = re.compile(r"^## ch(\d{3,})\b", re.M)


def ledger_path(book_root: Path) -> Path:
    """v3: prefer continuity/facts.md; fall back to legacy ledger.md."""
    facts = book_root / "continuity/facts.md"
    if facts.is_file():
        return facts
    return book_root / "continuity/ledger.md"


def summaries_rel(volume: str) -> str:
    return f"continuity/summaries/{volume}.md"


def summaries_path(book_root: Path, volume: str) -> Path:
    return book_root / summaries_rel(volume)


def locked_terms_path(book_root: Path) -> Path:
    return book_root / "canon/locked-terms.yaml"


def load_locked_terms(book_root: Path) -> tuple[dict, list, dict]:
    """v3: parse canon/locked-terms.yaml defensively.
    Returns (locked_until_map, compliance_terms, alias_map)."""
    p = locked_terms_path(book_root)
    if not p.is_file():
        return {}, [], {}
    try:
        text = read_book_text(p)
        data = load_yaml_map(text)
    except Exception:
        return {}, [], {}
    locked = data.get("locked_until") or {}
    compliance = nonempty_list(data.get("compliance"))
    aliases = data.get("aliases") or {}
    return locked, compliance, aliases


def scan_locked_terms(book_root: Path, prose: str, current_volume: str) -> list:
    """Scan prose for locked terms. Returns [(term, reason, count)].
    A term under locked_until for a volume later than current_volume is locked.
    compliance terms are always locked.
    Aliases are scanned only when their base term is currently locked
    (still in locked_until for a later volume, or listed under compliance)."""
    locked, compliance, aliases = load_locked_terms(book_root)
    cur_num = volume_number(current_volume)
    to_check = []
    active_bases: set[str] = set()
    for vol_tag, terms in locked.items():
        vol_num = volume_number(vol_tag)
        if not re.search(r"\d", str(vol_tag)):
            continue
        if vol_num > cur_num:
            for t in nonempty_list(terms):
                to_check.append((t, f"locked until {vol_tag}"))
                active_bases.add(t)
    for t in compliance:
        to_check.append((t, "compliance"))
        active_bases.add(t)
    for base, alist in (aliases or {}).items():
        if base not in active_bases:
            continue
        for a in nonempty_list(alist):
            to_check.append((a, f"alias of {base}"))
    hits = []
    seen = set()
    for term, reason in to_check:
        if not term or term in seen:
            continue
        seen.add(term)
        count = prose.count(term)
        if count > 0:
            hits.append((term, reason, count))
    return hits


def apply_lock_and_scale_checks(
    book_root: Path, unit_id: str, u: dict, prose: str, r: Report
) -> None:
    """Hard checks for lock_terms / word_floor / unit_size (precommit + postcommit)."""
    current_volume = unit_id.split("-")[0] if "-" in unit_id else ""
    lock_hits = scan_locked_terms(book_root, prose, current_volume)
    if lock_hits:
        detail = "; ".join(f"{t}({why})x{n}" for t, why, n in lock_hits)
        r.blocking("lock_terms", f"locked terms leaked in prose: {detail}")

    unit_runes = rune_count(prose)
    floor = int(u.get("word_floor") or 0)
    ceiling = int(u.get("word_ceiling") or 0)
    r.add_counts(unit_runes=unit_runes)
    if floor and unit_runes < floor:
        r.blocking(
            "word_floor",
            f"unit runes={unit_runes} below word_floor={floor} (expand before Commit)",
        )
    if ceiling and unit_runes > ceiling:
        r.advisory(
            "word_ceiling",
            f"unit runes={unit_runes} above word_ceiling={ceiling} (consider splitting unit)",
        )

    a, b = chapter_range_of(u)
    if a > 0 and b >= a:
        n_ch = b - a + 1
        if n_ch > 10:
            r.blocking(
                "unit_size",
                f"unit has {n_ch} chapters (hard cap 10; split into two units)",
            )


def has_reader_continuity(book_root: Path) -> bool:
    """Prefer facts.md (v3); fall back to ledger.md / legacy public-lore+tracking."""
    if ledger_path(book_root).is_file():
        return True
    return file_exists(book_root, "continuity/public-lore.md") and file_exists(
        book_root, "continuity/tracking.md"
    )


def continuity_open_loops_text(book_root: Path, cache=None) -> str:
    """Text used to count open foreshadow / loop rows."""
    if cache is not None:
        return cache.open_loops_text()
    ledger = ledger_path(book_root)
    if ledger.is_file():
        return read_book_text(ledger)
    tracker = book_root / "continuity/foreshadow-tracker.md"
    if tracker.is_file():
        return read_book_text(tracker)
    tracking = book_root / "continuity/tracking.md"
    if tracking.is_file():
        return read_book_text(tracking)
    return ""


def count_open_foreshadows(tracker: str) -> int:
    n = 0
    for line in tracker.splitlines():
        if "|" not in line or "---" in line:
            continue
        low = line.lower()
        if "status" in low and "summary" in low:
            continue
        if OPEN_STATUS.search(line) or ADVANCED_STATUS.search(line):
            n += 1
    return n


def open_debt_count(book_root: Path, contract: dict, cache=None) -> int:
    n = len(nonempty_list(contract.get("reader_debt")))
    n += count_open_foreshadows(continuity_open_loops_text(book_root, cache))
    return n


# --- chapter summaries ---


def summary_source_text(book_root: Path) -> tuple[str, str]:
    """Return (text, rel) for legacy chapter summary blocks — ledger preferred."""
    ledger = ledger_path(book_root)
    if ledger.is_file():
        try:
            rel = ledger.relative_to(book_root).as_posix()
        except ValueError:
            rel = "continuity/facts.md"
        return read_book_text(ledger), rel
    summaries = book_root / "continuity/chapter_summaries.md"
    if summaries.is_file():
        return read_book_text(summaries), "continuity/chapter_summaries.md"
    return "", ""


def has_summary(summaries: str, ch: int) -> bool:
    return f"## ch{ch:03d}" in summaries


def archived_summary_text(book_root: Path, ch: int) -> str:
    """Find a ## chNNN block under continuity/summaries/vNN.md."""
    d = book_root / "continuity" / "summaries"
    if not d.is_dir():
        return ""
    for path in sorted(d.glob("*.md")):
        text = read_book_text(path)
        if has_summary(text, ch):
            return text
    return ""


def volume_summary_text(book_root: Path, volume: str) -> str:
    p = summaries_path(book_root, volume)
    return read_book_text(p) if p.is_file() else ""


def find_summary_block(book_root: Path, volume: str, ch: int) -> tuple[str, str]:
    """(block, source_rel). Volume summaries file first, then any summaries file,
    then legacy facts/ledger."""
    text = volume_summary_text(book_root, volume)
    if has_summary(text, ch):
        return extract_chapter_summary_block(text, ch), summaries_rel(volume)
    arch = archived_summary_text(book_root, ch)
    if arch:
        return extract_chapter_summary_block(arch, ch), "continuity/summaries/"
    legacy, rel = summary_source_text(book_root)
    if has_summary(legacy, ch):
        return extract_chapter_summary_block(legacy, ch), rel
    return "", ""


def extract_chapter_summary_block(text: str, ch: int) -> str:
    needle = f"## ch{ch:03d}"
    i = text.find(needle)
    if i < 0:
        return ""
    rest = text[i:]
    nxt = re.search(r"\n## ch\d+", rest[1:])
    return rest if not nxt else rest[: nxt.start() + 1]


def summary_has_five_keys(block: str) -> list[str]:
    missing = []
    for key in SUMMARY_KEYS:
        if not re.search(rf"[-*]\s*{re.escape(key)}\s*[：:]", block):
            missing.append(key)
    return missing


def split_chapter_summary_blocks(text: str) -> tuple[str, list[tuple[int, str]]]:
    """Split text into (head_without_blocks, [(chapter, block_text)]).
    Blocks start at `## chNNN` and run to the next `## ` heading."""
    lines = text.splitlines()
    head: list[str] = []
    blocks: list[tuple[int, list[str]]] = []
    cur: list[str] | None = None
    for line in lines:
        m = re.match(r"^## ch(\d+)\b", line)
        if m:
            cur = [line]
            blocks.append((int(m.group(1)), cur))
            continue
        if cur is not None and line.startswith("## "):
            cur = None
        if cur is not None:
            cur.append(line)
        else:
            head.append(line)
    out = [(ch, "\n".join(b).rstrip() + "\n") for ch, b in blocks]
    return "\n".join(head).rstrip() + "\n", out


# --- facts.md sections ---


def parse_cast_snapshot_names(ledger_text: str) -> set[str]:
    names: set[str] = set()
    in_cast = False
    for line in ledger_text.splitlines():
        if re.match(r"^###\s*Cast snapshot", line, re.I) or "Cast snapshot" in line and line.startswith("#"):
            in_cast = True
            continue
        if in_cast and line.startswith("#"):
            break
        if in_cast and line.strip().startswith("|"):
            cells = [c.strip() for c in line.strip().strip("|").split("|")]
            if not cells or cells[0] in ("角色", "----", "---") or set(cells[0]) <= {"-", ":"}:
                continue
            if cells[0]:
                names.add(cells[0])
    return names


def parse_open_loop_ids(ledger_text: str) -> set[str]:
    ids: set[str] = set()
    in_loops = False
    for line in ledger_text.splitlines():
        if re.match(r"^##\s*Open loops", line, re.I):
            in_loops = True
            continue
        if in_loops and line.startswith("## ") and "Open loops" not in line:
            break
        if in_loops:
            for m in FS_ID_RE.finditer(line):
                ids.add(m.group(0).upper())
    return ids


def foreshadow_ids_from_contract(contract: dict) -> list[str]:
    info = contract.get("info_control") or {}
    if not isinstance(info, dict):
        return []
    raw = info.get("foreshadowing") or []
    items = raw if isinstance(raw, list) else nonempty_list(raw)
    found: list[str] = []
    for item in items:
        for m in FS_ID_RE.finditer(str(item)):
            found.append(m.group(0).upper())
    return found


def cast_snapshot_rows(ledger_text: str) -> list[str]:
    rows: list[str] = []
    in_cast = False
    for line in ledger_text.splitlines():
        if re.match(r"^###\s*Cast snapshot", line, re.I) or (
            "Cast snapshot" in line and line.startswith("#")
        ):
            in_cast = True
            continue
        if in_cast and line.startswith("#"):
            break
        if in_cast and line.strip().startswith("|"):
            cells = [c.strip() for c in line.strip().strip("|").split("|")]
            if not cells or cells[0] in ("角色",) or set(cells[0]) <= {"-", ":"}:
                continue
            rows.append("| " + " | ".join(cells) + " |")
    return rows


def open_loops_rows(ledger_text: str) -> list[str]:
    rows: list[str] = []
    in_loops = False
    for line in ledger_text.splitlines():
        if re.match(r"^##\s*Open loops", line, re.I):
            in_loops = True
            continue
        if in_loops and line.startswith("## ") and "Open loops" not in line:
            break
        if in_loops and line.strip().startswith("|"):
            cells = [c.strip() for c in line.strip().strip("|").split("|")]
            if not cells or cells[0].upper() in ("ID",) or set(cells[0]) <= {"-", ":"}:
                continue
            rows.append("| " + " | ".join(cells) + " |")
    return rows


RELATION_DELTA_RE = re.compile(r"信任|站队|债务|秘密|结盟|背叛|决裂|反目|投靠|欠")


def relation_deltas(items: list[str]) -> list[str]:
    """state_deltas lines whose text describes a relationship change."""
    return [s for s in items if RELATION_DELTA_RE.search(s)]


# --- postcommit ---


def check_postcommit(book_root: Path, st: dict, unit_id: str, r: Report) -> None:
    try:
        u, rel = load_unit(book_root, unit_id)
    except OSError as e:
        r.blocking("contract", f"{unit_outline_rel(unit_id)}: {e}")
        return
    if str(u.get("status") or "").strip() != "reviewed":
        r.blocking("contract", f"{rel} status={u.get('status')} (Commit requires reviewed)")
    if not file_exists(book_root, unit_prose_rel(unit_id)):
        r.blocking("prose", f"{unit_prose_rel(unit_id)} missing")
    volume = unit_id.split("-")[0]
    vol_rel = summaries_rel(volume)
    legacy_text, legacy_src = summary_source_text(book_root)
    if not summaries_path(book_root, volume).is_file() and not legacy_src:
        r.blocking(
            "commit",
            f"missing {vol_rel} (chapter summaries) and continuity/facts.md",
        )
        return
    a, b = chapter_range_of(u)
    if a <= 0:
        r.blocking("chapter_range", f"{rel} chapter_range invalid")
        return
    for ch in range(a, b + 1):
        block, src = find_summary_block(book_root, volume, ch)
        if not block:
            r.blocking("commit", f"no ## ch{ch:03d} block in {vol_rel}")
            continue
        if src != vol_rel and src != "continuity/summaries/":
            r.advisory("commit", f"## ch{ch:03d} lives in {src} — run --action migrate to move it into {vol_rel}")
        missing = summary_has_five_keys(block)
        if missing:
            r.blocking(
                "commit",
                f"## ch{ch:03d} missing summary keys: {', '.join(missing)} "
                f"(need 事件/状态变化/伏笔/钩子/下章指向)",
            )
    ledger_text = ""
    lp = ledger_path(book_root)
    if lp.is_file():
        ledger_text = read_book_text(lp)
    deltas = nonempty_list(u.get("state_deltas"))
    who = state_delta_who(deltas)
    if who and ledger_text:
        names = parse_cast_snapshot_names(ledger_text)
        for w in who:
            if not any(w in n or n in w for n in names):
                r.blocking(
                    "commit",
                    f"state_deltas who={w} not found in Cast snapshot after Commit",
                )
    fs_ids = foreshadow_ids_from_contract(u)
    if fs_ids and ledger_text:
        loop_ids = parse_open_loop_ids(ledger_text)
        for fs in fs_ids:
            if fs not in loop_ids:
                r.blocking(
                    "commit",
                    f"foreshadowing {fs} not found in Open loops after Commit",
                )
    rel_deltas = relation_deltas(deltas)
    if rel_deltas:
        from .cast import load_cast_cards

        cards = load_cast_cards(book_root)
        for stem in state_delta_who(rel_deltas):
            card = cards.get(stem)
            if card is None:
                continue
            touched = any(unit_id in row.get("last_change", "") for row in card.relations)
            if not touched:
                r.advisory(
                    "relation",
                    f"state_deltas mark a relation change for {stem} but no 关系 row's 最近变化点 mentions {unit_id}",
                )
    last = _as_int(st.get("last_committed_ch"))
    if last < b:
        r.blocking("state", f"last_committed_ch={last} want ≥{b} after Commit")
    if not has_reader_continuity(book_root):
        r.blocking(
            "lore-tracks",
            "Commit must refresh continuity/facts.md (or legacy ledger.md)",
        )

    # Re-check lock/scale as safety net (primary hard gate is precommit).
    prose_rel = unit_prose_rel(unit_id)
    if file_exists(book_root, prose_rel):
        apply_lock_and_scale_checks(
            book_root, unit_id, u, read_text(book_root, prose_rel), r
        )
