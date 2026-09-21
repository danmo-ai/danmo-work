# Outline

## Hierarchy

1. **总纲** — 核心纲、读者承诺、强设定指针、全书结构钩/双线、分卷结构表、主线伏笔（计划层）、结局方向（终局储备 unlock **只在** `book-bible.md`，总纲链过去即可）；模板 `book-outline.md` 文首有**锁纲 checklist**
2. **卷纲** — 卷目标（含结构钩/双线）、冲突与起终、终局边界、节奏锚点、**单元索引**（unit_id / 章范围 / 一句话功能 / 本单元禁碰 / 钩子类型）、情绪/人物弧、反转、伏笔；模板 `volume-outline.md` 文首有**锁卷 checklist**
3. **单元细纲** — 下一技能 `novel-write`（`unit-outline.md`）；YAML `outline/units/vNN-U#.yaml` 是单元级**唯一**合同

卷纲只写到**单元索引**为止。禁止在卷纲写 desire/obstacle/choice/payoff/pleasure/forbidden/scenes、场面、单章任务、爽点文案——那些只进 yaml。

剧情单元在卷纲用 **索引行**（模板 `volume-outline.md`「单元索引」表）：必填 unit_id、章范围、一句话功能、本单元终局边界（禁碰）、下一单元钩子类型。单元节拍与欲望/阻碍/选择/兑现等合同字段只写在 `outline/units/vNN-U#.yaml`。缺索引行或章范围对不上 → **不要进入单元细纲**。

## Rules

- Plot branches / what-if docs **must not** mutate Canon until the user picks one.
- Use the templates: `assets/templates/book-outline.md`, `assets/templates/volume-outline.md` — fixed sections, no free-form reinvention.
- 总纲批准前勾 **锁纲 checklist**；卷纲批准前勾 **锁卷 checklist**。全书结构钩/双线在总纲；本卷结构钩/双线在卷纲。
- **规划「情绪/人物弧」前，先读本卷点名人物卡**（四件套/矛盾/弧光/关系表）——卷纲的人物走向必须与卡的弧光、关系现状兼容；卷纲批准时一并核对。关键选择等单元级字段在写 yaml 细纲时再对齐人物卡。
- `search_kb` **至多一次**「节奏与结构」before locking volume shape. 终局细节只写 `canon/author-lore.md`；unlock 卷号只维护在圣经；锁词只在 `canon/locked-terms.yaml`。
- After user OK on volume outline, update `novel-state.yaml` (`stage: outline`, artifacts).
- Do not write unit prose until asset_gate: core cast + world skeleton are `canon`.
- **Next stage:** `novel-write` 单元细纲（从索引行下推），然后一份 `units/vNN-U#.md`。
- 场面与章切口只进单元细纲。卷纲不写。

## 总纲 / 卷纲防崩检查

| 检查 | 通过标准 | 模板落点 |
|------|----------|----------|
| 核心纲 | 全书主线一句话；读者承诺与题材切口一致 | 总纲「核心纲」+ 锁纲 checklist |
| 强设定 | `world` 四层能持续生冲突；非背景板 | 总纲「强设定」指针 → `canon/world.md` |
| 主线立住 | 每卷一个升级目标 + 一次主高潮；单元索引因果链不断 | 分卷表 + 卷目标 + 单元索引 |
| 双线（有则填） | 全书弧在总纲；本卷推进+交织在卷纲 | 总纲「双线」→ 卷纲「双线」 |
| 结构钩 | 全书长线在总纲；本卷长/短在卷纲；短线单元下钩；设了必兑 | 总纲「结构钩」→ 卷纲「结构钩」→ yaml `next_hook` |
| 体量 | 分卷表能覆盖 Length target（新手宜按 ≥30 万字练控场估算） | 总纲分卷表 + 体量备注 |

缺单元索引、章范围缺口/重叠 → **不要进入单元细纲**。

## Outputs

- `novel/<book-id>/outline/book_outline.md` — 结构/卷地图（不复制终局储备表）
- `outline/volumes/vXX.md` — 单元索引；单元细纲 `unit_id` = `vXX-U#`
- 单元细纲：交给 `novel-write`（`outline/units/vXX-U#.yaml`）
