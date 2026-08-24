---
name: novel-write
source: builtin
description: Chapter loop for a planned novel. Use for 章合同, batch freeze, drafting or continuing chapters, preflight, scene routing, and Frozen_Canon continuation. Not for book-level outline or final review/Commit.
license: MIT
compatibility: Requires write, edit, read_file, grep, glob; Core table_*, memory_*, search_kb; ask_user
metadata:
  author: danmo-work
  version: "2.0"
  category: creative-writing
---

# Novel Write（章循环）

Contract → draft only. Review and Commit belong to `novel-review`.

## When to load

写第 N 章 / 章合同 / 批次冻结 / 续写 / 接手 / 写前预检.

## Do

| Intent | Load | search_kb |
|--------|------|-----------|
| 章合同 | `novel-write/references/chapter-contract.md` | 节奏与结构, 爽点与追读, 题材与平台, 强约束 |
| 批次冻结 | `novel-write/references/batch-freeze.md` | 节奏与结构, 爽点与追读 |
| 写正文 | `preflight.md` + `chapter-write.md` + `scene-routing.md` | 文风与去 AI 味, 爽点与追读, 情绪与场景 |
| 开篇 ch1–3 | 上栏 + `opening-chapters.md` | 节奏与结构, 题材与平台 |
| 续写接手 | `novel-write/references/continuation.md` | 文风与去 AI 味, 情绪与场景 |
| 写前预检 | `novel-write/references/preflight.md` | — |

Templates: `chapter-contract.yaml`, `batch-freeze.yaml`, `style-fingerprint.md`. Set `novel-state.yaml` `stage: writing`.

## Hard stops

- No prose without an accepted contract and loaded `canon/cast` (`asset_gate`).
- Frozen_Canon unconfirmed → no prose (`continuation.md` CP1–CP3).
- Batch write >1 chapter without freeze → `ask_user` or `batch-freeze.md`.
- `candidate` stays out of prose.

## Stop

Draft file on disk. Do not claim 定稿; hand off to `novel-review`.
