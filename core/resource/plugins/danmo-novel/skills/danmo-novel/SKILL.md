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
  version: "1.0.0"
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
| `health` | Probe the server; abort if not ok |
| `list_books` / `get_book` | Shelf / one book + computed `primary` next action |
| `create_book` / `import_book` | Scaffold 立项 / import a `novel/<book-id>/` tree |
| `read_book_file` / `write_book_file` | Author-side 补洞 only (never production prose) |
| `preview_run` | Inspect context packet without starting a run |
| `start_run` | Start a controlled generation run (TaskSpec) |
| `get_run` / `list_runs` / `cancel_run` | Run lifecycle |
| `list_candidates` / `get_candidate` | Open candidates |
| `accept_candidate` / `reject_candidate` | The ONLY write path (human-confirmed) |
| `accept_volume` / `promote_cast` | Approve a volume / promote a cast card |

## TaskSpec for `start_run`

`{kind, bookId, volumeId?, unitId?, mode?, instruction?, knowledgeIds?}` —
`kind`: `init | plan | outline | write | review | revise | finalize | polish | …`.
`mode` only for `plan`: `init | reinit | volume`.

## Workflow

1. **`health`** — abort if not ok.
2. **`get_book(bookId)`** (or `list_books`) → read `primary` (`kind`, `unitId`,
   `volume`) when the user did not name a target.
3. **Decide the run** from `primary.kind` / user intent:

   | Stage | Skill intent | Run |
   |-------|--------------|-----|
   | 立项 | new book | `create_book` → `start_run kind=plan mode=init` |
   | 规划 | cast + volume outline | `start_run kind=plan mode=volume` |
   | 一批细纲 | unit contracts | `start_run kind=outline` |
   | 写单元 | one unit prose | `start_run kind=write` |
   | 定稿 | review / finalize | `start_run kind=review` (findings → human) |

4. **`start_run(spec)`** — returns the run; surfaces candidates.
5. **Show candidates** (`get_candidate`) and **stop for the human**. Do not
   auto-accept.
6. On approval: **`accept_candidate`** (optional `acceptFindingIds` /
   `forceRiskAccept`). Volume-approve may use `accept_volume`; one card →
   `promote_cast`. Status advances `proposed → accepted → drafted → reviewed`.

## Anti-patterns

- Skipping `health` or `get_book`
- Inventing prose / outline and writing it with `write_book_file` or filesystem tools
- Calling `accept_candidate` before the human has seen and approved the candidate
- Calling Danmo Novel REST directly instead of the MCP tools
- Claiming success without a finished run + candidate/accept ids
