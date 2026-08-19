package sqlite

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"danmo-work/core/domain"
	"danmo-work/core/paths"
	"danmo-work/core/port"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

var _ port.Repository = (*Store)(nil)

type Store struct {
	mu     sync.Mutex
	db     *gorm.DB
	dbPath string
}

func New(dbPath string) (*Store, error) {
	if dbPath == "" {
		dbPath = paths.DatabaseFile()
	}
	if abs, err := filepath.Abs(dbPath); err == nil {
		dbPath = abs
	}
	s := &Store{dbPath: dbPath}
	if err := s.open(); err != nil {
		return nil, err
	}
	if err := s.migrate(); err != nil {
		_ = s.closeUnlocked()
		return nil, fmt.Errorf("migrate database %s: %w", dbPath, err)
	}
	return s, nil
}

func (s *Store) open() error {
	dir := filepath.Dir(s.dbPath)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create database dir %s: %w", dir, err)
	}
	// Probe directory writability early — SQLite reports cryptic READONLY_* codes
	// (1032 DBMOVED / 1544 DIRECTORY) when the parent dir cannot host journals/WAL.
	probe := filepath.Join(dir, ".work-db-write-probe")
	if err := os.WriteFile(probe, []byte("ok"), 0o644); err != nil {
		return fmt.Errorf("database directory not writable (%s): %w", dir, err)
	}
	_ = os.Remove(probe)

	db, err := gorm.Open(sqlite.Open(s.dbPath), &gorm.Config{})
	if err != nil {
		return fmt.Errorf("open database %s: %w", s.dbPath, err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		return fmt.Errorf("database sql handle %s: %w", s.dbPath, err)
	}
	// Pure-Go SQLite + concurrent pool connections is a common source of
	// SQLITE_BUSY / SQLITE_READONLY_DBMOVED (1032) under desktop sidecar load.
	sqlDB.SetMaxOpenConns(1)
	sqlDB.SetMaxIdleConns(1)
	if _, err := sqlDB.Exec(`PRAGMA journal_mode=WAL`); err != nil {
		_ = sqlDB.Close()
		return fmt.Errorf("database WAL mode %s: %w", s.dbPath, err)
	}
	if _, err := sqlDB.Exec(`PRAGMA busy_timeout=5000`); err != nil {
		_ = sqlDB.Close()
		return fmt.Errorf("database busy_timeout %s: %w", s.dbPath, err)
	}
	// Fail fast if the connection cannot write (e.g. file replaced / inode moved).
	if _, err := sqlDB.Exec(`BEGIN IMMEDIATE; ROLLBACK;`); err != nil {
		_ = sqlDB.Close()
		return fmt.Errorf("database not writable %s: %w", s.dbPath, err)
	}
	s.db = db
	return nil
}

func (s *Store) closeUnlocked() error {
	if s.db == nil {
		return nil
	}
	sqlDB, err := s.db.DB()
	s.db = nil
	if err != nil {
		return err
	}
	return sqlDB.Close()
}

// Path returns the absolute SQLite file path.
func (s *Store) Path() string { return s.dbPath }

func (s *Store) DB() *gorm.DB { return s.db }

func isDBMovedErr(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "readonly database") ||
		strings.Contains(msg, "1032") ||
		strings.Contains(msg, "dbmoved")
}

// withWrite retries once after reopening when SQLite reports DBMOVED/readonly.
func (s *Store) withWrite(fn func(*gorm.DB) error) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	err := fn(s.db)
	if err == nil || !isDBMovedErr(err) {
		return err
	}
	_ = s.closeUnlocked()
	if reopenErr := s.open(); reopenErr != nil {
		return fmt.Errorf("%w (reopen %s failed: %v)", err, s.dbPath, reopenErr)
	}
	return fn(s.db)
}

func (s *Store) migrate() error {
	if err := s.db.AutoMigrate(
		&sessionModel{},
		&projectModel{},
		&llmConfigModel{},
		&approvalModel{},
		&pendingMessageModel{},
		&memoryModel{},
		&streamEventModel{},
		&turnModel{},
		&turnLogEntryModel{},
		&secretModel{},
		&automationModel{},
		&weixinAccountModel{},
		&weixinBindingModel{},
		&channelBindingModel{},
		&usageRollupModel{},
		&appMetaModel{},
	); err != nil {
		return err
	}
	migrator := s.db.Migrator()
	if migrator.HasColumn(&llmConfigModel{}, "default_model") {
		if err := migrator.DropColumn(&llmConfigModel{}, "default_model"); err != nil {
			return err
		}
	}
	if migrator.HasColumn(&llmConfigModel{}, "is_active") {
		if err := migrator.DropColumn(&llmConfigModel{}, "is_active"); err != nil {
			return err
		}
	}
	if err := migrateKnowledgeSchema(s.db); err != nil {
		return err
	}
	return nil
}

