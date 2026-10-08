"""Argument parsing: argparse subcommands (primary) + legacy --action mapping."""
from __future__ import annotations

import argparse
import json
import os
import sys
from pathlib import Path

from .book import BookCache
from .cast import check_cast_lint
from .common import (
    UNIT_ID_RE,
    VOLUME_ID_RE,
    Report,
    _force_stdio_utf8,
    read_book_text,
    resolve_book,
    set_state_nested_scalar,
    write_book_text,
)
from .context import check_outline_pack, check_preflight
from .pack import (
    STAGE_FINALIZE,
    STAGE_OUTLINE,
    STAGE_WRITE,
    materialize_pack,
)
from .doctor import check_doctor
from .init import init_book
from .ledger import check_postcommit
from .migrate import run_migrate
from .outline import accept_volume, lint_units
from .qc import (
    check_commit,
    check_deslop,
    check_length,
    check_precommit,
    check_qc_pack,
    check_scan_deslop,
)

# Legacy --action names kept for tests / transition; skills use subcommands only.
LEGACY_ACTION_MAP = {
    "lint-units": "lint-outline",
    "scan-deslop": "check-deslop",
}


def _record_cast_registry(book_root: Path, verdict: str) -> None:
    sp = book_root / "novel-state.yaml"
    if not sp.is_file():
        return
    text = read_book_text(sp)
    new = set_state_nested_scalar(text, "artifacts", "cast_registry", "ok" if verdict == "PASS" else "fail")
    if new != text:
        write_book_text(sp, new)


def _require_unit(unit: str) -> str:
    unit_id = (unit or "").strip()
    if not unit_id:
        raise ValueError("requires --unit vNN-U#")
    if not UNIT_ID_RE.match(unit_id):
        raise ValueError(f"--unit {unit_id} must match vNN-U#")
    return unit_id


def _require_volume(volume: str) -> str:
    vol = (volume or "").strip()
    if not vol or not VOLUME_ID_RE.match(vol):
        raise ValueError("requires --volume vNN")
    return vol


def cmd_pack_outline(
    workdir: str, book_id: str, volume: str = "", **_kw
) -> tuple[Report, list[str]]:
    root, st = resolve_book(workdir, book_id)
    bid = str(st.get("book_id") or root.name)
    vol = _require_volume(volume)
    r = Report("pack-outline", bid, root, 0)
    check_outline_pack(root, st, vol, r, BookCache(root))
    r.finalize()
    materialize_pack(r, root, STAGE_OUTLINE, volume=vol)
    return r, []


def cmd_pack_write(
    workdir: str, book_id: str, unit: str = "", **_kw
) -> tuple[Report, list[str]]:
    root, st = resolve_book(workdir, book_id)
    bid = str(st.get("book_id") or root.name)
    unit_id = _require_unit(unit)
    r = Report("pack-write", bid, root, 0, unit_id)
    check_preflight(root, st, unit_id, r, BookCache(root), write_pack=True)
    r.finalize()
    materialize_pack(r, root, STAGE_WRITE, unit_id=unit_id)
    return r, []


def cmd_pack_finalize(
    workdir: str, book_id: str, unit: str = "", **_kw
) -> tuple[Report, list[str]]:
    root, st = resolve_book(workdir, book_id)
    bid = str(st.get("book_id") or root.name)
    unit_id = _require_unit(unit)
    r = Report("pack-finalize", bid, root, 0, unit_id)
    hit_lines = check_qc_pack(root, st, unit_id, r, BookCache(root), seal=False)
    r.finalize()
    materialize_pack(r, root, STAGE_FINALIZE, unit_id=unit_id)
    return r, hit_lines


def cmd_lint_outline(
    workdir: str, book_id: str, volume: str = "", **_kw
) -> tuple[Report, list[str]]:
    root, st = resolve_book(workdir, book_id)
    bid = str(st.get("book_id") or root.name)
    vol = _require_volume(volume)
    r = Report("lint-outline", bid, root, 0)
    results = lint_units(root, st, vol, r, BookCache(root))
    body = []
    for uid, verdict, msgs in results:
        body.append(f"{uid}: {verdict}")
        body.extend(f"  - {m}" for m in msgs)
    r.section("UNITS", body if body else ["no outline/units/*.yaml for " + vol])
    from .seal import maybe_seal_lint

    maybe_seal_lint(root, vol, r, results)
    r.finalize()
    return r, []


