---
name: novel-review
source: builtin
description: Review, deslop, and Continuity Commit for drafted chapters. Use when 审稿, 去AI味, batch review, 卷收束, or committing continuity. Not for opening a book or writing first drafts.
license: MIT
compatibility: Requires write, edit, read_file, grep, exec_shell; Core table_*, memory_*, search_kb; ask_user
metadata:
  author: danmo-work
  version: "2.4"
  category: creative-writing
---

# Novel Review（审稿 · 润色 · Commit）

One review → gate precommit → fix P0 → Commit (gate postcommit). `qc_gate` FAIL blocks 定稿.

**Pipeline steps 7–8/8.** 本技能不换模型。

## When to load

审稿 / 去 AI 味（含量化硬检）/ Continuity Commit / 批量审稿 / 卷收束 / 审→润→定.

## Do

| Intent | Load | search_kb（≤1） |
|--------|------|-----------------|
| 审稿 / 批量审 | `review-gates.md`（10 维加权评分门） | 文风与去 AI 味 |
| 去 AI 味 | `polish-deslop.md`（先跑 scan-deslop 拿 COUNTS） | 文风与去 AI 味 |
| Commit | `continuity-commit.md` | — |
| 卷收束 | `continuity-commit.md` 卷收束节 + `review-gates.md` Assembly Checklist | — |

**PASS：不写 `reviews/` 文件**，只更新 `gates.qc`。**FAIL / 深审：写全文六镜 + 10 维评分。** Commit = 一次 patch（ledger 五要素摘要 + Cast snapshot + Open loops + 合同 `reviewed` + state）+ `postcommit` exit 0.
反 AI 量化硬检（破折号/英文泄漏/禁词/比喻）exit 0 才可宣称定稿；审稿报告引用 gate `### COUNTS` 四计数。

## Stop

Completion = tool evidence. Do not start the next book.
