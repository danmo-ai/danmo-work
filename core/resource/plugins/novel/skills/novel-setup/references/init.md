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

If platform = 番茄/免费网文, also `write` `canon/writing-rules.md` seeding the defaults from KB 题材与平台（章字数 2000–3500、断章必钩、3–5 章一爽点、黄金三章闭环）so later stages inherit them without re-searching.

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
2. `search_kb` **一次**：题材与平台（knowledge_gate）。刑侦/探案/社会派 → `novel-state.craft_lane: crime-human`（查案向可 `qc_profile: mystery`），并把 `novel-write/assets/templates/style-fingerprint-crime.md` **全部条目**写入 `canon/style-fingerprint.md`「禁语」（或 bible Style card）；**不要**另查人味篇或文风篇。灵异/怪谈/悬疑爽不要标人味车道。  
3. Create the **standard English tree** under `novel/<book-id>/`:  
   `canon/` (+ `cast/`), `outline/` (+ `volumes/`), `chapters/`, `continuity/`, `reviews/`.  
4. `write` `book-bible.md`（含终局储备表）and `novel-state.yaml` (stage=`init`). All new text via `write`/`edit`/`apply_patch` — **UTF-8 only**.  
5. Seed `canon/world.md` + `canon/author-lore.md` from templates。人物卡用 `cast-card.md`，先 `candidate`。金手指默认写入主角卡。术语稀少时写在 `world.md`.  
6. Seed `continuity/ledger.md`（可空表，不可缺文件）.  
7. `memory_update` project: promise, genre, taboos, 终局储备卷号（不要把 author-lore 细节写入 memory）.  
8. Gate 脚本 `--action doctor` once; fix blocking layout holes. Stop for human confirmation before mass outlining if bible is still thin.

**默认不做：** `table_*` 镜像。

## Done when

- Standard layout exists on disk (see `project-layout.md`)  
- `novel-state.yaml` points at next action  
- Bible has framing + promise + 终局储备表（可待定，不可缺表）  
- `canon/world.md` 四层骨架已落盘  
- `canon/author-lore.md` + `continuity/ledger.md` 已落盘  
- Gate 脚本 doctor 无 blocking layout holes
- 开书筹备 checklist 五项已有答案（可写在 bible 备注）

## Legacy migrate

If cold-start finds `public-lore.md` / `tracking.md` / `chapter_summaries.md` without `ledger.md`, merge into `continuity/ledger.md` then move old files to `_archive/`.

Gate also one-shot renames `chapters/chNNN-contract.yaml` → `chNNN-outline.yaml` (any gate action; idempotent). Do not keep dual filenames.
