# Preflight（写前预检 + 读取回执）

每章写/审/Commit 前执行。日更只加载最小集；工作台 UI 与 Agent 共用同一清单。

## 日更最小加载

**必读（上限：这些文件本身，禁止扩读）：**

1. `novel-state.yaml` — stage、`frozen_batch`、`gates`、`blockers`、`qc_profile`。
2. 本章 `chapters/chNNN-contract.yaml`（写正文时须 `accepted` 或用户明示豁免；`unit_id` 非空且能在本卷纲剧情单元表对上）。
3. `continuity/chapter_summaries.md` **近 3 章**尾部（不要整本摘要）。
4. `table_query` 开放 `foreshadows`、`continuity_issues`、`reader_debts`（若有）。

**按需：**

5. 本章 `beats` / `state_deltas` 点名的 `canon/cast/*.md`（asset_gate）。未上场角色不读。
6. `search_kb` 本阶段 **1–2** 个主题（knowledge_gate）。写钩子/接钩用「爽点与追读」。

**禁止本轮加载：** 整本 `book-bible.md`、全卷纲正文、全书 `canon/` 通读、已提交章全文（除非合同 `continuity_risks` 点名）。

## 读取回执

写入 `continuity/preflight-log.md` 追加一行（或会话 SUMMARY）：

```text
[YYYY-MM-DD chNNN] state:writing | contract:accepted | cast:3 | foreshadows:open:2 | KB:爽点与追读,题材与平台
```

讨论模式：向用户展示路径 + 每层 1–2 关键事实，等确认。  
快速模式：一行摘要即可。

## 更新 novel-state gates

Agent 完成 preflight 后更新（供工作台展示）：

```yaml
gates:
  knowledge: pass|fail|unknown
  asset: pass|fail|unknown
  qc: pass|fail|unknown
blockers: []  # 人类可读阻断原因
```

## 阻断

缺少必读文件、合同非 `accepted`（写正文时）、`unit_id` 空或对不上卷纲单元、Frozen_Canon 空（续写）、批次未冻结（批量写）→ **停止**，列缺失项。空/错 `unit_id` 退回 `novel-plan` 补卷纲，不要空造合同。
