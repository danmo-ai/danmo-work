# Novel gate script

Deterministic layout / contract / deslop checks. **Not a Go builtin.** Run with `exec_shell` only.

Same pack as the other `novel-setup` resources. After plugin sync:

`${WORK_HOME}/plugins/novel/skills/novel-setup/scripts/novel_gate.py`

`read_skill` path=`novel-setup/scripts/novel_gate.py` works the same as `novel-setup/references/gate.md`. `glob` on the book project will not see the pack — use `${WORK_HOME}` (injected in every `exec_shell`, including containers). Do not use `$HOME/.danmo-work` — container `$HOME` is not the host home.

## Command (cwd = project root)

```bash
python3 "${WORK_HOME}/plugins/novel/skills/novel-setup/scripts/novel_gate.py" \
  --workdir . --book-id <slug> --action doctor

python3 "${WORK_HOME}/plugins/novel/skills/novel-setup/scripts/novel_gate.py" \
  --workdir . --book-id <slug> --action preflight --chapter N

python3 "${WORK_HOME}/plugins/novel/skills/novel-setup/scripts/novel_gate.py" \
  --workdir . --book-id <slug> --action precommit --chapter N

python3 "${WORK_HOME}/plugins/novel/skills/novel-setup/scripts/novel_gate.py" \
  --workdir . --book-id <slug> --action postcommit --chapter N
```

If the book root is the workdir (it contains `novel-state.yaml`), omit `--book-id`.

Exit: `0` PASS, `1` FAIL, `2` usage/IO error. FAIL prints `### VERDICT` / BLOCKING — copy into `novel-state.yaml` `blockers`. Do not claim 定稿 without exit 0 on the matching action.

`exec_shell` is allowed **only** for this script. Do not use shell for file IO or other commands.