type appMetaModel struct {
	Key   string `gorm:"primaryKey;column:key"`
	Value string `gorm:"column:value"`
}

func (appMetaModel) TableName() string { return "app_meta" }
func (s *Store) Sessions() port.SessionRepo               { return &sessionRepo{s} }
func (s *Store) Projects() port.ProjectRepo               { return &projectRepo{s} }
func (s *Store) LLMConfig() port.LLMConfigRepo            { return &llmConfigRepo{s} }
func (s *Store) Approvals() port.ApprovalRepo             { return &approvalRepo{s} }
func (s *Store) PendingMessages() port.PendingMessageRepo { return &pendingMessageRepo{s} }
func (s *Store) StreamEvents() port.StreamEventRepo       { return &streamEventRepo{s} }
func (s *Store) Turns() port.TurnRepo                     { return &turnRepo{s} }
func (s *Store) TurnLogs() port.TurnLogRepo               { return &turnLogRepo{s} }
func (s *Store) Secrets() port.SecretStore                { return newSecretStore(s.db) }
func (s *Store) Automations() port.AutomationRepo         { return &automationRepo{s} }
func (s *Store) Memories() port.MemoryRepo                { return &memoryRepo{s} }
func (s *Store) WeixinAccounts() port.WeixinAccountRepo   { return &weixinAccountRepo{s} }
func (s *Store) WeixinBindings() port.WeixinBindingRepo   { return &weixinBindingRepo{s} }
func (s *Store) ChannelBindings() port.ChannelBindingRepo { return &channelBindingRepo{s} }
func (s *Store) AppMeta() port.AppMetaRepo                { return &appMetaRepo{s} }

// ---- SessionRepo ----

type sessionRepo struct{ s *Store }

func (r *sessionRepo) Create(ctx context.Context, s domain.Session) error {
	m := sessionFromDomain(s)
	if err := r.s.withWrite(func(db *gorm.DB) error {
		return db.WithContext(ctx).Create(&m).Error
	}); err != nil {
		return fmt.Errorf("create session (%s): %w", r.s.dbPath, err)
	}
	return nil
}

func (r *sessionRepo) Update(ctx context.Context, s domain.Session) error {
	m := sessionFromDomain(s)
	return r.s.db.WithContext(ctx).Model(&sessionModel{}).Where("id = ?", s.ID).Updates(map[string]any{
		"title":      m.Title,
		"project_id": m.ProjectID,
		"agent_id":   m.AgentID,
		"model_id":   m.ModelID,
		"plan_mode":  m.PlanMode,
		"content":    m.Content,
		"status":     m.Status,
		"updated_at": m.UpdatedAt,
	}).Error
}

func (r *sessionRepo) Delete(ctx context.Context, id string) error {
	return r.s.db.WithContext(ctx).Delete(&sessionModel{}, "id = ?", id).Error
}

func (r *sessionRepo) Get(ctx context.Context, id string) (domain.Session, error) {
	var row sessionModel
	if err := r.s.db.WithContext(ctx).First(&row, "id = ?", id).Error; err != nil {
		return domain.Session{}, err
	}
	return sessionToDomain(row), nil
}

func (r *sessionRepo) List(ctx context.Context) ([]domain.Session, error) {
	var rows []sessionModel
	if err := r.s.db.WithContext(ctx).Order("created_at desc").Find(&rows).Error; err != nil {
		return nil, err
	}
	out := make([]domain.Session, len(rows))
	for i, row := range rows {
		out[i] = sessionToDomain(row)
	}
	return out, nil
}

func (r *sessionRepo) ListByProject(ctx context.Context, projectID string) ([]domain.Session, error) {
	var rows []sessionModel
	if err := r.s.db.WithContext(ctx).Where("project_id = ?", projectID).Order("created_at desc").Find(&rows).Error; err != nil {
		return nil, err
	}
	out := make([]domain.Session, len(rows))
	for i, row := range rows {
		out[i] = sessionToDomain(row)
	}
	return out, nil
}

// ---- ProjectRepo ----

type projectRepo struct{ s *Store }

