# Preflight（写前预检 + 读取回执）

每章写/审/Commit 前执行；工作台 UI 与 Agent 共用同一检查清单。

## 步骤

1. `read_file` `novel-state.yaml` — stage、`frozen_batch`、`gates`、`blockers`、`qc_profile`。
2. `read_file` 本章/目标章 `chapters/chNNN-contract.yaml`（若存在）。
3. `read_file` 近 3–5 章 `continuity/chapter_summaries.md` 尾部。
4. `table_query` 开放 `foreshadows`、`continuity_issues`、`reader_debts`（若有）。
5. `read_file` 涉及角色 `canon/cast/*.md`（asset_gate）。
6. `search_kb` 本阶段主题（knowledge_gate）。

## 读取回执

写入 `continuity/preflight-log.md` 追加一行（或会话 SUMMARY）：

```text
[YYYY-MM-DD chNNN] state:chapter_loop | contract:accepted | cast:3 | foreshadows:open:2 | KB:爽点,番茄
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

缺少必读文件、合同非 `accepted`（写正文时）、Frozen_Canon 空（续写）、批次未冻结（批量写）→ **停止**，列缺失项。
