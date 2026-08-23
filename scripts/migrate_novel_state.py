#!/usr/bin/env python3
"""One-shot: rewrite novel-state.yaml stage under WORK_DATA_DIR; write .active-book when a novel/ has one book."""

from __future__ import annotations

import os
import re
import sys
from pathlib import Path

KEEP = {"init", "setup", "outline", "writing", "review", "idle"}
TO_REVIEW = {"reviewing", "polish"}
# everything else that looks like chapter work → writing

STAGE_RE = re.compile(r"^(\s*stage:\s*)(\S+)", re.M)


def map_stage(raw: str) -> str:
    token = raw.split("#", 1)[0].strip().strip("\"'")
    if token in KEEP:
        return token
    if token in TO_REVIEW:
        return "review"
    return "writing"


def rewrite_state(path: Path) -> bool:
    text = path.read_text(encoding="utf-8")
    m = STAGE_RE.search(text)
    if not m:
        print(f"skip (no stage): {path}")
        return False
    old = m.group(2)
    new = map_stage(old)
    if old == new:
        return False
    updated = STAGE_RE.sub(lambda mm: f"{mm.group(1)}{new}", text, count=1)
    path.write_text(updated, encoding="utf-8")
    print(f"{path}: {old} → {new}")
    return True


def main() -> int:
    data = Path(os.environ.get("WORK_DATA_DIR") or Path.home() / ".danmo-work" / "data")
    if not data.is_dir():
        print(f"no data dir: {data}", file=sys.stderr)
        return 1
    changed = 0
    for state in sorted(data.glob("proj-*/files/novel/*/novel-state.yaml")):
        if rewrite_state(state):
            changed += 1
    for novel_dir in sorted(data.glob("proj-*/files/novel")):
        if not novel_dir.is_dir():
            continue
        books = [p for p in novel_dir.iterdir() if p.is_dir() and not p.name.startswith(".")]
        if len(books) != 1:
            continue
        marker = novel_dir / ".active-book"
        marker.write_text(books[0].name + "\n", encoding="utf-8")
        print(f"active-book: {marker} → {books[0].name}")
    print(f"done. stages rewritten: {changed}")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