def cmd_check_length(
    workdir: str, book_id: str, unit: str = "", **_kw
) -> tuple[Report, list[str]]:
    root, st = resolve_book(workdir, book_id)
    bid = str(st.get("book_id") or root.name)
    unit_id = _require_unit(unit)
    r = Report("check-length", bid, root, 0, unit_id)
    check_length(root, unit_id, r)
    r.finalize()
    return r, []


def cmd_check_deslop(
    workdir: str, book_id: str, unit: str = "", **_kw
) -> tuple[Report, list[str]]:
    root, st = resolve_book(workdir, book_id)
    bid = str(st.get("book_id") or root.name)
    unit_id = _require_unit(unit)
    r = Report("check-deslop", bid, root, 0, unit_id)
    hits = check_deslop(root, unit_id, r)
    r.finalize()
    return r, hits


def cmd_check_commit(
    workdir: str, book_id: str, unit: str = "", **_kw
) -> tuple[Report, list[str]]:
    root, st = resolve_book(workdir, book_id)
    bid = str(st.get("book_id") or root.name)
    unit_id = _require_unit(unit)
    r = Report("check-commit", bid, root, 0, unit_id)
    check_commit(root, st, unit_id, r)
    r.finalize()
    return r, []


def cmd_seal_write(
    workdir: str, book_id: str, unit: str = "", **_kw
) -> tuple[Report, list[str]]:
    from .seal import seal_write

    root, st = resolve_book(workdir, book_id)
    bid = str(st.get("book_id") or root.name)
    unit_id = _require_unit(unit)
    r = Report("seal-write", bid, root, 0, unit_id)
    seal_write(root, unit_id, r)
    return r, []


def cmd_seal_commit(
    workdir: str, book_id: str, unit: str = "", **_kw
) -> tuple[Report, list[str]]:
    from .seal import seal_commit

    root, st = resolve_book(workdir, book_id)
    bid = str(st.get("book_id") or root.name)
    unit_id = _require_unit(unit)
    r = Report("seal-commit", bid, root, 0, unit_id)
    seal_commit(root, st, unit_id, r, BookCache(root))
    return r, []


def cmd_accept_volume(
    workdir: str, book_id: str, volume: str = "", **_kw
) -> tuple[Report, list[str]]:
    root, st = resolve_book(workdir, book_id)
    bid = str(st.get("book_id") or root.name)
    vol = _require_volume(volume)
    r = Report("accept-volume", bid, root, 0)
    info = accept_volume(root, st, vol, r)
    if info and "seeded" in info:
        r.section(
            "ACCEPTED",
            [f"canon: {', '.join(info.get('promoted') or []) or '(all already canon)'}"]
            + [f"+ outline/units/{u}.yaml (proposed)" for u in info.get("seeded", [])]
            + [f"= outline/units/{u}.yaml (kept)" for u in info.get("kept", [])],
        )
    r.finalize()
    return r, []


def cmd_doctor(workdir: str, book_id: str, **_kw) -> tuple[Report, list[str]]:
    root, st = resolve_book(workdir, book_id)
    bid = str(st.get("book_id") or root.name)
    r = Report("doctor", bid, root, 0)
    check_doctor(root, st, r)
    r.finalize()
    return r, []


def cmd_init(
    workdir: str, book_id: str, title: str = "", genre: str = "", **_kw
) -> tuple[Report, list[str]]:
    if not book_id:
        raise ValueError("init requires --book-id <slug>")
    r = Report("init", book_id, Path(workdir).resolve() / "novel" / book_id)
    info = init_book(workdir, book_id, r, title=title, genre=genre)
    if info:
        r.section(
            "CREATED",
            [f"root: {info['root']}"]
            + [f"+ {p}" for p in info["copied"]]
            + [f"= {p} (kept)" for p in info["kept"]],
        )
        r.context_lines = [
            "- 下一步: 填 book-bible.md（读者承诺、Style card）、canon/world.md；",
            "  novel-state.yaml 定 genre / subgenre / qc_profile；然后 novel-plan 一轮出人物卡 + 总纲 + 第 1 卷卷纲。",
        ]
    r.finalize()
    return r, []


