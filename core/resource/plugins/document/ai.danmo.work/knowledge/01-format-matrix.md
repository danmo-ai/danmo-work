# 格式矩阵（Markdown vs Univer IR）

Agent 改 Office 产物时：**只用** `read_file` / `write` / `edit` / `apply_patch`。不要 `web_search` Univer 官方文档来猜 schema——本知识库 + 技能 `references/ir-*.md` 已够用。

## 扩展名

| 扩展名 | Stage kind | 是否可编 SoT |
|--------|------------|--------------|
| `.md` | doc | 是（长文默认） |
| `.udoc.json` | doc | 是（Univer 文档 IR） |
| `.uslides.json` | slides | 是（幻灯片 IR） |
| `.csv` | sheet | 是 |
| `.usheet.json` | sheet | 是（表格 IR） |
| `.docx` `.pptx` `.xlsx` | ms-office | 只读；先转为同名旁路 IR |

## 新建时怎么选

- **报告 / RFC / 说明** → `.md`
- **幻灯片** → `.uslides.json`（禁止 `*-slides.md` / Marp）
- **简单表** → `.csv`；多表/公式 → `.usheet.json`
- 用户丢来 OOXML → Stage 转换后改 `.udoc.json` / `.uslides.json` / `.usheet.json`

## 信封

所有 Univer IR 文件：

```json
{ "danmo": { "format": "univer-sheet|univer-doc|univer-slides", "version": 1 }, "snapshot": { } }
```

`format` 必须与扩展名一致。
