---
name: novel-plan
source: builtin
description: Plan a novel already scaffolded. Use for 人设/世界观, 金手指, book/volume outlines. Not for 章合同, batch freeze, prose, or review.
license: MIT
compatibility: Requires write, edit, read_file, glob; Core table_*, memory_*, search_kb; ask_user
metadata:
  author: danmo-work
  version: "2.2"
  category: creative-writing
---

# Novel Plan（设定 · 总纲 · 卷纲）

Lock canon and outlines. **No chapter contracts, no chapter bodies.** New entities stay `candidate` until `ask_user`.

**Pipeline steps 2–4/8：设定 → 总纲 → 卷纲.** 用户可自行切换推理模型；本技能不换模型。

## When to load

人设 / 世界观 / 金手指 / 总纲 / 卷纲 / 补设定.

## Do（少交互）

| Intent | Load | search_kb（≤1） |
|--------|------|-----------------|
| 总纲 / 卷纲 | `novel-plan/references/outline.md` | 节奏与结构 |
| 人设 / 世界观 | templates `world.md` + `cast-card.md` | 人设与群像 |
| 金手指 | `cast-card.md` / goldfinger 段 | 世界观与金手指 |

Templates: `novel-plan/assets/templates/*` + `novel-setup/assets/templates/`. After user OK on a volume, set `novel-state.yaml` `stage: outline` and refresh `artifacts`. 终局细节写入 `canon/author-lore.md`；**终局储备 unlock 表只在 `book-bible.md`**，总纲不复制。

Table upserts for cast/world are **optional** mirrors — files are authoritative.

## Stop

Wait for confirmation on volume outlines. Do not write 章合同 or prose. Batch freeze and contracts belong to `novel-write`.