def cmd_cast_lint(workdir: str, book_id: str, **_kw) -> tuple[Report, list[str]]:
    root, st = resolve_book(workdir, book_id)
    bid = str(st.get("book_id") or root.name)
    r = Report("cast-lint", bid, root, 0)
    check_cast_lint(root, r, BookCache(root))
    r.finalize()
    _record_cast_registry(root, r.verdict)
    return r, []


def cmd_migrate(workdir: str, book_id: str, **_kw) -> tuple[Report, list[str]]:
    root, st = resolve_book(workdir, book_id)
    bid = str(st.get("book_id") or root.name)
    r = Report("migrate", bid, root, 0)
    pre = Report("doctor", bid, root, 0)
    check_doctor(root, st, pre)
    pre.finalize()
    hard = [
        f
        for f in pre.findings
        if f["severity"] == "blocking" and f["check"] in ("layout", "encoding", "migrate")
    ]
    if hard:
        for f in hard:
            r.blocking("doctor", f"fix before migrate: [{f['check']}] {f['message']}")
    else:
        changes = run_migrate(root, st, r)
        r.section("CHANGES", changes if changes else ["nothing to migrate"])
    r.finalize()
    return r, []


def cmd_preflight(
    workdir: str, book_id: str, unit: str = "", **_kw
) -> tuple[Report, list[str]]:
    root, st = resolve_book(workdir, book_id)
    bid = str(st.get("book_id") or root.name)
    unit_id = _require_unit(unit)
    r = Report("preflight", bid, root, 0, unit_id)
    check_preflight(root, st, unit_id, r, BookCache(root))
    r.finalize()
    return r, []


def cmd_precommit(
    workdir: str, book_id: str, unit: str = "", **_kw
) -> tuple[Report, list[str]]:
    root, st = resolve_book(workdir, book_id)
    bid = str(st.get("book_id") or root.name)
    unit_id = _require_unit(unit)
    r = Report("precommit", bid, root, 0, unit_id)
    check_precommit(root, st, unit_id, r, BookCache(root))
    r.finalize()
    return r, []


def cmd_outline_pack_legacy(
    workdir: str, book_id: str, volume: str = "", **_kw
) -> tuple[Report, list[str]]:
    """Legacy outline-pack: stdout OUTLINE_PACK (no disk materialize)."""
    root, st = resolve_book(workdir, book_id)
    bid = str(st.get("book_id") or root.name)
    vol = _require_volume(volume)
    r = Report("outline-pack", bid, root, 0)
    check_outline_pack(root, st, vol, r, BookCache(root))
    r.finalize()
    return r, []


def cmd_postcommit_legacy(
    workdir: str, book_id: str, unit: str = "", **_kw
) -> tuple[Report, list[str]]:
    """Legacy postcommit: full require_reviewed + cursor checks."""
    root, st = resolve_book(workdir, book_id)
    bid = str(st.get("book_id") or root.name)
    unit_id = _require_unit(unit)
    r = Report("postcommit", bid, root, 0, unit_id)
    check_postcommit(root, st, unit_id, r)
    r.finalize()
    if r.verdict == "PASS":
        r.section(
            "SEAL",
            [
                f"postcommit sealed: {unit_id}",
                "stop — Commit complete this turn.",
            ],
        )
    return r, []


def cmd_qc_pack_legacy(
    workdir: str, book_id: str, unit: str = "", **_kw
) -> tuple[Report, list[str]]:
    """Deprecated: use pack-finalize + check-* + seal-commit."""
    root, st = resolve_book(workdir, book_id)
    bid = str(st.get("book_id") or root.name)
    unit_id = _require_unit(unit)
    r = Report("qc-pack", bid, root, 0, unit_id)
    hits = check_qc_pack(root, st, unit_id, r, BookCache(root), seal=True)
    return r, hits


