---
name: novel-review
source: builtin
description: 定稿 round for one drafted unit — prompt-pack --stage finalize (word/HITS/COUNTS/CONTINUITY on disk), skip expansion when length is fine, 10-dim review, one-patch Commit and postcommit. Prefer a new session (no chat history). Not for opening a book or first drafts.
license: MIT
compatibility: Requires write, edit, read_file, grep, exec_shell; Core table_*, memory_*, search_kb; ask_user
metadata:
  author: danmo-work
  version: "4.0"
  category: creative-writing
---

# Novel Review（定稿一轮 · 卷收束）

## Pipeline system

定稿只读本 pack。禁止写首稿、扫树、`search_kb`（人味对照已在 pack）。

1. `exec_shell` gate `--action prompt-pack --stage finalize --unit vNN-U#`。
2. 按 pack 的 LENGTH / HITS / COUNTS / CONTINUITY 选路：`expand_needed` 才扩写；HITS 非空才按行号润色；10 维审。PASS 不写 `reviews/`。
3. 一次 Commit 补丁后 `postcommit --unit`。exit ≠ 0 停。不要第三轮自动重试。

**Stage 5/5.** 一个单元一轮定稿（可与写作分模）：`qc-pack --unit` 一份 stdout → 字数够则跳扩写、HITS 空则跳润色 → 10 维审 → 修 P0 → 一次补丁 Commit → `postcommit --unit`。**与 novel-write 分开**：禁止在首稿 turn 定稿。本技能不换模型。

## When to load

定稿 / 扩→审→润→Commit / 审稿 / 去 AI 味 / Continuity Commit / 卷收束. 对象是 **一个单元的一份正文**。

**不要**用本技能写首稿正文（→ `novel-write`）。

## Do

先跑 `exec_shell` gate `--action prompt-pack --stage finalize --unit vNN-U#`，`read_file` stdout 的 `file:`（LENGTH / HITS / COUNTS / CONTINUITY）。按 pack 选路。

| 步 | 触发 | Load | search_kb（≤1，整轮共用） |
|----|------|------|-----------------|
| 扩写 | pack `expand_needed: yes` 或 `word_floor` blocking | 仅 pack 不足才 `expansion.md` | 默认不查 |
| 审本单元 | 总是 | 仅 pack 不足才 `review-gates.md` | 默认不查 |
| 去 AI 味 | pack HITS 非空、审稿 P0 | 仅 pack 不足才 `polish-deslop.md` | 默认不查 |
| Commit | 审 PASS | 优先 pack CONTINUITY + `state_deltas`；不足才 `continuity-commit.md` | — |
| 卷收束 | 卷末单元 Commit 后，人确认 | `continuity-commit.md` 卷收束节 | — |

改完正文再跑一次 `qc-pack` 直到 exit 0，再 Commit。**PASS：不写 `reviews/` 文件**，只更新 `gates.qc`。**FAIL / 深审：写 `reviews/vNN-U#-review.md`** 并停（human stop）。

**Commit = 一次 patch：**
- `continuity/summaries/vNN.md`：该单元每一章 `## chNNN` 五要素块（新卷新建文件 + facts 索引行）
- `continuity/facts.md`：Public facts + cursor + Cast snapshot 增量（含年龄/职位，按 `state_deltas` 重放）+ Open loops（**不写章摘要**）
- 相关 `canon/cast/<stem>.md` 关系表「当前质态」「最近变化点（含本单元 id）」两列（仅当 `state_deltas` 涉及关系变化；两张卡都改；**不改**开卷身份基线）
- 细纲结构化 `state_deltas` 与正文身份变迁对齐；commits 日志可写身份转变摘要
- `continuity/commits/vNN-U#.md`：执行日志（gate 结果 / 四计数 / 锁词 / 字数 / 扩写 / 偏离）
- 细纲 `status=reviewed` + `novel-state.yaml`（`last_committed_ch` = 章范围末章，`active_unit` 指向下一单元）
- `postcommit --unit` exit 0

反 AI 量化硬检 exit 0 才可宣称定稿；报告 GATES 段引用 `### COUNTS`。一轮只定稿一个单元。不要按章拆成多次 Commit。

## Stop

Completion = tool evidence（qc-pack + postcommit VERDICT）。卷末单元 → 提示「可做卷收束」；下一卷卷纲 → `novel-plan`。Do not start the next unit's first draft here.
