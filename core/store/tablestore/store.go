// Package tablestore is the agent data-plane SQLite store (store.db).
// It must stay separate from core/store/sqlite (work.db control plane).
package tablestore

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"danmo-work/core/domain"
	"danmo-work/core/paths"
	"danmo-work/core/port"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

var _ port.TableStoreRepo = (*Store)(nil)

// Store is an independent SQLite database for schema-free table rows.
type Store struct {
	db *gorm.DB
}

// New opens (or creates) store.db at dbPath.
func New(dbPath string) (*Store, error) {
	if dbPath == "" {
		dbPath = paths.StoreDatabaseFile()
	}
	if abs, err := filepath.Abs(dbPath); err == nil {
		dbPath = abs
	}
	if err := os.MkdirAll(filepath.Dir(dbPath), 0o755); err != nil {
		return nil, err
	}
	db, err := gorm.Open(sqlite.Open(dbPath), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		return nil, err
	}
	sqlDB, err := db.DB()
	if err != nil {
		return nil, err
	}
	sqlDB.SetMaxOpenConns(1)
	sqlDB.SetMaxIdleConns(1)
	if _, err := sqlDB.Exec(`PRAGMA journal_mode=WAL`); err != nil {
		_ = sqlDB.Close()
		return nil, err
	}
	if _, err := sqlDB.Exec(`PRAGMA busy_timeout=5000`); err != nil {
		_ = sqlDB.Close()
		return nil, err
	}
	s := &Store{db: db}
	if err := s.migrate(); err != nil {
		_ = sqlDB.Close()
		return nil, err
	}
	return s, nil
}

func (s *Store) migrate() error {
	return s.db.Exec(`
CREATE TABLE IF NOT EXISTS store_rows (
  scope      TEXT NOT NULL,
  scope_id   TEXT NOT NULL,
  table_name TEXT NOT NULL,
  row_key    TEXT NOT NULL,
  data       TEXT NOT NULL CHECK (json_valid(data) AND json_type(data) = 'object'),
  created_at TEXT NOT NULL,
  updated_at TEXT NOT NULL,
  PRIMARY KEY (scope, scope_id, table_name, row_key)
);
CREATE INDEX IF NOT EXISTS idx_store_rows_list
  ON store_rows (scope, scope_id, table_name, updated_at DESC);
`).Error
}

func (s *Store) Close() error {
	if s == nil || s.db == nil {
		return nil
	}
	sqlDB, err := s.db.DB()
	if err != nil {
		return err
	}
	return sqlDB.Close()
}

type rowModel struct {
	Scope     string `gorm:"column:scope;primaryKey"`
	ScopeID   string `gorm:"column:scope_id;primaryKey"`
	Tbl       string `gorm:"column:table_name;primaryKey"`
	RowKey    string `gorm:"column:row_key;primaryKey"`
	Data      string `gorm:"column:data"`
	CreatedAt string `gorm:"column:created_at"`
	UpdatedAt string `gorm:"column:updated_at"`
}

func (rowModel) TableName() string { return "store_rows" }

func (s *Store) Upsert(ctx context.Context, row domain.TableRow) (domain.TableRow, error) {
	if err := validateIdentity(row.Scope, row.ScopeID, row.Table, row.Key); err != nil {
		return domain.TableRow{}, err
	}
	if row.Data == nil {
		row.Data = map[string]any{}
	}
	raw, err := json.Marshal(row.Data)
	if err != nil {
		return domain.TableRow{}, fmt.Errorf("data must be JSON-serializable: %w", err)
	}
	now := time.Now().UTC()
	nowStr := now.Format(time.RFC3339Nano)

	var existing rowModel
	err = s.db.WithContext(ctx).
		Where("scope = ? AND scope_id = ? AND table_name = ? AND row_key = ?",
			string(row.Scope), row.ScopeID, row.Table, row.Key).
		First(&existing).Error
	createdAt := nowStr
	if err == nil {
		createdAt = existing.CreatedAt
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		return domain.TableRow{}, err
	}

	m := rowModel{
		Scope:     string(row.Scope),
		ScopeID:   row.ScopeID,
		Tbl:       row.Table,
		RowKey:    row.Key,
		Data:      string(raw),
		CreatedAt: createdAt,
		UpdatedAt: nowStr,
	}
	if err := s.db.WithContext(ctx).Save(&m).Error; err != nil {
		return domain.TableRow{}, err
	}
	return modelToRow(m)
}