def cmd_prompt_pack_legacy(
    workdir: str,
    book_id: str,
    unit: str = "",
    volume: str = "",
    stage: str = "",
    **_kw,
) -> tuple[Report, list[str]]:
    """Deprecated: use pack-outline | pack-write | pack-finalize."""
    stg = (stage or "").strip().lower()
    if stg in ("qc", "review", "finalize") or (not stg and unit and not volume):
        if stg in ("qc", "review"):
            stg = STAGE_FINALIZE
        if not stg:
            stg = STAGE_WRITE
    if (volume or "").strip() and not (unit or "").strip() and stg in ("", STAGE_OUTLINE):
        stg = STAGE_OUTLINE
    if stg == STAGE_OUTLINE:
        return cmd_pack_outline(workdir, book_id, volume=volume)
    if stg == STAGE_FINALIZE:
        return cmd_pack_finalize(workdir, book_id, unit=unit)
    return cmd_pack_write(workdir, book_id, unit=unit)


COMMANDS = {
    "pack-outline": cmd_pack_outline,
    "pack-write": cmd_pack_write,
    "pack-finalize": cmd_pack_finalize,
    "lint-outline": cmd_lint_outline,
    "check-length": cmd_check_length,
    "check-deslop": cmd_check_deslop,
    "check-commit": cmd_check_commit,
    "seal-write": cmd_seal_write,
    "seal-commit": cmd_seal_commit,
    "accept-volume": cmd_accept_volume,
    "doctor": cmd_doctor,
    "init": cmd_init,
    "cast-lint": cmd_cast_lint,
    "migrate": cmd_migrate,
    "preflight": cmd_preflight,
    "precommit": cmd_precommit,
    # legacy aliases (also via --action)
    "qc-pack": cmd_qc_pack_legacy,
    "prompt-pack": cmd_prompt_pack_legacy,
    "outline-pack": cmd_outline_pack_legacy,
    "lint-units": cmd_lint_outline,
    "scan-deslop": cmd_check_deslop,
    "postcommit": cmd_postcommit_legacy,
}

ACTIONS = frozenset(COMMANDS)


def run_with_hits(
    workdir: str,
    book_id: str,
    action: str,
    unit: str = "",
    volume: str = "",
    title: str = "",
    genre: str = "",
    stage: str = "",
) -> tuple[Report, list[str]]:
    action = (action or "").strip().lower()
    mapped = LEGACY_ACTION_MAP.get(action, action)
    fn = COMMANDS.get(mapped) or COMMANDS.get(action)
    if not fn:
        raise ValueError(
            f"unknown command {action!r}; try: "
            + ", ".join(
                [
                    "pack-outline",
                    "pack-write",
                    "pack-finalize",
                    "lint-outline",
                    "check-length",
                    "check-deslop",
                    "check-commit",
                    "seal-write",
                    "seal-commit",
                    "accept-volume",
                ]
            )
        )
    return fn(
        workdir,
        book_id,
        unit=unit,
        volume=volume,
        title=title,
        genre=genre,
        stage=stage,
    )


def run(workdir: str, book_id: str, action: str, unit: str = "", volume: str = "", **kw) -> Report:
    rep, _ = run_with_hits(workdir, book_id, action, unit, volume, **kw)
    return rep


def _add_common(sp: argparse.ArgumentParser, *, unit: bool = False, volume: bool = False) -> None:
    sp.add_argument("--workdir", default=".", help="project root or book root")
    sp.add_argument("--book-id", default="", help="slug under novel/<book-id>/")
    if unit:
        sp.add_argument("--unit", default="", help="plot unit id, e.g. v01-U1")
    if volume:
        sp.add_argument("--volume", default="", help="volume id, e.g. v01")
    sp.add_argument("--json", action="store_true")


