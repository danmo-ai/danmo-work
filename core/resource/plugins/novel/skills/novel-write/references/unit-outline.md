# 单元细纲

**No prose without an accepted 单元细纲** for that unit.

Official name: **单元细纲**. Do not introduce other product names. This file replaces per-chapter 章纲. Do not write `chapters/chNNN-outline.yaml`.

## Canonical path

| Rule | Value |
|------|--------|
| Path | `novel/<book-id>/outline/units/vNN-U#.yaml` (id matches the volume **单元索引表**, e.g. `v01-U1.yaml`) |
| Format | **YAML only** — copy `assets/templates/unit-outline.yaml` via `read_skill` |
| Forbidden | Markdown unit outlines, per-chapter outline files, copying scene/`cut_hook` text into the volume outline |

**本文件是单元级唯一事实源。** 卷纲只索引（unit_id / 章范围 / 一句话功能 / 本单元禁碰 / 钩子类型），不重复本文件字段。写细纲时从卷纲索引行 + 一句话功能出发，展开为完整合同。

写细纲前先读本卷纲：定位 **单元索引表** 中本单元的行，`unit_id` 与章范围必须对上。卷纲没有这一行、或章范围对不上 → 退回 `novel-plan`，不要空造细纲。

## 从索引下推

卷纲索引只给你：`unit_id` / 章范围 / 一句话功能 / 本单元禁碰 / 下一单元钩子类型。你要把它展开为完整合同：

- 一句话功能 → `function`
- 上一单元 `next_hook.out` → `entry`（因果入口）
- 你设计：主角局部目标 → `desire`
- 你设计：得不到的具体原因 → `obstacle`
- 你设计：关键选择（谁/在哪/代价）→ `choice`
- 你设计：主兑现 → `payoff`；主爽点形态 → `pleasure`（单元一条，连续3单元不雷同）
- 本单元禁碰（索引表 + bible unlock 卷 + `canon/locked-terms.yaml`）→ `forbidden` / `endgame_boundary`
- 钩子类型（索引表已给）→ `next_hook.type`；你设计：具体事件 → `next_hook.out`

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

1. Read the volume **单元索引表** for this unit. Missing row or range mismatch → stop, back to `novel-plan`.
2. **状态对齐（只读小节）**：读 `continuity/facts.md` `### Cast snapshot` 中本单元角色的行（`state_deltas` 的「从X」必须与 snapshot 一致）。场面含双人对手戏时，另读相关人物卡「关系」段（关系卡只写质态，不写编年史）。
3. Write `outline/units/vNN-U#.yaml`. Set `status=accepted` when ready to draft（默认不 `ask_user`）。新正式姓名过取名短清单；重要新角色先写 `candidate` 卡。
4. Set `novel-state.yaml` `active_unit` to this id，`stage: writing`.
5. Proceed to `unit-write.md`.

## Status

- 细纲 `status`: `proposed | accepted | drafted | reviewed`.
- `novel-state.yaml` artifacts only: `missing | in_progress | ready | stale | blocked`.

Do not mix the two vocabularies.
