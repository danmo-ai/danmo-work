# Univer sheet IR (`.usheet.json`)

Danmo envelope around Univer `IWorkbookData`. Edit with file tools only.

## Envelope

```json
{
  "danmo": { "format": "univer-sheet", "version": 1 },
  "snapshot": { }
}
```

## Minimal workbook (one sheet, header + row)

```json
{
  "danmo": { "format": "univer-sheet", "version": 1 },
  "snapshot": {
    "id": "workbook_demo",
    "name": "Demo",
    "appVersion": "0.25.1",
    "locale": "enUS",
    "styles": {},
    "sheetOrder": ["sheet1"],
    "sheets": {
      "sheet1": {
        "id": "sheet1",
        "name": "Sheet1",
        "rowCount": 100,
        "columnCount": 26,
        "mergeData": [],
        "cellData": {
          "0": {
            "0": { "v": "Name", "t": 1 },
            "1": { "v": "Score", "t": 1 }
          },
          "1": {
            "0": { "v": "Ada", "t": 1 },
            "1": { "v": 10, "t": 2 }
          }
        },
        "rowData": {},
        "columnData": {},
        "defaultColumnWidth": 88,
        "defaultRowHeight": 24,
        "showGridlines": 1,
        "freeze": { "startRow": -1, "startColumn": -1, "ySplit": 0, "xSplit": 0 },
        "scrollTop": 0,
        "scrollLeft": 0,
        "zoomRatio": 1,
        "hidden": 0,
        "tabColor": "",
        "rightToLeft": 0
      }
    }
  }
}
```

## cellData notes

- Keys are **string** row / column indices: `cellData["0"]["1"]`.
- `t: 1` = string (`v` string); `t: 2` = number (`v` number) or formula (`f: "=A1+1"`, often with `v`).
- Multi-sheet: add more entries under `sheets` and list ids in `sheetOrder`.
- Merges: `mergeData: [{ "startRow": 0, "endRow": 0, "startColumn": 0, "endColumn": 2 }]`.

## Tool edits

1. `read_file` → find `snapshot.sheets[sheetId].cellData`.
2. `apply_patch` the JSON cells you need.
3. Keep `sheetOrder` and sheet `id`/`name` consistent.
