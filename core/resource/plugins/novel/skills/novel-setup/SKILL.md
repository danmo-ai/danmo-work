---
name: novel-setup
source: builtin
description: Open a new long-form / webnovel book . Use when 立项, scaffolding novel/<book-id>/ (gate --action init builds the tree), writing book-bible and novel-state (genre / subgenre / qc_profile), or migrating an old book (--action migrate). Not for cast cards, outlining, drafting, or review.
license: MIT
compatibility: Requires write, edit, read_file, glob, exec_shell; Core table_*, memory_*, search_kb; ask_user
metadata:
  author: danmo-work
  version: "4.0"
  category: creative-writing
---

# Novel Setup（立项开书）

**Stage 1/5.** 脚本建树，模型填圣经。**Stop when bible, state and world exist.** 本技能不换模型。

## When to load

开书 / 立项 / 想写一本 / 接手旧书结构迁移. Vague premise → `brainstorming` + **one packed** `ask_user`.

## Do

1. `read_skill` `novel-setup/references/init.md`（按需 `project-layout.md`）；过一遍「开书筹备 checklist」，一次 `ask_user` 收齐书名 / slug / 题材 / 读者承诺 / 体量 / POV / 禁忌。
2. `search_kb` **至多一次**，查题材标题，不查「题材与平台」：玄幻、仙侠（含武侠）、都市、悬疑（含推理/灵异/刑侦/探案）、现代言情、古代言情、仕途扫黑、系统穿越（含脑洞/快穿）。对不上才查「题材与平台」。命中的标题就是 `novel-state.genre`。刑侦、探案、社会派在「悬疑」里命中 → `subgenre: 刑侦探案`。灵异、怪谈、悬疑爽用各自的悬疑子类，不写刑侦探案。
3. `exec_shell` gate `--action init --book-id <slug> --title <书名> --genre <题材>`（`references/gate.md`）。脚本一次建 `canon/cast`、`outline/volumes`、`outline/units`、`units`、`continuity/summaries`、`continuity/commits`、`reviews`，并拷 state / bible / world / author-lore / locked-terms / facts / style-fingerprint / book-outline 模板。已存在文件不覆盖。
4. 模型只填（`write` / `edit` / `apply_patch`，UTF-8）：
   - `book-bible.md`：读者承诺、Style card、终局储备 unlock 表（只写卷号不写真相）
   - `novel-state.yaml`：`genre`、`subgenre`（「题材与平台」子类闭集里同一 genre 的一行）、`qc_profile`、`stage: setup`
   - `canon/world.md` 四层骨架、`canon/author-lore.md` 终局细节
   - `canon/locked-terms.yaml`：author-lore「最早解锁卷」之前不得出现的词 → `locked_until`；平台红线 → `compliance`
   - `subgenre=刑侦探案` 时把 `novel-write/assets/templates/style-fingerprint-crime.md` 全部条目写入 `canon/style-fingerprint.md`「禁语」
5. **人物卡不在本技能写**（→ `novel-plan` 规划轮一起出）。
6. Gate `--action doctor`。Non-UTF-8 text → BLOCK；先跑仓库 `scripts/migrate_novel_encoding.py`。doctor ADVISORY `[migrate]`（旧书：facts 有 `## chNNN` / 细纲无 `on_stage` / 卡无 `role` / state 无 `genre`）→ `--action migrate`，核对 `continuity/commits/migrate-<date>.md`，人工确认 `genre` 后清 blocker。
7. 提示存稿习惯（非强制）：连载前尽量备一个单元的缓冲。

## Stop

Report paths + gate VERDICT. Next: 规划轮（人物 + 总纲 + 卷纲）→ `novel-plan`.
