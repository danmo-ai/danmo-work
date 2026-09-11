---
name: novel-write
source: builtin
description: Chapter outline and first-draft prose only. Use for 章纲, batch freeze, drafting/continuing chapters, and Frozen_Canon continuation. Not for 扩写, deslop, review, or Continuity Commit — those are novel-review (separate turn / model).
license: MIT
compatibility: Requires write, edit, read_file, grep, glob, exec_shell; Core table_*, memory_*, search_kb; ask_user
metadata:
  author: danmo-work
  version: "2.8"
  category: creative-writing
---

# Novel Write（章纲 · 正文初稿）

Chapter outline → **first draft only**. 扩写 / 审 / 润色 / Commit → `novel-review`（**另开一轮**，便于写作用更好模型）。

**Pipeline steps 5–6/8.** 本技能不换模型；写完正文后停下，由用户换模再定稿。

## When to load

写第 N 章 / 章纲 / 批次冻结 / 批量正文首稿 / 续写 / 接手 / 卡文救援 / 爽点强化（对白·钩·反转）.

**不要**在本技能回合做：字数扩写、去 AI 味、审稿、连续性定稿。

## Do

| Intent | Load | search_kb（≤1） |
|--------|------|-----------------|
| 章纲 | `chapter-outline.md` | 节奏与结构 |
| 批次冻结 | `batch-freeze.md` | — |
| 写正文（单章，ch≥4） | `chapter-write.md`（含 preflight） | 默认「文风与去 AI 味」；`craft_lane=crime-human` →「刑侦人味文风」 |
| 批量正文首稿 | `batch-draft.md`（冻结批次 + 用户明示 / Workbench） | 同上（批内含 ch1–3 则整批改查「节奏与结构」） |
| 开篇 ch1–3 | 上栏 + `opening-chapters.md` | **节奏与结构**（人味不占配额） |
| 续写 / 卡文 | `continuation.md`（含卡文四法） | 同「写正文（单章，ch≥4）」 |
| 爽点强化 | `chapter-write.md` | 爽点与追读 |
| 场景/对白质感 | `chapter-write.md` + 按需 `scene-routing.md` | 情绪与场景；本 turn 已定人味则按 scene-routing 加载人味节 |

写正文：**gate preflight → 只消费 `### CONTEXT` + 本章纲。** 禁止扫树；禁止 `author-lore`。批次冻结按单元章范围默认写入 `frozen_batch`（见 `batch-freeze.md`）。批量首稿用 `preflight --from/--to`（见 `batch-draft.md`）。

## Hard stops

- No prose without accepted chapter outline + `canon` cast.
- Gate preflight FAIL → no prose.
- Frozen_Canon unconfirmed → no prose.
- Batch write >1 chapter without freeze → run `batch-freeze` first.
- **Draft turn ends at disk prose.** Do not expand / deslop / review / Commit in the same turn unless the user explicitly demands a single-turn exception.
- Frozen batch + explicit batch intent → multi-chapter first drafts in one turn OK (`batch-draft.md`); still no finalize in that turn.

## Stop

Draft on disk → status `drafted`. Hand off `novel-review`（扩写如需 → 审 → 润色 → Commit；批量 → `batch-review.md`）. Suggest user switch model before that turn.
