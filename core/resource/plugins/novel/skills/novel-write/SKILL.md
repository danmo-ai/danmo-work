---
name: novel-write
source: builtin
description: Unit outline and first-draft prose only . Use for 单元细纲, drafting one unit file (chapters separated by ---), and Frozen_Canon continuation. Unit YAML is the single source of unit-level truth; volume outline is a slim index. Not for 扩写, deslop, review, or Continuity Commit — those are novel-review (separate turn / model).
license: MIT
compatibility: Requires write, edit, read_file, grep, glob, exec_shell; Core table_*, memory_*, search_kb; ask_user
metadata:
  author: danmo-work
  version: "3.0"
  category: creative-writing
---

# Novel Write（单元细纲 · 单元正文首稿）

单元细纲 → **one unit file, first draft only**. 扩写 / 审 / 润色 / Commit → `novel-review`（**另开一轮**）。

**Pipeline steps 5–6/8.** 写完正文后停下，由用户换模再定稿。一轮只写一个单元。

## When to load

写单元细纲 / 写单元正文 / 续写 / 接手 / 卡文救援.

**不要**在本技能回合做：字数扩写、去 AI 味、审稿、连续性定稿。不要写 `chapters/chNNN.md` 或章纲。

## Do

| Intent | Load | search_kb（≤1） |
|--------|------|-----------------|
| 单元细纲 | `unit-outline.md` + `unit-scale.md`（单元规模控制） | 节奏与结构（新名用文内取名短清单） |
| 写单元正文 | `unit-write.md`（含 preflight） | 默认「文风与去 AI 味」；单元含 ch1–3 →「节奏与结构」；`craft_lane=crime-human` 且不含 ch1–3 →「刑侦人味文风」 |
| 开篇（单元含 ch1–3） | 上栏 + `opening-chapters.md` | **节奏与结构**（人味不占配额） |
| 续写 / 卡文 | `continuation.md` | 同「写单元正文」 |
| 场景/对白质感 | `unit-write.md` + 按需 `scene-routing.md` | 情绪与场景 |

写正文：**gate preflight --unit → 只消费 `### CONTEXT` + 本单元细纲。** 禁止扫树；禁止 `author-lore`。
preflight CONTEXT 会注入本单元的锁词清单（来自 `canon/locked-terms.yaml`）和字数 floor/ceiling；不需要手动复述。

## Hard stops

- No prose without accepted 单元细纲 + `canon` cast.
- Gate preflight FAIL → no prose.
- Frozen_Canon unconfirmed → no prose.
- **One unit per turn.** Do not draft the next unit in the same turn.
- **Draft turn ends at the unit file.** Do not expand / deslop / review / Commit in the same turn unless the user explicitly demands a single-turn exception.
- **单元规模**：3–8 章默认，硬上限 10 章。>10 章必拆（见 `unit-scale.md`）。

## Stop

`units/vNN-U#.md` on disk → 细纲 `status=drafted`. Hand off `novel-review`. Suggest user switch model before that turn.
