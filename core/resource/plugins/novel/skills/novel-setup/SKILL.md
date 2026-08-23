---
name: novel-setup
source: builtin
description: Open a new long-form / webnovel book. Use when 立项, scaffolding novel/<book-id>/, writing book-bible and novel-state, or seeding candidate cast. Not for outlining volumes, drafting chapters, or review.
license: MIT
compatibility: Requires write, edit, read_file, glob; Core table_*, memory_*, search_kb; ask_user
metadata:
  author: danmo-work
  version: "2.0"
  category: creative-writing
---

# Novel Setup（立项开书）

Scaffold one book. **Stop when the tree and bible exist.** Do not outline volumes or write prose.

## When to load

`read_skill` this skill for 开书 / 立项 / 想写一本. Vague premise → `brainstorming` + `ask_user` first.

## Do

1. `read_skill` `novel-setup/references/init.md` + `project-layout.md` + `table-schema.md`.
2. `search_kb` 题材与平台 + 人设与群像.
3. Create `novel/<book-id>/` English tree; `write` `book-bible.md`（含终局储备）、`novel-state.yaml` (`stage: init`)、`canon/world.md`、`canon/glossary.md`.
4. Cast starts `candidate` from `cast-card.md`; confirm via `ask_user` before `canon`.
5. Templates: `novel-setup/assets/templates/*`（bible / state / world / glossary / cast-card / goldfinger-card）.

## Stop

Report paths written. Do not invent the next book or jump to volume outline unless asked.
