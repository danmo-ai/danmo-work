---
name: novel-write
source: builtin
description: Chapter loop for a planned novel. Use for 章合同, batch freeze, drafting or continuing chapters, and Frozen_Canon continuation. Not for book-level outline or final review/Commit.
license: MIT
compatibility: Requires write, edit, read_file, grep, glob, exec_shell; Core table_*, memory_*, search_kb; ask_user
metadata:
  author: danmo-work
  version: "2.3"
  category: creative-writing
---

# Novel Write（章合同 · 正文）

Contract → draft only. Review / Commit → `novel-review`.

**Pipeline steps 5–6/8.** 本技能不换模型。

## When to load

写第 N 章 / 章合同 / 批次冻结 / 续写 / 接手 / 爽点强化.

## Do

| Intent | Load | search_kb（≤1） |
|--------|------|-----------------|
| 章合同 | `chapter-contract.md` | 节奏与结构 |
| 批次冻结 | `batch-freeze.md` | — |
| 写正文 | `chapter-write.md`（含 preflight） | 文风与去 AI 味 |
| 开篇 ch1–3 | 上栏 + `opening-chapters.md` | 节奏与结构 |
| 续写接手 | `continuation.md` | 文风与去 AI 味 |
| 爽点强化 | `chapter-write.md` | 爽点与追读 |

写正文：**gate preflight → 只消费 `### CONTEXT` + 本章合同。** 禁止扫树；禁止 `author-lore`。批次冻结按单元章范围默认写入 `frozen_batch`（见 `batch-freeze.md`）。

## Hard stops

- No prose without accepted contract + `canon` cast.
- Gate preflight FAIL → no prose.
- Frozen_Canon unconfirmed → no prose.
- Batch write >1 chapter without freeze → run `batch-freeze` first.

## Stop

Draft on disk. Hand off `novel-review`.
