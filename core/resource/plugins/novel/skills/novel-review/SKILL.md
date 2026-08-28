---
name: novel-review
source: builtin
description: Review, deslop, and Continuity Commit for drafted chapters. Use when 审稿, 去AI味, batch review, or committing continuity. Not for opening a book or writing first drafts.
license: MIT
compatibility: Requires write, edit, read_file, grep, exec_shell; Core table_*, memory_*, search_kb; ask_user
metadata:
  author: danmo-work
  version: "2.1"
  category: creative-writing
---

# Novel Review（审改定稿）

One review round → gate script precommit → fix P0 → Commit (gate script postcommit). `qc_gate` FAIL blocks「定稿」.

## When to load

审稿 / 检查 / 去 AI 味 / 润色 / Continuity Commit / 批量审稿.

## Do

| Intent | Load | search_kb |
|--------|------|-----------|
| 审稿 / 批量审 | `novel-review/references/review-gates.md` | 文风与去 AI 味, 强约束, 题材与平台 |
| 去 AI 味 | `novel-review/references/polish-deslop.md` | 文风与去 AI 味 |
| Commit | `novel-review/references/continuity-commit.md` | 世界观与金手指（若涉及） |

Write reviews under `reviews/`. Commit updates chapter file + `table_*` + `memory_*` + `novel-state.yaml` + `continuity/public-lore.md` + `tracking.md`. Template: `foreshadow-tracker.md`.

## Stop

Report files and table keys. Completion = tool evidence. Do not start the next book.
