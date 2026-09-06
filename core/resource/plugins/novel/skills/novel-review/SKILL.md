---
name: novel-review
source: builtin
description: Post-draft finalize for novels — 字数扩写, review, deslop, and Continuity Commit. Use after a drafted chapter when expanding thin prose, 审稿, 去AI味, batch review, 卷收束, or committing continuity. Not for opening a book or writing first drafts (those stay novel-write / better model).
license: MIT
compatibility: Requires write, edit, read_file, grep, exec_shell; Core table_*, memory_*, search_kb; ask_user
metadata:
  author: danmo-work
  version: "2.5"
  category: creative-writing
---

# Novel Review（扩写 · 审稿 · 润色 · Commit）

Post-draft lane（可与写作分模）：字数不足先扩写 → 一轮审稿 → gate precommit → 修 P0 → 可选去 AI 味 → Continuity Commit（gate postcommit）。`qc_gate` FAIL blocks 定稿.

**Pipeline steps 7–8/8**（扩写属定稿前修补，仍本技能）. 本技能不换模型。

## When to load

字数不足扩写 / 审稿 / 去 AI 味（含量化硬检）/ Continuity Commit / 批量审稿 / 卷收束 / 审→润→定 / 扩→审→润→定.

**不要**用本技能写首稿正文（首稿 → `novel-write`）。

## Do

| Intent | Load | search_kb（≤1） |
|--------|------|-----------------|
| 字数不足/扩写 | `expansion.md` | 扩写与字数控制 |
| 审稿 / 批量审 | `review-gates.md`（10 维加权评分门） | 文风与去 AI 味 |
| 去 AI 味 | `polish-deslop.md`（先跑 scan-deslop 拿 COUNTS） | 文风与去 AI 味 |
| Commit | `continuity-commit.md` | — |
| 卷收束 | `continuity-commit.md` 卷收束节 + `review-gates.md` Assembly Checklist | — |
| 定稿串行（扩→审→润→Commit） | 上表按需依次：先 `expansion.md`（仅字数/薄稿需要）→ `review-gates.md` → 可选 `polish-deslop.md` → `continuity-commit.md` | 按步各 ≤1 |

**PASS：不写 `reviews/` 文件**，只更新 `gates.qc`。**FAIL / 深审：写全文六镜 + 10 维评分。** Commit = 一次 patch（ledger 五要素摘要 + Cast snapshot + Open loops + 章纲 `reviewed` + state）+ `postcommit` exit 0.
反 AI 量化硬检（破折号/英文泄漏/禁词/比喻）exit 0 才可宣称定稿；审稿报告引用 gate `### COUNTS` 四计数。

## Stop

Completion = tool evidence. Do not start the next book’s first draft here.
