---
name: codegraph
description: >-
  Query the local CodeGraph index for symbols, callers, and impact analysis.
  Use when the user or parent agent needs structural code intelligence.
license: MIT
compatibility: Requires built-in codegraph connector; mcp_codegraph_* tools
metadata:
  author: danmo
  version: "0.2.0"
  category: code-intelligence
---

# CodeGraph

Backend: [sunerpy/codegraph-rust](https://github.com/sunerpy/codegraph-rust)
(tree-sitter → SQLite/FTS5, MCP over stdio). Call tools with full names from the
tool list (`mcp_codegraph_<tool>`). Short names below are the MCP action suffix
only.

The parent turn may prepend an `[codegraph-index: …]` hint. Trust it.

## Index states

| State | Meaning | What to do |
|-------|---------|------------|
| `ready` | `.codegraph/` is usable | Use MCP tools |
| `indexing` | Background `codegraph init` running | **Do not wait.** Degrade to `read_file` / `grep` this turn; mention indexing briefly |
| `failed` / missing binary / empty | No usable index | Degrade; explain briefly |
| unknown | No hint | Call `status` once; if not ready, degrade |

## Workflow (index ready)

1. **`status`** — confirm the project (injected `projectPath` / working directory).
2. **`explore`** — PRIMARY for "how does X work", architecture, flows, surveying an area. One call returns relevant symbols' source + relationships.
3. **`callers`** / **`impact`** — when the goal is who-calls-whom or change blast-radius.
4. **`search`** / **`node`** — optional: locate a symbol by name, or read one symbol/file with caller/callee trail.
5. Only if still incomplete: `read_file` / `grep` for remaining gaps (configs, docs, or staleness).

## Workflow (indexing / failed / no index)

1. Skip MCP graph tools (or stop after one failed call).
2. Answer with `grep` + `read_file` only.
3. In NOTES set status to `indexing` or `degraded`.
4. Do **not** run `codegraph init` yourself and do **not** poll/sleep until ready.

## Anti-patterns

- Blocking or looping until indexing finishes
- Claiming precise callers/impact without tool evidence
- Re-verifying graph results with broad grep when the index is ready
- Calling Make/creative MCP or `http_request` for this job
- Ignoring the `[codegraph-index: …]` hint
