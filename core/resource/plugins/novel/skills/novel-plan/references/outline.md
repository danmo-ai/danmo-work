# Outline

## Hierarchy

1. **总纲** — 核心纲、读者承诺、强设定指针、全书结构钩/双线、分卷结构表、主线伏笔（计划层）、结局方向（终局储备 unlock **只在** `book-bible.md`，总纲链过去即可）；模板 `book-outline.md` 文首有**锁纲 checklist**
2. **卷纲** — 卷目标（含结构钩/双线）、冲突与起终、终局边界、节奏锚点、**单元索引**（unit_id / 章范围 / 一句话功能 / 本单元禁碰 / 钩子类型）、情绪/人物弧、反转、伏笔；模板 `volume-outline.md` 文首有**锁卷 checklist**
3. **单元细纲** — 下一技能 `novel-write`（`unit-outline.md`）；YAML `outline/units/vNN-U#.yaml` 是单元级**唯一**合同

卷纲只写到**单元索引**为止。禁止在卷纲写 desire/obstacle/choice/payoff/pleasure/forbidden/scenes、场面、单章任务、爽点文案——那些只进 yaml。

剧情单元在卷纲用 **索引行**（模板 `volume-outline.md`「单元索引」表）：必填 unit_id、章范围、一句话功能、本单元终局边界（禁碰）、下一单元钩子类型。单元节拍与欲望/阻碍/选择/兑现等合同字段只写在 `outline/units/vNN-U#.yaml`。缺索引行或章范围对不上 → **不要进入单元细纲**。

## 怎么切单元

单元是生产原子：一次矛盾，从起到收。本单元结束时，读者能指出主角得到或失去了什么。说不清，这一刀还没切对。

### 建议规模

| 维度 | 建议 | 硬限 |
|------|------|------|
| 单元章数 | 3–8 章 | ≤10 章（gate FAIL）；≥2 章 |
| 单章字数 | 2000–3500 | 地板 2000（precommit 硬检） |
| 短单元 2–3 章 | 高密度兑现、卷末收束、换地图过渡、时限很紧的危机 | <2 章不成立；过渡须在细纲标 `unit_type: transition` |
| 长单元 6–8 章 | 主线大案、感情摊牌、升级台阶（立住 + 兑现 + 余波） | >5 章至少两个爽点（一小一大） |

数字细则与字数地板见 `novel-write/references/unit-scale.md`。规划只负责把章范围切进上表。

### 划分顺序

1. 先定本卷高潮（最大冲突、最强情绪），再往前排单元。
2. 一刀切在「一次矛盾从起到收」。相邻单元换冲突形态；上一单元的钩子类型接下一单元的入口。
3. 切完再数章。落在 3–8 最好。超过 10 章拆成两个单元，中间留一拍兑现或余波。不到 3 章时，先并进相邻单元的余波或过渡，再考虑单独成行。
4. 卷纲索引填章范围。gate 校验：连续、无缺口、无重叠、盖住本卷全部章、单单元 ≤10 章。

按章数凑、按地图或年代硬切、按「开端 / 对抗 / 结局」各切一刀，都会切错。发展段往往占一卷的三到四成，按三幕各切一单元会把中段胀过 10 章。

### 幕

幕是卷的形状，不是单元的刀口。卷纲「节奏锚点」（开篇、发展、中点、高潮、收束）就是这一卷的幕：标章位，用来核对。一个锚点可以跨多个单元。高潮和中点必须落在某个单元的决断或兑现上。

不要在卷和单元之间再加「幕」索引或幕文件。单元已经是这一层的生产单位。

### 章内场景

规划不写场景。卷纲停在单元索引。

章内那一层在细纲里叫**场面**（`outline/units/*.yaml` 的 `scenes`）：同一段连续时空里的一次转折（谁要什么、发生了什么转向），一条场面不是一章，每章至少 2 场。章只记切口（从哪场到哪场、章末钩、字数份额）。不另建场景表、场景文件或章纲。

正文里换场用空行。「场景沉浸」是写正文时按场面需要才查的技法，不是规划产物。

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
