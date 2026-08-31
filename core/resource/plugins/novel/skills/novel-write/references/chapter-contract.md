# Chapter contract（章合同）

**No prose without an accepted contract** for that chapter (or an explicit user waiver).

Official name: **章合同** / chapter contract. Do not introduce other product names for this artifact.

## Canonical path & format

| Rule | Value |
|------|--------|
| Path | `novel/<book-id>/chapters/chNNN-contract.yaml` (zero-pad chapter to 3+ digits) |
| Format | **YAML only** — copy `assets/templates/chapter-contract.yaml` via `read_skill` |
| Forbidden | Markdown contracts, `outline/*-contract.*`, alternate directories or suffixes |

`table_upsert` into `chapter_contracts` is an **index/mirror** (queryable status), not a substitute for the YAML file. Always write the file first; table row should point at that path (e.g. `file: chapters/ch001-contract.yaml`).

Book/volume planning stays in `outline/` (`outline.md`) and **stops at 剧情单元卡（一段章）**. Per-chapter planning is **only** 章合同 under `chapters/`. Do not copy `purpose` / `pleasure_point` / `hook` into 卷纲 or novel-state freeze fields.

写合同前先读本卷纲：定位本章所属 **单元卡** + 最近锚点，填 `unit_id`（`vNN-U#`，如 `v04-U2`）。按卡上 **单元节拍** 确定本章角色（建立期待/尝试/加压/决断/兑现/余波），再下推：
- 单元功能 + 本章节拍角色 → `purpose` / `beats` 序
- 主爽点形态 + 兑现归属 → `pleasure_point` / `micro_payoff`
- 禁止提前释放 + 终局边界 → `forbidden` / `constraint_checks`
- 主角局部目标 / 关键选择 / 核心阻碍 → `beats` / `state_deltas` 动机与冲突面
- 段末「下一单元钩子」只定 `hook.type` 方向；`hook.out` 在合同写具体事件

卷纲只有目标句、没有单元卡，节拍未覆盖本章，或 `unit_id` 对不上 → 退回 `novel-plan` 补卷纲，不要空造合同。

A batch of contracts: **连续 3 章 `pleasure_point` 为空必须重排** before freeze or draft.

## Template fields

Copy `assets/templates/chapter-contract.yaml`. Keep it lean — fill only what this chapter uses; empty optional lists are fine.

| Field | Purpose |
|-------|---------|
| `chapter` / `title_working` | Identity |
| `unit_id` | Required. Volume unit id, `vNN-U#`. Empty or mismatch → stop, back to `novel-plan`. |
| `scene` | One line: `pov \| time \| location` |
| `purpose` | One-sentence chapter job (from unit 功能 + 本章节拍角色) |
| `beats` | 3–5 beats that must land, in order (guided by unit 节拍 + 阻碍/关键选择) |
| `forbidden` | Ban list / spoilers to avoid |
| `pleasure_point` | 本章爽点（网文必填：读者爽在哪） |
| `emotion_line` | One-line curve, e.g. `压抑→爆发→留钩` |
| `state_deltas` | `"谁: 从X→Y"` — deltas only |
| `info_control.reveals` / `.foreshadowing` | Information control; FS-ids with `plant\|advance\|payoff` |
| `hook.type` / `hook.out` | Type from KB「爽点与追读」选型表; `out` = concrete event, not slogan |
| `micro_payoff` | 本章微兑现（追读力 KB） |
| `reader_debt` | 待接钩子列表（开放债务） |
| `constraint_checks` | 强约束自检（金手指/时间线/叙事线/终局储备/代理权） |
| `word_target` | 番茄常见 2000–3500 |
| `continuity_risks` | What could break |
| `status` | `proposed` → `accepted` → `drafted` → `reviewed` |

## Process

1. Read the volume outline **unit card** covering this chapter (节拍 + 功能 + 阻碍…); if missing or beats don't cover this chapter, stop and send back to `novel-plan`.
2. Set `unit_id` to that card (`vNN-U#`). Locate this chapter's role in 单元节拍. Push down: 单元功能+节拍角色 → `purpose`/`beats`; 主爽点形态+兑现归属 → `pleasure_point`/`micro_payoff`; 禁止提前释放 → `forbidden`; 主角局部目标/关键选择 into `beats`/`state_deltas` as needed.
3. Draft contract at `chapters/chNNN-contract.yaml` (YAML template).
4. `table_upsert` `chapter_contracts` with matching `book_id` / `chapter` / `unit_id` / `status` / `file`.
5. `ask_user` to accept if this is a milestone chapter or first chapter of a batch.
6. Only then proceed to `chapter-write.md`.

## Status vocabulary

- **Contract `status`** (this file + `chapter_contracts` table): `proposed | accepted | drafted | reviewed`.
- **Artifact status** in `novel-state.yaml` only: `missing | in_progress | ready | stale | blocked`.

Do not mix the two vocabularies.
