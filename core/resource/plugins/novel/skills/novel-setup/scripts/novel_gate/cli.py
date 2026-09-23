"""Argument parsing and action dispatch."""
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
from .context import check_preflight
from .doctor import check_doctor
from .init import init_book
from .ledger import check_postcommit
from .migrate import run_migrate
from .outline import accept_volume, lint_units
from .qc import check_precommit, check_qc_pack, check_scan_deslop

UNIT_ACTIONS = {"preflight", "precommit", "postcommit", "scan-deslop", "qc-pack"}
VOLUME_ACTIONS = {"accept-volume", "lint-units"}
BOOK_ACTIONS = {"doctor", "cast-lint", "migrate"}
ACTIONS = UNIT_ACTIONS | VOLUME_ACTIONS | BOOK_ACTIONS | {"init"}
ACTION_HELP = (
    "doctor | init | accept-volume | lint-units | cast-lint | preflight | "
    "qc-pack | precommit | scan-deslop | postcommit | migrate"
)


def _record_cast_registry(book_root: Path, verdict: str) -> None:
    sp = book_root / "novel-state.yaml"
    if not sp.is_file():
        return
    text = read_book_text(sp)
    new = set_state_nested_scalar(text, "artifacts", "cast_registry", "ok" if verdict == "PASS" else "fail")
    if new != text:
        write_book_text(sp, new)


def run_with_hits(
    workdir: str,
    book_id: str,
    action: str,
    unit: str = "",
    volume: str = "",
    title: str = "",
    genre: str = "",
) -> tuple[Report, list[str]]:
    action = (action or "").strip().lower()
    if action not in ACTIONS:
        raise ValueError(f"unknown action {action!r} ({ACTION_HELP})")
    hit_lines: list[str] = []
    if action == "init":
        if not book_id:
            raise ValueError("init requires --book-id <slug>")
        r = Report(action, book_id, Path(workdir).resolve() / "novel" / book_id)
        info = init_book(workdir, book_id, r, title=title, genre=genre)
        if info:
            r.section("CREATED", [f"root: {info['root']}"] + [f"+ {p}" for p in info["copied"]] + [f"= {p} (kept)" for p in info["kept"]])
            r.context_lines = [
                "- 下一步: 填 book-bible.md（读者承诺、Style card）、canon/world.md；",
                "  novel-state.yaml 定 genre / qc_profile / craft_lane；然后 novel-plan 一轮出人物卡 + 总纲 + 第 1 卷卷纲。",
            ]
        r.finalize()
        return r, hit_lines

    root, st = resolve_book(workdir, book_id)
    bid = str(st.get("book_id") or root.name)
    if action in BOOK_ACTIONS:
        r = Report(action, bid, root, 0)
        if action == "doctor":
            check_doctor(root, st, r)
        elif action == "cast-lint":
            check_cast_lint(root, r, BookCache(root))
            r.finalize()
            _record_cast_registry(root, r.verdict)
            return r, hit_lines
        else:
            pre = Report("doctor", bid, root, 0)
            check_doctor(root, st, pre)
            pre.finalize()
            hard = [f for f in pre.findings if f["severity"] == "blocking" and f["check"] in ("layout", "encoding", "migrate")]
            if hard:
                for f in hard:
                    r.blocking("doctor", f"fix before migrate: [{f['check']}] {f['message']}")
            else:
                changes = run_migrate(root, st, r)
                r.section("CHANGES", changes if changes else ["nothing to migrate"])
        r.finalize()
        return r, hit_lines
    if action in VOLUME_ACTIONS:
        vol = (volume or "").strip()
        if not vol or not VOLUME_ID_RE.match(vol):
            raise ValueError(f"{action} requires --volume vNN")
        r = Report(action, bid, root, 0)
        if action == "accept-volume":
            info = accept_volume(root, st, vol, r)
            if info and "seeded" in info:
                r.section(
                    "ACCEPTED",
                    [f"canon: {', '.join(info.get('promoted') or []) or '(all already canon)'}"]
                    + [f"+ outline/units/{u}.yaml (proposed)" for u in info.get("seeded", [])]
                    + [f"= outline/units/{u}.yaml (kept)" for u in info.get("kept", [])],
                )
        else:
            results = lint_units(root, st, vol, r, BookCache(root))
            body = []
            for uid, verdict, msgs in results:
                body.append(f"{uid}: {verdict}")
                body.extend(f"  - {m}" for m in msgs)
            r.section("UNITS", body if body else ["no outline/units/*.yaml for " + vol])
        r.finalize()
        return r, hit_lines

    unit_id = (unit or "").strip()
    if not unit_id:
        raise ValueError(f"{action} requires --unit vNN-U#")
    if not UNIT_ID_RE.match(unit_id):
        raise ValueError(f"--unit {unit_id} must match vNN-U#")
    r = Report(action, bid, root, 0, unit_id)
    cache = BookCache(root)
    if action == "scan-deslop":
        hit_lines = check_scan_deslop(root, unit_id, r)
    elif action == "qc-pack":
        hit_lines = check_qc_pack(root, st, unit_id, r, cache)
    elif action == "postcommit":
        check_postcommit(root, st, unit_id, r)
    elif action == "preflight":
        check_preflight(root, st, unit_id, r, cache)
    else:
        check_precommit(root, st, unit_id, r, cache)
    r.finalize()
    return r, hit_lines


def run(workdir: str, book_id: str, action: str, unit: str = "", volume: str = "", **kw) -> Report:
    rep, _ = run_with_hits(workdir, book_id, action, unit, volume, **kw)
    return rep


def main(argv: list[str] | None = None) -> int:
    _force_stdio_utf8()
    sys.dont_write_bytecode = True
    p = argparse.ArgumentParser(description="Novel write-gate / doctor / deslop scan (skill script)")
    p.add_argument("--action", default="doctor", help=ACTION_HELP)
    p.add_argument("--workdir", default=".", help="project root or book root")
    p.add_argument("--book-id", default="", help="slug under novel/<book-id>/")
    p.add_argument("--unit", default="", help="plot unit id, e.g. v01-U1")
    p.add_argument("--volume", default="", help="volume id for accept-volume / lint-units, e.g. v01")
    p.add_argument("--title", default="", help="book title (init)")
    p.add_argument("--genre", default="", help="genre for init: 玄幻|仙侠|都市|悬疑|现代言情|古代言情|仕途扫黑|系统穿越")
    p.add_argument("--json", action="store_true")
    args = p.parse_args(argv)
    try:
        rep, hit_lines = run_with_hits(
            os.path.abspath(args.workdir),
            args.book_id,
            args.action,
            args.unit,
            args.volume,
            args.title,
            args.genre,
        )
    except UnicodeDecodeError as e:
        print(e.reason if e.reason else str(e), file=sys.stderr)
        return 2
    except (ValueError, FileNotFoundError, OSError) as e:
        print(e, file=sys.stderr)
        return 2
    action = (args.action or "").strip().lower()
    if args.json:
        payload = rep.as_dict()
        if action in ("scan-deslop", "qc-pack"):
            payload["hits"] = hit_lines
        json.dump(payload, sys.stdout, ensure_ascii=False, indent=2)
        sys.stdout.write("\n")
    else:
        if action == "scan-deslop":
            sys.stdout.write("### HITS\n")
            if hit_lines:
                for line in hit_lines:
                    sys.stdout.write(line + "\n")
            else:
                sys.stdout.write("None.\n")
            sys.stdout.write("\n")
        sys.stdout.write(rep.format())
    return 0 if rep.verdict == "PASS" else 1
