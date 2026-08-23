---
name: novel-plan
source: builtin
description: Plan a novel already scaffolded. Use for 人设/世界观, 金手指, book/volume outlines. Not for 章合同, batch freeze, prose, or review.
license: MIT
compatibility: Requires write, edit, read_file, glob; Core table_*, memory_*, search_kb; ask_user
metadata:
  author: danmo-work
  version: "2.0"
  category: creative-writing
---

# Novel Plan（设定与大纲）

Lock canon and outlines. **No chapter contracts, no chapter bodies.** New entities stay `candidate` until `ask_user`.

## When to load

人设 / 世界观 / 金手指 / 总纲 / 卷纲.

## Do

| Intent | Load | search_kb |
|--------|------|-----------|
| 总纲 / 卷纲 | `novel-plan/references/outline.md` | 节奏与结构, 爽点与追读, 题材与平台, 强约束 |
| 人设 / 世界观 | `canon/world.md` + `cast-card.md` | 人设与群像, 世界观与金手指 |
| 金手指 | `novel-setup/assets/templates/goldfinger-card.md` | 世界观与金手指 |

Templates: `novel-plan/assets/templates/*`（总纲/卷纲）+ `novel-setup/assets/templates/`（world / cast-card / goldfinger-card）. After user OK on a volume, set `novel-state.yaml` `stage: outline`.

## Stop

Wait for confirmation on volume outlines. Do not write 章合同 or prose. Batch freeze and contracts belong to `novel-write`.
