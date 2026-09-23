#!/usr/bin/env python3
"""Deterministic novel write-gate — entry shell. Stdlib only. Invoked by novel skills via exec_shell.

python3 novel_gate.py --action doctor|init|accept-volume|lint-units|cast-lint|preflight|qc-pack|precommit|scan-deslop|postcommit|migrate \\
  --workdir PROJECT [--book-id SLUG] [--unit vNN-U#] [--volume vNN] [--json]
Unit actions require --unit; accept-volume / lint-units require --volume; init requires --book-id.
Exit 0 PASS, 1 FAIL, 2 usage/error. Implementation lives in the novel_gate/ package next to this file.
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
