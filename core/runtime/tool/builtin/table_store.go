package builtin

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"unicode/utf8"

	"danmo-work/core/domain"
	"danmo-work/core/port"
)

const tableStorePolicyHint = "Use table_* for queryable business rows (digests, counters, cursors). " +
	"Use memory_* for lasting preferences/conventions. Use write for long-form documents. " +
	"Pick a stable key (date or external id) for idempotent upserts. " +
	"Do NOT store secrets, large binaries, or full report bodies (store a file path instead). " +
	"Scopes: user | project | agent (same as memory)."

// TableTurnBudget counts table_upsert rows per turn for hard quotas.
type TableTurnBudget struct {
	mu     sync.Mutex
	counts map[string]int
}

func NewTableTurnBudget() *TableTurnBudget {
	return &TableTurnBudget{counts: map[string]int{}}
}

func (b *TableTurnBudget) Add(turnID string, n int) int {
	if b == nil {
		return n
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.counts == nil {
		b.counts = map[string]int{}
	}
	b.counts[turnID] += n
	return b.counts[turnID]
}

// TryAdd increments by n only when the resulting total stays <= max.
func (b *TableTurnBudget) TryAdd(turnID string, n, max int) (int, error) {
	if b == nil {
		return n, nil
	}
	if turnID == "" {
		return n, nil
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.counts == nil {
		b.counts = map[string]int{}
	}
	next := b.counts[turnID] + n
	if max > 0 && next > max {
		return b.counts[turnID], fmt.Errorf("table_store_quota_exceeded: max_rows_per_turn=%d", max)
	}
	b.counts[turnID] = next
	return next, nil
}

// TableQuotasFunc loads live table-store quotas.
type TableQuotasFunc func() domain.ConfigTableSection

func resolveTableQuotas(fn TableQuotasFunc) domain.ConfigTableSection {
	q := domain.DefaultTableSection()
	if fn != nil {
		got := fn()
		if got.MaxRowsPerUpsert > 0 {
			q.MaxRowsPerUpsert = got.MaxRowsPerUpsert
		}
		if got.MaxRowsPerTurn > 0 {
			q.MaxRowsPerTurn = got.MaxRowsPerTurn
		}
		if got.MaxRowsPerTable > 0 {
			q.MaxRowsPerTable = got.MaxRowsPerTable
		}
		if got.MaxRowBytes > 0 {
			q.MaxRowBytes = got.MaxRowBytes
		}
		if got.MaxTablesPerScope > 0 {
			q.MaxTablesPerScope = got.MaxTablesPerScope
		}
		if got.QueryDefaultLimit > 0 {
			q.QueryDefaultLimit = got.QueryDefaultLimit
		}
		if got.QueryMaxLimit > 0 {
			q.QueryMaxLimit = got.QueryMaxLimit
		}
		if got.MaxRowChars > 0 {
			q.MaxRowChars = got.MaxRowChars
		}
	}
	return q
}

// TableUpsert writes or replaces one schema-free row.
type TableUpsert struct {
	Store  port.TableStoreRepo
	Quotas TableQuotasFunc
	Budget *TableTurnBudget
}

func (h *TableUpsert) Name() string                { return "table_upsert" }
func (h *TableUpsert) RiskLevel() domain.RiskLevel { return domain.RiskLow }
func (h *TableUpsert) Describe(args map[string]any) string {
	table := strVal(args, "table")
	key := strVal(args, "key")
	if table == "" {
		return "table_upsert"
	}
	if key != "" {
		return fmt.Sprintf("table_upsert %s/%s", table, key)
	}
	return "table_upsert " + table
}

func (h *TableUpsert) Schema() domain.ToolSchema {
	return domain.ToolSchema{
		Name: "table_upsert",
		Description: "Write or update one row in the agent table store (schema-free JSON).\n\n" +
			tableStorePolicyHint + "\n\n" +
			"mode=replace overwrites data (default); mode=merge shallow-merges top-level fields.",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"scope": map[string]any{
					"type":        "string",
					"description": "user | project | agent",
					"enum":        []string{"user", "project", "agent"},
				},
				"table": map[string]any{
					"type":        "string",
					"description": "Logical table name, e.g. email_digests ([a-z][a-z0-9_]*)",
				},
				"key": map[string]any{
					"type":        "string",
					"description": "Stable row key, e.g. 2026-07-26",
				},
				"data": map[string]any{
					"type":                 "object",
					"description":          "JSON object payload (schema-free)",
					"additionalProperties": true,
				},
				"mode": map[string]any{
					"type":        "string",
					"description": "replace (default) or merge",
					"enum":        []string{"replace", "merge"},
				},
			},
			"required": []string{"scope", "table", "key", "data"},
		},
	}
}