func (r *projectRepo) Create(ctx context.Context, p domain.Project) error {
	m := projectFromDomain(p)
	return r.s.db.WithContext(ctx).Create(&m).Error
}

func (r *projectRepo) Update(ctx context.Context, p domain.Project) error {
	return r.s.db.WithContext(ctx).Model(&projectModel{}).Where("id = ?", p.ID).Updates(projectFromDomain(p)).Error
}

func (r *projectRepo) Get(ctx context.Context, id string) (domain.Project, error) {
	var row projectModel
	if err := r.s.db.WithContext(ctx).First(&row, "id = ?", id).Error; err != nil {
		return domain.Project{}, err
	}
	return projectToDomain(row), nil
}

func (r *projectRepo) List(ctx context.Context) ([]domain.Project, error) {
	var rows []projectModel
	if err := r.s.db.WithContext(ctx).Order("created_at desc").Find(&rows).Error; err != nil {
		return nil, err
	}
	out := make([]domain.Project, len(rows))
	for i, row := range rows {
		out[i] = projectToDomain(row)
	}
	return out, nil
}

func (r *projectRepo) Delete(ctx context.Context, id string) error {
	return r.s.db.WithContext(ctx).Delete(&projectModel{}, "id = ?", id).Error
}

// ---- LLMConfigRepo ----

type llmConfigRepo struct{ s *Store }

func (r *llmConfigRepo) GetAll(ctx context.Context) ([]domain.LLMProviderConfig, error) {
	var rows []llmConfigModel
	if err := r.s.db.WithContext(ctx).Find(&rows).Error; err != nil {
		return nil, err
	}
	out := make([]domain.LLMProviderConfig, len(rows))
	for i, row := range rows {
		out[i] = row.toDomain()
	}
	return out, nil
}

func (r *llmConfigRepo) GetByID(ctx context.Context, id string) (domain.LLMProviderConfig, error) {
	var row llmConfigModel
	if err := r.s.db.WithContext(ctx).First(&row, "id = ?", id).Error; err != nil {
		return domain.LLMProviderConfig{}, err
	}
	return row.toDomain(), nil
}

func (r *llmConfigRepo) Upsert(ctx context.Context, cfg domain.LLMProviderConfig) error {
	row := llmConfigModelFromDomain(cfg)
	var existing llmConfigModel
	if err := r.s.db.WithContext(ctx).First(&existing, "id = ?", cfg.ID).Error; err != nil {
		return r.s.db.WithContext(ctx).Create(&row).Error
	}
	row.CreatedAt = existing.CreatedAt
	return r.s.db.WithContext(ctx).Model(&existing).Updates(&row).Error
}

func (r *llmConfigRepo) Delete(ctx context.Context, id string) error {
	return r.s.db.WithContext(ctx).Delete(&llmConfigModel{}, "id = ?", id).Error
}

// ---- ApprovalRepo ----

type approvalRepo struct{ s *Store }

func (r *approvalRepo) Create(ctx context.Context, a domain.Approval) error {
	m := approvalFromDomain(a)
	return r.s.db.WithContext(ctx).Create(&m).Error
}

func (r *approvalRepo) Get(ctx context.Context, id string) (domain.Approval, error) {
	var row approvalModel
	if err := r.s.db.WithContext(ctx).First(&row, "id = ?", id).Error; err != nil {
		return domain.Approval{}, err
	}
	return approvalToDomain(row), nil
}

func (r *approvalRepo) Update(ctx context.Context, a domain.Approval) error {
	return r.s.db.WithContext(ctx).Model(&approvalModel{}).Where("id = ?", a.ID).Updates(approvalFromDomain(a)).Error
}

func (r *approvalRepo) ListByStatus(ctx context.Context, status string) ([]domain.Approval, error) {
	var rows []approvalModel
	if err := r.s.db.WithContext(ctx).Where("status = ?", status).Find(&rows).Error; err != nil {
		return nil, err
	}
	out := make([]domain.Approval, len(rows))
	for i, row := range rows {
		out[i] = approvalToDomain(row)
	}
	return out, nil
}

// ---- PendingMessageRepo ----

type pendingMessageRepo struct{ s *Store }

func (r *pendingMessageRepo) ListBySession(ctx context.Context, sessionID string) ([]domain.PendingMessage, error) {
	var rows []pendingMessageModel
	if err := r.s.db.WithContext(ctx).
		Where("session_id = ? AND status IN ?", sessionID, []string{
			string(domain.PendingQueued),
			string(domain.PendingSteering),
		}).
		Order("position ASC, created_at ASC").
		Find(&rows).Error; err != nil {
		return nil, err
	}
	out := make([]domain.PendingMessage, len(rows))
	for i, row := range rows {
		out[i] = pendingMessageToDomain(row)
	}
	return out, nil
}

