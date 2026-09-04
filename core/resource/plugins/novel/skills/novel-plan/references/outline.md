# Outline

## Hierarchy

1. **总纲** — 一句话故事, 读者承诺, 分卷结构表, 主线伏笔, 结局方向（终局储备 unlock **只在** `book-bible.md`，总纲链过去即可）
2. **卷纲** — 卷目标, 冲突与起终, 终局边界, 节奏锚点, **剧情单元卡（一段章）**, 情绪/人物弧, 反转, 伏笔
3. **章合同** — 下一技能 `novel-write`（`chapter-contract.md`）；YAML under `chapters/`；必填 `unit_id`

卷纲写到「一段章」的剧情单元为止。禁止在 `outline/` 写单章任务/爽点/钩子文案。

剧情单元用 **单元卡**（模板 `volume-outline.md`）：必填单元ID、章范围、**单元节拍（章功能分配）**、单元功能、主角局部目标、因果入口、核心阻碍、关键选择、主爽点形态、兑现归属、禁止提前释放、下一单元钩子、终局边界。节拍须覆盖该单元章范围（建立期待→尝试→加压→决断→兑现→余波，可按题材删并）。缺卡、关键字段空、或节拍未覆盖章范围 → **不要进入章合同**：合同的 `unit_id` / `purpose` / `beats` / `pleasure_point` 必须能指回某个单元卡 + 锚点。

## Rules

- Plot branches / what-if docs **must not** mutate Canon until the user picks one.
- Use the templates: `assets/templates/book-outline.md`, `assets/templates/volume-outline.md` — fixed sections, no free-form reinvention.
- **规划「情绪/人物弧」与单元卡关键选择前，先读本卷点名人物卡**（四件套/矛盾/弧光/关系表）——卷纲的人物走向必须与卡的弧光、关系现状兼容；卷纲批准时一并核对。
- `search_kb` **至多一次**「节奏与结构」before locking volume shape. 终局细节只写 `canon/author-lore.md`；unlock 卷号只维护在圣经。
- After user OK on volume outline, update `novel-state.yaml` (`stage: outline`, artifacts).
- Do not batch-write chapters until asset_gate: core cast + world skeleton are `canon`.
- **Next stage:** `novel-write` 章合同；批量写正文前再走 `batch-freeze.md`（只更新 `novel-state.frozen_batch`）。
- Per-chapter planning belongs in **章合同** only. Never under `outline/`.

## Outputs

- `novel/<book-id>/outline/book_outline.md` — 结构/卷地图（不复制终局储备表）
- `outline/volumes/vXX.md` — 单元卡；合同 `unit_id` = `vXX-U#`
- 章合同：交给 `novel-write`（文件权威；table 镜像可选）
