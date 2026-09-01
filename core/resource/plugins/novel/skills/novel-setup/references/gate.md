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

python3 "${WORK_HOME}/plugins/novel/skills/novel-setup/scripts/novel_gate.py" \
  --workdir . --book-id <slug> --action scan-deslop --chapter N

python3 "${WORK_HOME}/plugins/novel/skills/novel-setup/scripts/novel_gate.py" \
  --workdir . --book-id <slug> --action scan-deslop --from A --to B
```

`scan-deslop` prints `### HITS` lines as `chapters/chNNN.md:L42: 一级词「仿佛」 | …excerpt…`, then the usual `### VERDICT`. Same P0 thresholds as precommit (toxic ≥1 / level-one ≥3 / soup ending). Use before/after polish edits; do not invent a second deslop script.

`preflight` also prints `### CONTEXT`（接钩 / 人物现场 / 开放债务 / 本章硬约束 / 单元功能）— 写正文只消费这一段 + 本章合同。

`postcommit` 硬检摘要五要素、`state_deltas`→Cast snapshot、合同 FS-id→Open loops。

If the book root is the workdir (it contains `novel-state.yaml`), omit `--book-id`.

Exit: `0` PASS, `1` FAIL, `2` usage/IO error. FAIL prints `### VERDICT` / BLOCKING — copy into `novel-state.yaml` `blockers`. Do not claim 定稿 without exit 0 on the matching action.

`exec_shell` is allowed **only** for this script. Do not use shell for file IO or other commands.