func (r *pendingMessageRepo) Get(ctx context.Context, id string) (domain.PendingMessage, error) {
	var row pendingMessageModel
	if err := r.s.db.WithContext(ctx).First(&row, "id = ?", id).Error; err != nil {
		return domain.PendingMessage{}, err
	}
	return pendingMessageToDomain(row), nil
}

func (r *pendingMessageRepo) Create(ctx context.Context, m domain.PendingMessage) error {
	row := pendingMessageFromDomain(m)
	return r.s.db.WithContext(ctx).Create(&row).Error
}

func (r *pendingMessageRepo) Update(ctx context.Context, m domain.PendingMessage) error {
	row := pendingMessageFromDomain(m)
	return r.s.db.WithContext(ctx).Model(&pendingMessageModel{}).Where("id = ?", m.ID).Updates(map[string]any{
		"content":          row.Content,
		"attachments_json": row.AttachmentsJSON,
		"position":         row.Position,
		"status":           row.Status,
		"agent_id":         row.AgentID,
		"model_id":         row.ModelID,
		"updated_at":       row.UpdatedAt,
	}).Error
}

func (r *pendingMessageRepo) Delete(ctx context.Context, id string) error {
	return r.s.db.WithContext(ctx).Delete(&pendingMessageModel{}, "id = ?", id).Error
}

func (r *pendingMessageRepo) DeleteBySession(ctx context.Context, sessionID string) error {
	return r.s.db.WithContext(ctx).
		Where("session_id = ? AND status IN ?", sessionID, []string{
			string(domain.PendingQueued),
			string(domain.PendingSteering),
		}).
		Delete(&pendingMessageModel{}).Error
}

func (r *pendingMessageRepo) MaxPosition(ctx context.Context, sessionID string) (int, error) {
	var maxPos *int
	err := r.s.db.WithContext(ctx).Model(&pendingMessageModel{}).
		Select("MAX(position)").
		Where("session_id = ? AND status IN ?", sessionID, []string{
			string(domain.PendingQueued),
			string(domain.PendingSteering),
		}).
		Scan(&maxPos).Error
	if err != nil {
		return 0, err
	}
	if maxPos == nil {
		return 0, nil
	}
	return *maxPos, nil
}

func (r *pendingMessageRepo) PopFront(ctx context.Context, sessionID string) (domain.PendingMessage, bool, error) {
	r.s.mu.Lock()
	defer r.s.mu.Unlock()
	var row pendingMessageModel
	err := r.s.db.WithContext(ctx).
		Where("session_id = ? AND status = ?", sessionID, string(domain.PendingQueued)).
		Order("position ASC, created_at ASC").
		First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return domain.PendingMessage{}, false, nil
	}
	if err != nil {
		return domain.PendingMessage{}, false, err
	}
	now := time.Now().UTC()
	if err := r.s.db.WithContext(ctx).Model(&pendingMessageModel{}).Where("id = ?", row.ID).Updates(map[string]any{
		"status":     string(domain.PendingSending),
		"updated_at": now,
	}).Error; err != nil {
		return domain.PendingMessage{}, false, err
	}
	row.Status = string(domain.PendingSending)
	row.UpdatedAt = now
	return pendingMessageToDomain(row), true, nil
}

func (r *pendingMessageRepo) ClaimSteering(ctx context.Context, sessionID string) ([]domain.PendingMessage, error) {
	r.s.mu.Lock()
	defer r.s.mu.Unlock()
	var rows []pendingMessageModel
	if err := r.s.db.WithContext(ctx).
		Where("session_id = ? AND status = ?", sessionID, string(domain.PendingSteering)).
		Order("position ASC, created_at ASC").
		Find(&rows).Error; err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, nil
	}
	ids := make([]string, len(rows))
	out := make([]domain.PendingMessage, len(rows))
	for i, row := range rows {
		ids[i] = row.ID
		out[i] = pendingMessageToDomain(row)
	}
	if err := r.s.db.WithContext(ctx).
		Where("id IN ?", ids).
		Delete(&pendingMessageModel{}).Error; err != nil {
		return nil, err
	}
	return out, nil
}

