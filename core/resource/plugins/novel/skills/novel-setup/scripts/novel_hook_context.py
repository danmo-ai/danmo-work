#!/usr/bin/env python3
"""Novel plugin hook: classify goal, eagerly run prompt-pack, inject PACK ready.

Protocol (Codex-style hooks v1):
  stdin  JSON: {event, agent_id, session_id, project_id, workdir, goal}
  stdout JSON: {additionalContext: "..."}  — empty string = no injection
Never blocks a turn; failures print {"additionalContext":""} or a soft G= fallback.
"""
from __future__ import annotations

import json
import sys
import traceback
from pathlib import Path


def _ensure_gate_import() -> None:
    here = Path(__file__).resolve().parent
    if str(here) not in sys.path:
        sys.path.insert(0, str(here))


def _find_state(workdir: Path) -> Path | None:
    direct = workdir / "novel-state.yaml"
    if direct.is_file():
        return direct
    root = workdir / "novel"
    if root.is_dir():
        candidates = sorted(root.glob("*/novel-state.yaml"))
        if candidates:
            return candidates[0]
    return None


def _kv(text: str, key: str) -> str:
    for line in text.splitlines():
        s = line.strip()
        if s.startswith(f"{key}:"):
            return s.split(":", 1)[1].strip().strip("\"'")
    return ""


def _blocking_summary(report) -> str:
    msgs = []
    for f in getattr(report, "findings", None) or []:
        if f.get("severity") == "blocking":
            msgs.append(f"[{f.get('check')}] {f.get('message')}")
        if len(msgs) >= 4:
            break
    return "; ".join(msgs) if msgs else ""


def _build_context(
    *,
    book: str,
    stage: str,
    unit: str,
    volume: str,
    pack_rel: str,
    write_hint: str,
    verdict: str,
    blocking: str,
    g_line: str,
) -> str:
    if book and pack_rel and not str(pack_rel).startswith("novel/"):
        pack_path = f"novel/{book}/{pack_rel}"
    else:
        pack_path = pack_rel or "(none)"
    if stage == "finalize":
        do = (
            "STATE: read PACK → edit via EXPAND anchors + HITS (one batch; do not reread full prose) "
            "→ qc-pack ONCE. If PASS: prose FROZEN — commit only COMMIT card (summaries/facts/state/log), "
            "postcommit, stop. If FAIL: report/reviews and stop. Never edit units/*.md after PASS."
        )
        forbid = (
            "forbid: find/glob, read_skill, search_kb, reread facts/yaml/cast, "
            "qc-pack>1, prose edits after PASS, until-exit-0"
        )
    elif stage == "write":
        do = (
            "STATE: read PACK CONTEXT → write one units file (do not read old prose or YAML) "
            "→ status=drafted → stop. No preflight. No qc-pack."
        )
        forbid = (
            "forbid: find/glob, read_skill, search_kb, reread YAML/cast/facts/fingerprint, "
            "preflight, qc-pack"
        )
    elif stage == "outline":
        do = (
            "STATE: read PACK → fill ≤4 YAML → lint-units ONCE. "
            "PASS: stop. FAIL: patch failures, lint ONCE more, then stop."
        )
        forbid = "forbid: find/glob, read_skill, read cast cards, write prose, lint-units>2"
    else:
        do = "do: read_file PACK only → write/edit the write target → stop"
        forbid = (
            "forbid: find/glob for novel_gate.py, read_skill, search_kb, "
            "second-read YAML/cast/facts, scan tree"
        )
    lines = [
        f"NOVEL_ROUTE stage={stage} unit={unit or '—'} volume={volume or '—'} book={book or '—'}",
        f"PACK ready: {pack_path}" if pack_rel else "PACK ready: (failed — use G= below)",
        f"write target: {write_hint}" if write_hint else "",
        f"VERDICT: {verdict}" + (f" | BLOCKING: {blocking}" if blocking else ""),
        do,
        forbid,
        f"G= (fallback only if PACK missing): {g_line}",
    ]
    return "\n".join(x for x in lines if x)


