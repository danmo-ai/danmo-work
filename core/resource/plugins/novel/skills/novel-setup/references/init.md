# Init (立项)

## Sufficiency gate

Do **not** create a half-baked project. Before seeding files/tables, collect via `ask_user` (one question at a time when needed):

1. Working title / book-id slug  
2. Genre + channel (网文长篇 / 短篇 / 文学向等)  
3. Reader promise (看什么爽/虐/悬)  
4. Length target (短篇 / 长篇量级)  
5. POV + tense  
6. Hard taboos / rating constraints  

Optional: platform tone (番茄/起点等) as **preference**, not hard rules.

If platform = 番茄/免费网文, also `write` `canon/writing-rules.md` seeding the defaults from KB 题材与平台（章字数 2000–3500、断章必钩、3–5 章一爽点、黄金三章闭环）so later stages inherit them without re-searching.

## Steps

1. `read_skill` → `project-layout.md`, `table-schema.md`, templates.  
2. `search_kb` 题材与平台 + 人设与群像（knowledge_gate）.  
3. Create the **standard English tree** under `novel/<book-id>/`:  
   `canon/` (+ `cast/`), `outline/` (+ `volumes/`), `chapters/`, `continuity/`, `reviews/`  
   (optional later: `extras/`, `_archive/`).  
4. `write` `book-bible.md`（含终局储备三行，未定可写「待定」+ 卷号占位）and `novel-state.yaml` (stage=`init`, status fields).  
5. Seed `canon/world.md` + `canon/glossary.md` from templates（骨架即可）。人物卡用 `cast-card.md`，先 `candidate`。金手指用 `goldfinger-card.md` → `canon/goldfinger.md` 或主角卡。  
6. Seed tables with `book_id` (characters may start as `candidate`).  
7. `memory_update` project: promise, genre, taboos, 终局储备; user: language/style prefs if stated.  
8. Stop for human confirmation before mass outlining if bible is still thin.

## Done when

- Standard layout exists on disk (see `project-layout.md`)  
- `novel-state.yaml` points at next action  
- Bible has framing + promise + 终局储备表（可待定，不可缺表）  
- `canon/world.md` 四层骨架已落盘  
- Tables seeded (even if empty cast with schema understood)
