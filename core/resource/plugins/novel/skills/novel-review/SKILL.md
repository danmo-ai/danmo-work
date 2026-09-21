---
name: novel-review
source: builtin
description: Post-draft finalize for novels  — 字数扩写, review, deslop, and Continuity Commit. Use after a drafted unit when expanding thin prose, 审稿, 去AI味, or committing continuity. Commit splits facts.md (reader facts) and commits/vNN-U#.md (execution log). Not for opening a book or writing first drafts.
license: MIT
compatibility: Requires write, edit, read_file, grep, exec_shell; Core table_*, memory_*, search_kb; ask_user
metadata:
  author: danmo-work
  version: "3.0"
  category: creative-writing
---

# Novel Review（扩写 · 审稿 · 润色 · Commit）

Post-draft lane（可与写作分模）：字数不足先扩写 → 一轮审稿 → gate precommit → 修 P0 → 可选去 AI 味 → Continuity Commit（gate postcommit）。`qc_gate` FAIL blocks 定稿.

**Pipeline steps 7–8/8**. 本技能不换模型。**与 novel-write 分开**：禁止在首稿 turn 定稿。

## When to load

字数不足扩写 / 审稿 / 去 AI 味（含量化硬检）/ Continuity Commit / 卷收束 / 审→润→定 / 扩→审→润→定. 对象是 **一个单元的一份正文**。

**不要**用本技能写首稿正文（首稿 → `novel-write`）。

## Do

| Intent | Load | search_kb（≤1） |
|--------|------|-----------------|
| 字数不足/扩写 | `expansion.md`（word_floor 前置硬检，不靠后验扩写） | 扩写与字数控制 |
| 审本单元 | `review-gates.md`（10 维加权 + 发稿前四步自查） | 文风与去 AI 味（人味对照用文内清单，不另查） |
| 去 AI 味 | `polish-deslop.md`（先跑 scan-deslop --unit） | 默认「文风与去 AI 味」；`craft_lane=crime-human` →「刑侦人味文风」 |
| Commit | `continuity-commit.md` + `commit-log.md`（facts + commits 分文件） | — |
| 卷收束 | `continuity-commit.md` 卷收束节 + `review-gates.md` Assembly Checklist | — |
| 定稿串行（扩→审→润→Commit） | 上表按需依次 | 按步各 ≤1 |

**PASS：不写 `reviews/` 文件**，只更新 `gates.qc`。**FAIL / 深审：写 `reviews/vNN-U#-review.md`。**

**Commit = 一次 patch：**
- `continuity/facts.md`：该单元每一章 `## chNNN` 五要素摘要 + Cast snapshot + Open loops（读者事实，不写执行日志）
- `continuity/commits/vNN-U#.md`：本单元执行日志（gate 结果/四计数/锁词扫描/字数实测/扩写技术/偏离登记）
- 细纲 `status=reviewed` + `last_committed_ch` 推到章范围末章
- `postcommit --unit` exit 0

反 AI 量化硬检（破折号/英文泄漏/禁词/比喻/锁词/字数 floor）exit 0 才可宣称定稿；审稿报告引用 gate `### COUNTS`。

一轮只定稿一个单元。不要按章拆成多次 Commit。

## Stop

Completion = tool evidence. Do not start the next book’s first draft here.