func (h *TableUpsert) Execute(ctx context.Context, input map[string]any) (domain.ToolResult, error) {
	if h.Store == nil {
		return domain.ToolResult{}, fmt.Errorf("table store is not configured")
	}
	quotas := resolveTableQuotas(h.Quotas)
	scope, scopeID, err := resolveMemoryScope(input, strVal(input, "scope"), true)
	if err != nil {
		return domain.ToolResult{}, err
	}
	table := strings.TrimSpace(strVal(input, "table"))
	key := strings.TrimSpace(strVal(input, "key"))
	mode := strings.ToLower(strings.TrimSpace(strVal(input, "mode")))
	if mode == "" {
		mode = "replace"
	}
	if mode != "replace" && mode != "merge" {
		return domain.ToolResult{}, fmt.Errorf("mode must be replace or merge")
	}
	data, err := asObjectMap(input["data"])
	if err != nil {
		return domain.ToolResult{}, err
	}

	raw, err := json.Marshal(data)
	if err != nil {
		return domain.ToolResult{}, fmt.Errorf("data must be JSON-serializable")
	}
	if len(raw) > quotas.MaxRowBytes {
		return domain.ToolResult{}, fmt.Errorf("table_store_quota_exceeded: max_row_bytes=%d", quotas.MaxRowBytes)
	}

	turnID := strings.TrimSpace(strVal(input, "__turn_id"))
	if _, err := h.Budget.TryAdd(turnID, 1, quotas.MaxRowsPerTurn); err != nil {
		return domain.ToolResult{}, err
	}

	existing, getErr := h.Store.Get(ctx, scope, scopeID, table, key)
	switch {
	case getErr == nil:
		if mode == "merge" {
			merged := map[string]any{}
			for k, v := range existing.Data {
				merged[k] = v
			}
			for k, v := range data {
				merged[k] = v
			}
			data = merged
			raw, _ = json.Marshal(data)
			if len(raw) > quotas.MaxRowBytes {
				return domain.ToolResult{}, fmt.Errorf("table_store_quota_exceeded: max_row_bytes=%d", quotas.MaxRowBytes)
			}
		}
	case strings.Contains(strings.ToLower(getErr.Error()), "not found"):
		count, err := h.Store.CountTable(ctx, scope, scopeID, table)
		if err != nil {
			return domain.ToolResult{}, err
		}
		if count >= int64(quotas.MaxRowsPerTable) {
			return domain.ToolResult{}, fmt.Errorf("table_store_quota_exceeded: max_rows_per_table=%d table=%s", quotas.MaxRowsPerTable, table)
		}
		if count == 0 {
			tables, err := h.Store.ListTables(ctx, []domain.TableScopeRef{{Scope: scope, ScopeID: scopeID}})
			if err != nil {
				return domain.ToolResult{}, err
			}
			if len(tables) >= quotas.MaxTablesPerScope {
				exists := false
				for _, info := range tables {
					if info.Table == table {
						exists = true
						break
					}
				}
				if !exists {
					return domain.ToolResult{}, fmt.Errorf("table_store_quota_exceeded: max_tables_per_scope=%d", quotas.MaxTablesPerScope)
				}
			}
		}
	default:
		return domain.ToolResult{}, getErr
	}

	saved, err := h.Store.Upsert(ctx, domain.TableRow{
		Scope:   scope,
		ScopeID: scopeID,
		Table:   table,
		Key:     key,
		Data:    data,
	})
	if err != nil {
		return domain.ToolResult{}, err
	}

	payload := map[string]any{
		"ok":    true,
		"table": saved.Table,
		"key":   saved.Key,
		"scope": string(saved.Scope),
		"bytes": len(raw),
		"mode":  mode,
	}
	out, _ := json.Marshal(payload)
	return domain.ToolResult{
		Content: string(out),
		Meta:    payload,
	}, nil
}

