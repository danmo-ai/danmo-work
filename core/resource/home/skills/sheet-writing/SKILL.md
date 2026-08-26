---
name: sheet-writing
source: builtin
description: "Produce/edit tables as `.csv` or Univer `.usheet.json` via write/edit/apply_patch. Do not web-search Univer schema — use references/ir-sheet.md or kb-office-ir."
license: MIT
compatibility: Requires write, edit, apply_patch, read_file; optional read_skill
metadata:
  author: danmo-work
  version: "3.0"
  category: work
---

# Sheet Writing

## Hard rules

1. Prefer **`.csv`** for simple rectangular data; use **`.usheet.json`** for multi-sheet / formulas / Univer Stage editing.
2. **Change files only with tools** (`read_file` + `edit` / `apply_patch` / `write`).
3. **Do not** `web_search` / `web_fetch` Univer docs for the IR schema — `read_skill` `references/ir-sheet.md` (or `search_kb`「表格 IR」).
4. **Do not** emit `.danmo-sheet.json` or treat `.xlsx` as editable SoT (view-only until converted to `.usheet.json`).
5. **Do not** write slide decks as Markdown or sheets as prose `.md`.

## Format pick

| Need | File |
|------|------|
| Simple table / export | `.csv` (UTF-8) |
| Multi-sheet, formulas, merges, Stage grid | `.usheet.json` |
| User handed `.xlsx` | Convert in Stage → edit sibling `.usheet.json` |
| Narrative report | **Wrong skill** → `.md` |

## Tool workflow (create)

1. Confirm columns when ambiguous.
2. For CSV: `write` UTF-8 text. For IR: `read_skill` `references/ir-sheet.md`, then `write` envelope JSON.
3. Deliver path. Stop.

## Tool workflow ([office-edit])

When `[office-edit]` and `kind: sheet`:

1. `read_file` `path`.
2. If `.csv`: patch rows/columns as text. If `.usheet.json`: patch `snapshot.sheets.*.cellData` (row/col keys are strings).
3. Never overwrite `.xlsx` in place when `engine: ms-office`.
4. SUMMARY: rows/cols/sheets touched.

## Anti-patterns

- Learning IR from the public web instead of project tools + this skill
- Defaulting to xlsx as SoT
- Creating `.danmo-sheet.json`

## Stop condition

Deliver `.csv` or `.usheet.json` path. Stop.
