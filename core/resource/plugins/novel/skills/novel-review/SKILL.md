---
name: novel-review
source: builtin
description: Triggers on 定稿 / 重新定稿 / 审阅定稿 / finalize. Prefer hook PACK ready → read_file → patch → postcommit. Else prompt-pack --stage finalize. New session. Not for first drafts (novel-write).
license: MIT
compatibility: Requires write, edit, read_file, grep, exec_shell; Core table_*, memory_*, search_kb; ask_user
metadata:
  author: danmo-work
  version: "4.0"
  category: creative-writing
---

# Novel Review（定稿一轮 · 卷收束）

## Pipeline system

定稿只读本 pack。`qc-pack` 就是审。禁止写首稿、扫树、`search_kb`、`find` gate、通读正文、二读 facts/YAML。

1. 若 ephemeral 已 `PACK ready`：直接 `read_file`。否则钉死 `G=` 跑 `--action prompt-pack --stage finalize --unit`。
2. **EXPAND**：`expand_needed: no` 则不扩写。否则只按 pack 的锚点做 **一次** `edit_batch`（含 HITS 行号）。不要 `offset` 通读 `units/*.md`。
3. **验证一次** `qc-pack`。
4. **PASS → 正文冻结。** 禁止再改 `units/*.md`，禁止第二次 `qc-pack`。按 pack `## COMMIT` 只改 summaries / facts 游标 / 细纲 `reviewed` / state / commits 日志，然后 `postcommit`。
5. **FAIL → 停**（可写 `reviews/`）。不要「再改再 qc」。不要「直到 exit 0」。

**Stage 5/5.** 与 novel-write 分开。本技能不换模型。

## When to load

定稿 / 重新定稿 / 审阅定稿 / 扩→审→润→Commit / 审稿 / 去 AI 味 / Continuity Commit / 卷收束. 对象是 **一个单元的一份正文**。

**不要**用本技能写首稿正文（→ `novel-write`）。

## Do

优先读 hook/`PACK ready` 包文件（LENGTH / HITS / COUNTS / CONTINUITY）。无 PACK 才 `prompt-pack --stage finalize`。按 pack 选路——**不要**把 `qc-pack` 当循环条件。

| 步 | 触发 | 做什么 |
|----|------|--------|
| 扩写+润色 | EXPAND / HITS | 一次 edit；只用锚点与行号 |
| 验证 | 改完后 | `qc-pack` **恰好 1 次** |
| Commit | 该次 PASS | 正文冻结；只按 `## COMMIT` 改账本 → `postcommit` |
| 停 | 该次 FAIL | 报告后停；不改完再验 |

**PASS 后禁止再改正文。** 十维审不另开一轮改稿。不要为 Commit 去读 facts / YAML / 人物卡。

**Commit = 一次 patch：**
- `continuity/summaries/vNN.md`：该单元每一章 `## chNNN` 五要素块（新卷新建文件 + facts 索引行）
- `continuity/facts.md`：Public facts + cursor + Cast snapshot 增量（含年龄/职位，按 `state_deltas` 重放）+ Open loops（**不写章摘要**）
- 相关 `canon/cast/<stem>.md` 关系表「当前质态」「最近变化点（含本单元 id）」两列（仅当 `state_deltas` 涉及关系变化；两张卡都改；**不改**开卷身份基线）
- 细纲结构化 `state_deltas` 与正文身份变迁对齐；commits 日志可写身份转变摘要
- `continuity/commits/vNN-U#.md`：执行日志（gate 结果 / 四计数 / 锁词 / 字数 / 扩写 / 偏离）
- 细纲 `status=reviewed` + `novel-state.yaml`（`last_committed_ch` = 章范围末章，`active_unit` 指向下一单元）
- `postcommit --unit` exit 0

验证 `qc-pack` PASS（或仅 advisory）才可宣称定稿；报告 GATES 段引用 `### COUNTS`。一轮只定稿一个单元。不要按章拆成多次 Commit。二次验证仍 blocking → 停，不要循环。

## Stop

Completion = tool evidence（至多 1 次验证 qc-pack + postcommit VERDICT，或 FAIL 报告）。卷末单元 → 提示「可做卷收束」；下一卷卷纲 → `novel-plan`。Do not start the next unit's first draft here.
