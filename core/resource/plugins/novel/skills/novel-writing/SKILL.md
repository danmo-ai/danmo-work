---
name: novel-writing
source: builtin
description: Long-form / webnovel production with durable canon files, chapter contracts, continuity ledgers, batch freeze, continuation engine, and review gates. Use when opening a book, outlining volumes, freezing batch outlines, writing or continuing chapters, checking continuity, deslopping AI prose, designing 爽点/金手指/hooks, or resuming a novel project.
license: MIT
compatibility: Requires write, edit, read_file, grep, glob; Core table_*, memory_*, search_kb/list_kb_docs/get_kb_doc; ask_user; todowrite optional
metadata:
  author: danmo-work
  version: "1.1"
  category: creative-writing
---

# Novel Writing

Engineering-style long-form fiction for Danmo Work. **Skills guide; tools mutate; completion = file/table results.**

## Core rules

1. **Canon ≠ chat.** Authoritative state lives in project files + `table_*`. Craft/theory lives in knowledge (`kb-novel-craft` and book lore KBs). LLM context is a projection.
2. **Pipeline:** 章合同 → draft → one review round → Commit. `qc_gate` FAIL blocks「定稿」.  
   Layout dirs are English-only (`canon/`, `outline/`, `chapters/`, `continuity/`, `reviews/`).  
   Contract path/format is fixed: `chapters/chNNN-contract.yaml` (YAML only).  
   **Lean artifacts:** copy templates as-is; fill only what the current stage uses. No invented extra sections.
3. **`candidate` stays out of prose** until the user confirms via `ask_user`.
4. **Minimal load per chapter:** contract + relevant table rows + last 3–5 chapter summaries + KB top-k. Never dump the whole bible.
5. **No fake completion.** Do not claim Commit without `write`/`edit`/`table_upsert`/`memory_update` evidence.
6. **Long prose → files; prefs → `memory_*`; structured state → `table_*`.**

## Three gates (every chapter / batch)

| Gate | Meaning | Pass when |
|------|---------|-----------|
| `knowledge_gate` | Craft rules loaded | `search_kb` / `get_kb_doc` or `read_skill` refs cited for the stage |
| `asset_gate` | Character canon check | 1. `table_query characters` — verify all cast in this scene are `status=canon`<br>2. `read_file canon/cast/<name>.md` — load the sheet for each involved character<br>3. If any cast is missing or `candidate`: stop, create draft sheet → `ask_user` to promote to `canon` |
| `qc_gate` | Review done | Review file written; `### VERDICT` PASS; no open P0 |

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
| New book / 立项 | `references/init.md`, `project-layout.md`, `table-schema.md` | 题材速览, 人设 |
| Outline / 卷纲 | `references/outline.md` | 节奏与结构, 番茄平台 |
| Batch freeze | `references/batch-freeze.md`, `preflight.md` | 节奏, 爽点 |
| Chapter contract | `references/chapter-contract.md` | 节奏, 爽点, 番茄平台 |
| Write chapter | `references/preflight.md`, `chapter-write.md`, `scene-routing.md` | 文风, 爽点, 追读力 |
| Review | `references/review-gates.md` | 去 AI 味, 题材 QC |
| Batch review | `references/review-gates.md` | 去 AI 味, 追读力 |
| Deslop / polish | `references/polish-deslop.md` | 去 AI 味, 语言与文风 |
| Commit / resume | `references/continuity-commit.md` | 金手指（若涉及） |
| Continuation | `references/continuation.md` | 文风, 情绪 |
| Routing help | `references/routes.md` | — |

Templates: `novel-writing/assets/templates/*` via `read_skill`.

## Chapter loop (mandatory)

```text
preflight → knowledge_gate → asset_gate
  → chapter_contract → draft(write) → review(one round)
  → fix P0 → commit(table + memory + chapter file)
```

Batch path: `outline → batch-freeze →` per-chapter loop above.

Every 10 committed chapters: write a phase summary into `continuity/` and `memory_update` (agent/project).

## Human stops

Use `ask_user` before: locking reader promise; promoting `candidate`→`canon`; approving volume outline; batch-writing >3 chapters; proceeding past major QC FAIL; **续写 Frozen_Canon 未确认**; **批次未冻结却批量写正文**.

## Stop condition

Report concrete paths and table keys touched. Stop when the requested stage is done; do not invent the next book unless asked.
