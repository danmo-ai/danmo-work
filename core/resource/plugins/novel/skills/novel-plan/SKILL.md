---
name: novel-plan
source: builtin
description: Plan a novel already scaffolded . Use for 人设/世界观, 金手指, book/volume outlines. Volume outline is a slim unit-index; unit-level contract lives in YAML. Not for 单元细纲, unit prose, or review.
license: MIT
compatibility: Requires write, edit, read_file, glob; Core table_*, memory_*, search_kb; ask_user
metadata:
  author: danmo-work
  version: "3.0"
  category: creative-writing
---

# Novel Plan（设定 · 总纲 · 卷纲）

Lock canon and outlines. **No 单元细纲, no unit prose.**

**Pipeline steps 2–4/8.** 本技能不换模型。

## 分层硬规则

- **总纲** `outline/book_outline.md`：全书地图 + 分卷表 + 伏笔**计划**清单（FS-id/内容/埋点卷/计划回收卷）。运行时状态不写这里。
- **卷纲** `outline/volumes/vNN.md`：卷级判断 + **单元索引表**（unit_id / 章范围 / 一句话功能 / 本单元禁碰 / 下一单元钩子类型）。
- **卷纲禁止**写 desire/obstacle/choice/payoff/pleasure/forbidden/scenes/场面/章切口——那些全部在 `outline/units/vNN-U#.yaml`。
- **伏笔运行时状态**（planted/advanced/paid、实际埋点章、实际回收章）由 `continuity/facts.md` 的 Open loops 表维护。
- **锁词**登记在 `canon/locked-terms.yaml`，不写进卷纲或总纲。

## When to load

人设 / 世界观 / 金手指 / 总纲 / 卷纲 / 补设定.

## Do

| Intent | Load | search_kb（≤1） |
|--------|------|-----------------|
| 总纲 | `novel-plan/references/outline.md` + 模板 `book-outline.md`（锁纲 checklist） | 节奏与结构 |
| 卷纲 | `novel-plan/references/outline.md` + 模板 `volume-outline.md`（锁卷 checklist / **单元索引表**） | 节奏与结构 |
| 人设 / 世界观 | templates `world.md` + `cast-card.md`（关系表只写质态+节点） | 人设与群像（含取名反 AI）；仅 `craft_lane=crime-human` 且写反派来路/配角执念时改查人味 |
| 金手指 | `cast-card.md` 金手指段 | 世界观与金手指 |

新实体先 `candidate`。**卷纲批准时**把本卷点名人物一并 `canon`（一次确认，不逐卡 ask）。终局细节 → `author-lore.md`；unlock 表只在 `book-bible.md`；新解锁的词同步从 `locked-terms.yaml` 移除。默认不做 `table_*`。

## Stop

Wait for volume outline confirmation. 单元细纲 / 正文 → `novel-write`.
