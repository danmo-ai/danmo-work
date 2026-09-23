# Init (立项)

## Sufficiency gate

Do **not** create a half-baked project. Before seeding files, collect via **one** `ask_user` covering:

1. Working title / book-id slug  
2. Genre + channel (网文长篇 / 短篇 / 文学向等)  
3. Reader promise (看什么爽/虐/悬)  
4. Length target (短篇 / 长篇量级)  
5. POV + tense  
6. Hard taboos / rating constraints  

Optional in the same ask: platform tone (番茄/起点等) as **preference**, not hard rules.

If platform = 番茄/免费网文, also `write` `canon/writing-rules.md` seeding the unit defaults（章配额 2000–3500、单元正文一份文件以保住章间衔接、单元主爽点、黄金三章闭环见「节奏与结构」）. 这些不占第二次 `search_kb`。类型差异只来自下面那一次检索。

## 开书筹备 checklist（写入 bible 前自检）

与 sufficiency 一并想清；不必另开一轮 ask，可挤进同一 `ask_user` 或作者自答：

| 项 | 问什么 | 落盘 |
|----|--------|------|
| 题材切口 | 第一场冲突从哪进？擅长标签是什么（非纯追热点） | bible 读者承诺 + genre |
| 主角目标 | 开局欲望是否可观察、能否随剧情演变 | 主角卡欲望/伤口 |
| 关系冲突 | 主对立与拉扯是否够撑长篇（人物 > 噱头） | cast 关系 / 卷对手气味 |
| 体量 | 框架能否撑约 **30 万字+**（新手练控场量级） | Length target；总纲分卷表 |
| 核心纲+强设定 | 主线一句说清；设定能否持续生矛盾 | bible + `world.md` 四层骨架 |

习惯建议（非强制）：日更前尽量 **存稿 3–5 章**；前几本以完本练控场为主，允许扑街换书。

## Steps（少交互）

1. `read_skill` → `project-layout.md`（按需再开 `table-schema.md` / templates）.  
2. `search_kb` **一次**（knowledge_gate），查题材标题：玄幻、仙侠（含武侠）、都市、悬疑（含推理/灵异/刑侦/探案）、现代言情、古代言情、仕途扫黑、系统穿越（含脑洞/快穿）。题材对不上这八个，才查「题材与平台」。命中的标题就是 `novel-state.genre`。刑侦/探案/社会派 → 查的是「悬疑」，命中后写 `craft_lane: crime-human`（查案向可 `qc_profile: mystery`），并把 `novel-write/assets/templates/style-fingerprint-crime.md` **全部条目**写入 `canon/style-fingerprint.md`「禁语」；**不要**另查人味篇或文风篇。灵异/怪谈/悬疑爽不要标人味车道。  
3. `exec_shell` gate `--action init --book-id <slug> --title <书名> --genre <题材>`：脚本建 `canon/`(+`cast/`)、`outline/`(+`volumes/`+`units/`)、`units/`、`continuity/`(+`summaries/`+`commits/`)、`reviews/`，拷 state / bible / world / author-lore / locked-terms / facts / style-fingerprint / book-outline 模板并写好 `book_id` / `title` / `genre` / `stage: setup`。已存在文件不覆盖。Do not create `chapters/`.  
4. 模型填 `book-bible.md`（读者承诺 / Style card / 终局储备表）与 `novel-state.yaml` 的 `qc_profile` / `craft_lane`（`genre` 已由 init 写入）。All text via `write`/`edit`/`apply_patch` — **UTF-8 only**.  
5. 填 `canon/world.md` 四层骨架 + `canon/author-lore.md` 终局细节。术语稀少时写在 `world.md`。**人物卡留给 `novel-plan` 规划轮**（金手指写主角卡）。  
6. 填 `canon/locked-terms.yaml`（可先空 `locked_until` / `compliance`，不可缺文件）.  
7. `continuity/facts.md` 由 init 落盘（可空表）；章摘要将写 `continuity/summaries/vNN.md`，不写 facts。**不要**再以 `ledger.md` 作为新书种子（旧书兼容见 Legacy）.  
8. `memory_update` project: promise, genre, taboos, 终局储备卷号（不要把 author-lore 细节写入 memory）.  
9. Gate `--action doctor` once; fix blocking layout holes. Stop for human confirmation before planning if bible is still thin.

**默认不做：** `table_*` 镜像。

## Done when

- Standard layout exists on disk (see `project-layout.md`)  
- `novel-state.yaml` points at next action  
- Bible has framing + promise + 终局储备表（可待定，不可缺表）  
- `canon/world.md` 四层骨架已落盘  
- `canon/author-lore.md` + `canon/locked-terms.yaml` + `continuity/facts.md` + `continuity/summaries/` + `continuity/commits/` 已落盘
- `novel-state.yaml` 有 `genre`（八题材之一）、`qc_profile`、`craft_lane`  
- Gate 脚本 doctor 无 blocking layout holes
- 开书筹备 checklist 五项已有答案（可写在 bible 备注）

## Legacy migrate

旧书两类迁移：

1. **账本合并**：cold-start finds `ledger.md` / `public-lore.md` / `tracking.md` / `chapter_summaries.md` without `facts.md` → split/merge into `continuity/facts.md`（读者事实）+ `continuity/commits/`（执行日志）then move old files to `_archive/`.
2. **结构迁移**（脚本）：`doctor` ADVISORY `[migrate]` → `exec_shell` gate `--action migrate --book-id <slug>`。它把 facts 里的 `## chNNN` 按章归卷写 `continuity/summaries/vNN.md`（facts 留索引行）、state 缺 `genre` 时猜一次并加 blocker「确认 genre」、细纲补 `on_stage`（从 `state_deltas` 预填）/ `pov`（留空 warning）、人物卡补 `role`、卷纲补「本卷人物」。改动清单在 `continuity/commits/migrate-<date>.md`；人工核对后清 blocker。
