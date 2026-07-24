package builtin

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"danmo-work/core/domain"
)

const (
	memoryPolicyHint = "When to update: user preferences/corrections, lasting project conventions, stable decisions, or when the user says \"remember\". " +
		"Do NOT store one-off task details, large code dumps, secrets, file contents readable from the repo, or short-lived todos (use todowrite). " +
		"Scopes: user = cross-project preferences; project = this project only; agent = this agent role only. " +
		"Use a short stable key; keep content to a few factual sentences; update in place instead of duplicating."
)

// MemoryStore is the persistence surface used by memory tools.
type MemoryStore interface {
	Upsert(ctx context.Context, m domain.Memory) (domain.Memory, error)
	GetByKey(ctx context.Context, scope domain.MemoryScope, scopeID, key string) (domain.Memory, error)
	Search(ctx context.Context, q domain.MemoryQuery) ([]domain.Memory, error)
}

// MemoryUpdate upserts a durable memory entry.
type MemoryUpdate struct {
	Store MemoryStore
}

func (h *MemoryUpdate) Name() string                { return "memory_update" }
func (h *MemoryUpdate) RiskLevel() domain.RiskLevel { return domain.RiskLow }
func (h *MemoryUpdate) Describe(args map[string]any) string {
	key := strVal(args, "key")
	scope := strVal(args, "scope")
	if key == "" {
		return "memory_update"
	}
	if scope != "" {
		return fmt.Sprintf("memory_update %s/%s", scope, key)
	}
	return "memory_update " + key
}

func (h *MemoryUpdate) Schema() domain.ToolSchema {
	return domain.ToolSchema{
		Name: "memory_update",
		Description: "Write or update a durable memory that persists across sessions.\n\n" +
			memoryPolicyHint + "\n\n" +
			"mode=set replaces content (default); mode=append appends to existing content.",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"scope": map[string]any{
					"type":        "string",
					"description": "Memory scope: user | project | agent",
					"enum":        []string{"user", "project", "agent"},
				},
				"key": map[string]any{
					"type":        "string",
					"description": "Short stable identifier, e.g. preferred_language or api_style",
				},
				"content": map[string]any{
					"type":        "string",
					"description": "Factual memory content (one to a few sentences)",
				},
				"mode": map[string]any{
					"type":        "string",
					"description": "set (default) or append",
					"enum":        []string{"set", "append"},
				},
			},
			"required": []string{"scope", "key", "content"},
		},
	}
}

func (h *MemoryUpdate) Execute(ctx context.Context, input map[string]any) (domain.ToolResult, error) {
	if h.Store == nil {
		return domain.ToolResult{}, fmt.Errorf("memory store is not configured")
	}
	scope, scopeID, err := resolveMemoryScope(input, strVal(input, "scope"), true)
	if err != nil {
		return domain.ToolResult{}, err
	}
	key := strings.TrimSpace(strVal(input, "key"))
	content := strings.TrimSpace(strVal(input, "content"))
	if key == "" {
		return domain.ToolResult{}, fmt.Errorf("key is required")
	}
	if content == "" {
		return domain.ToolResult{}, fmt.Errorf("content is required")
	}
	mode := strings.ToLower(strings.TrimSpace(strVal(input, "mode")))
	if mode == "" {
		mode = "set"
	}
	if mode != "set" && mode != "append" {
		return domain.ToolResult{}, fmt.Errorf("mode must be set or append")
	}

	if mode == "append" {
		if existing, err := h.Store.GetByKey(ctx, scope, scopeID, key); err == nil {
			if existing.Content != "" {
				content = strings.TrimSpace(existing.Content) + "\n" + content
			}
		}
	}

	saved, err := h.Store.Upsert(ctx, domain.Memory{
		Scope:   scope,
		ScopeID: scopeID,
		Key:     key,
		Content: content,
	})
	if err != nil {
		return domain.ToolResult{}, err
	}

	summary := fmt.Sprintf("Memory saved [%s] %s: %s", saved.Scope, saved.Key, truncateRunes(saved.Content, 200))
	return domain.ToolResult{
		Content: summary,
		Meta: map[string]any{
			"scope":    string(saved.Scope),
			"scope_id": saved.ScopeID,
			"key":      saved.Key,
			"mode":     mode,
		},
	}, nil
}

// MemoryRead retrieves durable memories visible in the current turn.
type MemoryRead struct {
	Store MemoryStore
	TopK  int
}

func (h *MemoryRead) Name() string                { return "memory_read" }
func (h *MemoryRead) RiskLevel() domain.RiskLevel { return domain.RiskLow }
func (h *MemoryRead) Describe(args map[string]any) string {
	if key := strVal(args, "key"); key != "" {
		return "memory_read " + key
	}
	if q := strVal(args, "query"); q != "" {
		return "memory_read " + q
	}
	if scope := strVal(args, "scope"); scope != "" {
		return "memory_read " + scope
	}
	return "memory_read"
}

