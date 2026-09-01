---
name: novel-plan
source: builtin
description: Plan a novel already scaffolded. Use for 人设/世界观, 金手指, book/volume outlines. Not for 章合同, batch freeze, prose, or review.
license: MIT
compatibility: Requires write, edit, read_file, glob; Core table_*, memory_*, search_kb; ask_user
metadata:
  author: danmo-work
  version: "2.3"
  category: creative-writing
---

# Novel Plan（设定 · 总纲 · 卷纲）

Lock canon and outlines. **No chapter contracts, no chapter bodies.**

**Pipeline steps 2–4/8.** 本技能不换模型。

## When to load

人设 / 世界观 / 金手指 / 总纲 / 卷纲 / 补设定.

## Do

| Intent | Load | search_kb（≤1） |
|--------|------|-----------------|
| 总纲 / 卷纲 | `novel-plan/references/outline.md` | 节奏与结构 |
| 人设 / 世界观 | templates `world.md` + `cast-card.md` | 人设与群像 |
| 金手指 | `cast-card.md` 金手指段 | 世界观与金手指 |

新实体先 `candidate`。**卷纲批准时**把本卷点名人物一并 `canon`（一次确认，不逐卡 ask）。终局细节 → `author-lore.md`；unlock 表只在 `book-bible.md`。默认不做 `table_*`。

## Stop

Wait for volume outline confirmation. Contracts / prose → `novel-write`.
