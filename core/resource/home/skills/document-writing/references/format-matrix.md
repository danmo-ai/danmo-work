# Office format matrix (Danmo File Stage)

Use file tools (`read_file` / `write` / `edit` / `apply_patch`). **Never** web-search Univer docs to invent schemas — use skill `references/ir-*.md` or knowledge base `kb-office-ir`.

## Extensions → Stage

| Path | Kind | Editable SoT? |
|------|------|----------------|
| `*.md` | doc | Yes (GFM / TipTap) |
| `*.udoc.json` | doc | Yes (Univer Docs IR) |
| `*.uslides.json` | slides | Yes (Univer Slides IR) |
| `*.csv` | sheet | Yes (grid) |
| `*.usheet.json` | sheet | Yes (Univer Sheets IR) |
| `*.docx` / `*.pptx` / `*.xlsx` | ms-office | **View only** → convert to sibling IR |

## Choosing for new work

| Ask | Create |
|-----|--------|
| 报告 / 方案 / RFC / 说明 | `.md` |
| 演讲 / 路演 / 培训幻灯片 | `.uslides.json` |
| 简单表 | `.csv` |
| 多表 / 公式 / 要在 Stage 里当表格编 | `.usheet.json` |
| 用户只要 Word/PPT/Excel 二进制 | still author IR or MD/CSV first; binary is export/import |

## Envelope (all Univer IR files)

```json
{
  "danmo": { "format": "univer-sheet|univer-doc|univer-slides", "version": 1 },
  "snapshot": { }
}
```

Wrong `format` or missing envelope breaks Stage routing.

## Forbidden

- `*-slides.md` / Marp as slide SoT
- `.danmo-sheet.json`
- Editing `.docx`/`.pptx`/`.xlsx` bytes as the AI deliverable
- Replacing a `.uslides.json` office-edit with a new `.md` deck