def _build_parser() -> argparse.ArgumentParser:
    p = argparse.ArgumentParser(
        description="Novel write-gate — subcommands (pack-* / check-* / seal-*)",
    )
    # Legacy flat --action (deprecated)
    p.add_argument(
        "--action",
        default="",
        help="DEPRECATED: use subcommands. Still mapped for transition.",
    )
    p.add_argument("--workdir", default=".", help=argparse.SUPPRESS)
    p.add_argument("--book-id", default="", help=argparse.SUPPRESS)
    p.add_argument("--unit", default="", help=argparse.SUPPRESS)
    p.add_argument("--volume", default="", help=argparse.SUPPRESS)
    p.add_argument("--title", default="", help=argparse.SUPPRESS)
    p.add_argument("--genre", default="", help=argparse.SUPPRESS)
    p.add_argument("--stage", default="", help=argparse.SUPPRESS)
    p.add_argument("--json", action="store_true", help=argparse.SUPPRESS)

    sub = p.add_subparsers(dest="command")

    sp = sub.add_parser("pack-outline", help="Materialize outline pack for a volume")
    _add_common(sp, volume=True)
    sp = sub.add_parser("pack-write", help="Materialize write pack for a unit")
    _add_common(sp, unit=True)
    sp = sub.add_parser("pack-finalize", help="Materialize finalize pack (HITS/LENGTH/COMMIT)")
    _add_common(sp, unit=True)
    sp = sub.add_parser("lint-outline", help="Lint unit YAML for a volume")
    _add_common(sp, volume=True)
    sp = sub.add_parser("check-length", help="Hard length gate")
    _add_common(sp, unit=True)
    sp = sub.add_parser("check-deslop", help="Deslop / AI-味 hits")
    _add_common(sp, unit=True)
    sp = sub.add_parser("check-commit", help="Ledger / summaries hard checks")
    _add_common(sp, unit=True)
    sp = sub.add_parser("seal-write", help="Mark outline drafted after prose lands")
    _add_common(sp, unit=True)
    sp = sub.add_parser("seal-commit", help="Re-check then write reviewed/cursor")
    _add_common(sp, unit=True)
    sp = sub.add_parser("accept-volume", help="Promote cast + seed outline heads")
    _add_common(sp, volume=True)
    sp = sub.add_parser("doctor", help="Book layout doctor")
    _add_common(sp)
    sp = sub.add_parser("init", help="Init book tree")
    _add_common(sp)
    sp.add_argument("--title", default="")
    sp.add_argument("--genre", default="")
    sp = sub.add_parser("cast-lint", help="Lint cast cards")
    _add_common(sp)
    sp = sub.add_parser("migrate", help="Migrate legacy layout")
    _add_common(sp)
    sp = sub.add_parser("preflight", help="Preflight before write (legacy)")
    _add_common(sp, unit=True)
    return p


def main(argv: list[str] | None = None) -> int:
    _force_stdio_utf8()
    sys.dont_write_bytecode = True
    argv = list(sys.argv[1:] if argv is None else argv)
    p = _build_parser()
    args = p.parse_args(argv)

    command = (getattr(args, "command", None) or "").strip().lower()
    legacy_action = (getattr(args, "action", None) or "").strip().lower()

    if not command and legacy_action:
        print(
            f"warning: --action {legacy_action} is deprecated; use subcommand "
            f"'{LEGACY_ACTION_MAP.get(legacy_action, legacy_action)}' instead",
            file=sys.stderr,
        )
        command = legacy_action
    if not command:
        p.print_help()
        return 2

    workdir = os.path.abspath(getattr(args, "workdir", None) or ".")
    book_id = getattr(args, "book_id", "") or ""
    unit = getattr(args, "unit", "") or ""
    volume = getattr(args, "volume", "") or ""
    title = getattr(args, "title", "") or ""
    genre = getattr(args, "genre", "") or ""
    stage = getattr(args, "stage", "") or ""
    as_json = bool(getattr(args, "json", False))

    try:
        rep, hit_lines = run_with_hits(
            workdir, book_id, command, unit, volume, title, genre, stage
        )
    except UnicodeDecodeError as e:
        print(e.reason if e.reason else str(e), file=sys.stderr)
        return 2
    except (ValueError, FileNotFoundError, OSError) as e:
        print(e, file=sys.stderr)
        return 2

    if as_json:
        payload = rep.as_dict()
        if command in ("check-deslop", "scan-deslop", "qc-pack", "pack-finalize"):
            payload["hits"] = hit_lines
        json.dump(payload, sys.stdout, ensure_ascii=False, indent=2)
        sys.stdout.write("\n")
    else:
        if command in ("check-deslop", "scan-deslop"):
            sys.stdout.write("### HITS\n")
            if hit_lines:
                for line in hit_lines:
                    sys.stdout.write(line + "\n")
            else:
                sys.stdout.write("None.\n")
            sys.stdout.write("\n")
        sys.stdout.write(rep.format())
    return 0 if rep.verdict == "PASS" else 1
