"""Post-draft checks: precommit (shape + deslop + lock/scale), scan-deslop (HIT lines),
and qc-pack (both in one stdout for the 定稿 round)."""
from __future__ import annotations

from pathlib import Path

from .common import Report, _as_int, file_exists, read_text, rune_count
from .deslop import apply_deslop_to_report, hit_lines_for
from .ledger import MAX_OPEN_DEBTS, apply_lock_and_scale_checks, open_debt_count
from .outline import (
    chapter_rows,
    load_unit,
    next_hook_of,
    split_unit_prose,
    unit_outline_rel,
    unit_prose_rel,
)


def check_precommit(book_root: Path, st: dict, unit_id: str, r: Report, cache=None) -> None:
    try:
        u, rel = load_unit(book_root, unit_id)
    except OSError as e:
        r.blocking("contract", f"{unit_outline_rel(unit_id)}: {e}")
        return
    prose_rel = unit_prose_rel(unit_id)
    try:
        prose = read_text(book_root, prose_rel)
    except OSError:
        r.blocking("prose", f"{prose_rel} missing")
        return
    status = str(u.get("status") or "").strip()
    if status not in ("drafted", "accepted", "reviewed"):
        r.advisory("contract", f"{rel} status={status} expected drafted before review")
    apply_deslop_to_report(prose_rel, prose, r)
    slices, errors = split_unit_prose(prose)
    for msg in errors:
        r.blocking("chapters", f"{prose_rel} {msg}")
    want = [_as_int(c.get("chapter")) for c in chapter_rows(u)]
    got = [s["chapter"] for s in slices]
    if want and got != want:
        r.blocking("chapters", f"{prose_rel} headings {got} != outline chapters {want}")
    _, hout = next_hook_of(u)
    if hout and hout not in prose and rune_count(hout) >= 8:
        r.advisory("hook", "next_hook.out not found verbatim in prose — confirm the event landed")
    wt = _as_int(u.get("word_target"))
    if wt > 0:
        n = rune_count(prose)
        if n < wt * 8 // 10:
            r.advisory("length", f"unit prose ~{n} runes vs word_target={wt}")
    shares = {_as_int(c.get("chapter")): _as_int(c.get("word_share")) for c in chapter_rows(u)}
    for sl in slices:
        share = shares.get(sl["chapter"], 0)
        if share > 0 and rune_count(sl["body"]) < share * 7 // 10:
            r.advisory(
                "length",
                f"第{sl['chapter']}章 ~{rune_count(sl['body'])} runes vs word_share={share}",
            )
    debts = open_debt_count(book_root, u, cache)
    if debts > MAX_OPEN_DEBTS:
        r.blocking("reader_debt", f"open foreshadows+reader_debt={debts} exceeds {MAX_OPEN_DEBTS}")
    apply_lock_and_scale_checks(book_root, unit_id, u, prose, r)


def check_scan_deslop(book_root: Path, unit_id: str, r: Report) -> list[str]:
    """Printable HIT lines; findings go on Report (same P0 thresholds as precommit)."""
    prose_rel = unit_prose_rel(unit_id)
    if not file_exists(book_root, prose_rel):
        r.blocking("prose", f"{prose_rel} missing")
        return []
    prose = read_text(book_root, prose_rel)
    hits = apply_deslop_to_report(prose_rel, prose, r)
    return hit_lines_for(prose_rel, prose, hits)


def check_qc_pack(book_root: Path, st: dict, unit_id: str, r: Report, cache=None) -> list[str]:
    """precommit + scan-deslop in one report: verdict, 四计数, word floor/ceiling,
    lock hits, and HITS with line numbers. Adds a LENGTH section so the 定稿 round can
    decide 扩写 / 润色 without a second gate call."""
    check_precommit(book_root, st, unit_id, r, cache)
    prose_rel = unit_prose_rel(unit_id)
    if not file_exists(book_root, prose_rel):
        return []
    prose = read_text(book_root, prose_rel)
    # precommit already added deslop findings + counts; only line-level HITS are missing.
    from .deslop import iter_deslop_hits

    hits = hit_lines_for(prose_rel, prose, iter_deslop_hits(prose))
    try:
        u, _ = load_unit(book_root, unit_id)
    except OSError:
        u = {}
    runes = rune_count(prose)
    floor = _as_int(u.get("word_floor"))
    ceiling = _as_int(u.get("word_ceiling"))
    target = _as_int(u.get("word_target"))
    # floor is the hard gate; without one fall back to precommit's 80 % of word_target advisory
    threshold = floor or (target * 8 // 10)
    need_expand = bool(threshold and runes < threshold)
    length = [
        f"unit_runes: {runes}",
        f"word_target: {target}",
        f"word_floor: {floor}",
        f"word_ceiling: {ceiling}",
        f"expand_needed: {'yes' if need_expand else 'no'}",
        f"polish_needed: {'yes' if hits else 'no'}",
    ]
    slices, _ = split_unit_prose(prose)
    shares = {_as_int(c.get("chapter")): _as_int(c.get("word_share")) for c in chapter_rows(u)}
    for sl in slices:
        length.append(f"ch{sl['chapter']}: {rune_count(sl['body'])} / share {shares.get(sl['chapter'], 0)}")
    r.section("LENGTH", length)
    r.section("HITS", hits)
    return hits
