"""Mechanical seal after write / qc-pack / lint — ledger writes the model used to skip."""
from __future__ import annotations

import hashlib
import json
import re
from pathlib import Path

from .cast import load_cast_cards
from .common import (
    Report,
    file_exists,
    read_book_text,
    set_state_scalar,
    write_book_text,
)
from .identity import normalize_state_deltas, parse_cast_snapshot_table
from .ledger import check_postcommit, ledger_path
from .outline import chapter_range_of, load_unit, unit_outline_rel, unit_prose_rel


def _pack_dir(book_root: Path) -> Path:
    d = book_root / ".pack"
    d.mkdir(parents=True, exist_ok=True)
    return d


def prose_hash(book_root: Path, unit_id: str) -> str:
    rel = unit_prose_rel(unit_id)
    if not file_exists(book_root, rel):
        return ""
    data = read_book_text(book_root / rel).encode("utf-8")
    return hashlib.sha256(data).hexdigest()[:16]


def qc_seal_path(book_root: Path, unit_id: str) -> Path:
    return _pack_dir(book_root) / f"seal-qc-{unit_id}.json"


def lint_seal_path(book_root: Path, volume: str) -> Path:
    return _pack_dir(book_root) / f"seal-lint-{volume}.json"


def is_qc_frozen(book_root: Path, unit_id: str) -> bool:
    p = qc_seal_path(book_root, unit_id)
    if not p.is_file():
        return False
    try:
        meta = json.loads(p.read_text(encoding="utf-8"))
    except Exception:
        return False
    return meta.get("prose_hash") == prose_hash(book_root, unit_id)


def mark_qc_frozen(book_root: Path, unit_id: str) -> None:
    meta = {"unit": unit_id, "prose_hash": prose_hash(book_root, unit_id)}
    qc_seal_path(book_root, unit_id).write_text(
        json.dumps(meta, ensure_ascii=False) + "\n", encoding="utf-8"
    )


def bump_lint_count(book_root: Path, volume: str) -> int:
    p = lint_seal_path(book_root, volume)
    n = 0
    if p.is_file():
        try:
            n = int(json.loads(p.read_text(encoding="utf-8")).get("count") or 0)
        except Exception:
            n = 0
    n += 1
    p.write_text(json.dumps({"volume": volume, "count": n}) + "\n", encoding="utf-8")
    return n


def set_outline_status(book_root: Path, unit_id: str, status: str) -> bool:
    """Rewrite `status:` line in unit YAML. Returns True if changed or already set."""
    rel = unit_outline_rel(unit_id)
    path = book_root / rel
    if not path.is_file():
        return False
    text = read_book_text(path)
    pat = re.compile(r"^(status\s*:\s*)(\S+)(.*)$", re.M)
    m = pat.search(text)
    if not m:
        return False
    if m.group(2).strip() == status:
        return True
    new = text[: m.start()] + f"{m.group(1)}{status}{m.group(3)}" + text[m.end() :]
    write_book_text(path, new)
    return True


def set_last_committed_ch(book_root: Path, chapter: int) -> None:
    """Write last_committed_ch. chapter=0 is valid (revert / not yet committed)."""
    sp = book_root / "novel-state.yaml"
    if not sp.is_file() or chapter < 0:
        return
    text = read_book_text(sp)
    new = set_state_scalar(text, "last_committed_ch", str(chapter))
    if new != text:
        write_book_text(sp, new)


def stem_display_names(book_root: Path) -> dict[str, str]:
    """stem → Chinese title from cast cards (title first line)."""
    out: dict[str, str] = {}
    for stem, card in load_cast_cards(book_root).items():
        label = (card.title or "").strip()
        # Drop parenthetical suffixes for matching: 「陆沉（格局）」→ still keep full for append
        if label:
            out[stem] = label
    return out


def _title_aliases(title: str) -> list[str]:
    title = (title or "").strip()
    if not title:
        return []
    out = [title]
    bare = re.split(r"[（(]", title, 1)[0].strip()
    if bare and bare not in out:
        out.append(bare)
    return out


def resolve_who_aliases(book_root: Path, who: str) -> list[str]:
    """Names that count as a match for this state_deltas who."""
    aliases = [who]
    cards = load_cast_cards(book_root)
    if who in cards:
        for a in _title_aliases(cards[who].title):
            if a not in aliases:
                aliases.append(a)
        return aliases
    # who may be the Chinese title while stem is the card filename
    for stem, card in cards.items():
        for a in _title_aliases(card.title):
            if who == a or who in a or a in who:
                if stem not in aliases:
                    aliases.append(stem)
                for x in _title_aliases(card.title):
                    if x not in aliases:
                        aliases.append(x)
                break
    return aliases


def who_in_snapshot(names: set[str], who: str, book_root: Path) -> bool:
    for alias in resolve_who_aliases(book_root, who):
        if any(alias in n or n in alias for n in names):
            return True
    return False


