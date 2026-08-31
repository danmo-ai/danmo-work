---
name: novel-write
source: builtin
description: Chapter loop for a planned novel. Use for 章合同, batch freeze, drafting or continuing chapters, preflight, scene routing, and Frozen_Canon continuation. Not for book-level outline or final review/Commit.
license: MIT
compatibility: Requires write, edit, read_file, grep, glob, exec_shell; Core table_*, memory_*, search_kb; ask_user
metadata:
  author: danmo-work
  version: "2.2"
  category: creative-writing
---

# Novel Write（章合同 · 正文）

Contract → draft only. Review and Commit belong to `novel-review`.

**Pipeline steps 5–6/8：章合同 → 正文.** 用户可自行切换写作模型；本技能不换模型。

## When to load

写第 N 章 / 章合同 / 批次冻结 / 续写 / 接手 / 写前预检 / 爽点强化.

## Do（少交互）

| Intent | Load | search_kb（≤1） |
|--------|------|-----------------|
| 章合同 | `novel-write/references/chapter-contract.md` | 节奏与结构 |
| 批次冻结 | `novel-write/references/batch-freeze.md` | — |
| 写正文 | `preflight.md` + `chapter-write.md` | 文风与去 AI 味 |
| 开篇 ch1–3 | 上栏 + `opening-chapters.md` | 节奏与结构 |
| 续写接手 | `continuation.md` | 文风与去 AI 味 |
| 爽点强化（钩/反转） | `chapter-write.md` / `scene-routing.md` | 爽点与追读 |
| 写前预检 | `preflight.md` | — |

Templates: `chapter-contract.yaml`, `style-fingerprint.md`. Set `novel-state.yaml` `stage: writing`. Batch freeze → **only** `novel-state.frozen_batch`（no `batch-freeze.yaml`).

**日更最小加载：** `novel-state` + `continuity/ledger.md`（尾部）+ 合同 + 出场 cast。禁止为走流程扫全树。`table_*` upsert 可选。

## Hard stops

- No prose without an accepted contract and loaded `canon/cast` (`asset_gate`).
- Gate script preflight FAIL → no prose.
- Frozen_Canon unconfirmed → no prose (`continuation.md` CP1–CP3).
- Batch write >1 chapter without freeze → `ask_user` or `batch-freeze.md`.
- `candidate` stays out of prose.
- Do not load `canon/author-lore.md` while drafting.

## Stop

Draft file on disk. Do not claim 定稿; hand off to `novel-review`.