func (h *MemoryRead) Schema() domain.ToolSchema {
	return domain.ToolSchema{
		Name: "memory_read",
		Description: "Read durable memories for the current user/project/agent.\n\n" +
			"When to read: start of a new session, when the user refers to prior preferences/decisions, " +
			"or before architecture/style choices that may already be memorized.\n\n" +
			"Omit scope to search all visible scopes (user + current project + current agent). " +
			"Provide key for exact lookup, and/or query for keyword search.",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"scope": map[string]any{
					"type":        "string",
					"description": "Optional filter: user | project | agent",
					"enum":        []string{"user", "project", "agent"},
				},
				"key": map[string]any{
					"type":        "string",
					"description": "Exact memory key (optional)",
				},
				"query": map[string]any{
					"type":        "string",
					"description": "Keyword search over key and content (optional)",
				},
			},
		},
	}
}

func (h *MemoryRead) Execute(ctx context.Context, input map[string]any) (domain.ToolResult, error) {
	if h.Store == nil {
		return domain.ToolResult{}, fmt.Errorf("memory store is not configured")
	}
	visible, err := visibleMemoryScopes(input)
	if err != nil {
		return domain.ToolResult{}, err
	}

	scopeFilter := domain.MemoryScope(strings.TrimSpace(strVal(input, "scope")))
	if scopeFilter != "" {
		if _, _, err := resolveMemoryScope(input, string(scopeFilter), false); err != nil {
			return domain.ToolResult{}, err
		}
	}

	topK := h.TopK
	if topK <= 0 {
		topK = 10
	}

	hits, err := h.Store.Search(ctx, domain.MemoryQuery{
		Scopes: visible,
		Scope:  scopeFilter,
		Key:    strings.TrimSpace(strVal(input, "key")),
		Query:  strings.TrimSpace(strVal(input, "query")),
		TopK:   topK,
	})
	if err != nil {
		return domain.ToolResult{}, err
	}

	type item struct {
		Scope   string `json:"scope"`
		Key     string `json:"key"`
		Content string `json:"content"`
	}
	items := make([]item, 0, len(hits))
	for _, m := range hits {
		items = append(items, item{
			Scope:   string(m.Scope),
			Key:     m.Key,
			Content: m.Content,
		})
	}
	raw, err := json.MarshalIndent(items, "", "  ")
	if err != nil {
		return domain.ToolResult{}, err
	}
	if len(items) == 0 {
		return domain.ToolResult{Content: "[]"}, nil
	}
	return domain.ToolResult{
		Content: string(raw),
		Meta: map[string]any{
			"count": len(items),
		},
	}, nil
}

func resolveMemoryScope(input map[string]any, scopeRaw string, requireWritable bool) (domain.MemoryScope, string, error) {
	scope := domain.MemoryScope(strings.ToLower(strings.TrimSpace(scopeRaw)))
	switch scope {
	case domain.MemoryScopeUser:
		return scope, domain.MemoryUserScopeID, nil
	case domain.MemoryScopeProject:
		projectID := strings.TrimSpace(strVal(input, "__project_id"))
		if projectID == "" {
			return "", "", fmt.Errorf("project scope requires an active project")
		}
		return scope, projectID, nil
	case domain.MemoryScopeAgent:
		agentID := strings.TrimSpace(strVal(input, "__agent_id"))
		if agentID == "" {
			return "", "", fmt.Errorf("agent scope requires an active agent")
		}
		return scope, agentID, nil
	default:
		if requireWritable || scopeRaw != "" {
			return "", "", fmt.Errorf("scope must be user, project, or agent")
		}
		return "", "", nil
	}
}

func visibleMemoryScopes(input map[string]any) ([]domain.MemoryScopeRef, error) {
	refs := []domain.MemoryScopeRef{{
		Scope:   domain.MemoryScopeUser,
		ScopeID: domain.MemoryUserScopeID,
	}}
	if projectID := strings.TrimSpace(strVal(input, "__project_id")); projectID != "" {
		refs = append(refs, domain.MemoryScopeRef{
			Scope:   domain.MemoryScopeProject,
			ScopeID: projectID,
		})
	}
	if agentID := strings.TrimSpace(strVal(input, "__agent_id")); agentID != "" {
		refs = append(refs, domain.MemoryScopeRef{
			Scope:   domain.MemoryScopeAgent,
			ScopeID: agentID,
		})
	}
	return refs, nil
}

func truncateRunes(s string, max int) string {
	if max <= 0 || s == "" {
		return s
	}
	r := []rune(s)
	if len(r) <= max {
		return s
	}
	return string(r[:max]) + "…"
}