def is_cast_person(book_root: Path, who: str) -> bool:
    cards = load_cast_cards(book_root)
    if who in cards:
        return True
    for card in cards.values():
        for a in _title_aliases(card.title):
            if who == a or who in a or a in who:
                return True
    return False


def append_cast_snapshot_rows(
    book_root: Path,
    unit: dict,
    unit_id: str,
) -> list[str]:
    """Ensure person stems with title `to` appear in Cast snapshot. Returns change notes."""
    lp = ledger_path(book_root)
    if not lp.is_file():
        return []
    text = read_book_text(lp)
    snap = parse_cast_snapshot_table(text)
    names = set(snap.keys())
    cards = load_cast_cards(book_root)
    notes: list[str] = []
    deltas = normalize_state_deltas(unit.get("state_deltas"))
    # Find Cast snapshot table end to append
    lines = text.splitlines(keepends=True)
    cast_start = -1
    cast_end = len(lines)
    for i, line in enumerate(lines):
        if re.match(r"^###\s*Cast snapshot", line, re.I) or (
            "Cast snapshot" in line and line.startswith("#")
        ):
            cast_start = i
            continue
        if cast_start >= 0 and i > cast_start and line.startswith("#"):
            cast_end = i
            break
    if cast_start < 0:
        return []

    # Detect header columns from first header row after cast_start
    headers = ["角色", "状态", "备注", "年龄", "职位", "位置"]
    for line in lines[cast_start:cast_end]:
        if line.strip().startswith("|") and "角色" in line:
            headers = [c.strip() for c in line.strip().strip("|").split("|")]
            break

    to_append: list[str] = []
    for row in deltas:
        stem = row.get("stem") or ""
        if not stem or stem not in cards:
            continue
        title = (cards[stem].title or stem).strip()
        bare = re.split(r"[（(]", title, 1)[0].strip() or title
        # Already present under Chinese name or stem?
        if who_in_snapshot(names, stem, book_root):
            # May still need title column refresh for identity — append only if missing title match
            matched = None
            for n in names:
                if bare in n or n in bare or stem in n:
                    matched = n
                    break
            if matched and row.get("field") == "title" and row.get("to"):
                entry = snap.get(matched) or {}
                cur = (entry.get("职位") or entry.get("本职") or "").strip()
                to = row["to"]
                if cur and (to in cur or cur in to):
                    continue
                # Update existing row's 职位 cell in place when possible
                if "职位" in headers or "本职" in headers:
                    col = "职位" if "职位" in headers else "本职"
                    updated = _rewrite_snapshot_cell(lines, cast_start, cast_end, matched, headers, col, to)
                    if updated:
                        notes.append(f"snapshot {matched}.{col} ← {to[:40]}")
                        text = "".join(lines)
                        write_book_text(lp, text)
                        snap = parse_cast_snapshot_table(text)
                        names = set(snap.keys())
            continue
        # Append a new row for this person
        cells = {h: "" for h in headers}
        cells["角色"] = bare
        if "职位" in cells and row.get("field") == "title" and row.get("to"):
            cells["职位"] = row["to"]
        elif "本职" in cells and row.get("field") == "title" and row.get("to"):
            cells["本职"] = row["to"]
        if "备注" in cells:
            cells["备注"] = f"{unit_id} seal"
        row_line = "| " + " | ".join(cells.get(h, "") for h in headers) + " |\n"
        to_append.append(row_line)
        names.add(bare)
        notes.append(f"+ snapshot {bare}")

    if to_append:
        # Insert before cast_end
        lines[cast_end:cast_end] = to_append
        write_book_text(lp, "".join(lines))
    return notes


def _rewrite_snapshot_cell(
    lines: list[str],
    cast_start: int,
    cast_end: int,
    name: str,
    headers: list[str],
    col: str,
    value: str,
) -> bool:
    try:
        col_i = headers.index(col)
    except ValueError:
        return False
    for i in range(cast_start, cast_end):
        line = lines[i]
        if not line.strip().startswith("|"):
            continue
        cells = [c.strip() for c in line.strip().strip("|").split("|")]
        if not cells or cells[0] != name:
            continue
        while len(cells) < len(headers):
            cells.append("")
        cells[col_i] = value
        lines[i] = "| " + " | ".join(cells) + " |\n"
        return True
    return False


def seal_write(book_root: Path, unit_id: str, r: Report) -> None:
    """After prose lands: set outline drafted and emit ### SEAL."""
    prose = unit_prose_rel(unit_id)
    if not file_exists(book_root, prose):
        r.blocking("prose", f"{prose} missing — write the unit file first")
        r.finalize()
        return
    try:
        u, rel = load_unit(book_root, unit_id)
    except OSError as e:
        r.blocking("contract", str(e))
        r.finalize()
        return
    status = str(u.get("status") or "").strip()
    changed = False
    if status in ("accepted", "proposed", "drafted", ""):
        changed = set_outline_status(book_root, unit_id, "drafted")
    r.section(
        "SEAL",
        [
            f"write sealed: {prose}",
            f"outline: {rel} status=drafted" + (" (updated)" if changed else " (ok)"),
            "stop — do not edit further this turn.",
        ],
    )
    r.finalize()


