// Package runtime — durable agent memory.
//
// Durable cross-session memory is provided by the always-on tools
// memory_update / memory_read (see tool/builtin/memory.go), backed by the
// SQLite memories table with user / project / agent scopes.
//
// System prompt guidance lives in <memory-policy> / <ask-user-policy> /
// <knowledge-policy> / <mcp-tool-naming> (and <delegation-policy> when
// CanDelegate) in prompt_builder.go.
// Memories are tool-driven: they are NOT auto-injected every turn.
//
// Future (separate from durable tool memory) — compaction-time BM25 indexing:
//
// Rather than indexing per-turn summaries, index the full conversation as it is
// about to be dropped by compaction. This captures all messages (tool calls,
// tool results, LLM responses) — not just summaries — and ensures zero information
// loss, even for content that was never summarized.
//
//   - Record: at compaction time (maybeCompact), before messages[0:cutIdx] are
//     discarded, index them into a session-scoped BM25 index. Each message is
//     a document with fields: role, content (text), tool_call_id, turn.
//   - Recall: before each turn, BM25-query the index with the current goal.
//     Return top-K message chunks as "Agent memory" in the system prompt.
//   - Index pruning: when a session ends, discard the index (pure in-memory).
//     For long-running sessions, evict oldest documents by token budget.
//
// Original episodic / notepad design was removed (near-zero recall rate and
// redundant injection with tool results).
package runtime
