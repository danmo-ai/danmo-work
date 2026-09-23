# 单元细纲

**No prose without an accepted 单元细纲** for that unit.

Official name: **单元细纲**. Do not introduce other product names. This file replaces per-chapter 章纲. Do not write `chapters/chNNN-outline.yaml`.

## Canonical path

| Rule | Value |
|------|--------|
| Path | `novel/<book-id>/outline/units/vNN-U#.yaml` (id matches the volume **单元索引表**, e.g. `v01-U1.yaml`) |
| Format | **YAML only** — copy `assets/templates/unit-outline.yaml` via `read_skill` |
| Forbidden | Markdown unit outlines, per-chapter outline files, copying scene/`cut_hook` text into the volume outline |

**本文件是单元级唯一事实源。** 卷纲只分配（unit_id / 章范围 / 一句话功能 / 终局边界短语 / 钩子类型 / 本卷人物），不重复本文件字段。

**头部已由 `accept-volume` 种下**（`unit_id` / `chapter_range` / `function` / `next_hook.type` / `status: proposed` / `word_*`）。你的活是把 `proposed` 填成 `accepted`：合同、`on_stage` / `pov`、场面、章切口。没有头文件 → 卷纲没批准或没跑 `accept-volume`，退回 `novel-plan`，不要空造。

## 一批细纲

一轮填 **≤4 个** `proposed` 单元（按 unit 顺序），写完 `exec_shell` gate `--action lint-units --volume vNN`，读 `### UNITS` 逐单元 PASS/FAIL；FAIL 只补失败的那几个再跑。本卷仍有 `proposed` → 下一轮再发一批。不要在同一轮写正文。

## 上场人物

- `on_stage`：本单元开口或被写到的角色 stem（= `canon/cast/<stem>.md` 文件名），**必须 ⊆ 卷纲「本卷人物」**，且全部 `canon`（`accept-volume` 已提升；不在名单的新角色 → 退回 `novel-plan` 补卡 + 加进本卷人物）。
- `pov`：本单元默认 POV stem，∈ `on_stage`。preflight 会给他注入「不知」一行。
- 场面级可选 `who`（本场在场子集）/ `pov`（覆盖）。
- 龙套用工称，不进 `on_stage`。`on_stage` 为空 → preflight 不带人物并 warning。

## 从索引下推

头部已给：`unit_id` / 章范围 / `function` / `next_hook.type` / 终局边界短语。你要把它展开为完整合同：

- `function` 已种下，**不改**（改了 gate 只 warning，以卷纲为准）
- 上一单元 `next_hook.out` → `entry`（因果入口）
- 你设计：主角局部目标 → `desire`
- 你设计：得不到的具体原因 → `obstacle`
- 你设计：关键选择（谁/在哪/代价）→ `choice`
- 你设计：主兑现 → `payoff`；主爽点形态 → `pleasure`（单元一条，连续3单元不雷同）
- 本单元禁碰（索引表 + bible unlock 卷 + `canon/locked-terms.yaml`）→ `forbidden` / `endgame_boundary`
- `next_hook.type` 已种下（改了 blocking）；你设计：具体事件 → `next_hook.out`
- 卷纲终局边界格非空时，`forbidden` 与 `endgame_boundary` 不得皆空（blocking）

## 场面与切章

场面是写作骨架。一条 `scenes` = 一个可写场面，不是一章。

- 每章至少 2 场。`must_land` 是可在正文落地的动作或对白事实，禁止口号。
- `beat` 只许：建立期待、尝试、加压、决断、兑现、余波。节拍覆盖章范围。
- `chapters[]` 只记切口：从哪场到哪场、`cut_hook`、`word_share`。
- 单章 `word_share` 2000–3500。`word_target` 等于各章之和。
- **`word_floor` = 章数 × 2000，`word_ceiling` = 章数 × 3500**。precommit 实测 runes 硬检。
- **单元规模**：默认 3–8 章，硬上限 10 章。详见 `unit-scale.md`。

## 锁词与终局边界

- 写 `forbidden` 时，对照 `canon/locked-terms.yaml`：本单元所在卷号之前 `locked_until` 里的词，正文不得出现。
- gate precommit 会自动扫描正文命中锁词，命中即 FAIL。**不要**在细纲里手写"本单元零出现 XXX"——gate 会做。
- 终局边界只写"本单元禁碰什么"，不写真相细节（真相在 `author-lore.md`）。

## 人物取名

`scenes` / `state_deltas` / `chapters` 里**新出现的正式姓名**须过「人设与群像 → 人物取名反 AI」（P0 表）。

- 禁止同批文艺双字同构、同批高度相似名、现代文无故复姓堆砌、寓意说明书名。
- 龙套默认工称/绰号。不要为过场配完整姓名。
- 需要回访的新角色：点名前后补 `canon/cast/` `candidate` 卡；未 canon 不得进正文。
- 本单元新出名有姓角色 ≤3（已在场主角外）。

## Process

1. `glob outline/units/vNN-U*.yaml`，取 `status: proposed` 的前 ≤4 个。没有 → 卷纲未批准 / 未 `accept-volume`，back to `novel-plan`。
2. 读卷纲「本卷人物」列表（只这一节）与上一单元 `next_hook.out`。
3. **状态对齐（只读小节）**：读 `continuity/facts.md` `### Cast snapshot` 中涉及角色的行（`state_deltas` 的「从X」必须与 snapshot 一致）。场面含双人对手戏时，另读相关人物卡「关系」段（只写质态，不写编年史）。
4. 逐个 `edit` `outline/units/vNN-U#.yaml`：`on_stage` / `pov` / 合同 / `scenes` / `chapters` / `state_deltas` / `info_control`，`status=accepted`（默认不 `ask_user`）。新正式姓名过取名短清单。
5. `exec_shell` gate `--action lint-units --volume vNN`；FAIL 只补失败单元。
6. Set `novel-state.yaml` `active_unit` 为本批第一个，`stage: writing`。停；写正文另开一轮（`unit-write.md`）。

## Status

- 细纲 `status`: `proposed | accepted | drafted | reviewed`.
- `novel-state.yaml` artifacts only: `missing | in_progress | ready | stale | blocked`.

Do not mix the two vocabularies.