func (s *Store) Get(ctx context.Context, scope domain.TableScope, scopeID, table, key string) (domain.TableRow, error) {
	if err := validateIdentity(scope, scopeID, table, key); err != nil {
		return domain.TableRow{}, err
	}
	var m rowModel
	err := s.db.WithContext(ctx).
		Where("scope = ? AND scope_id = ? AND table_name = ? AND row_key = ?",
			string(scope), scopeID, table, key).
		First(&m).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return domain.TableRow{}, fmt.Errorf("row not found")
	}
	if err != nil {
		return domain.TableRow{}, err
	}
	return modelToRow(m)
}

func (s *Store) Query(ctx context.Context, q domain.TableQuery) ([]domain.TableRow, error) {
	if err := validateScope(q.Scope, q.ScopeID); err != nil {
		return nil, err
	}
	if err := validateTableName(q.Table); err != nil {
		return nil, err
	}

	limit := q.Limit
	if limit <= 0 {
		limit = 50
	}
	offset := q.Offset
	if offset < 0 {
		offset = 0
	}

	orderSQL := "updated_at DESC"
	switch strings.ToLower(strings.TrimSpace(q.Order)) {
	case "", "updated_at_desc":
		orderSQL = "updated_at DESC"
	case "updated_at_asc":
		orderSQL = "updated_at ASC"
	case "key_asc":
		orderSQL = "row_key ASC"
	case "key_desc":
		orderSQL = "row_key DESC"
	default:
		return nil, fmt.Errorf("order must be updated_at_desc, updated_at_asc, key_asc, or key_desc")
	}

	query := s.db.WithContext(ctx).Model(&rowModel{}).
		Where("scope = ? AND scope_id = ? AND table_name = ?",
			string(q.Scope), q.ScopeID, q.Table)

	for field, val := range q.Filter {
		path, err := jsonPath(field)
		if err != nil {
			return nil, err
		}
		switch v := val.(type) {
		case nil:
			query = query.Where("json_extract(data, ?) IS NULL", path)
		case bool:
			bit := 0
			if v {
				bit = 1
			}
			query = query.Where("json_extract(data, ?) = ?", path, bit)
		case float64:
			query = query.Where("CAST(json_extract(data, ?) AS REAL) = ?", path, v)
		case float32:
			query = query.Where("CAST(json_extract(data, ?) AS REAL) = ?", path, float64(v))
		case int:
			query = query.Where("CAST(json_extract(data, ?) AS INTEGER) = ?", path, v)
		case int32:
			query = query.Where("CAST(json_extract(data, ?) AS INTEGER) = ?", path, int64(v))
		case int64:
			query = query.Where("CAST(json_extract(data, ?) AS INTEGER) = ?", path, v)
		case json.Number:
			query = query.Where("CAST(json_extract(data, ?) AS TEXT) = ?", path, v.String())
		case string:
			query = query.Where("CAST(json_extract(data, ?) AS TEXT) = ?", path, v)
		default:
			raw, err := json.Marshal(v)
			if err != nil {
				return nil, fmt.Errorf("filter.%s: invalid value", field)
			}
			var s string
			if err := json.Unmarshal(raw, &s); err == nil {
				query = query.Where("CAST(json_extract(data, ?) AS TEXT) = ?", path, s)
			} else {
				query = query.Where("CAST(json_extract(data, ?) AS TEXT) = ?", path, string(raw))
			}
		}
	}

	var models []rowModel
	if err := query.Order(orderSQL).Limit(limit).Offset(offset).Find(&models).Error; err != nil {
		return nil, err
	}
	out := make([]domain.TableRow, 0, len(models))
	for _, m := range models {
		row, err := modelToRow(m)
		if err != nil {
			return nil, err
		}
		out = append(out, row)
	}
	return out, nil
}

func (s *Store) Delete(ctx context.Context, scope domain.TableScope, scopeID, table, key string) error {
	if err := validateIdentity(scope, scopeID, table, key); err != nil {
		return err
	}
	res := s.db.WithContext(ctx).
		Where("scope = ? AND scope_id = ? AND table_name = ? AND row_key = ?",
			string(scope), scopeID, table, key).
		Delete(&rowModel{})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return fmt.Errorf("row not found")
	}
	return nil
}

func (s *Store) CountTable(ctx context.Context, scope domain.TableScope, scopeID, table string) (int64, error) {
	if err := validateTableName(table); err != nil {
		return 0, err
	}
	if err := validateScope(scope, scopeID); err != nil {
		return 0, err
	}
	var n int64
	err := s.db.WithContext(ctx).Model(&rowModel{}).
		Where("scope = ? AND scope_id = ? AND table_name = ?",
			string(scope), scopeID, table).
		Count(&n).Error
	return n, err
}

