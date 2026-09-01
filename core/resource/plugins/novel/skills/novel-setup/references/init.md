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

## Steps（少交互）

1. `read_skill` → `project-layout.md`（按需再开 `table-schema.md` / templates）.  
2. `search_kb` **一次**：题材与平台（knowledge_gate）.  
3. Create the **standard English tree** under `novel/<book-id>/`:  
   `canon/` (+ `cast/`), `outline/` (+ `volumes/`), `chapters/`, `continuity/`, `reviews/`.  
4. `write` `book-bible.md`（含终局储备表）and `novel-state.yaml` (stage=`init`).  
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

## Legacy migrate

If cold-start finds `public-lore.md` / `tracking.md` / `chapter_summaries.md` without `ledger.md`, merge into `continuity/ledger.md` then move old files to `_archive/`.
