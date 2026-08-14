---
name: sheet-writing
source: builtin
description: Design and produce tabular data as CSV or lightweight sheet JSON for the Danmo Office Stage. Use when the user wants a spreadsheet, table deliverable, data grid, or structured rows/columns — not Excel binary by default.
license: MIT
compatibility: Requires write, edit, read_file
metadata:
  author: danmo-work
  version: "1.0"
  category: work
---

# Sheet Writing

Produce tables as project files the Office **Sheet** surface can open and edit.

## Formats

| Format | When |
|--------|------|
| `.csv` | Default. Simple rectangular data, UTF-8. |
| `.danmo-sheet.json` | Multiple logical sheets or metadata needed: `{ "sheets": [{ "name": "Sheet1", "rows": [["A","B"],["1","2"]] }] }` |

Do **not** emit `.xlsx` unless the user explicitly asks for Excel binary (then use office-export / OfficeCLI if available).

## Workflow

1. Confirm columns and 1–2 example rows with the user when ambiguous.
2. Write the file with `write` (or `edit` for updates).
3. Deliver the path and note it opens in **Sheet Stage** (Files → click the file).

## [office-edit] turns

When the user message starts with `[office-edit]` and `kind: sheet`:

1. `read_file` the given `path`.
2. Apply `instruction` to the `selection` (Markdown table or CSV snippet).
3. `edit` / `write` only that path.
4. SUMMARY: what rows/columns changed.

## Anti-patterns

- Defaulting to xlsx/xls
- HTML tables as the only deliverable
- Leaving TBD column headers

## Stop condition

Deliver the `.csv` or `.danmo-sheet.json` path. Stop.