def clear_qc_frozen(book_root: Path, unit_id: str) -> None:
    p = qc_seal_path(book_root, unit_id)
    if p.is_file():
        try:
            p.unlink()
        except OSError:
            pass


def seal_commit(book_root: Path, st: dict, unit_id: str, r: Report, cache=None) -> None:
    """Re-run length + deslop + commit; only then write reviewed/cursor + ### SEAL.

    Never skip checks because of a prior freeze marker.
    """
    from .qc import check_commit, check_deslop, check_length

    check_length(book_root, unit_id, r)
    check_deslop(book_root, unit_id, r)
    check_commit(book_root, st, unit_id, r)
    # All three must be clean before touching ledger status.
    if any(f.get("severity") == "blocking" for f in r.findings):
        r.finalize()
        return
    seal_after_qc_pass(book_root, st, unit_id, r)
    r.finalize()


def seal_after_qc_pass(book_root: Path, st: dict, unit_id: str, r: Report) -> None:
    """Quality PASS only starts a Commit attempt. Ledger status/cursor advance and
    ### SEAL only when postcommit is also clean.

    If Commit blockers remain: revert reviewed/cursor, do not freeze prose — the
    model must still be able to fix HITS/length and finish summaries.
    """
    try:
        u, rel = load_unit(book_root, unit_id)
    except OSError as e:
        r.blocking("contract", str(e))
        return
    prev_status = str(u.get("status") or "drafted").strip() or "drafted"
    prev_ch = st.get("last_committed_ch")
    a, b = chapter_range_of(u)
    set_outline_status(book_root, unit_id, "reviewed")
    if b > 0:
        set_last_committed_ch(book_root, b)
        st = dict(st)
        st["last_committed_ch"] = b
    notes = append_cast_snapshot_rows(book_root, u, unit_id)
    n_before = sum(1 for f in r.findings if f.get("severity") == "blocking")
    check_postcommit(book_root, st, unit_id, r)
    n_after = sum(1 for f in r.findings if f.get("severity") == "blocking")
    head = [
        f"unit: {unit_id}",
        f"outline: {rel}",
        f"state: last_committed_ch candidate={b or '?'}",
    ]
    if notes:
        head.extend(f"ledger: {n}" for n in notes[:8])
    if n_after > n_before:
        # Never keep a false "定稿通过". If status was already wrongly reviewed,
        # force drafted. Cursor must not sit inside this unit without Commit.
        restore_status = prev_status if prev_status not in ("reviewed",) else "drafted"
        set_outline_status(book_root, unit_id, restore_status)
        restore_ch = 0
        try:
            restore_ch = int(prev_ch) if prev_ch is not None else 0
        except (TypeError, ValueError):
            restore_ch = 0
        if a > 0 and restore_ch >= a:
            restore_ch = a - 1
        set_last_committed_ch(book_root, restore_ch)
        clear_qc_frozen(book_root, unit_id)
        r.section(
            "COMMIT_INCOMPLETE",
            head
            + [
                f"outline status → {restore_status}; last_committed_ch → {restore_ch}.",
                "prose quality gates passed this run — still fix any new HITS if re-qc finds them.",
                "write summaries/facts/commits (and fix prose if needed), then re-run checks + seal-commit.",
                f"commit blockers: {n_after - n_before}",
            ],
        )
        return
    mark_qc_frozen(book_root, unit_id)
    r.section(
        "SEAL",
        head
        + [
            f"commit sealed: {unit_id}",
            f"outline: {rel} status=reviewed",
            f"state: last_committed_ch={b or '?'}",
            "stop — quality + Commit both clean.",
        ],
    )


def report_qc_frozen(book_root: Path, unit_id: str, r: Report) -> None:
    """Informational only for legacy qc-pack; seal-commit never skips checks."""
    r.section(
        "SEAL",
        [
            f"already frozen: {unit_id} (prose unchanged since last full SEAL)",
            "do not edit units/*.md; do not re-seal.",
        ],
    )
    r.finalize()


def maybe_seal_lint(book_root: Path, volume: str, r: Report, unit_results: list) -> None:
    """Emit ### SEAL when all units PASS or this is the 2nd lint for the volume."""
    count = bump_lint_count(book_root, volume)
    all_pass = bool(unit_results) and all(v == "PASS" for _, v, _ in unit_results)
    if all_pass or count >= 2:
        r.section(
            "SEAL",
            [
                f"lint sealed: {volume} (pass={all_pass} call={count})",
                "stop — do not lint-units again this turn.",
            ],
        )
