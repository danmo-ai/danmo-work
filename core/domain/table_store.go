package domain

import "time"

// TableScope reuses MemoryScope values (user | project | agent).
type TableScope = MemoryScope

const (
	TableScopeUser    = MemoryScopeUser
	TableScopeProject = MemoryScopeProject
	TableScopeAgent   = MemoryScopeAgent
	TableUserScopeID  = MemoryUserScopeID
)

// TableRow is one schema-free document in the agent table store.
type TableRow struct {
	Scope     TableScope     `json:"scope"`
	ScopeID   string         `json:"scopeId"`
	Table     string         `json:"table"`
	Key       string         `json:"key"`
	Data      map[string]any `json:"data"`
	CreatedAt time.Time      `json:"createdAt"`
	UpdatedAt time.Time      `json:"updatedAt"`
}

// TableQuery filters rows for table_query / debug API.
type TableQuery struct {
	Scope  TableScope
	ScopeID string
	Table  string
	// Filter is top-level equality only: field → scalar JSON value.
	Filter map[string]any
	// Order: "updated_at_desc" (default) or "updated_at_asc" or "key_asc" / "key_desc".
	Order  string
	Limit  int
	Offset int
}

// TableInfo is a table name with row count in a scope bucket.
type TableInfo struct {
	Scope   TableScope `json:"scope"`
	ScopeID string     `json:"scopeId"`
	Table   string     `json:"table"`
	Count   int64      `json:"count"`
}

// TableScopeRef identifies one concrete scope bucket for list/query visibility.
type TableScopeRef struct {
	Scope   TableScope
	ScopeID string
}

// ConfigTableSection controls agent table-store quotas and query limits.
type ConfigTableSection struct {
	MaxRowsPerUpsert  int `json:"maxRowsPerUpsert" mapstructure:"max_rows_per_upsert"`
	MaxRowsPerTurn    int `json:"maxRowsPerTurn" mapstructure:"max_rows_per_turn"`
	MaxRowsPerTable   int `json:"maxRowsPerTable" mapstructure:"max_rows_per_table"`
	MaxRowBytes       int `json:"maxRowBytes" mapstructure:"max_row_bytes"`
	MaxTablesPerScope int `json:"maxTablesPerScope" mapstructure:"max_tables_per_scope"`
	QueryDefaultLimit int `json:"queryDefaultLimit" mapstructure:"query_default_limit"`
	QueryMaxLimit     int `json:"queryMaxLimit" mapstructure:"query_max_limit"`
	MaxRowChars       int `json:"maxRowChars" mapstructure:"max_row_chars"`
}

// DefaultTableSection returns MVP quota defaults from the landing plan.
func DefaultTableSection() ConfigTableSection {
	return ConfigTableSection{
		MaxRowsPerUpsert:  50,
		MaxRowsPerTurn:    200,
		MaxRowsPerTable:   50000,
		MaxRowBytes:       65536,
		MaxTablesPerScope: 100,
		QueryDefaultLimit: 50,
		QueryMaxLimit:     200,
		MaxRowChars:       8000,
	}
}
