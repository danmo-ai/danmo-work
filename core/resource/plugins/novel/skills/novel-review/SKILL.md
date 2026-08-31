---
name: novel-review
source: builtin
description: Review, deslop, and Continuity Commit for drafted chapters. Use when 审稿, 去AI味, batch review, or committing continuity. Not for opening a book or writing first drafts.
license: MIT
compatibility: Requires write, edit, read_file, grep, exec_shell; Core table_*, memory_*, search_kb; ask_user
metadata:
  author: danmo-work
  version: "2.2"
  category: creative-writing
---

# Novel Review（审稿 · 润色 · Commit）

One review round → gate script precommit → fix P0 → Commit (gate script postcommit). `qc_gate` FAIL blocks「定稿」.

**Pipeline steps 7–8/8：润色审稿 → 一致性 Commit.** 用户可自行切换旗舰模型；本技能不换模型。

## When to load

审稿 / 检查 / 去 AI 味 / 润色 / Continuity Commit / 批量审稿.

## Do（少交互）

| Intent | Load | search_kb（≤1） |
|--------|------|-----------------|
| 审稿 / 批量审 | `novel-review/references/review-gates.md` | 文风与去 AI 味 |
| 去 AI 味 | `polish-deslop.md` | 文风与去 AI 味 |
| Commit | `continuity-commit.md` | — |

Write reviews under `reviews/`: **PASS → 短 stub**；**FAIL → 全文六镜**. Commit：**一次** `apply_patch` 更新 `continuity/ledger.md` + contract status + `novel-state.yaml`. Table upserts optional.

## Stop

Report files. Completion = tool evidence. Do not start the next book.
