# Novel gate script

Deterministic layout / 卷纲 / 单元细纲 / 人物 / deslop checks + tree building. **Not a Go builtin.** Run with `exec_shell` only.

After plugin sync:

`${WORK_HOME}/plugins/novel/skills/novel-setup/scripts/novel_gate.py`

## Command (cwd = project root)

```bash
G="${WORK_HOME}/plugins/novel/skills/novel-setup/scripts/novel_gate.py"

python3 "$G" init --workdir . --book-id <slug> [--title 书名] [--genre 悬疑]
python3 "$G" doctor --workdir . --book-id <slug>
python3 "$G" cast-lint --workdir . --book-id <slug>
python3 "$G" accept-volume --workdir . --book-id <slug> --volume v01
python3 "$G" pack-outline --workdir . --book-id <slug> --volume v01
python3 "$G" lint-outline --workdir . --book-id <slug> --volume v01
python3 "$G" pack-write --workdir . --book-id <slug> --unit v01-U1
python3 "$G" seal-write --workdir . --book-id <slug> --unit v01-U1
python3 "$G" pack-finalize --workdir . --book-id <slug> --unit v01-U1
python3 "$G" check-length --workdir . --book-id <slug> --unit v01-U1
python3 "$G" check-deslop --workdir . --book-id <slug> --unit v01-U1
python3 "$G" check-commit --workdir . --book-id <slug> --unit v01-U1
python3 "$G" seal-commit --workdir . --book-id <slug> --unit v01-U1
python3 "$G" migrate --workdir . --book-id <slug>
```

| Subcommand | Stage | Notes |
|------------|-------|-------|
| `init` | 立项 | 建树并拷模板。 |
| `doctor` | 任何时候 | Layout + UTF-8 + state 字段。 |
| `cast-lint` | 规划 / 补卡后 | 人物卡与关系表。 |
| `accept-volume` | 卷纲批准后 | 提升 canon + 种 proposed 细纲头。 |
| `pack-outline` | 细纲 | 薄包写到 `.pack/`；只读 pack 填 YAML。 |
| `lint-outline` | 细纲写完 | 结构/合同；最多再跑一次。 |
| `pack-write` | 写正文 | CONTEXT 包；随后 `seal-write`。 |
| `seal-write` | 写正文后 | 细纲 `drafted`。 |
| `pack-finalize` | 定稿 | LENGTH / HITS / COMMIT 骨架包（缺 commits 文件时含 `COMMIT_SKELETON`）。exit≠0 且有 PACK = 有缺口，继续改包。 |
| `check-length` / `check-deslop` / `check-commit` | 定稿 | 只读；同一助手步三条 `exec_shell` 并行。 |
| `seal-commit` | 定稿收口 | 再跑三检；全过才 reviewed/cursor。 |
| `migrate` | 旧书 | 迁移 legacy 布局。 |

Exit: `0` PASS, `1` FAIL, `2` usage/IO error. Skills own which verbs to run; scripts own verdicts.

`exec_shell` is allowed **only** for this script. Never `find`/`glob` for it.

## stem 一致性

四处必须同一 stem（= `canon/cast/<stem>.md` 文件名）：卷纲「本卷人物」、细纲 `on_stage` / `pov` / 场面 `who`、人物卡关系表「对方」。

## Encoding (UTF-8)

Book text must be UTF-8. `doctor` detects non-UTF-8 and BLOCKS.
