# Outline

## Hierarchy

1. **总纲** — 核心纲、读者承诺、强设定指针、全书结构钩/双线、分卷结构表、主线伏笔、结局方向（终局储备 unlock **只在** `book-bible.md`，总纲链过去即可）；模板 `book-outline.md` 文首有**锁纲 checklist**
2. **卷纲** — 卷目标（含结构钩/双线）、冲突与起终, 终局边界, 节奏锚点, **剧情单元卡（一段章）**, 情绪/人物弧, 反转, 伏笔；模板 `volume-outline.md` 文首有**锁卷 checklist**
3. **单元细纲** — 下一技能 `novel-write`（`unit-outline.md`）；YAML `outline/units/vNN-U#.yaml`

卷纲写到「一段章」的剧情单元为止。禁止在卷纲写场面、单章任务、爽点文案。

剧情单元用 **单元卡**（模板 `volume-outline.md`）：必填单元ID、章范围、**单元节拍**、单元功能、因果入口、主角局部目标（欲望）、核心阻碍、关键选择、主爽点形态、兑现归属、禁止提前释放、下一单元钩子（短线结构钩）、终局边界；可选 **推进线**（事业/感情/双）。节拍须覆盖该单元章范围（建立期待→尝试→加压→决断→兑现→余波，可按题材删并）。缺卡、关键字段空、或节拍未覆盖章范围 → **不要进入单元细纲**。

## Rules

- Plot branches / what-if docs **must not** mutate Canon until the user picks one.
- Use the templates: `assets/templates/book-outline.md`, `assets/templates/volume-outline.md` — fixed sections, no free-form reinvention.
- 总纲批准前勾 **锁纲 checklist**；卷纲批准前勾 **锁卷 checklist**。全书结构钩/双线在总纲；本卷结构钩/双线/单元推进线在卷纲。
- **规划「情绪/人物弧」与单元卡关键选择前，先读本卷点名人物卡**（四件套/矛盾/弧光/关系表）——卷纲的人物走向必须与卡的弧光、关系现状兼容；卷纲批准时一并核对。
- `search_kb` **至多一次**「节奏与结构」before locking volume shape. 终局细节只写 `canon/author-lore.md`；unlock 卷号只维护在圣经。
- After user OK on volume outline, update `novel-state.yaml` (`stage: outline`, artifacts).
- Do not write unit prose until asset_gate: core cast + world skeleton are `canon`.
- **Next stage:** `novel-write` 单元细纲，然后一份 `units/vNN-U#.md`。
- 场面与章切口只进单元细纲。卷纲不写。

## 总纲 / 卷纲防崩检查

| 检查 | 通过标准 | 模板落点 |
|------|----------|----------|
| 核心纲 | 全书主线一句话；读者承诺与题材切口一致 | 总纲「核心纲」+ 锁纲 checklist |
| 强设定 | `world` 四层能持续生冲突；非背景板 | 总纲「强设定」指针 → `canon/world.md` |
| 主线立住 | 每卷一个升级目标 + 一次主高潮；单元卡因果链不断 | 分卷表 + 卷目标 + 单元卡 |
| 双线（有则填） | 全书弧在总纲；本卷推进+交织在卷纲；单元可标推进线 | 总纲「双线」→ 卷纲「双线」→ 单元「推进线」 |
| 结构钩 | 全书长线在总纲；本卷长/短在卷纲；短线单元下钩；设了必兑 | 总纲「结构钩」→ 卷纲「结构钩」→ 单元「下一单元钩子」 |
| 体量 | 分卷表能覆盖 Length target（新手宜按 ≥30 万字练控场估算） | 总纲分卷表 + 体量备注 |

缺单元卡、关键字段空、或节拍未覆盖章范围 → **不要进入单元细纲**。

## Outputs

- `novel/<book-id>/outline/book_outline.md` — 结构/卷地图（不复制终局储备表）
- `outline/volumes/vXX.md` — 单元卡；单元细纲 `unit_id` = `vXX-U#`
- 单元细纲：交给 `novel-write`（`outline/units/vXX-U#.yaml`）
