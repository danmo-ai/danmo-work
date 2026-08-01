package domain

import "time"

// MemoryScope is the durability scope for an agent-authored memory entry.
type MemoryScope string

const (
	MemoryScopeUser    MemoryScope = "user"
	MemoryScopeProject MemoryScope = "project"
	MemoryScopeAgent   MemoryScope = "agent"
)

// MemoryUserScopeID is the scope_id used for user-global memories on single-user installs.
const MemoryUserScopeID = "default"

// Memory is a durable fact authored by the agent via memory_update.
type Memory struct {
	ID        string      `json:"id"`
	Scope     MemoryScope `json:"scope"`
	ScopeID   string      `json:"scopeId"`
	Key       string      `json:"key"`
	Content   string      `json:"content"`
	UpdatedAt time.Time   `json:"updatedAt"`
}

// MemoryQuery filters memories for memory_read.
type MemoryQuery struct {
	// Scopes lists (scope, scopeID) pairs that are visible for this turn.
	Scopes []MemoryScopeRef
	// IncludeAllAgents adds every agent-scoped memory (any scope_id). Used by
	// the Memory panel so subagent writes are visible beside the lead agent.
	IncludeAllAgents bool
	// Scope optionally restricts results to one scope name (user|project|agent).
	Scope MemoryScope
	// Key exact-match filter (optional).
	Key string
	// Query keyword search over key + content (optional).
	Query string
	// TopK limits returned rows (default applied by caller if <= 0).
	TopK int
}

// MemoryScopeRef identifies one concrete scope bucket.
type MemoryScopeRef struct {
	Scope   MemoryScope
	ScopeID string
}