func (r *pendingMessageRepo) DemoteSteering(ctx context.Context, sessionID string) error {
	return r.s.db.WithContext(ctx).Model(&pendingMessageModel{}).
		Where("session_id = ? AND status = ?", sessionID, string(domain.PendingSteering)).
		Updates(map[string]any{
			"status":     string(domain.PendingQueued),
			"updated_at": time.Now().UTC(),
		}).Error
}

func (r *pendingMessageRepo) Reorder(ctx context.Context, sessionID string, ids []string) error {
	return r.s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		for i, id := range ids {
			res := tx.Model(&pendingMessageModel{}).
				Where("id = ? AND session_id = ? AND status IN ?", id, sessionID, []string{
					string(domain.PendingQueued),
					string(domain.PendingSteering),
				}).
				Updates(map[string]any{
					"position":   i + 1,
					"updated_at": time.Now().UTC(),
				})
			if res.Error != nil {
				return res.Error
			}
		}
		return nil
	})
}

// ---- StreamEventRepo ----

type streamEventRepo struct{ s *Store }

func (r *streamEventRepo) Save(ctx context.Context, ev domain.StreamEvent) error {
	m := streamEventModel{
		SessionID: ev.SessionID, TurnID: ev.TurnID, Seq: ev.Seq,
		Type: ev.Type, Payload: string(ev.Payload), CreatedAt: ev.CreatedAt,
	}
	return r.s.db.WithContext(ctx).Create(&m).Error
}

func (r *streamEventRepo) ListBySession(ctx context.Context, sessionID string, since int64) ([]domain.StreamEvent, error) {
	var rows []streamEventModel
	q := r.s.db.WithContext(ctx).Where("session_id = ?", sessionID)
	if since > 0 {
		q = q.Where("seq > ?", since)
	}
	if err := q.Order("seq asc").Find(&rows).Error; err != nil {
		return nil, err
	}
	out := make([]domain.StreamEvent, len(rows))
	for i, row := range rows {
		out[i] = streamEventToDomain(row)
	}
	return out, nil
}

func (r *streamEventRepo) MaxSeq() int64 {
	var max int64
	r.s.db.Model(&streamEventModel{}).Select("COALESCE(MAX(seq), 0)").Scan(&max)
	return max
}

// ---- TurnRepo ----

type turnRepo struct{ s *Store }

func (r *turnRepo) Create(ctx context.Context, t domain.TurnLog) error {
	m := turnFromDomain(t)
	var existing turnModel
	if err := r.s.db.WithContext(ctx).First(&existing, "id = ?", t.ID).Error; err != nil {
		return r.s.db.WithContext(ctx).Create(&m).Error
	}
	return nil
}

func (r *turnRepo) UpdateStatus(ctx context.Context, id string, status domain.TurnStatus) error {
	return r.s.db.WithContext(ctx).Model(&turnModel{}).Where("id = ?", id).Update("status", string(status)).Error
}

func (r *turnRepo) Get(ctx context.Context, id string) (domain.TurnLog, error) {
	var row turnModel
	if err := r.s.db.WithContext(ctx).First(&row, "id = ?", id).Error; err != nil {
		return domain.TurnLog{}, err
	}
	return turnToDomain(row), nil
}

func (r *turnRepo) ListBySession(ctx context.Context, sessionID string) ([]domain.TurnLog, error) {
	var rows []turnModel
	if err := r.s.db.WithContext(ctx).Where("session_id = ?", sessionID).Order("id asc").Find(&rows).Error; err != nil {
		return nil, err
	}
	out := make([]domain.TurnLog, len(rows))
	for i, row := range rows {
		out[i] = turnToDomain(row)
	}
	return out, nil
}

func (r *turnRepo) ListByStatus(ctx context.Context, status domain.TurnStatus) ([]domain.TurnLog, error) {
	var rows []turnModel
	if err := r.s.db.WithContext(ctx).Where("status = ?", string(status)).Find(&rows).Error; err != nil {
		return nil, err
	}
	out := make([]domain.TurnLog, len(rows))
	for i, row := range rows {
		out[i] = turnToDomain(row)
	}
	return out, nil
}

// ---- MemoryRepo ----

type memoryModel struct {
	ID        string    `gorm:"primaryKey"`
	Scope     string    `gorm:"uniqueIndex:idx_memory_scope_key;not null"`
	ScopeID   string    `gorm:"column:scope_id;uniqueIndex:idx_memory_scope_key;not null"`
	Key       string    `gorm:"uniqueIndex:idx_memory_scope_key;not null"`
	Content   string    `gorm:"not null"`
	UpdatedAt time.Time `gorm:"not null"`
}