def main() -> int:
    try:
        raw = sys.stdin.read()
        payload = json.loads(raw) if raw.strip() else {}
    except Exception:
        print(json.dumps({"additionalContext": ""}, ensure_ascii=False))
        return 0

    workdir = Path(str(payload.get("workdir") or ".")).expanduser()
    goal = str(payload.get("goal") or "")

    state_path = _find_state(workdir)
    stage_st = active = book = ""
    if state_path and state_path.is_file():
        try:
            text = state_path.read_text(encoding="utf-8")
            stage_st = _kv(text, "stage")
            active = _kv(text, "active_unit")
            book = _kv(text, "book_id") or state_path.parent.name
        except Exception:
            pass

    try:
        _ensure_gate_import()
        from novel_gate.intent import classify_goal, gate_cmd
        from novel_gate.pack import write_hint
        from novel_gate import run as gate_run
    except Exception:
        # Minimal nudge if imports fail
        ctx = (
            "novel hook: gate import failed; use pinned G= only.\n"
            'G=: python3 "$WORK_HOME/plugins/novel/skills/novel-setup/scripts/novel_gate.py" '
            "--workdir . --book-id <slug> --action prompt-pack --stage write|outline|finalize ..."
        )
        print(json.dumps({"additionalContext": ctx}, ensure_ascii=False))
        return 0

    intent = classify_goal(goal, active_unit=active, state_stage=stage_st)
    if intent.kind == "none" or not intent.stage:
        lines = [
            "novel 流程 nudge（未识别为生产 pack 短任务）:",
            f"- book={book or '（未检出）'} stage={stage_st or '（空）'} active_unit={active or '（空）'}",
            f"- goal_kind={intent.kind} reason={intent.reason}",
            "- 生产路径：写细纲/写正文/定稿 → prompt-pack；优先工作台按钮开新会话。",
            f'- G=: python3 "$WORK_HOME/plugins/novel/skills/novel-setup/scripts/novel_gate.py" '
            f"--workdir . --book-id {book or '<slug>'} --action prompt-pack --stage write|outline|finalize ...",
            "forbid: find/glob for novel_gate.py",
        ]
        print(json.dumps({"additionalContext": "\n".join(lines)}, ensure_ascii=False))
        return 0

    unit = intent.unit_id
    volume = intent.volume
    if intent.stage == "outline" and not volume and unit:
        volume = unit.split("-")[0] if "-" in unit else ""
    if intent.stage in ("write", "finalize") and not unit:
        # Cannot pack without unit — inject G= with placeholders
        g = gate_cmd(book_id=book or "<slug>", stage=intent.stage, unit_id="", volume=volume)
        ctx = (
            f"NOVEL_ROUTE stage={intent.stage} unit=（缺）\n"
            "PACK ready: (skipped — goal missing vNN-U#; set active_unit or include unit id)\n"
            f"G=: {g}\n"
            "forbid: find/glob for novel_gate.py"
        )
        print(json.dumps({"additionalContext": ctx}, ensure_ascii=False))
        return 0
    if intent.stage == "outline" and not volume:
        g = gate_cmd(book_id=book or "<slug>", stage="outline", volume="")
        ctx = (
            "NOVEL_ROUTE stage=outline volume=（缺）\n"
            "PACK ready: (skipped — need --volume vNN)\n"
            f"G=: {g}\n"
            "forbid: find/glob for novel_gate.py"
        )
        print(json.dumps({"additionalContext": ctx}, ensure_ascii=False))
        return 0

    bid = book or ""
    g_line = gate_cmd(book_id=bid or "<slug>", stage=intent.stage, unit_id=unit, volume=volume)
    pack_rel = ""
    verdict = "FAIL"
    blocking = ""
    hint = write_hint(intent.stage, unit, volume)

    try:
        from novel_gate.common import resolve_book
        from novel_gate.pack import pack_filename

        project = workdir
        book_root, st = resolve_book(str(project), bid)
        bid = str(st.get("book_id") or book_root.name)
        g_line = gate_cmd(book_id=bid, stage=intent.stage, unit_id=unit, volume=volume)
        hint = write_hint(intent.stage, unit, volume)

        rep = gate_run(
            str(project),
            bid,
            "prompt-pack",
            unit=unit,
            volume=volume,
            stage=intent.stage,
        )
        verdict = getattr(rep, "verdict", "FAIL") or "FAIL"
        blocking = _blocking_summary(rep)
        name = pack_filename(intent.stage, unit, volume)
        pack_file = book_root / ".pack" / name
        if pack_file.is_file():
            pack_rel = f".pack/{name}"
    except Exception as exc:
        blocking = f"pack-error: {exc}"
        traceback.print_exc(file=sys.stderr)

    ctx = _build_context(
        book=bid,
        stage=intent.stage,
        unit=unit,
        volume=volume,
        pack_rel=pack_rel,
        write_hint=hint,
        verdict=verdict,
        blocking=blocking,
        g_line=g_line,
    )
    print(json.dumps({"additionalContext": ctx}, ensure_ascii=False))
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
