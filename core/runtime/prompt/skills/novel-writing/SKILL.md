---
name: novel-writing
description: Long-form / webnovel production with durable canon files, chapter contracts, continuity ledgers, and review gates. Use when opening a book, outlining volumes, writing or continuing chapters, checking continuity, deslopping AI prose, designing 爽点/金手指/hooks, or resuming a novel project.
license: MIT
compatibility: Requires write, edit, read_file, grep, glob; Core table_*, memory_*, search_kb/list_kb_docs/get_kb_doc; ask_user; todowrite optional
metadata:
  author: danmo-work
  version: "1.0"
  category: creative-writing
---

# Novel Writing

Engineering-style long-form fiction for Danmo Work. **Skills guide; tools mutate; completion = file/table results.**

## Core rules

1. **Canon ≠ chat.** Authoritative state lives in project files + `table_*`. Craft/theory lives in knowledge (`kb-novel-craft` and book lore KBs). LLM context is a projection.
2. **Pipeline:** 章合同 → draft → one review round → Commit. `qc_gate` FAIL blocks「定稿」.
   Layout dirs are English-only (`canon/`, `outline/`, `chapters/`, `continuity/`, `reviews/`).  
   Contract path/format is fixed: `chapters/chNNN-contract.yaml` (YAML only).
3. **`candidate` stays out of prose** until the user confirms via `ask_user`.
4. **Minimal load per chapter:** contract + relevant table rows + last 3–5 chapter summaries + KB top-k. Never dump the whole bible.
5. **No fake completion.** Do not claim Commit without `write`/`edit`/`table_upsert`/`memory_update` evidence.
6. **Long prose → files; prefs → `memory_*`; structured state → `table_*`.**

## Three gates (every chapter / batch)

| Gate | Meaning | Pass when |
|------|---------|-----------|
| `knowledge_gate` | Craft rules loaded | `search_kb` / `get_kb_doc` or `read_skill` refs cited for the stage |
| `asset_gate` | Identities ready | Required characters/locations/canon rows exist (`status=canon` for cast in scene) |
| `qc_gate` | Review done | Review file written; no open P0 / blocking issues |

## Danmo tool binding

| Need | Tool |
|------|------|
| Chapter / outline / review files | `write` / `edit` / `read_file` |
| Registries (characters, foreshadows, …) | `table_upsert` / `table_query` / `table_get` / `table_list` |
| Author prefs, rolling summaries, checkpoints | `memory_read` / `memory_update` |
| Craft & lore | `search_kb` / `list_kb_docs` / `get_kb_doc` |
| Confirm directions / Canon | `ask_user` |
| Multi-step work | `todowrite` |
| Research facts | `web_search` / `web_fetch` |

## Route table

Load the matching reference with `read_skill` **before** heavy work. Path = skill-relative, e.g. `novel-writing/references/init.md`.

| Intent | Load | Also search_kb themes |
|--------|------|------------------------|
| New book / 立项 | `references/init.md`, `references/project-layout.md`, `references/table-schema.md` | 题材速览, 人设 |
| Outline / 卷纲 / 章纲列表 | `references/outline.md` | 节奏与结构 |
| Chapter contract / 章合同 | `references/chapter-contract.md` | 节奏, 爽点 |
| Write chapter | `references/chapter-write.md` | 文风, 情绪, 爽点 |
| Review | `references/review-gates.md` | 去 AI 味, 世界观 |
| Deslop / polish | `references/polish-deslop.md` | 去 AI 味, 语言与文风 |
| Commit / resume | `references/continuity-commit.md` | 金手指（若涉及） |
| Routing help | `references/routes.md` | — |

Templates: `novel-writing/assets/templates/*` via `read_skill`.

## Chapter loop (mandatory)

```text
preflight(novel-state) → knowledge_gate → asset_gate
  → chapter_contract → draft(write) → review(one round)
  → fix P0 → commit(table + memory + chapter file)
```

Every 10 committed chapters: write a phase summary into `continuity/` and `memory_update` (agent/project).

## Human stops

Use `ask_user` before: locking reader promise; promoting `candidate`→`canon`; approving volume outline; batch-writing >3 chapters; proceeding past major QC FAIL.

## Stop condition

Report concrete paths and table keys touched. Stop when the requested stage is done; do not invent the next book unless asked.
