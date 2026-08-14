# Project layout

Canonical tree under the active project workdir. **Directory names are English and fixed.** Do not invent parallel Chinese trees (`设定集/`, `人物/`, `卷纲/`, `台账/`, …) for the same roles.

```text
novel/<book-id>/
  novel-state.yaml              # stage machine / next action
  book-bible.md                 # reader promise + canon index
  canon/                        # long-form lore (authoritative with tables)
    world.md
    glossary.md
    writing-rules.md            # optional craft/platform rules for this book
    cast/                       # character sheets
    research/                   # optional factual research notes
  outline/                      # book / volume planning (not chapter contracts)
    book_outline.md
    volumes/                    # v01.md … or equivalent volume briefs
  chapters/
    ch001-contract.yaml         # 章合同 (YAML only; required before draft)
    ch001.md                    # prose after draft / Commit
  continuity/                   # ledgers after / between chapters
    chapter_summaries.md
    foreshadow-tracker.md
    decision-log.md             # continuity issues / author rulings
  reviews/
    ch001-review.md
  extras/                       # optional non-Canon (author notes, travelogue, …)
  _archive/                     # optional migrated / replaced materials
```

## Project-root source briefs (optional)

Author-imported briefs may live at the **project files root** (sibling of `novel/`), e.g. `提纲.md`, `分卷.md`. Treat them as source material: promote into `canon/` / `outline/` / tables; do not treat project-root Chinese filenames as the book layout.

## Rules

- `<book-id>`: short slug (ascii or pinyin), stable for the book.
- **Prose truth:** `chapters/*.md` after Commit (never `*-contract.yaml`).
- **Chapter contract truth:** `chapters/chNNN-contract.yaml` only (YAML). See `chapter-contract.md`.
- **Canon truth:** `canon/*` + `table_*` rows with matching `book_id`. Prefer tables for queryable fields; Markdown for long lore.
- **Outline truth:** book/volume plans under `outline/` only. One-line chapter lists may live in outline files; per-chapter acceptance specs are 章合同, not outline.
- **Continuity truth:** open loops / rulings / rolling summaries under `continuity/` (+ matching `table_*` when queryable).
- **Proposals / what-if:** stay in `outline/` or `canon/proposals.md` until user confirms → then promote to Canon + tables.
- **context packages** (if written) are assemblies with source paths — not a second Canon.

## Role map (do not fork)

| Role | Directory |
|------|-----------|
| World / glossary / cast / writing rules | `canon/` |
| Book & volume outlines | `outline/` |
| 章合同 + prose | `chapters/` |
| Foreshadows, summaries, CI rulings | `continuity/` |
| Review reports | `reviews/` |
| Non-Canon extras | `extras/` |
| Replaced migrations | `_archive/` |

Copy blanks from `novel-writing/assets/templates/` via `read_skill` then `write`.
