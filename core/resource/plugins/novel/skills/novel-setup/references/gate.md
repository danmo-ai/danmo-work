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
python3 "$G" --workdir . --book-id <slug> --action lint-units --volume v01
python3 "$G" --workdir . --book-id <slug> --action preflight --unit v01-U1
python3 "$G" --workdir . --book-id <slug> --action qc-pack --unit v01-U1
python3 "$G" --workdir . --book-id <slug> --action postcommit --unit v01-U1
python3 "$G" --workdir . --book-id <slug> --action migrate
```

| Action | Stage | Notes |
|--------|-------|-------|
| `init` | 立项 | 一次建树（canon/cast、outline/volumes、outline/units、units、continuity/summaries、continuity/commits、reviews）并拷模板（state / bible / world / author-lore / locked-terms / facts / style-fingerprint / book-outline）。已存在文件不覆盖。模型只填圣经、`genre`、`subgenre`、`qc_profile`。 |
| `doctor` | 任何时候 | Layout + UTF-8 + state 字段（`genre` 在八题材内；`subgenre` 须属于该题材）。`chapters/` 有文件而 `units/` 无正文 → BLOCK。旧结构（facts 里有 `## chNNN`、细纲无 `on_stage`、卡无 `role`、仍有 `craft_lane`）→ ADVISORY `[migrate]`。 |
| `cast-lint` | 规划 / 补卡后 | 每张卡 `status` / `role` 键值行；关系表「对方」须是某张卡的 stem；A→B 有行而 B→A 无行 → blocking（质态可不同）；质态空 → warning；canon 卡缺最小必填 → warning。结果写 state `artifacts.cast_registry: ok/fail`。 |
| `accept-volume` | 卷纲批准后 | `--volume vNN`。校验单元索引（章范围连续、九类钩、功能非空、≤10 章）与「本卷人物」；把本卷人物 `candidate → canon`；按索引每行种 `outline/units/vNN-U#.yaml` 头（`unit_id` / `chapter_range` / `function` / `next_hook.type` / `status: proposed` / 按 `unit_scale` 预填 `word_*`）。已存在的 YAML 不覆盖。 |
| `lint-units` | 一批细纲写完 | `--volume vNN`。一次校验本卷全部细纲：shape（章连续、每章 ≥2 场、`word_share` 之和、节拍/钩子枚举）+ 卷纲对准（章范围 / 钩子类型 blocking，`function` 只 warning，终局边界）+ `on_stage ⊆ 本卷人物`、无 `candidate`、`pov ∈ on_stage`。`### UNITS` 逐单元 PASS/FAIL。`proposed` 且无场面 = 待细纲，不报错。 |
| `preflight` | 写正文前 | `--unit vNN-U#`。exit ≠ 0 不写。`### CONTEXT` 依次：风格指纹 → 题材专有文全文（`genre`；`subgenre=刑侦探案` 再接子类专有文「刑侦人味文风」）→ 卷纲索引行 → 单元卡（YAML 渲染）→ 上一单元钩子 → `on_stage` 人物（snapshot 行 + 三锚点 + 1 条台词；`pov` 加「不知」；双方在场的关系行）→ 开放债务 ≤8 → 锁词 → 加载纪律。asset 门：`on_stage` 全 canon 且存在 canon protagonist。 |
| `qc-pack` | 定稿轮 | `--unit`。precommit + scan-deslop 一份 stdout：`### LENGTH`（runes vs floor/ceiling/target，`expand_needed` / `polish_needed`）、`### HITS`（行号）、`### COUNTS` 四计数、锁词命中。字数够则跳扩写，HITS 空则跳润色。 |
| `precommit` / `scan-deslop` | 保留 | 供单独调用；规则与 `qc-pack` 相同。 |
| `postcommit` | Commit 后 | `--unit`。到 `continuity/summaries/vNN.md` 核对每章 `## chNNN` 五要素（块仍在 facts → 过但提示 migrate）；`state_deltas` 的谁在 Cast snapshot；FS-id 在 Open loops；关系变化未回写卡「最近变化点」→ warning；`last_committed_ch` ≥ 范围末章。 |
| `migrate` | 旧书 | facts `## chNNN` 按章归卷写 `summaries/vNN.md`（facts 留索引行）；state 缺 `genre` 猜一次 + blocker「确认 genre」；细纲缺 `on_stage` 从 `state_deltas` 预填、`pov` 留空 warning；卡缺 `role` 推断；卷纲缺「本卷人物」由细纲 `on_stage` 并集生成。写 `continuity/commits/migrate-<date>.md`。layout / encoding blocking 时不迁。 |

Exit: `0` PASS, `1` FAIL, `2` usage/IO error. `--json` 输出同样内容（`sections` 含 UNITS / LENGTH / HITS / CHANGES 等）。

`exec_shell` is allowed **only** for this script.

## stem 一致性

四处必须同一 stem（= `canon/cast/<stem>.md` 文件名）：卷纲「本卷人物」、细纲 `on_stage` / `pov` / 场面 `who`、人物卡关系表「对方」。`cast-lint` / `lint-units` / `preflight` 三处都检。

## Encoding (UTF-8)

Book text must be UTF-8. `doctor` detects non-UTF-8 and BLOCKS. Convert with `scripts/migrate_novel_encoding.py`. Gate does not rewrite encoding and does not migrate `chapters/` into `units/`.