// TableGet fetches one row by key.
type TableGet struct {
	Store  port.TableStoreRepo
	Quotas TableQuotasFunc
}

func (h *TableGet) Name() string                { return "table_get" }
func (h *TableGet) RiskLevel() domain.RiskLevel { return domain.RiskLow }
func (h *TableGet) Describe(args map[string]any) string {
	table := strVal(args, "table")
	key := strVal(args, "key")
	if table != "" && key != "" {
		return fmt.Sprintf("table_get %s/%s", table, key)
	}
	return "table_get"
}

func (h *TableGet) Schema() domain.ToolSchema {
	return domain.ToolSchema{
		Name:        "table_get",
		Description: "Get one row from the agent table store by exact key.\n\n" + tableStorePolicyHint,
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"scope": map[string]any{"type": "string", "enum": []string{"user", "project", "agent"}},
				"table": map[string]any{"type": "string"},
				"key":   map[string]any{"type": "string"},
			},
			"required": []string{"scope", "table", "key"},
		},
	}
}

func (h *TableGet) Execute(ctx context.Context, input map[string]any) (domain.ToolResult, error) {
	if h.Store == nil {
		return domain.ToolResult{}, fmt.Errorf("table store is not configured")
	}
	quotas := resolveTableQuotas(h.Quotas)
	scope, scopeID, err := resolveMemoryScope(input, strVal(input, "scope"), true)
	if err != nil {
		return domain.ToolResult{}, err
	}
	table := strings.TrimSpace(strVal(input, "table"))
	key := strings.TrimSpace(strVal(input, "key"))
	row, err := h.Store.Get(ctx, scope, scopeID, table, key)
	if err != nil {
		return domain.ToolResult{}, err
	}
	return encodeTableRows([]domain.TableRow{row}, quotas.MaxRowChars, false)
}

// TableQuery lists/filters rows in one table.
type TableQuery struct {
	Store  port.TableStoreRepo
	Quotas TableQuotasFunc
}

func (h *TableQuery) Name() string                { return "table_query" }
func (h *TableQuery) RiskLevel() domain.RiskLevel { return domain.RiskLow }
func (h *TableQuery) Describe(args map[string]any) string {
	if table := strVal(args, "table"); table != "" {
		return "table_query " + table
	}
	return "table_query"
}

func (h *TableQuery) Schema() domain.ToolSchema {
	return domain.ToolSchema{
		Name: "table_query",
		Description: "Query rows from one table. filter supports top-level equality only. " +
			"Always prefer a small limit.\n\n" + tableStorePolicyHint,
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"scope": map[string]any{"type": "string", "enum": []string{"user", "project", "agent"}},
				"table": map[string]any{"type": "string"},
				"filter": map[string]any{
					"type":                 "object",
					"description":          "Top-level equality filter, e.g. {\"date\":\"2026-07-26\"}",
					"additionalProperties": true,
				},
				"order": map[string]any{
					"type":        "string",
					"description": "updated_at_desc (default) | updated_at_asc | key_asc | key_desc",
					"enum":        []string{"updated_at_desc", "updated_at_asc", "key_asc", "key_desc"},
				},
				"limit":  map[string]any{"type": "integer", "description": "Max rows (default 50, hard max 200)"},
				"offset": map[string]any{"type": "integer"},
			},
			"required": []string{"scope", "table"},
		},
	}
}

func (h *TableQuery) Execute(ctx context.Context, input map[string]any) (domain.ToolResult, error) {
	if h.Store == nil {
		return domain.ToolResult{}, fmt.Errorf("table store is not configured")
	}
	quotas := resolveTableQuotas(h.Quotas)
	scope, scopeID, err := resolveMemoryScope(input, strVal(input, "scope"), true)
	if err != nil {
		return domain.ToolResult{}, err
	}
	table := strings.TrimSpace(strVal(input, "table"))
	limit := intVal(input, "limit")
	if limit <= 0 {
		limit = quotas.QueryDefaultLimit
	}
	if limit > quotas.QueryMaxLimit {
		limit = quotas.QueryMaxLimit
	}
	offset := intVal(input, "offset")
	if offset < 0 {
		offset = 0
	}
	filter, err := optionalObjectMap(input["filter"])
	if err != nil {
		return domain.ToolResult{}, err
	}
	rows, err := h.Store.Query(ctx, domain.TableQuery{
		Scope:   scope,
		ScopeID: scopeID,
		Table:   table,
		Filter:  filter,
		Order:   strings.TrimSpace(strVal(input, "order")),
		Limit:   limit,
		Offset:  offset,
	})
	if err != nil {
		return domain.ToolResult{}, err
	}
	truncated := len(rows) >= limit
	return encodeTableQueryResult(table, rows, quotas.MaxRowChars, truncated)
}

