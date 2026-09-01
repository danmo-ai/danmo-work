---
name: novel-review
source: builtin
description: Review, deslop, and Continuity Commit for drafted chapters. Use when 审稿, 去AI味, batch review, or committing continuity. Not for opening a book or writing first drafts.
license: MIT
compatibility: Requires write, edit, read_file, grep, exec_shell; Core table_*, memory_*, search_kb; ask_user
metadata:
  author: danmo-work
  version: "2.3"
  category: creative-writing
---

# Novel Review（审稿 · 润色 · Commit）

One review → gate precommit → fix P0 → Commit (gate postcommit). `qc_gate` FAIL blocks 定稿.

**Pipeline steps 7–8/8.** 本技能不换模型。

## When to load

审稿 / 去 AI 味 / Continuity Commit / 批量审稿 / 审→润→定.

## Do

| Intent | Load | search_kb（≤1） |
|--------|------|-----------------|
| 审稿 / 批量审 | `review-gates.md` | 文风与去 AI 味 |
| 去 AI 味 | `polish-deslop.md` | 文风与去 AI 味 |
| Commit | `continuity-commit.md` | — |

**PASS：不写 `reviews/` 文件**，只更新 `gates.qc`。**FAIL / 深审：写全文六镜。** Commit = 一次 patch（ledger 五要素摘要 + Cast snapshot + Open loops + 合同 `reviewed` + state）+ `postcommit` exit 0.

## Stop

Completion = tool evidence. Do not start the next book.
