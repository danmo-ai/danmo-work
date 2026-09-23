---
name: novel-review
source: builtin
description: 定稿 round for one drafted unit — gate --action qc-pack (word floor/ceiling + 四计数 + 锁词 + HITS in one output), skip expansion when length is fine and polish when HITS is empty, 10-dim review, then one-patch Continuity Commit (summaries/vNN.md chapter blocks + facts cursor + cast relation columns + reviewed + state) and gate postcommit. Also 卷收束. Not for opening a book, cast, outlines, or first drafts.
license: MIT
compatibility: Requires write, edit, read_file, grep, exec_shell; Core table_*, memory_*, search_kb; ask_user
metadata:
  author: danmo-work
  version: "4.0"
  category: creative-writing
---

# Novel Review（定稿一轮 · 卷收束）

**Stage 5/5.** 一个单元一轮定稿（可与写作分模）：`qc-pack --unit` 一份 stdout → 字数够则跳扩写、HITS 空则跳润色 → 10 维审 → 修 P0 → 一次补丁 Commit → `postcommit --unit`。**与 novel-write 分开**：禁止在首稿 turn 定稿。本技能不换模型。

## When to load

定稿 / 扩→审→润→Commit / 审稿 / 去 AI 味 / Continuity Commit / 卷收束. 对象是 **一个单元的一份正文**。

**不要**用本技能写首稿正文（→ `novel-write`）。

## Do

先跑 `exec_shell` gate `--action qc-pack --unit vNN-U#`，读三段：`### LENGTH`（`expand_needed` / `polish_needed` / 各章 runes vs share）、`### HITS`（行号）、`### COUNTS`（四计数）+ BLOCKING（锁词 / 字数 floor / 章结构）。按结果只加载需要的那几步：

| 步 | 触发 | Load | search_kb（≤1，整轮共用） |
|----|------|------|-----------------|
| 扩写 | `expand_needed: yes` 或 `word_floor` blocking | `expansion.md` | 扩写与字数控制 |
| 审本单元 | 总是 | `review-gates.md`（10 维加权 + 发稿前四步自查） | 文风与去 AI 味 |
| 去 AI 味 | `HITS` 非空或审稿 P0 | `polish-deslop.md`（用 qc-pack 的 HITS 行号，不再单跑 scan-deslop） | 同上；`subgenre=刑侦探案` 人味对照用 CONTEXT 里已注入的人味篇，不另查 |
| Commit | 审 PASS | `continuity-commit.md` + `commit-log.md` | — |
| 卷收束 | 卷末单元 Commit 后，人确认 | `continuity-commit.md` 卷收束节 + `review-gates.md` Assembly Checklist | — |

改完正文再跑一次 `qc-pack` 直到 exit 0，再 Commit。**PASS：不写 `reviews/` 文件**，只更新 `gates.qc`。**FAIL / 深审：写 `reviews/vNN-U#-review.md`** 并停（human stop）。

**Commit = 一次 patch：**
- `continuity/summaries/vNN.md`：该单元每一章 `## chNNN` 五要素块（新卷新建文件 + facts 索引行）
- `continuity/facts.md`：Public facts + cursor + Cast snapshot 增量 + Open loops（**不写章摘要**）
- 相关 `canon/cast/<stem>.md` 关系表「当前质态」「最近变化点（含本单元 id）」两列（仅当 `state_deltas` 涉及关系变化；两张卡都改）
- `continuity/commits/vNN-U#.md`：执行日志（gate 结果 / 四计数 / 锁词 / 字数 / 扩写 / 偏离）
- 细纲 `status=reviewed` + `novel-state.yaml`（`last_committed_ch` = 章范围末章，`active_unit` 指向下一单元）
- `postcommit --unit` exit 0

反 AI 量化硬检 exit 0 才可宣称定稿；报告 GATES 段引用 `### COUNTS`。一轮只定稿一个单元。不要按章拆成多次 Commit。

## Stop

Completion = tool evidence（qc-pack + postcommit VERDICT）。卷末单元 → 提示「可做卷收束」；下一卷卷纲 → `novel-plan`。Do not start the next unit's first draft here.
