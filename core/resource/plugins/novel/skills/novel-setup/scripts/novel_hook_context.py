#!/usr/bin/env python3
"""Thin novel plugin hook: inject stage / active_unit nudge into EphemeralContext.

Protocol (Codex-style hooks v1):
  stdin  JSON: {event, agent_id, session_id, project_id, workdir, goal}
  stdout JSON: {additionalContext: "..."}  — empty string = no injection
Never blocks a turn; failures print {"additionalContext":""}.
"""
from __future__ import annotations

import json
import sys
from pathlib import Path


def _find_state(workdir: Path) -> Path | None:
    """Prefer novel/*/novel-state.yaml under workdir; else workdir/novel-state.yaml."""
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


def main() -> int:
    try:
        raw = sys.stdin.read()
        payload = json.loads(raw) if raw.strip() else {}
    except Exception:
        print(json.dumps({"additionalContext": ""}, ensure_ascii=False))
        return 0
    workdir = Path(str(payload.get("workdir") or ".")).expanduser()
    state_path = _find_state(workdir)
    stage = active = last = ""
    book = ""
    if state_path and state_path.is_file():
        try:
            text = state_path.read_text(encoding="utf-8")
            stage = _kv(text, "stage")
            active = _kv(text, "active_unit")
            last = _kv(text, "last_committed_ch")
            book = _kv(text, "book_id") or state_path.parent.name
        except Exception:
            pass
    lines = [
        "novel 流程 nudge（非硬 CONTEXT；写单元仍须 gate preflight）:",
        f"- book={book or '（未检出）'} stage={stage or '（空）'} active_unit={active or '（空）'} last_committed_ch={last or '0'}",
        "- 细纲：先 outline-pack --volume … 消费 ### OUTLINE_PACK，再 lint-units。",
        "- 写单元：先 preflight --unit …，只消费 ### CONTEXT。",
        "- 定稿：qc-pack → 审（### CONTINUITY）→ Commit → postcommit。",
    ]
    ctx = "\n".join(lines)
    if len(ctx) > 400:
        ctx = ctx[:400] + "…"
    print(json.dumps({"additionalContext": ctx}, ensure_ascii=False))
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