func (s *Store) ListTables(ctx context.Context, scopes []domain.TableScopeRef) ([]domain.TableInfo, error) {
	if len(scopes) == 0 {
		return nil, nil
	}
	var out []domain.TableInfo
	for _, ref := range scopes {
		if err := validateScope(ref.Scope, ref.ScopeID); err != nil {
			return nil, err
		}
		type agg struct {
			Tbl   string `gorm:"column:table_name"`
			Count int64  `gorm:"column:count"`
		}
		var rows []agg
		err := s.db.WithContext(ctx).Model(&rowModel{}).
			Select("table_name, COUNT(*) as count").
			Where("scope = ? AND scope_id = ?", string(ref.Scope), ref.ScopeID).
			Group("table_name").
			Order("table_name ASC").
			Scan(&rows).Error
		if err != nil {
			return nil, err
		}
		for _, r := range rows {
			out = append(out, domain.TableInfo{
				Scope:   ref.Scope,
				ScopeID: ref.ScopeID,
				Table:   r.Tbl,
				Count:   r.Count,
			})
		}
	}
	return out, nil
}

func modelToRow(m rowModel) (domain.TableRow, error) {
	var data map[string]any
	if err := json.Unmarshal([]byte(m.Data), &data); err != nil {
		return domain.TableRow{}, err
	}
	if data == nil {
		data = map[string]any{}
	}
	created, _ := time.Parse(time.RFC3339Nano, m.CreatedAt)
	if created.IsZero() {
		created, _ = time.Parse(time.RFC3339, m.CreatedAt)
	}
	updated, _ := time.Parse(time.RFC3339Nano, m.UpdatedAt)
	if updated.IsZero() {
		updated, _ = time.Parse(time.RFC3339, m.UpdatedAt)
	}
	return domain.TableRow{
		Scope:     domain.TableScope(m.Scope),
		ScopeID:   m.ScopeID,
		Table:     m.Tbl,
		Key:       m.RowKey,
		Data:      data,
		CreatedAt: created.UTC(),
		UpdatedAt: updated.UTC(),
	}, nil
}

func validateIdentity(scope domain.TableScope, scopeID, table, key string) error {
	if err := validateScope(scope, scopeID); err != nil {
		return err
	}
	if err := validateTableName(table); err != nil {
		return err
	}
	if err := validateRowKey(key); err != nil {
		return err
	}
	return nil
}

func validateScope(scope domain.TableScope, scopeID string) error {
	switch scope {
	case domain.TableScopeUser, domain.TableScopeProject, domain.TableScopeAgent:
	default:
		return fmt.Errorf("scope must be user, project, or agent")
	}
	if strings.TrimSpace(scopeID) == "" {
		return fmt.Errorf("scope_id is required")
	}
	return nil
}

func validateTableName(name string) error {
	name = strings.TrimSpace(name)
	if name == "" {
		return fmt.Errorf("table is required")
	}
	if len(name) > 64 {
		return fmt.Errorf("table name too long (max 64)")
	}
	for i, r := range name {
		if i == 0 {
			if r < 'a' || r > 'z' {
				return fmt.Errorf("table must match [a-z][a-z0-9_]{0,63}")
			}
			continue
		}
		if (r < 'a' || r > 'z') && (r < '0' || r > '9') && r != '_' {
			return fmt.Errorf("table must match [a-z][a-z0-9_]{0,63}")
		}
	}
	return nil
}

func validateRowKey(key string) error {
	key = strings.TrimSpace(key)
	if key == "" {
		return fmt.Errorf("key is required")
	}
	if len(key) > 256 {
		return fmt.Errorf("key too long (max 256)")
	}
	if strings.ContainsRune(key, 0) {
		return fmt.Errorf("key must not contain null bytes")
	}
	return nil
}

func jsonPath(field string) (string, error) {
	field = strings.TrimSpace(field)
	if field == "" {
		return "", fmt.Errorf("filter field is required")
	}
	if len(field) > 64 {
		return "", fmt.Errorf("filter field too long")
	}
	for i, r := range field {
		ok := (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || r == '_'
		if i > 0 {
			ok = ok || (r >= '0' && r <= '9')
		}
		if !ok {
			return "", fmt.Errorf("filter field must be a simple identifier")
		}
	}
	return "$." + field, nil
}

// Ping checks the underlying SQL connection.
func (s *Store) Ping(ctx context.Context) error {
	sqlDB, err := s.db.DB()
	if err != nil {
		return err
	}
	return sqlDB.PingContext(ctx)
}