func (memoryModel) TableName() string { return "memories" }

type memoryRepo struct{ s *Store }

func (r *memoryRepo) Upsert(ctx context.Context, m domain.Memory) (domain.Memory, error) {
	now := time.Now().UTC()
	existing, err := r.GetByKey(ctx, m.Scope, m.ScopeID, m.Key)
	if err == nil {
		m.ID = existing.ID
		m.UpdatedAt = now
		row := memoryToModel(m)
		if err := r.s.db.WithContext(ctx).Save(&row).Error; err != nil {
			return domain.Memory{}, err
		}
		return m, nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return domain.Memory{}, err
	}
	if m.ID == "" {
		m.ID = fmt.Sprintf("mem-%d", now.UnixNano())
	}
	m.UpdatedAt = now
	row := memoryToModel(m)
	if err := r.s.db.WithContext(ctx).Create(&row).Error; err != nil {
		return domain.Memory{}, err
	}
	return m, nil
}

func (r *memoryRepo) GetByKey(ctx context.Context, scope domain.MemoryScope, scopeID, key string) (domain.Memory, error) {
	var row memoryModel
	err := r.s.db.WithContext(ctx).
		Where("scope = ? AND scope_id = ? AND key = ?", string(scope), scopeID, key).
		First(&row).Error
	if err != nil {
		return domain.Memory{}, err
	}
	return memoryToDomain(row), nil
}

func (r *memoryRepo) Search(ctx context.Context, q domain.MemoryQuery) ([]domain.Memory, error) {
	if len(q.Scopes) == 0 && !q.IncludeAllAgents {
		return nil, nil
	}

	clauses := make([]string, 0, len(q.Scopes)+1)
	args := make([]any, 0, len(q.Scopes)*2)
	for _, ref := range q.Scopes {
		clauses = append(clauses, "(scope = ? AND scope_id = ?)")
		args = append(args, string(ref.Scope), ref.ScopeID)
	}
	if q.IncludeAllAgents {
		clauses = append(clauses, "scope = ?")
		args = append(args, string(domain.MemoryScopeAgent))
	}
	if len(clauses) == 0 {
		return nil, nil
	}

	db := r.s.db.WithContext(ctx).Model(&memoryModel{}).
		Where(strings.Join(clauses, " OR "), args...)

	if q.Scope != "" {
		db = db.Where("scope = ?", string(q.Scope))
	}
	if q.Key != "" {
		db = db.Where("key = ?", q.Key)
	}

	var rows []memoryModel
	if err := db.Order("updated_at DESC").Find(&rows).Error; err != nil {
		return nil, err
	}

	out := make([]domain.Memory, 0, len(rows))
	query := strings.ToLower(strings.TrimSpace(q.Query))
	for _, row := range rows {
		m := memoryToDomain(row)
		if query != "" {
			hay := strings.ToLower(m.Key + " " + m.Content)
			score := 0
			for _, w := range strings.Fields(query) {
				if strings.Contains(hay, w) {
					score++
				}
			}
			if score == 0 && !strings.Contains(hay, query) {
				continue
			}
		}
		out = append(out, m)
	}

	if q.TopK > 0 && len(out) > q.TopK {
		out = out[:q.TopK]
	}
	return out, nil
}

func (r *memoryRepo) Delete(ctx context.Context, scope domain.MemoryScope, scopeID, key string) error {
	return r.s.db.WithContext(ctx).
		Where("scope = ? AND scope_id = ? AND key = ?", string(scope), scopeID, key).
		Delete(&memoryModel{}).Error
}

func memoryToModel(m domain.Memory) memoryModel {
	return memoryModel{
		ID:        m.ID,
		Scope:     string(m.Scope),
		ScopeID:   m.ScopeID,
		Key:       m.Key,
		Content:   m.Content,
		UpdatedAt: m.UpdatedAt,
	}
}

func memoryToDomain(m memoryModel) domain.Memory {
	return domain.Memory{
		ID:        m.ID,
		Scope:     domain.MemoryScope(m.Scope),
		ScopeID:   m.ScopeID,
		Key:       m.Key,
		Content:   m.Content,
		UpdatedAt: m.UpdatedAt,
	}
}

type weixinAccountRepo struct{ s *Store }

