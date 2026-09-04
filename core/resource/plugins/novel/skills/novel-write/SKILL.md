---
name: novel-write
source: builtin
description: Chapter loop for a planned novel. Use for 章纲, batch freeze, drafting or continuing chapters, 字数不足扩写, and Frozen_Canon continuation. Not for book-level outline or final review/Commit.
license: MIT
compatibility: Requires write, edit, read_file, grep, glob, exec_shell; Core table_*, memory_*, search_kb; ask_user
metadata:
  author: danmo-work
  version: "2.5"
  category: creative-writing
---

# Novel Write（章纲 · 正文）

Chapter outline → draft only. Review / Commit → `novel-review`.

**Pipeline steps 5–6/8.** 本技能不换模型。

## When to load

写第 N 章 / 章纲 / 批次冻结 / 续写 / 接手 / 爽点强化 / 字数不足扩写.

## Do

| Intent | Load | search_kb（≤1） |
|--------|------|-----------------|
| 章纲 | `chapter-outline.md` | 节奏与结构 |
| 批次冻结 | `batch-freeze.md` | — |
| 写正文 | `chapter-write.md`（含 preflight） | 文风与去 AI 味 |
| 开篇 ch1–3 | 上栏 + `opening-chapters.md` | 节奏与结构 |
| 续写接手 | `continuation.md` | 文风与去 AI 味 |
| 爽点强化 | `chapter-write.md` | 爽点与追读 |
| 字数不足/扩写 | `expansion.md` | 扩写与字数控制 |

写正文：**gate preflight → 只消费 `### CONTEXT` + 本章纲。** 禁止扫树；禁止 `author-lore`。批次冻结按单元章范围默认写入 `frozen_batch`（见 `batch-freeze.md`）。

## Hard stops

- No prose without accepted chapter outline + `canon` cast.
- Gate preflight FAIL → no prose.
- Frozen_Canon unconfirmed → no prose.
- Batch write >1 chapter without freeze → run `batch-freeze` first.

## Stop

Draft on disk. Hand off `novel-review`.
