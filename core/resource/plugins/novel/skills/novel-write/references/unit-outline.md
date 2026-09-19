# 单元细纲

**No prose without an accepted 单元细纲** for that unit.

Official name: **单元细纲**. Do not introduce other product names. This file replaces per-chapter 章纲. Do not write `chapters/chNNN-outline.yaml`.

## Canonical path

| Rule | Value |
|------|--------|
| Path | `novel/<book-id>/outline/units/vNN-U#.yaml` (id matches the volume card, e.g. `v01-U1.yaml`) |
| Format | **YAML only** — copy `assets/templates/unit-outline.yaml` via `read_skill` |
| Forbidden | Markdown unit outlines, per-chapter outline files, copying scene/`cut_hook` text into the volume card |

Book/volume planning stays in `outline/` and **stops at 剧情单元卡**. The unit outline is the only production contract.

写细纲前先读本卷纲：定位 **单元卡**，`unit_id` 与章范围必须对上。卷纲只有目标句、没有单元卡、或章范围对不上 → 退回 `novel-plan`，不要空造细纲。

从卡下推，写具体事件，不抄空话：

- 单元功能 → `function`
- 因果入口 → `entry`
- 欲望 / 阻碍 / 关键选择 / 兑现归属 → `desire` / `obstacle` / `choice` / `payoff`
- 主爽点形态 → `pleasure`（单元一条，不要求每章一条）
- 禁止提前释放 / 终局边界 → `forbidden` / `endgame_boundary`
- 下一单元钩子 → `next_hook`（类型 + 具体事件；末章 `cut_hook` 与 `out` 相同）

## 场面与切章

场面是写作骨架。一条 `scenes` = 一个可写场面，不是一章。

- 每章至少 2 场。`must_land` 是可在正文落地的动作或对白事实，禁止口号。
- `beat` 只许：建立期待、尝试、加压、决断、兑现、余波。节拍覆盖章范围，但是单元上的功能段。
- `chapters[]` 只记切口：从哪场到哪场、`cut_hook`、`word_share`。
- 单章 `word_share` 3500–5000。`word_target` 等于各章之和。常见单元 1.2 万–2.5 万。
- 删掉「连续 3 章爽点为空必须重排」。相邻单元主爽点不连续雷同，查卷纲锁卷 checklist，不在本文件重排。

## 人物取名

`scenes` / `state_deltas` / `chapters` 里**新出现的正式姓名**须过「人设与群像 → 人物取名反 AI」（P0 表）。本 turn `search_kb` 仍优先「节奏与结构」；取名用下列短清单，不另占配额：

- 禁止同批文艺双字同构、同批高度相似名、现代文无故复姓堆砌、寓意说明书名。
- 龙套默认工称/绰号。不要为过场配完整姓名。
- 需要回访的新角色：点名前后补 `canon/cast/` `candidate` 卡；未 canon 不得进正文。
- 本单元新出名有姓角色 ≤3（已在场主角外）。

## Process

1. Read the volume **单元卡** for this unit. Missing card or range mismatch → stop, back to `novel-plan`.
2. **状态对齐（只读小节）**：读 ledger `### Cast snapshot` 中本单元角色的行（`state_deltas` 的「从X」必须与 snapshot 一致）。场面含双人对手戏时，另读相关人物卡「关系」段。
3. Write `outline/units/vNN-U#.yaml`. Set `status=accepted` when ready to draft（默认不 `ask_user`）。新正式姓名过取名短清单；重要新角色先写 `candidate` 卡。
4. Set `novel-state.yaml` `active_unit` to this id，`stage: writing`. 这就是冻结，不要再写批次章纲。
5. Proceed to `unit-write.md`.

## Status

- 细纲 `status`: `proposed | accepted | drafted | reviewed`.
- `novel-state.yaml` artifacts only: `missing | in_progress | ready | stale | blocked`.

Do not mix the two vocabularies.
