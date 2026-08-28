# Novel gate script

Deterministic layout / contract / deslop checks. **Not a Go builtin.** Run with `exec_shell` only.

## Script

`novel-setup/scripts/novel_gate.py` (Python 3 stdlib). After plugin sync:

`${WORK_HOME:-$HOME/.danmo-work}/plugins/novel/skills/novel-setup/scripts/novel_gate.py`

## Command (cwd = project root)

```bash
python3 "${WORK_HOME:-$HOME/.danmo-work}/plugins/novel/skills/novel-setup/scripts/novel_gate.py" \
  --workdir . --book-id <slug> --action doctor

python3 "${WORK_HOME:-$HOME/.danmo-work}/plugins/novel/skills/novel-setup/scripts/novel_gate.py" \
  --workdir . --book-id <slug> --action preflight --chapter N

python3 "${WORK_HOME:-$HOME/.danmo-work}/plugins/novel/skills/novel-setup/scripts/novel_gate.py" \
  --workdir . --book-id <slug> --action precommit --chapter N

python3 "${WORK_HOME:-$HOME/.danmo-work}/plugins/novel/skills/novel-setup/scripts/novel_gate.py" \
  --workdir . --book-id <slug> --action postcommit --chapter N
```

If the book root is the workdir (it contains `novel-state.yaml`), omit `--book-id`.

Exit: `0` PASS, `1` FAIL, `2` usage/IO error. FAIL prints `### VERDICT` / BLOCKING — copy into `novel-state.yaml` `blockers`. Do not claim 定稿 without exit 0 on the matching action.

`exec_shell` is allowed **only** for this script. Do not use shell for file IO or other commands.
