# Project layout

Create under the active project workdir:

```text
novel/<book-id>/
  novel-state.yaml
  book-bible.md
  canon/
    world.md
    glossary.md
  outline/
    book_outline.md
    volumes/
  chapters/
    ch001.md
  reviews/
    ch001-review.md
  continuity/
    chapter_summaries.md
    foreshadow-tracker.md
```

## Rules

- `<book-id>`: short slug (ascii or pinyin), stable for the book.
- **Prose truth:** `chapters/*.md` after Commit.
- **Canon truth:** `canon/*` + `table_*` rows with matching `book_id`. Prefer tables for queryable fields; Markdown for long lore.
- **Proposals / what-if:** stay in `outline/` or `canon/proposals.md` until user confirms → then promote to Canon + tables.
- **context packages** (if written) are assemblies with source paths — not a second Canon.

Copy blanks from `novel-writing/assets/templates/` via `read_skill` then `write`.