func (r *weixinAccountRepo) List(ctx context.Context) ([]domain.WeixinAccount, error) {
	var rows []weixinAccountModel
	if err := r.s.db.WithContext(ctx).Order("updated_at desc").Find(&rows).Error; err != nil {
		return nil, err
	}
	out := make([]domain.WeixinAccount, 0, len(rows))
	for _, m := range rows {
		out = append(out, weixinAccountToDomain(m))
	}
	return out, nil
}

func (r *weixinAccountRepo) Get(ctx context.Context, accountID string) (domain.WeixinAccount, error) {
	var m weixinAccountModel
	if err := r.s.db.WithContext(ctx).First(&m, "account_id = ?", accountID).Error; err != nil {
		return domain.WeixinAccount{}, err
	}
	return weixinAccountToDomain(m), nil
}

func (r *weixinAccountRepo) Upsert(ctx context.Context, a domain.WeixinAccount) error {
	now := time.Now().UTC()
	if a.CreatedAt.IsZero() {
		a.CreatedAt = now
	}
	a.UpdatedAt = now
	m := weixinAccountModel{
		AccountID: a.AccountID,
		Token:     a.Token,
		BaseURL:   a.BaseURL,
		UserID:    a.UserID,
		ProjectID: a.ProjectID,
		SyncBuf:   a.SyncBuf,
		CreatedAt: a.CreatedAt,
		UpdatedAt: a.UpdatedAt,
	}
	return r.s.db.WithContext(ctx).Save(&m).Error
}

func (r *weixinAccountRepo) Delete(ctx context.Context, accountID string) error {
	return r.s.db.WithContext(ctx).Delete(&weixinAccountModel{}, "account_id = ?", accountID).Error
}

func (r *weixinAccountRepo) UpdateSyncBuf(ctx context.Context, accountID, syncBuf string) error {
	return r.s.db.WithContext(ctx).Model(&weixinAccountModel{}).
		Where("account_id = ?", accountID).
		Updates(map[string]any{"sync_buf": syncBuf, "updated_at": time.Now().UTC()}).Error
}

func (r *weixinAccountRepo) UpdateProjectID(ctx context.Context, accountID, projectID string) error {
	return r.s.db.WithContext(ctx).Model(&weixinAccountModel{}).
		Where("account_id = ?", accountID).
		Updates(map[string]any{"project_id": projectID, "updated_at": time.Now().UTC()}).Error
}

type appMetaRepo struct{ s *Store }

func (r *appMetaRepo) Get(ctx context.Context, key string) (string, bool, error) {
	var m appMetaModel
	err := r.s.db.WithContext(ctx).First(&m, "`key` = ?", key).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return "", false, nil
	}
	if err != nil {
		return "", false, err
	}
	return m.Value, true, nil
}

func (r *appMetaRepo) Set(ctx context.Context, key, value string) error {
	return r.s.db.WithContext(ctx).Save(&appMetaModel{Key: key, Value: value}).Error
}

type weixinBindingRepo struct{ s *Store }

func (r *weixinBindingRepo) List(ctx context.Context) ([]domain.WeixinBinding, error) {
	var rows []weixinBindingModel
	if err := r.s.db.WithContext(ctx).Order("updated_at desc").Find(&rows).Error; err != nil {
		return nil, err
	}
	out := make([]domain.WeixinBinding, 0, len(rows))
	for _, m := range rows {
		out = append(out, weixinBindingToDomain(m))
	}
	return out, nil
}

func (r *weixinBindingRepo) GetByPeer(ctx context.Context, accountID, peerUserID string) (domain.WeixinBinding, error) {
	var m weixinBindingModel
	if err := r.s.db.WithContext(ctx).Where("account_id = ? AND peer_user_id = ?", accountID, peerUserID).First(&m).Error; err != nil {
		return domain.WeixinBinding{}, err
	}
	return weixinBindingToDomain(m), nil
}

func (r *weixinBindingRepo) GetBySession(ctx context.Context, sessionID string) (domain.WeixinBinding, error) {
	var m weixinBindingModel
	if err := r.s.db.WithContext(ctx).Where("session_id = ?", sessionID).First(&m).Error; err != nil {
		return domain.WeixinBinding{}, err
	}
	return weixinBindingToDomain(m), nil
}