// TableDelete deletes one row by key.
type TableDelete struct {
	Store port.TableStoreRepo
}

func (h *TableDelete) Name() string                { return "table_delete" }
func (h *TableDelete) RiskLevel() domain.RiskLevel { return domain.RiskLow }
func (h *TableDelete) Describe(args map[string]any) string {
	table := strVal(args, "table")
	key := strVal(args, "key")
	if table != "" && key != "" {
		return fmt.Sprintf("table_delete %s/%s", table, key)
	}
	return "table_delete"
}

func (h *TableDelete) Schema() domain.ToolSchema {
	return domain.ToolSchema{
		Name:        "table_delete",
		Description: "Delete one row from the agent table store by exact key.\n\n" + tableStorePolicyHint,
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"scope": map[string]any{"type": "string", "enum": []string{"user", "project", "agent"}},
				"table": map[string]any{"type": "string"},
				"key":   map[string]any{"type": "string"},
			},
			"required": []string{"scope", "table", "key"},
		},
	}
}

func (h *TableDelete) Execute(ctx context.Context, input map[string]any) (domain.ToolResult, error) {
	if h.Store == nil {
		return domain.ToolResult{}, fmt.Errorf("table store is not configured")
	}
	scope, scopeID, err := resolveMemoryScope(input, strVal(input, "scope"), true)
	if err != nil {
		return domain.ToolResult{}, err
	}
	table := strings.TrimSpace(strVal(input, "table"))
	key := strings.TrimSpace(strVal(input, "key"))
	if err := h.Store.Delete(ctx, scope, scopeID, table, key); err != nil {
		return domain.ToolResult{}, err
	}
	payload := map[string]any{"ok": true, "table": table, "key": key, "scope": string(scope)}
	raw, _ := json.Marshal(payload)
	return domain.ToolResult{Content: string(raw), Meta: payload}, nil
}

// TableList lists table names and counts visible in this turn.
type TableList struct {
	Store port.TableStoreRepo
}

func (h *TableList) Name() string                { return "table_list" }
func (h *TableList) RiskLevel() domain.RiskLevel { return domain.RiskLow }
func (h *TableList) Describe(args map[string]any) string {
	if scope := strVal(args, "scope"); scope != "" {
		return "table_list " + scope
	}
	return "table_list"
}

func (h *TableList) Schema() domain.ToolSchema {
	return domain.ToolSchema{
		Name: "table_list",
		Description: "List logical tables and row counts for visible scopes " +
			"(user + current project + current agent). Optional scope filter.\n\n" + tableStorePolicyHint,
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"scope": map[string]any{
					"type":        "string",
					"description": "Optional filter: user | project | agent",
					"enum":        []string{"user", "project", "agent"},
				},
			},
		},
	}
}

func (h *TableList) Execute(ctx context.Context, input map[string]any) (domain.ToolResult, error) {
	if h.Store == nil {
		return domain.ToolResult{}, fmt.Errorf("table store is not configured")
	}
	visible, err := visibleMemoryScopes(input)
	if err != nil {
		return domain.ToolResult{}, err
	}
	scopeFilter := strings.TrimSpace(strVal(input, "scope"))
	refs := make([]domain.TableScopeRef, 0, len(visible))
	for _, v := range visible {
		if scopeFilter != "" && string(v.Scope) != scopeFilter {
			continue
		}
		refs = append(refs, domain.TableScopeRef{Scope: v.Scope, ScopeID: v.ScopeID})
	}
	if scopeFilter != "" && len(refs) == 0 {
		if _, _, err := resolveMemoryScope(input, scopeFilter, false); err != nil {
			return domain.ToolResult{}, err
		}
	}
	infos, err := h.Store.ListTables(ctx, refs)
	if err != nil {
		return domain.ToolResult{}, err
	}
	type item struct {
		Scope string `json:"scope"`
		Table string `json:"table"`
		Count int64  `json:"count"`
	}
	items := make([]item, 0, len(infos))
	for _, info := range infos {
		items = append(items, item{Scope: string(info.Scope), Table: info.Table, Count: info.Count})
	}
	raw, err := json.Marshal(items)
	if err != nil {
		return domain.ToolResult{}, err
	}
	return domain.ToolResult{
		Content: string(raw),
		Meta:    map[string]any{"count": len(items)},
	}, nil
}

