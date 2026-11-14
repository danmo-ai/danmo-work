---
name: danmo-novel
source: builtin
description: >-
  Long-form / webnovel production via the Danmo Novel MCP connector. Use when
  the user or parent agent needs 立项 / 规划 / 细纲 / 写单元 / 定稿 (or a novel book
  tree under novel/<book-id>/).
license: MIT
compatibility: Requires Danmo Novel running (default http://127.0.0.1:7804); mcp_danmo_novel_* tools
metadata:
  author: danmo
  version: "2.0.0"
  category: creative
---

# Danmo Novel

Call tools with full names from the tool list (`mcp_danmo_novel_<tool>`).
Short names below are the MCP action suffix only. Generation runs **inside
danmo-novel** (its pipeline forbids LLM tools); you only orchestrate. Canon /
prose are written **only** by `accept_candidate` after a human confirms.

## Tools (suffix)

| Tool | Role |
|------|------|
| `next_unit` | 连续写作主入口：下一个该做哪个单元、该跑哪个阶段、能不能现在就跑 |
| `start_run` | 启动一个单元生产任务（细纲/正文/审校/修复/定稿） |
| `get_run` | 轮询一个 run 直到结束 |
| `accept_candidate` / `reject_candidate` | 接受/拒绝候选（唯一写盘路径） |
| `review_fixables` | 哪些审校意见能进修订（issueIds），按 roundsHit 排序 |
| `read_book_file` / `write_book_file` | 读/写一个文件（卷纲/细纲/人物卡/身份时间线） |
| `identity_segments` | 读身份时间线（action=list） |
| `get_book_template` | 读书籍结构模板（含修复指南） |

## start_run 参数

`bookId` + `unitId` + `kind` + `baseRevision`（`next_unit` 的 `nextSpec` 里已带好），可选 `instruction`。

- `kind`: `outline` | `write` | `review` | `revise` | `finalize`
- `review` 会自动 `supersedeReview`（归档旧清单后重审），不需要你传
- `revise` 不传 `issueIds` 时默认修**全部可打补丁的意见**（等同脚本全选）；
  传了则只修这几条。传错（含整单元结论或「没审完」标记）会直接报错并列出
  哪几条不能用 —— 拿不准就先调 `review_fixables`。

## 审校意见怎么挑

`review_fixables` 返回两类信息：

| 字段 | 含义 |
|------|------|
| `issueIds` | 可以直接进 `revise` 的 id（有章号、非未审完标记、属于正文） |
| `findings` | 上面这些条目的完整内容（含 severity / roundsHit / evidence / suggestion） |
| `skippedCount` | 不能进正文补丁的条数（整单元结论、审校未完成标记） |

两条路径都合法：
- **全修**：`start_run{kind:revise}` 不传 `issueIds`
- **挑着修**：先读 `review_fixables`，把选中的 id 传进 `issueIds`

`blockingCount` 用的是与 verdict 同一口径（`severity=blocking` 且 `roundsHit>=2`），
可以据此决定先修哪几条。

## 连续写作循环

```text
next_unit(bookId) 
  → 拿到 nextSpec（kind/unitId/baseRevision/issueIds）
  → start_run(...nextSpec) 
  → get_run(runId) 轮询直到结束
  → accept_candidate(candidateId) 
  → 再调 next_unit
```

如果失败，`next_unit` 会告诉你「卡在哪一步」，只需要重跑那一步。

## 出错时怎么修

1. **先读模板**：`get_book_template` → 里面有「常见错误与修复」表。
2. **按表改文件**：用 `read_book_file` 读当前内容，按模板理解结构，用 `write_book_file` 写回。
3. **改完再试**：重新 `start_run` 或 `accept_candidate`。

## Anti-patterns

- 跳过 `next_unit` 直接调 `start_run`（可能起错阶段）
- 用 `write_book_file` 直接写正文（正文必须走 candidate-accept）
- 调用 Danmo Novel REST 直接代替 MCP 工具
- 声称成功但没有 finished run + candidate/accept id
