# Novel gate script

Deterministic layout / 单元细纲 / deslop checks. **Not a Go builtin.** Run with `exec_shell` only.

Same pack as the other `novel-setup` resources. After plugin sync:

`${WORK_HOME}/plugins/novel/skills/novel-setup/scripts/novel_gate.py`

## Command (cwd = project root)

```bash
python3 "${WORK_HOME}/plugins/novel/skills/novel-setup/scripts/novel_gate.py" \
  --workdir . --book-id <slug> --action doctor

python3 "${WORK_HOME}/plugins/novel/skills/novel-setup/scripts/novel_gate.py" \
  --workdir . --book-id <slug> --action preflight --unit v01-U1

python3 "${WORK_HOME}/plugins/novel/skills/novel-setup/scripts/novel_gate.py" \
  --workdir . --book-id <slug> --action precommit --unit v01-U1

python3 "${WORK_HOME}/plugins/novel/skills/novel-setup/scripts/novel_gate.py" \
  --workdir . --book-id <slug> --action postcommit --unit v01-U1

python3 "${WORK_HOME}/plugins/novel/skills/novel-setup/scripts/novel_gate.py" \
  --workdir . --book-id <slug> --action scan-deslop --unit v01-U1
```

| Action | Notes |
|--------|-------|
| `doctor` | Layout + UTF-8. If `chapters/` has files and `units/` has no prose, BLOCK and point at migration. Does not read `chapters/`. |
| `preflight` | `--unit vNN-U#`. Prints `### CONTEXT` for that unit. |
| `precommit` | Same unit. Splits `units/vNN-U#.md` on `## 第N章`. Missing `---` / wrong chapter order is blocking. |
| `postcommit` | `--unit` only. Every chapter in the unit range needs a ledger `## chNNN` block. `last_committed_ch` ≥ range end. |
| `scan-deslop` | Hits are `units/vNN-U#.md:L42: …`. |

`preflight` CONTEXT includes 上一单元末钩 / 人物现场 / 开放债务 / 功能、入口、欲望、阻碍 / 场面序 / 章切口. 写正文只消费这一段 + 单元细纲。

Exit: `0` PASS, `1` FAIL, `2` usage/IO error.

`exec_shell` is allowed **only** for this script.

## Encoding (UTF-8)

Book text must be UTF-8. `doctor` detects non-UTF-8 and BLOCKS. Convert with `scripts/migrate_novel_encoding.py`. Gate does not rewrite encoding and does not migrate `chapters/` into `units/`.
