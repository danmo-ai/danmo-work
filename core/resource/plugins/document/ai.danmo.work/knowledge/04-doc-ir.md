# 文档：Markdown 与 `.udoc.json`

主题关键词：文档 IR、udoc、IDocumentData、document-writing、markdown。

## 默认

长文 **GFM `.md`**。不要为了“更像 Office”去新建 `.udoc.json`，除非：

- 用户路径已是 `.udoc.json`（`[office-edit]`），或
- 从 `.docx` 转换后的 IR 需要批改。

## `.udoc.json`

- `danmo.format` = `"univer-doc"`
- 正文主要在 `snapshot.body.dataStream`（含 `\r` 段结束与末尾 `\n`），并配合 `paragraphs` / `sectionBreaks` 索引
- 批改时优先小范围 `apply_patch`；结构乱时整文件 `write` 一个自洽 snapshot

`.docx` 不可直接当 SoT 写回。
