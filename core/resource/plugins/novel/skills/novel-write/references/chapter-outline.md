# Chapter outline（章纲）

**No prose without an accepted chapter outline** for that chapter (or an explicit user waiver).

Official name: **章纲** / chapter outline. Do not introduce other product names for this artifact.

## Canonical path & format

| Rule | Value |
|------|--------|
| Path | `novel/<book-id>/chapters/chNNN-outline.yaml` (zero-pad chapter to 3+ digits) |
| Format | **YAML only** — copy `assets/templates/chapter-outline.yaml` via `read_skill` |
| Forbidden | Markdown chapter outlines, files under `outline/` for per-chapter plans, alternate directories or suffixes |
| Migrate | Gate renames legacy `chNNN-contract.yaml` → `chNNN-outline.yaml` once (any action; doctor/cold-start). No dual-read. |

`table_upsert` into `chapter_outlines` is an **index/mirror** (queryable status), not a substitute for the YAML file. Always write the file first; table row should point at that path (e.g. `file: chapters/ch001-outline.yaml`).

Book/volume planning stays in `outline/` (`outline.md`) and **stops at 剧情单元卡（一段章）**. Per-chapter planning is **only** 章纲 under `chapters/`. Do not copy `purpose` / `pleasure_point` / `hook` into 卷纲 or novel-state freeze fields.

写章纲前先读本卷纲：定位本章所属 **单元卡** + 最近锚点，填 `unit_id`（`vNN-U#`，如 `v04-U2`）。按卡上 **单元节拍** 确定本章角色（建立期待/尝试/加压/决断/兑现/余波），再下推：
- 单元功能 + 本章节拍角色 → `purpose` / `beats` 序
- 主爽点形态 + 兑现归属 → `pleasure_point` / `micro_payoff`
- 禁止提前释放 + 终局边界 → `forbidden` / `constraint_checks`
- 主角局部目标 / 关键选择 / 核心阻碍 → `beats` / `state_deltas` 动机与冲突面
- 段末「下一单元钩子」只定 `hook.type` 方向；`hook.out` 在章纲写具体事件

卷纲只有目标句、没有单元卡，节拍未覆盖本章，或 `unit_id` 对不上 → 退回 `novel-plan` 补卷纲，不要空造章纲。

A batch of chapter outlines: **连续 3 章 `pleasure_point` 为空必须重排** before freeze or draft.

## Template fields

Copy `assets/templates/chapter-outline.yaml`. Keep it lean — fill only what this chapter uses; empty optional lists are fine.

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
2. **状态对齐（只读小节，不读全文）**：读 ledger `### Cast snapshot` 中本章涉及角色的行（`state_deltas` 的「从X」必须与 snapshot 当前值一致，不得凭空发明）；beats 涉及双人对手戏时，另读相关人物卡的「关系」段（当前关系不对 → 先 Commit 补登或调整 beats）。
3. Set `unit_id` to that card (`vNN-U#`). Locate this chapter's role in 单元节拍. Push down fields from the unit card.
4. Draft chapter outline at `chapters/chNNN-outline.yaml` (YAML template); set `status=accepted` when ready to draft (default path — no per-chapter `ask_user`).
5. Optional: `table_upsert` mirror — **默认不做**.
6. Proceed to `chapter-write.md`.

## Status vocabulary

- **Chapter outline `status`** (this file): `proposed | accepted | drafted | reviewed`.
- **Artifact status** in `novel-state.yaml` only: `missing | in_progress | ready | stale | blocked`.

Do not mix the two vocabularies.
