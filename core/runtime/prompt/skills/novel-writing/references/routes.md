# Intent → reference / KB map

Use this when the user request is ambiguous. Pick **one** primary stage; load its reference; pull KB themes with `search_kb`.

| User says (examples) | Stage | `read_skill` | KB themes |
|----------------------|-------|--------------|-----------|
| 开书 / 立项 / 想写一本 | init | `init.md`, `project-layout.md`, `table-schema.md` | 题材速览, 人设与群像 |
| 大纲 / 卷纲 / 细纲 | outline | `outline.md` | 节奏与结构, 爽点 |
| 章合同 / 这章写什么 | contract | `chapter-contract.md` | 节奏, 情绪 |
| 写第 N 章 / 续写 | write | `chapter-write.md`, `chapter-contract.md` | 文风, 爽点, 情绪 |
| 审稿 / 检查 / 会不会崩 | review | `review-gates.md` | 去 AI 味, 世界观 |
| 去 AI 味 / 润色 | polish | `polish-deslop.md` | 去 AI 味, 语言与文风 |
| 伏笔 / 状态 / 接着写哪 | commit/resume | `continuity-commit.md`, `table-schema.md` | 金手指（若有） |
| 金手指怎么设 | design | `table-schema.md` + template `goldfinger-card.md` | 金手指约束 |
| 人物卡 / 世界观 | assets | `init.md` + canon files | 人设, 世界观 |

If multiple intents appear, finish the earliest incomplete stage in:

`init → outline → contract → write → review → polish → commit`
