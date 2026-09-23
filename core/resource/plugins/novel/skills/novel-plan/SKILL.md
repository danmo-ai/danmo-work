---
name: novel-plan
source: builtin
description: One planning round for a scaffolded novel — cast cards, book outline and the next volume outline (单元索引 + 本卷人物) in one turn; after human approval run gate --action accept-volume to promote cast and seed unit YAML heads. Also cast-lint / 补人物卡 / 下一卷卷纲. Not for 单元细纲, unit prose, or review.
license: MIT
compatibility: Requires write, edit, read_file, glob, exec_shell; Core table_*, memory_*, search_kb; ask_user
metadata:
  author: danmo-work
  version: "4.0"
  category: creative-writing
---

# Novel Plan（规划一轮：人物 · 总纲 · 卷纲）

**Stage 2/5.** 一轮出：人物卡（`candidate`）+ `outline/book_outline.md` + 本卷 `outline/volumes/vNN.md`（含单元索引 + 本卷人物）。**人只在卷纲批准处停。** 批准后脚本 `accept-volume` 提升人物并种细纲头。**No 单元细纲, no unit prose.** 本技能不换模型。

## 分层硬规则

- **总纲** `outline/book_outline.md`：全书地图 + 分卷表 + 伏笔**计划**清单（FS-id/内容/埋点卷/计划回收卷）。运行时状态不写这里。
- **卷纲** `outline/volumes/vNN.md`：卷级判断 + **单元索引表**（unit_id / 章范围 / 一句话功能 / 终局边界短语 / 下一钩类型）+ **`## 本卷人物`**（stem 列表；批准即 canon）。卷纲分配，细纲落笔。
- **卷纲禁止**写 desire/obstacle/choice/payoff/pleasure/forbidden/scenes/场面/章切口——那些全部在 `outline/units/vNN-U#.yaml`。
- **人物卡** `canon/cast/<stem>.md`：文件名 stem 即角色 id，卷纲「本卷人物」、细纲 `on_stage`、关系表「对方」都用它。卡首 `` `status` `` / `` `role` `` 键值行。
- **伏笔运行时状态**由 `continuity/facts.md` Open loops 维护；**锁词**在 `canon/locked-terms.yaml`。

## When to load

规划一轮 / 人设 / 世界观 / 金手指 / 总纲 / 卷纲 / 下一卷卷纲 / 补人物卡 / cast-lint.

## Do

| Intent | Load | search_kb（≤1） | Script |
|--------|------|-----------------|--------|
| 规划一轮（默认） | `novel-plan/references/outline.md` + 模板 `book-outline.md` / `volume-outline.md` / `cast-card.md` | 节奏与结构 | 写完 `--action cast-lint`；批准后 `--action accept-volume --volume vNN` |
| 下一卷卷纲 | `outline.md` + `volume-outline.md`（锁卷 checklist / 单元索引 / 本卷人物） | 节奏与结构 | 批准后 `accept-volume --volume vNN` |
| 补人物卡 / 改关系 | `cast-card.md`（卡首 `role` 定完整度；关系表只写质态+节点，对方写 stem，**两边都要有行**） | 人设与群像（含取名反 AI）；仅 `craft_lane=crime-human` 且写反派来路/配角执念时改查「刑侦人味文风」 | `--action cast-lint` |
| 金手指 | `cast-card.md` 「金手指」段（主角卡） | 世界观与金手指 | — |

**规划一轮顺序：** 人物卡 → 总纲 → 本卷卷纲，三样同一 turn 写完；`cast-lint` exit 0（对边存在 / stem 存在 / `role` 合法）后再 `ask_user` 一次请人批准卷纲。批准 → `accept-volume`：本卷人物全部 `candidate → canon`，按索引每行种 `outline/units/vNN-U#.yaml` 头（`status: proposed`）。不要逐张卡改状态，不要手写细纲头。

**人物卡最小必填**：`protagonist` / `volume_antagonist` = 四件套 + 矛盾弧光 + 知识边界 + 三锚点 + 语言习惯 + 台词 3 条 + 退场（主角另加金手指）；`recurring` = 欲望 + 相交点 + 功能六型 1 + 视觉锚或行为锚 1 + 口头禅 + 台词 1 条 + 退场。龙套用工称不建卡。必须存在一张 `role: protagonist` 的卡，否则 asset 门不放正文。

新实体先 `candidate`。终局细节 → `author-lore.md`；unlock 表只在 `book-bible.md`；新解锁的词同步从 `locked-terms.yaml` 移除。默认不做 `table_*`。

## Stop

卷纲批准前停（human stop）。批准后跑 `accept-volume`，报 `### ACCEPTED`（canon 名单 + 种出的细纲头）。Next: 一批细纲 → `novel-write`.
