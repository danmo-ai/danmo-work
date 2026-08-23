# Intent → reference / KB map

Use this when the user request is ambiguous. Pick **one** primary stage; load its reference; pull KB themes with `search_kb`.

| User says (examples) | Stage | `read_skill` | KB themes |
|----------------------|-------|--------------|-----------|
| 开书 / 立项 / 想写一本 | init | `init.md`, `project-layout.md`, `table-schema.md` | 题材速览, 人设与群像 |
| 大纲 / 卷纲 / 章纲（一卷多章列表） | outline | `outline.md` | 节奏与结构, 番茄平台, 爽点 |
| 批次冻结 / 细纲冻结 | batch-freeze | `batch-freeze.md`, `preflight.md` | 节奏, 爽点 |
| 章合同 / 这章写什么 | contract | `chapter-contract.md` | 节奏, 情绪, 番茄平台 |
| 写第 N 章 / 续写 | write | `preflight.md`, `chapter-write.md`, `scene-routing.md` | 文风, 爽点, 情绪, 番茄平台 |
| 审稿 / 检查 / 会不会崩 | review | `review-gates.md` | 去 AI 味, 世界观, 追读力 |
| 批量审稿 | batch-review | `review-gates.md`, `preflight.md` | 去 AI 味, 题材 QC |
| 去 AI 味 / 润色 | polish | `polish-deslop.md` | 去 AI 味, 语言与文风 |
| 伏笔 / 状态 / 接着写哪 | commit/resume | `continuity-commit.md`, `table-schema.md` | 金手指（若涉及） |
| 续写 / 接手 / 卡文 | continuation | `continuation.md`, `batch-freeze.md` | 文风, 情绪 |
| 金手指怎么设 | design | `table-schema.md` + template `goldfinger-card.md` | 金手指约束 |
| 人物卡 / 世界观 | assets | `init.md` + canon files | 人设, 世界观 |
| 写前检查 | preflight | `preflight.md` | — |

If multiple intents appear, finish the earliest incomplete stage in:

`init → outline → batch_freeze → contract → write → review → polish → commit`

Continuation branch: `continuation (CP1–CP3) → batch_freeze → …`
