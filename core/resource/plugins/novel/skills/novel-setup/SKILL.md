---
name: novel-setup
source: builtin
description: Open a new long-form / webnovel book. Use when 立项, scaffolding novel/<book-id>/, writing book-bible and novel-state, or seeding candidate cast. Not for outlining volumes, drafting chapters, or review.
license: MIT
compatibility: Requires write, edit, read_file, glob, exec_shell; Core table_*, memory_*, search_kb; ask_user
metadata:
  author: danmo-work
  version: "2.3"
  category: creative-writing
---

# Novel Setup（立项开书）

Scaffold one book. **Stop when the tree and bible exist.**

**Pipeline step 1/8.** 本技能不换模型。

## When to load

开书 / 立项 / 想写一本. Vague premise → `brainstorming` + **one packed** `ask_user`.

## Do

1. `read_skill` `novel-setup/references/init.md`（按需 `project-layout.md`）.
2. `search_kb` **至多一次**：题材与平台.
3. Create `novel/<book-id>/`；`write` bible（含终局储备）、`novel-state.yaml`、`canon/world.md`、`canon/author-lore.md`、`continuity/ledger.md`.
4. Cast 起 `candidate`（`cast-card.md`）；金手指默认写在主角卡。promote 等到卷纲批准.
5. Templates: bible / state / world / cast-card / author-lore / ledger only.
6. Gate `--action doctor`. Legacy 无 ledger → merge 后 archive（见 init.md）.

## Stop

Report paths. Next: 设定/总纲 → `novel-plan`.