func (r *weixinBindingRepo) Upsert(ctx context.Context, b domain.WeixinBinding) error {
	now := time.Now().UTC()
	if b.ID == "" {
		b.ID = fmt.Sprintf("wxbind-%d", time.Now().UnixNano())
	}
	if b.CreatedAt.IsZero() {
		b.CreatedAt = now
	}
	b.UpdatedAt = now
	metaJSON, _ := json.Marshal(b.Meta)
	if b.Meta == nil {
		metaJSON = []byte("{}")
	}
	m := weixinBindingModel{
		ID: b.ID, AccountID: b.AccountID, PeerUserID: b.PeerUserID,
		SessionID: b.SessionID, ContextToken: b.ContextToken,
		MetaJSON:  string(metaJSON),
		CreatedAt: b.CreatedAt, UpdatedAt: b.UpdatedAt,
	}
	var existing weixinBindingModel
	err := r.s.db.WithContext(ctx).Where("account_id = ? AND peer_user_id = ?", b.AccountID, b.PeerUserID).First(&existing).Error
	if err == nil {
		m.ID = existing.ID
		m.CreatedAt = existing.CreatedAt
		// Preserve prior meta when caller omits it.
		if b.Meta == nil && existing.MetaJSON != "" {
			m.MetaJSON = existing.MetaJSON
		}
		return r.s.db.WithContext(ctx).Save(&m).Error
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return err
	}
	return r.s.db.WithContext(ctx).Create(&m).Error
}

func (r *weixinBindingRepo) UpdateContextToken(ctx context.Context, accountID, peerUserID, token string) error {
	return r.s.db.WithContext(ctx).Model(&weixinBindingModel{}).
		Where("account_id = ? AND peer_user_id = ?", accountID, peerUserID).
		Updates(map[string]any{"context_token": token, "updated_at": time.Now().UTC()}).Error
}

func (r *weixinBindingRepo) Count(ctx context.Context) (int, error) {
	var n int64
	if err := r.s.db.WithContext(ctx).Model(&weixinBindingModel{}).Count(&n).Error; err != nil {
		return 0, err
	}
	return int(n), nil
}

func (r *weixinBindingRepo) DeleteByAccount(ctx context.Context, accountID string) error {
	return r.s.db.WithContext(ctx).Delete(&weixinBindingModel{}, "account_id = ?", accountID).Error
}

type channelBindingRepo struct{ s *Store }

func (r *channelBindingRepo) GetByPeer(ctx context.Context, channelType, accountID, peerID string) (domain.ChannelBinding, error) {
	var m channelBindingModel
	err := r.s.db.WithContext(ctx).
		Where("channel_type = ? AND account_id = ? AND peer_id = ?", channelType, accountID, peerID).
		First(&m).Error
	if err != nil {
		return domain.ChannelBinding{}, err
	}
	return channelBindingToDomain(m), nil
}

func (r *channelBindingRepo) Upsert(ctx context.Context, b domain.ChannelBinding) error {
	now := time.Now().UTC()
	if b.ID == "" {
		b.ID = fmt.Sprintf("chbind-%d", time.Now().UnixNano())
	}
	if b.CreatedAt.IsZero() {
		b.CreatedAt = now
	}
	b.UpdatedAt = now
	metaJSON, _ := json.Marshal(b.Meta)
	m := channelBindingModel{
		ID: b.ID, ChannelType: b.ChannelType, AccountID: b.AccountID, PeerID: b.PeerID,
		SessionID: b.SessionID, MetaJSON: string(metaJSON),
		CreatedAt: b.CreatedAt, UpdatedAt: b.UpdatedAt,
	}
	var existing channelBindingModel
	err := r.s.db.WithContext(ctx).
		Where("channel_type = ? AND account_id = ? AND peer_id = ?", b.ChannelType, b.AccountID, b.PeerID).
		First(&existing).Error
	if err == nil {
		m.ID = existing.ID
		m.CreatedAt = existing.CreatedAt
		return r.s.db.WithContext(ctx).Save(&m).Error
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return err
	}
	return r.s.db.WithContext(ctx).Create(&m).Error
}

func (r *channelBindingRepo) UpdateMeta(ctx context.Context, channelType, accountID, peerID string, meta map[string]string) error {
	metaJSON, _ := json.Marshal(meta)
	return r.s.db.WithContext(ctx).Model(&channelBindingModel{}).
		Where("channel_type = ? AND account_id = ? AND peer_id = ?", channelType, accountID, peerID).
		Updates(map[string]any{"meta_json": string(metaJSON), "updated_at": time.Now().UTC()}).Error
}

func (r *channelBindingRepo) DeleteByAccount(ctx context.Context, channelType, accountID string) error {
	return r.s.db.WithContext(ctx).
		Where("channel_type = ? AND account_id = ?", channelType, accountID).
		Delete(&channelBindingModel{}).Error
}
