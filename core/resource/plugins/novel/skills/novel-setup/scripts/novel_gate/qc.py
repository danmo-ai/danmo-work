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


def check_deslop(book_root: Path, unit_id: str, r: Report) -> list[str]:
    """Subcommand alias: deslop / 毒句 / 破折号 only."""
    hits = check_scan_deslop(book_root, unit_id, r)
    r.section("HITS", hits if hits else ["None."])
    return hits


def check_length(book_root: Path, unit_id: str, r: Report) -> None:
    """Hard length gate: floor / ceiling (+ per-chapter shares as advisory)."""
    prose_rel = unit_prose_rel(unit_id)
    if not file_exists(book_root, prose_rel):
        r.blocking("prose", f"{prose_rel} missing")
        return
    prose = read_text(book_root, prose_rel)
    try:
        u, _ = load_unit(book_root, unit_id)
    except OSError as e:
        r.blocking("contract", str(e))
        return
    runes = rune_count(prose)
    floor = _as_int(u.get("word_floor"))
    ceiling = _as_int(u.get("word_ceiling"))
    target = _as_int(u.get("word_target"))
    threshold = floor or (target * 8 // 10)
    need_expand = bool(threshold and runes < threshold)
    over = bool(ceiling and runes > ceiling)
    if need_expand:
        r.blocking(
            "length",
            f"unit_runes={runes} below floor={threshold} (shortfall {threshold - runes})",
        )
    if over:
        r.blocking("length", f"unit_runes={runes} above ceiling={ceiling}")
    length = [
        f"unit_runes: {runes}",
        f"word_target: {target}",
        f"word_floor: {floor}",
        f"word_ceiling: {ceiling}",
        f"expand_needed: {'yes' if need_expand else 'no'}",
    ]
    slices, _ = split_unit_prose(prose)
    shares = {_as_int(c.get("chapter")): _as_int(c.get("word_share")) for c in chapter_rows(u)}
    for sl in slices:
        got = rune_count(sl["body"])
        share = shares.get(sl["chapter"], 0)
        length.append(f"ch{sl['chapter']}: {got} / share {share}")
        if share and got < share * 7 // 10:
            r.advisory("length", f"第{sl['chapter']}章 ~{got} runes vs word_share={share}")
    r.section("LENGTH", length)
    r.section("EXPAND", build_expand_anchors(slices, shares, need_expand, runes, threshold or 0))


def check_commit(book_root: Path, st: dict, unit_id: str, r: Report) -> None:
    """Ledger hard checks before seal-commit (status/cursor may still be pre-seal)."""
    from .ledger import check_postcommit

    check_postcommit(
        book_root,
        st,
        unit_id,
        r,
        require_reviewed=False,
        require_cursor=False,
    )


def check_qc_pack(
    book_root: Path,
    st: dict,
    unit_id: str,
    r: Report,
    cache=None,
    *,
    seal: bool = False,
) -> list[str]:
    """precommit + scan-deslop in one report: verdict, 四计数, word floor/ceiling,
    lock hits, HITS, and ### CONTINUITY for 定稿对照.

    When seal=True (qc-pack action only): PASS writes reviewed/cursor/snapshot and
    runs postcommit; a second call with unchanged prose returns ### SEAL only.
    prompt-pack --stage finalize must pass seal=False so packing does not commit.
    """
    from .seal import clear_qc_frozen, is_qc_frozen, report_qc_frozen, seal_after_qc_pass
    from .ledger import check_postcommit

    if seal and is_qc_frozen(book_root, unit_id):
        # Freeze marker only valid when Commit is still clean. A stale seal from a
        # prior buggy run must not skip HITS/length/commit (false PASS).
        n_before = sum(1 for f in r.findings if f.get("severity") == "blocking")
        check_postcommit(book_root, st, unit_id, r)
        n_after = sum(1 for f in r.findings if f.get("severity") == "blocking")
        if n_after > n_before:
            clear_qc_frozen(book_root, unit_id)
            # Drop commit findings so the full pack re-reports them after quality.
            r.findings = [f for f in r.findings if not (
                f.get("severity") == "blocking" and f.get("check") == "commit"
            )]
        else:
            # Wipe the probe findings; report_qc_frozen is the only output.
            r.findings = list(r.findings[:n_before])
            report_qc_frozen(book_root, unit_id, r)
            return []
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
    r.section("EXPAND", build_expand_anchors(slices, shares, need_expand, runes, threshold))
    try:
        from .context import build_continuity_lines

        r.section("CONTINUITY", build_continuity_lines(book_root, u, cache, st))
    except Exception as e:
        r.section("CONTINUITY", [f"（装配失败: {e}）"])
    r.section("COMMIT", build_commit_card_for_book(book_root, u, st))
    if seal:
        r.finalize()
        if r.verdict == "PASS":
            seal_after_qc_pass(book_root, st, unit_id, r)
            r.finalize()
    return hits


def build_expand_anchors(
    slices: list[dict],
    shares: dict,
    need_expand: bool,
    runes: int,
    threshold: int,
) -> list[str]:
    """Short-chapter deficits + tail anchors so finalize does not re-read the whole unit."""
    if not need_expand:
        return ["expand_needed: no — do not expand prose; polish HITS only if any."]
    gap = max(0, (threshold or 0) - runes)
    lines = [
        f"expand_needed: yes",
        f"unit_shortfall: {gap} runes (reach floor in ONE edit batch; do not re-read the full file)",
    ]
    short: list[str] = []
    for sl in slices:
        ch = sl.get("chapter") or 0
        share = shares.get(ch, 0) or 0
        got = rune_count(sl.get("body") or "")
        if share and got >= share:
            continue
        need = max(0, share - got) if share else 0
        anchor = _chapter_tail_anchor(sl)
        short.append(f"ch{ch}: {got}/{share or '—'} short {need} anchor {anchor}")
    if short:
        lines.extend(short[:12])
    else:
        lines.append("no per-chapter shortfall listed — add density inside existing scenes, one batch.")
    lines.append("edit only these anchors; do not offset-read the whole unit.")
    return lines


def _chapter_tail_anchor(sl: dict) -> str:
    body = str(sl.get("body") or "")
    nonempty = [ln.strip() for ln in body.splitlines() if ln.strip() and ln.strip() != "---"]
    head_line = int(sl.get("line") or 1)
    if not nonempty:
        return f"L{head_line} (empty chapter)"
    tail = nonempty[-1]
    if len(tail) > 42:
        tail = tail[:42] + "…"
    tail_line = head_line + max(1, len(body.splitlines()))
    return f"L{tail_line} 「{tail}」"


def build_commit_card(unit: dict, st: dict | None) -> list[str]:
    """Fallback COMMIT card without book_root (tests / thin callers)."""
    from .identity import normalize_state_deltas
    from .ledger import summaries_rel
    from .outline import chapter_range_of

    uid = str(unit.get("unit_id") or "").strip() or "vNN-U#"
    vol = uid.split("-")[0] if "-" in uid else "vNN"
    a, b = chapter_range_of(unit)
    last = (st or {}).get("last_committed_ch")
    lines: list[str] = [
        "After prose is clean: patch ledger, then parallel check-length + check-deslop + check-commit → seal-commit.",
        f"paths: {summaries_rel(vol)} · continuity/facts.md · continuity/commits/{uid}.md",
        f"seal-commit sets reviewed + last_committed_ch={b or '?'} (was {last if last is not None else '?'}).",
        "",
        "### SUMMARY_SKELETONS",
    ]
    if a and b:
        for n in range(a, b + 1):
            lines.extend(
                [
                    f"## ch{n:03d}",
                    "- 事件：",
                    "- 状态变化：",
                    "- 伏笔：",
                    "- 钩子：",
                    "- 下章指向：",
                    "",
                ]
            )
    else:
        lines.append("(chapter_range missing)")
    lines.append("### SNAPSHOT_ROWS")
    deltas = normalize_state_deltas(unit.get("state_deltas"))
    if deltas:
        for row in deltas[:12]:
            stem = row.get("stem") or "?"
            field = row.get("field") or ""
            to = row.get("to") or ""
            lines.append(f"| {stem} |  | {uid} |  | {to if field == 'title' else ''} | {to if field == 'location' else ''} |")
    else:
        lines.append("(no state_deltas)")
    ch_span = f"ch{a}–ch{b}" if a and b else "chNNN–chNNN"
    lines.extend(
        [
            "",
            "### COMMIT_SKELETON (write new file — do not read_skill / guess assets paths)",
            f"path: continuity/commits/{uid}.md",
            f"# Commit log — {uid}",
            f"- 单元 ID: {uid}",
            f"- 章范围: {ch_span}",
            "- Commit 时间: （今日）",
            "- 细纲 status: reviewed",
            "## 下一单元指引（一句话）",
            "- （承接 next_hook.out）",
        ]
    )
    return lines


def build_commit_card_for_book(book_root: Path, unit: dict, st: dict | None) -> list[str]:
    """COMMIT card: only unfinished work items (cast stems + missing summaries/commits)."""
    from .common import read_book_text
    from .identity import normalize_state_deltas, parse_cast_snapshot_table
    from .ledger import (
        find_summary_block,
        ledger_path,
        summaries_rel,
        summary_has_five_keys,
    )
    from .cast import load_cast_cards
    from .outline import chapter_range_of
    from .seal import who_in_snapshot

    uid = str(unit.get("unit_id") or "").strip() or "vNN-U#"
    vol = uid.split("-")[0] if "-" in uid else "vNN"
    a, b = chapter_range_of(unit)
    last = (st or {}).get("last_committed_ch")
    seal_line = (
        f"seal-commit sets reviewed + last_committed_ch={b or '?'} "
        f"(was {last if last is not None else '?'})."
    )

    summary_lines: list[str] = []
    if a and b:
        for n in range(a, b + 1):
            block, _src = find_summary_block(book_root, vol, n)
            missing_keys = summary_has_five_keys(block) if block else ["事件", "状态变化", "伏笔", "钩子", "下章指向"]
            if block and not missing_keys:
                continue
            note = "MISSING" if not block else f"incomplete: {', '.join(missing_keys)}"
            summary_lines.append(f"# {note}")
            summary_lines.extend(
                [
                    f"## ch{n:03d}",
                    "- 事件：",
                    "- 状态变化：",
                    "- 伏笔：",
                    "- 钩子：",
                    "- 下章指向：",
                    "",
                ]
            )

    lp = ledger_path(book_root)
    snap_names: set[str] = set()
    if lp.is_file():
        try:
            snap_names = set(parse_cast_snapshot_table(read_book_text(lp)).keys())
        except Exception:
            snap_names = set()
    cards = load_cast_cards(book_root)
    snap_rows: list[str] = []
    for row in normalize_state_deltas(unit.get("state_deltas")):
        stem = row.get("stem") or ""
        # Strict stem∈cards — same as append_cast_snapshot_rows (not fuzzy is_cast_person).
        if not stem or stem not in cards:
            continue
        if who_in_snapshot(snap_names, stem, book_root):
            continue
        field = row.get("field") or ""
        to = row.get("to") or ""
        title_cell = to if field == "title" else ""
        loc_cell = to if field == "location" else ""
        snap_rows.append(f"| {stem} |  | {uid} seal |  | {title_cell} | {loc_cell} |")

    commits_rel = f"continuity/commits/{uid}.md"
    commits_missing = not (book_root / commits_rel).is_file()

    if not summary_lines and not snap_rows and not commits_missing:
        return [
            "账本已齐，下一步三检（check-length + check-deslop + check-commit）再 seal-commit。",
            seal_line,
        ]

    paths: list[str] = []
    if summary_lines:
        paths.append(summaries_rel(vol))
    if snap_rows:
        paths.append("continuity/facts.md")
    if commits_missing:
        paths.append(commits_rel)

    lines: list[str] = [
        "Patch listed gaps, then parallel: check-length + check-deslop + check-commit.",
        f"paths: {' · '.join(paths)}",
        seal_line,
        "",
    ]
    if summary_lines:
        lines.append("### SUMMARY_SKELETONS (paste into summaries file)")
        lines.extend(summary_lines)
    if snap_rows:
        lines.append("### SNAPSHOT_ROWS (append under Cast snapshot)")
        lines.extend(snap_rows)
        lines.append("")
    if commits_missing:
        ch_span = f"ch{a}–ch{b}" if a and b else "chNNN–chNNN"
        lines.extend(
            [
                "### COMMIT_SKELETON (write new file — do not read_skill / guess assets paths)",
                f"path: {commits_rel}",
                f"# Commit log — {uid}",
                "",
                f"- 单元 ID: {uid}",
                f"- 章范围: {ch_span}",
                "- Commit 时间: （今日）",
                "- 细纲 status: reviewed",
                "",
                "## Gate 结果",
                "| 阶段 | VERDICT | 备注 |",
                "|------|---------|------|",
                "| pack-finalize | （记入口） | |",
                "| check-length / deslop / commit | PASS | seal 前三检 |",
                "",
                "## 字数实测",
                "- 单元 runes: （见 check-length COUNTS）",
                "",
                "## Deslop / 扩写（若本轮做了）",
                "- （一句）",
                "",
                "## 下一单元指引（一句话）",
                "- （承接 next_hook.out）",
            ]
        )
    return lines
