# 表格 IR（`.usheet.json` / CSV）

主题关键词：表格 IR、usheet、IWorkbookData、sheet-writing、csv。

## 怎么选

- 矩形数据、易导出 → `.csv`
- 多 sheet / 公式 / Stage Univer 网格 → `.usheet.json`

## `.usheet.json` 要点

- `danmo.format` = `"univer-sheet"`
- 单元格在 `snapshot.sheets[id].cellData["行"]["列"]`
- 字符串 `t: 1`；数字 `t: 2`；公式用 `f`
- 行/列索引是**字符串**

完整最小 workbook 见技能 `sheet-writing` → `references/ir-sheet.md`。

`.xlsx` 只读；转成旁路 `.usheet.json` 再改。不要写 `.danmo-sheet.json`。
