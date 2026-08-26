---
name: sheet-writing
source: builtin
description: Design and produce tabular data as CSV or Univer sheet IR (`.usheet.json`) for the Danmo Office Stage. Use when the user wants a spreadsheet, table deliverable, data grid, or structured rows/columns — not Excel binary by default.
license: MIT
compatibility: Requires write, edit, read_file
metadata:
  author: danmo-work
  version: "2.0"
  category: work
---

# Sheet Writing

Produce tables as project files the Office **Sheet** surface can open and edit.

## Formats

| Format | When |
|--------|------|
| `.csv` | Default. Simple rectangular data, UTF-8. |
| `.usheet.json` | Multi-sheet, merges, formulas, or Univer editing — Danmo envelope around `IWorkbookData`. |

Do **not** emit `.danmo-sheet.json` (removed). Do **not** emit `.xlsx` unless the user explicitly asks for Excel binary (then export / convert; Stage treats `.xlsx` as view-only until converted to `.usheet.json`).

### `.usheet.json` shape

```json
{
  "danmo": { "format": "univer-sheet", "version": 1 },
  "snapshot": { "id": "…", "name": "Workbook", "appVersion": "0.25.1", "locale": "enUS", "styles": {}, "sheetOrder": ["sheet1"], "sheets": { "sheet1": { "…IWorksheetData with cellData + mergeData…" } } }
}
```

## Workflow

1. Confirm columns and 1–2 example rows with the user when ambiguous.
2. Write `.csv` or `.usheet.json` with `write` (or `edit` for updates).
3. Deliver the path and note it opens in **Sheet Stage**.

## [office-edit] turns

When the user message starts with `[office-edit]` and `kind: sheet`:

1. `read_file` the given `path`.
2. Apply `instruction` to the `selection` (Markdown table, CSV snippet, or Univer cell range).
3. `edit` / `write` only that path (never write `.xlsx` in place when `engine: ms-office`).
4. SUMMARY: what rows/columns / merges changed.

## Anti-patterns

- Defaulting to xlsx/xls as editable SoT
- Creating `.danmo-sheet.json`
- HTML tables as the only deliverable
- Leaving TBD column headers

## Stop condition

Deliver the `.csv` or `.usheet.json` path. Stop.