func asObjectMap(v any) (map[string]any, error) {
	if v == nil {
		return nil, fmt.Errorf("data is required")
	}
	switch m := v.(type) {
	case map[string]any:
		return m, nil
	default:
		raw, err := json.Marshal(v)
		if err != nil {
			return nil, fmt.Errorf("data must be a JSON object")
		}
		var out map[string]any
		if err := json.Unmarshal(raw, &out); err != nil || out == nil {
			return nil, fmt.Errorf("data must be a JSON object")
		}
		return out, nil
	}
}

func optionalObjectMap(v any) (map[string]any, error) {
	if v == nil {
		return nil, nil
	}
	return asObjectMap(v)
}

func intVal(m map[string]any, key string) int {
	v, ok := m[key]
	if !ok || v == nil {
		return 0
	}
	switch n := v.(type) {
	case int:
		return n
	case int32:
		return int(n)
	case int64:
		return int(n)
	case float64:
		return int(n)
	case json.Number:
		i, _ := n.Int64()
		return int(i)
	default:
		return 0
	}
}

func encodeTableRows(rows []domain.TableRow, maxChars int, truncated bool) (domain.ToolResult, error) {
	type item struct {
		Key       string         `json:"key"`
		Data      map[string]any `json:"data"`
		UpdatedAt string         `json:"updated_at"`
	}
	items := make([]item, 0, len(rows))
	for _, r := range rows {
		items = append(items, item{
			Key:       r.Key,
			Data:      truncateTableData(r.Data, maxChars),
			UpdatedAt: r.UpdatedAt.UTC().Format("2006-01-02T15:04:05Z"),
		})
	}
	raw, err := json.Marshal(items)
	if err != nil {
		return domain.ToolResult{}, err
	}
	return domain.ToolResult{
		Content: string(raw),
		Meta: map[string]any{
			"count":     len(items),
			"truncated": truncated,
		},
	}, nil
}

func encodeTableQueryResult(table string, rows []domain.TableRow, maxChars int, truncated bool) (domain.ToolResult, error) {
	type item struct {
		Key       string         `json:"key"`
		Data      map[string]any `json:"data"`
		UpdatedAt string         `json:"updated_at"`
	}
	items := make([]item, 0, len(rows))
	for _, r := range rows {
		items = append(items, item{
			Key:       r.Key,
			Data:      truncateTableData(r.Data, maxChars),
			UpdatedAt: r.UpdatedAt.UTC().Format("2006-01-02T15:04:05Z"),
		})
	}
	payload := map[string]any{
		"table":     table,
		"count":     len(items),
		"truncated": truncated,
		"rows":      items,
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		return domain.ToolResult{}, err
	}
	return domain.ToolResult{Content: string(raw), Meta: payload}, nil
}

func truncateTableData(data map[string]any, maxChars int) map[string]any {
	if maxChars <= 0 || data == nil {
		return data
	}
	raw, err := json.Marshal(data)
	if err != nil || utf8.RuneCountInString(string(raw)) <= maxChars {
		return data
	}
	// Prefer truncating string fields; fall back to summary stub.
	out := map[string]any{}
	for k, v := range data {
		if s, ok := v.(string); ok {
			out[k] = truncateRunes(s, maxChars/2)
			continue
		}
		out[k] = v
	}
	raw2, err := json.Marshal(out)
	if err == nil && utf8.RuneCountInString(string(raw2)) <= maxChars {
		return out
	}
	return map[string]any{"...(truncated)": true, "keys": mapKeys(data)}
}

func mapKeys(m map[string]any) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}
