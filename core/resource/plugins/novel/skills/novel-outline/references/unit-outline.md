# 单元细纲

## Pipeline system

一批细纲 / 写细纲 / 重写细纲只读本 pack。禁止扫树、`find` gate、读题材全文、读人物卡、写正文。

1. `python3 "$G" pack-outline --workdir . --book-id <slug> --volume vNN`。exit ≠ 0 → 停。
2. 按 pack 把本批 ≤4 个 `proposed` 填成 `accepted`（或覆盖已有 YAML）。不改卷纲已定的 `function` 与 `next_hook.type`。头已由 accept-volume 种下，不要另抄模板。
3. `python3 "$G" lint-outline --workdir . --book-id <slug> --volume vNN` **一次**。全 PASS → 停。FAIL → 只补失败单元，再 lint **一次**；仍 FAIL → 停。禁止第三次 lint。

**No prose without an accepted 单元细纲** for that unit.

Official name: **单元细纲**. Do not introduce other product names. This file replaces per-chapter 章纲. Do not write `chapters/chNNN-outline.yaml`.

## Canonical path

| Rule | Value |
|------|--------|
| Path | `novel/<book-id>/outline/units/vNN-U#.yaml` (id matches the volume **单元索引表**, e.g. `v01-U1.yaml`) |
| Format | **YAML only** — heads already seeded; fill fields from the pack. Do not `read_skill` the template on a production turn. |
| Forbidden | Markdown unit outlines, per-chapter outline files, copying scene/`cut_hook` text into the volume outline |

**本文件是单元级唯一事实源。** 卷纲只分配（unit_id / 章范围 / 一句话功能 / 终局边界短语 / 钩子类型 / 本卷人物），不重复本文件字段。

**头部已由 `accept-volume` 种下**（`unit_id` / `chapter_range` / `function` / `next_hook.type` / `status: proposed` / `word_*` / 时钟字段空壳）。你的活是把 `proposed` 填成 `accepted`：合同、`on_stage` / `pov`、**故事时钟**、场面、章切口、**结构化 state_deltas**。没有头文件 → 卷纲没批准或没跑 `accept-volume`，退回 `novel-plan`，不要空造。

## 一批细纲

一轮填 **≤4 个** `proposed` 单元（按 unit 顺序），写完 `lint-outline --volume vNN` 一次。PASS 则停。FAIL 只补失败单元再 lint 一次，仍 FAIL 则停。本卷仍有 `proposed` → 下一轮再发一批。不要在同一轮写正文。

## 上场人物

- `on_stage`：本单元开口或被写到的角色 stem（= `canon/cast/<stem>.md` 文件名），**必须 ⊆ 卷纲「本卷人物」**，且全部 `canon`（`accept-volume` 已提升；不在名单的新角色 → 退回 `novel-plan` 补卡 + 加进本卷人物）。
- `pov`：本单元默认 POV stem，∈ `on_stage`。preflight 会给他注入「不知」一行。
- 场面级可选 `who`（本场在场子集）/ `pov`（覆盖）。
- 龙套用工称，不进 `on_stage`。`on_stage` 为空 → preflight 不带人物并 warning。

## 故事时钟与身份跃迁

- `story_day`：本单元开场相对日序（整数；与 `novel-state.time_system` / 卷纲「本卷时间线」对齐）。`time_label` 可读锚点可选。
- `gap_from_prev`：与上一单元时间差（「三日后」/「开卷」）；`accepted+` 空则 **blocking**。
- `flashback: true`：允许 `story_day` 回跳；正文明示闪回。否则 gate 对前序单元做 `timeline_monotonic` 硬检。
- 场面拆 `when`（故事时）与 `where`（地点）；旧稿 `where: "time | location"` 仍可读。
- `state_deltas` 推荐结构化：

```yaml
state_deltas:
  - stem: zhu-jue
    field: title   # title|age|location|power|relation|status|other
    from: "落魄捕快"
    to: "试用捕头"
```

  兼容旧串 `"zhu-jue: 落魄捕快→试用捕头"`。`from` 对齐 Cast snapshot / 人物卡基线；`to` 是**单元结束**目标。preflight 只把此时身份注入 `identity@unit`，**不**把 `to` 当开场人设。

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
- **场面契约（`accepted+` 必填）**：
  - `emotional_beat`：本场主情绪从 X→Y（一句，如「镇定→羞怒」）
  - `reader_effect`：期望读者感受（一句，如「替主角窝火」）
  - `subtext`：表面在谈什么 | 实际在争什么；无对白写「无」
- `beat` 只许：建立期待、尝试、加压、决断、兑现、余波。节拍覆盖章范围。
- `chapters[]` 只记切口：从哪场到哪场、`cut_hook`、`word_share`。
- 单章 `word_share` 2000–3500。`word_target` 等于各章之和。
- **`word_floor` = 章数 × 2000，`word_ceiling` = 章数 × 3500**。precommit 实测 runes 硬检。
- **单元规模**：默认 3–8 章，硬上限 10 章。详见 `unit-scale.md`。
- `status: proposed` 且尚无场面 → lint 只 advisory；一旦有场面或升为 `accepted`，契约字段与 `must_land` 同级 blocking。

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

1. `exec_shell` gate `--action outline-pack --volume vNN`，只消费 stdout `### OUTLINE_PACK`（本卷时间线 / 本卷人物 / Cast snapshot 身份行 / open loops / 锁词 / proposed≤4 接钩与上一时钟）。**不是**写正文 preflight CONTEXT；无题材全文、无单元卡渲染。
2. 无 proposed 队列 → 卷纲未批准 / 未 `accept-volume` / 本卷已填完；退回 `novel-plan` 或开始写单元。
3. 对手戏需要关系质态时，才点读相关人物卡「关系」段（开卷年龄/本职已在 pack 的 snapshot/基线行）。
4. 逐个 `edit` `outline/units/vNN-U#.yaml`：`on_stage` / `pov` / 时钟字段 / 合同 / `scenes`（`when`+`where`）/ `chapters` / 结构化 `state_deltas`（`from` 对齐 pack 身份行）/ `info_control`，`status=accepted`（默认不 `ask_user`）。新正式姓名过取名短清单。
5. `exec_shell` gate `--action lint-units --volume vNN`；FAIL 只补失败单元。
6. Set `novel-state.yaml` `active_unit` 为本批第一个，`stage: writing`。停；写正文另开一轮（`unit-write.md`）。

## Status

- 细纲 `status`: `proposed | accepted | drafted | reviewed`.
- `novel-state.yaml` artifacts only: `missing | in_progress | ready | stale | blocked`.

Do not mix the two vocabularies.
