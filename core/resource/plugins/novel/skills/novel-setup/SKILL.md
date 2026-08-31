---
name: novel-setup
source: builtin
description: Open a new long-form / webnovel book. Use when 立项, scaffolding novel/<book-id>/, writing book-bible and novel-state, or seeding candidate cast. Not for outlining volumes, drafting chapters, or review.
license: MIT
compatibility: Requires write, edit, read_file, glob, exec_shell; Core table_*, memory_*, search_kb; ask_user
metadata:
  author: danmo-work
  version: "2.2"
  category: creative-writing
---

# Novel Setup（立项开书）

Scaffold one book. **Stop when the tree and bible exist.** Do not outline volumes or write prose.

**Pipeline step 1/8：立项.** 用户可自行切换推理模型；本技能不换模型。

## When to load

`read_skill` this skill for 开书 / 立项 / 想写一本. Vague premise → `brainstorming` + `ask_user` first.

## Do（少交互）

1. `read_skill` `novel-setup/references/init.md`（按需再开 `project-layout.md`）.
2. `search_kb` **至多一次**：题材与平台.
3. Create `novel/<book-id>/` English tree; `write` `book-bible.md`（含终局储备）、`novel-state.yaml` (`stage: init`)、`canon/world.md`、`canon/author-lore.md`、`continuity/ledger.md`.
4. Cast starts `candidate` from `cast-card.md`; confirm via `ask_user` before `canon`. 金手指默认写在主角卡.
5. Templates: `novel-setup/assets/templates/*`（bible / state / world / cast-card / author-lore / ledger）.
6. Run gate script `--action doctor` before handing off. Legacy books without `ledger.md` → merge old continuity files first (see init.md).

## Stop

Report paths written. Do not invent the next book or jump to volume outline unless asked. Next steps: 设定/总纲 → `novel-plan`.
