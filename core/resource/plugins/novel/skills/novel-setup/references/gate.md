# Novel gate script

Deterministic layout / 卷纲 / 单元细纲 / 人物 / deslop checks + tree building. **Not a Go builtin.** Run with `exec_shell` only.

Same pack as the other `novel-setup` resources. After plugin sync:

`${WORK_HOME}/plugins/novel/skills/novel-setup/scripts/novel_gate.py`（入口薄壳；实现在同目录 `novel_gate/` 包里，命令行不变）

## Command (cwd = project root)

```bash
G="${WORK_HOME}/plugins/novel/skills/novel-setup/scripts/novel_gate.py"

python3 "$G" --workdir . --action init --book-id <slug> [--title 书名] [--genre 悬疑]
python3 "$G" --workdir . --book-id <slug> --action doctor
python3 "$G" --workdir . --book-id <slug> --action cast-lint
python3 "$G" --workdir . --book-id <slug> --action accept-volume --volume v01
python3 "$G" --workdir . --book-id <slug> --action outline-pack --volume v01
python3 "$G" --workdir . --book-id <slug> --action lint-units --volume v01
python3 "$G" --workdir . --book-id <slug> --action preflight --unit v01-U1
python3 "$G" --workdir . --book-id <slug> --action prompt-pack --stage write --unit v01-U1
python3 "$G" --workdir . --book-id <slug> --action prompt-pack --stage outline --volume v01
python3 "$G" --workdir . --book-id <slug> --action prompt-pack --stage finalize --unit v01-U1
python3 "$G" --workdir . --book-id <slug> --action qc-pack --unit v01-U1
python3 "$G" --workdir . --book-id <slug> --action postcommit --unit v01-U1
python3 "$G" --workdir . --book-id <slug> --action migrate
```

| Action | Stage | Notes |
|--------|-------|-------|
| `init` | 立项 | 一次建树（canon/cast、outline/volumes、outline/units、units、continuity/summaries、continuity/commits、reviews）并拷模板（state / bible / world / author-lore / locked-terms / facts / style-fingerprint / book-outline）。已存在文件不覆盖。模型只填圣经、`genre`、`subgenre`、`qc_profile`、`time_system`（默认 `relative_days`）。 |
| `doctor` | 任何时候 | Layout + UTF-8 + state 字段（`genre` 在八题材内；`subgenre` 须属于该题材；`time_system` ∈ relative_days\|calendar）。`chapters/` 有文件而 `units/` 无正文 → BLOCK。旧结构（facts 里有 `## chNNN`、细纲无 `on_stage`、卡无 `role`、仍有 `craft_lane`、`accepted+` 场面缺情感契约）→ ADVISORY `[migrate]`。 |
| `cast-lint` | 规划 / 补卡后 | 每张卡 `status` / `role` 键值行；关系表「对方」须是某张卡的 stem；A→B 有行而 B→A 无行 → blocking（质态可不同）；质态空 → warning；canon 卡缺最小必填 → warning。结果写 state `artifacts.cast_registry: ok/fail`。 |
| `accept-volume` | 卷纲批准后 | `--volume vNN`。校验单元索引（章范围连续、九类钩、功能非空、≤10 章）与「本卷人物」；把本卷人物 `candidate → canon`；按索引每行种 `outline/units/vNN-U#.yaml` 头（含 `story_day`/`gap_from_prev`/`flashback` 空壳与结构化 `state_deltas` 注释）。已存在的 YAML 不覆盖。 |
| `outline-pack` | 一批细纲写前 | `--volume vNN`。薄包（**非**写正文 CONTEXT）：本卷时间线、本卷人物、Cast snapshot 身份行、open loops、锁词、proposed≤4 的接钩/上一时钟。填完再 `lint-units`。 |
| `lint-units` | 一批细纲写完 | `--volume vNN`。shape + 卷纲对准 + `on_stage` + **时钟硬门**（accepted+：`gap_from_prev` 必填；`story_day`\|`time_label` 至少其一）+ `timeline_monotonic`。`### UNITS` 逐单元 PASS/FAIL。 |
| `preflight` | 写正文前 | `--unit`。exit ≠ 0 不写。`### CONTEXT` **状态优先**：书级 → 卷索引+本卷时间线 → 本/上单元时钟 → 单元卡 → 接钩 → identity@unit（无重复 snapshot 行）→ 身份转变目标 → 债务/锁词 → **题材文截断(~1200字)** → 风格指纹 → 纪律。 |
| `prompt-pack` | 写/细纲/定稿（短 stdout） | `--stage write\|outline\|finalize`。内部跑 preflight / outline-pack / qc-pack，把正文包写到 `.pack/`，stdout 只留 `### PACK`（file + write 目标）。agent 应 `read_file` 该路径，不要依赖 stdout CONTEXT。 |
| `qc-pack` | 定稿轮 | `--unit`。precommit + scan-deslop + `### LENGTH` + **`### CONTINUITY`**（开场 identity / 结束 to / 时钟 / 伏笔对照）。 |
| `postcommit` | Commit 后 | 摘要五要素；delta who ∈ snapshot；**title/location `to` 与 snapshot 不一致 → blocking**（age advisory）；FS-id ∈ Open loops；关系回写 warning；`last_committed_ch`。 |
| `migrate` | 旧书 | facts `## chNNN` 按章归卷写 `summaries/vNN.md`（facts 留索引行）；state 缺 `genre` 猜一次 + blocker「确认 genre」；细纲缺 `on_stage` 从 `state_deltas` 预填、`pov` 留空 warning；卡缺 `role` 推断；卷纲缺「本卷人物」由细纲 `on_stage` 并集生成。写 `continuity/commits/migrate-<date>.md`。layout / encoding blocking 时不迁。 |

Exit: `0` PASS, `1` FAIL, `2` usage/IO error. `--json` 输出同样内容（`sections` 含 UNITS / LENGTH / HITS / CHANGES 等）。

`exec_shell` is allowed **only** for this script.

## stem 一致性

四处必须同一 stem（= `canon/cast/<stem>.md` 文件名）：卷纲「本卷人物」、细纲 `on_stage` / `pov` / 场面 `who`、人物卡关系表「对方」。`cast-lint` / `lint-units` / `preflight` 三处都检。

## Encoding (UTF-8)

Book text must be UTF-8. `doctor` detects non-UTF-8 and BLOCKS. Convert with `scripts/migrate_novel_encoding.py`. Gate does not rewrite encoding and does not migrate `chapters/` into `units/`.
