#!/usr/bin/env python3
"""Deterministic novel write-gate — entry shell. Stdlib only. Invoked by novel skills via exec_shell.

Subcommands (preferred):
  python3 novel_gate.py pack-outline|pack-write|pack-finalize|lint-outline \\
    check-length|check-deslop|check-commit|seal-write|seal-commit|accept-volume \\
    --workdir PROJECT --book-id SLUG [--unit vNN-U#] [--volume vNN]

Legacy `--action …` still maps with a stderr warning. Skills must use subcommands only.
Exit 0 PASS, 1 FAIL, 2 usage/error. Implementation lives in novel_gate/.
"""
from __future__ import annotations

import sys
from pathlib import Path

sys.dont_write_bytecode = True
_HERE = str(Path(__file__).resolve().parent)
if _HERE not in sys.path:
    sys.path.insert(0, _HERE)

from novel_gate.cli import main  # noqa: E402

if __name__ == "__main__":
    sys.exit(main())
