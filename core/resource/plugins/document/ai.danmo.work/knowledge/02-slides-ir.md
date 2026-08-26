# 幻灯片 IR（`.uslides.json`）

主题关键词：幻灯片 IR、uslides、ISlideData、playable-slides。

## 工具流程

1. `read_file` 目标路径（或 `write` 新建）。
2. 改 `snapshot.body.pages.*.pageElements.*.richText.text`。
3. `edit` / `apply_patch` 写回。禁止联网查 Univer。

## 要点

- `danmo.format` = `"univer-slides"`
- 画布默认 `pageSize` 960×540
- 文本框元素 `type` = **2**（TEXT）；不要用 `0`（SHAPE）当标题
- `pageBackgroundFill.rgb` 用 `"rgb(255,255,255)"` 这类字符串
- `body.pageOrder` 与 `body.pages` 的 key 必须一致

完整骨架见技能 `playable-slides` → `references/ir-slides.md`（`read_skill`）。
