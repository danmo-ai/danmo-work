---
name: novel-setup
source: builtin
description: Open a new long-form / webnovel book . Use when 立项, scaffolding novel/<book-id>/, writing book-bible and novel-state, seeding candidate cast, or creating locked-terms.yaml. Not for outlining volumes, drafting chapters, or review.
license: MIT
compatibility: Requires write, edit, read_file, glob, exec_shell; Core table_*, memory_*, search_kb; ask_user
metadata:
  author: danmo-work
  version: "3.0"
  category: creative-writing
---

# Novel Setup（立项开书）

Scaffold one book. **Stop when the tree and bible exist.**

**Pipeline step 1/8.** 本技能不换模型。

## When to load

开书 / 立项 / 想写一本. Vague premise → `brainstorming` + **one packed** `ask_user`.

## Do

1. `read_skill` `novel-setup/references/init.md`（按需 `project-layout.md`）；过一遍「开书筹备 checklist」。
2. `search_kb` **至多一次**，查类型标题，不查「题材与平台」：玄幻、仙侠（含武侠）、都市、悬疑（含推理/灵异/刑侦/探案）、现代言情、古代言情、仕途扫黑、系统穿越（含脑洞/快穿）。对不上才查「题材与平台」。刑侦/探案/社会派在「悬疑」里命中 → `craft_lane: crime-human`；灵异/怪谈/悬疑爽**不要**标人味车道。
3. Create `novel/<book-id>/`；`write`：
   - `book-bible.md`（含终局储备 unlock 表，只写卷号不写真相）
   - `novel-state.yaml`
   - `canon/world.md`、`canon/author-lore.md`
   - **`canon/locked-terms.yaml`**（把 author-lore 里"最早解锁卷"之前不得出现的词填进 `locked_until`；平台合规红线填 `compliance`）
   - **`continuity/facts.md`**（原 ledger.md，只存读者事实+cursor+loops+当前卷章摘要）
   - **`continuity/commits/`**（空目录；每单元一个执行日志文件）
   - **`continuity/summaries/`**（空目录；卷收束归档用）
   - `craft_lane=crime-human` 时把 `style-fingerprint-crime.md` 全部条目写入指纹「禁语」。
4. Cast 起 `candidate`（`cast-card.md`）。卡首两行 `` `status` `` / `` `role` `` 用键值格式；`role` 三选一：`protagonist` / `volume_antagonist` / `recurring`。完整度按 role：主角与卷对手填满四件套、矛盾弧光、知识边界、三锚点、语言习惯、三条台词；`recurring` 只填欲望、相交点、功能六型选一、一条锚、口头禅、一条台词。金手指段写在主角卡。关系表只写质态+节点，对方写 stem。龙套不建卡。promote 由 `accept-volume` 在卷纲批准时批量执行。
5. Templates: bible / state / world / cast-card / author-lore / facts / locked-terms only.
6. Gate `--action doctor`. Legacy 无 facts → merge 后 archive。Non-UTF-8 text → BLOCK；先跑仓库 `scripts/migrate_novel_encoding.py`，再用 `write`/`edit`/`apply_patch`。
7. 提示存稿习惯（非强制）：连载前尽量备一个单元的缓冲。

## Stop

Report paths. Next: 设定/总纲 → `novel-plan`.
