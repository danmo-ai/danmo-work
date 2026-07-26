package sqlite

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
)

var _ port.Repository = (*Store)(nil)

type Store struct {
	db *gorm.DB
}

func New(dbPath string) (*Store, error) {
	if dbPath == "" {
		dbPath = paths.DatabaseFile()
	}
	if abs, err := filepath.Abs(dbPath); err == nil {
		dbPath = abs
	}
	if err := os.MkdirAll(filepath.Dir(dbPath), 0755); err != nil {
		return nil, err
	}
	db, err := gorm.Open(sqlite.Open(dbPath), &gorm.Config{})
	if err != nil {
		return nil, err
	}
	s := &Store{db: db}
	if err := s.migrate(); err != nil {
		return nil, err
	}
	return s, nil
}

func (s *Store) DB() *gorm.DB { return s.db }

func (s *Store) migrate() error {
	if err := s.db.AutoMigrate(
		&agentModel{},
		&skillModel{},
		&skillFileModel{},
		&sessionModel{},
		&projectModel{},
		&llmConfigModel{},
		&approvalModel{},
		&knowledgeDocModel{},
		&memoryModel{},
		&streamEventModel{},
		&turnModel{},
		&mcpServerModel{},
		&secretModel{},
		&automationModel{},
		&weixinAccountModel{},
		&weixinBindingModel{},
		&channelBindingModel{},
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
	// One-time: agent.steps 0 = follow runtime.turn.max_steps_default.
	if err := s.db.AutoMigrate(&appMetaModel{}); err != nil {
		return err
	}
	const metaKey = "agent_steps_follow_global_v1"
	var meta appMetaModel
	err := s.db.Where("key = ?", metaKey).First(&meta).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		if err := s.db.Exec("UPDATE agents SET steps = 0").Error; err != nil {
			return err
		}
		if err := s.db.Create(&appMetaModel{Key: metaKey, Value: "1"}).Error; err != nil {
			return err
		}
	} else if err != nil {
		return err
	}
	return nil
}

type appMetaModel struct {
	Key   string `gorm:"primaryKey;column:key"`
	Value string `gorm:"column:value"`
}

func (appMetaModel) TableName() string { return "app_meta" }

func (s *Store) Agents() port.AgentRepo             { return &agentRepo{s} }
func (s *Store) Skills() port.SkillRepo             { return &skillRepo{s} }
func (s *Store) SkillFiles() port.SkillFileRepo     { return &skillFileRepo{s} }
func (s *Store) Sessions() port.SessionRepo         { return &sessionRepo{s} }
func (s *Store) Projects() port.ProjectRepo         { return &projectRepo{s} }
func (s *Store) LLMConfig() port.LLMConfigRepo      { return &llmConfigRepo{s} }
func (s *Store) Approvals() port.ApprovalRepo       { return &approvalRepo{s} }
func (s *Store) StreamEvents() port.StreamEventRepo { return &streamEventRepo{s} }
func (s *Store) Turns() port.TurnRepo               { return &turnRepo{s} }
func (s *Store) MCPServers() port.MCPServerRepo     { return &mcpServerRepo{s} }
func (s *Store) Secrets() port.SecretStore          { return newSecretStore(s.db) }
func (s *Store) Automations() port.AutomationRepo   { return &automationRepo{s} }
func (s *Store) Memories() port.MemoryRepo          { return &memoryRepo{s} }
func (s *Store) WeixinAccounts() port.WeixinAccountRepo { return &weixinAccountRepo{s} }
func (s *Store) WeixinBindings() port.WeixinBindingRepo { return &weixinBindingRepo{s} }
func (s *Store) ChannelBindings() port.ChannelBindingRepo { return &channelBindingRepo{s} }
func (s *Store) AppMeta() port.AppMetaRepo              { return &appMetaRepo{s} }

func (s *Store) KnowledgeDocs() []KnowledgeDoc {
	var rows []knowledgeDocModel
	if err := s.db.Find(&rows).Error; err != nil {
		return nil
	}
	out := make([]KnowledgeDoc, len(rows))
	for i, r := range rows {
		out[i] = KnowledgeDoc{KBID: r.KBID, Title: r.Title, Content: r.Content}
	}
	return out
}

type KnowledgeDoc struct {
	KBID    string
	Title   string
	Content string
}

type knowledgeDocModel struct {
	ID      uint   `gorm:"primaryKey;autoIncrement"`
	KBID    string `gorm:"column:kb_id"`
	Title   string
	Content string
}

func (knowledgeDocModel) TableName() string { return "knowledge_docs" }

// ---- AgentRepo ----

type agentRepo struct{ s *Store }

func (r *agentRepo) List(ctx context.Context) ([]domain.Agent, error) {
	var rows []agentModel
	if err := r.s.db.WithContext(ctx).Find(&rows).Error; err != nil {
		return nil, err
	}
	out := make([]domain.Agent, len(rows))
	for i, row := range rows {
		out[i] = agentToDomain(row)
	}
	return out, nil
}

func (r *agentRepo) Get(ctx context.Context, id string) (domain.Agent, error) {
	var row agentModel
	if err := r.s.db.WithContext(ctx).First(&row, "id = ?", id).Error; err != nil {
		return domain.Agent{}, err
	}
	return agentToDomain(row), nil
}

func (r *agentRepo) Upsert(ctx context.Context, a domain.Agent) error {
	m := agentFromDomain(a)
	var existing agentModel
	err := r.s.db.WithContext(ctx).First(&existing, "id = ?", a.ID).Error
	if err != nil {
		return r.s.db.WithContext(ctx).Create(&m).Error
	}
	// Select all columns so zero values (steps=0 follow-global, can_delegate=false) persist.
	// Plain Updates() skips zero fields and left legacy per-agent step caps stuck.
	return r.s.db.WithContext(ctx).Model(&existing).Select(
		"Name", "Description", "Persona", "Mode", "SystemPrompt", "Steps",
		"SkillIDsJSON", "ToolsJSON", "KnowledgeJSON", "CanDelegate",
	).Updates(&m).Error
}

func (r *agentRepo) Delete(ctx context.Context, id string) error {
	return r.s.db.WithContext(ctx).Delete(&agentModel{}, "id = ?", id).Error
}

// ---- SkillRepo ----

type skillRepo struct{ s *Store }

func (r *skillRepo) List(ctx context.Context) ([]domain.Skill, error) {
	var rows []skillModel
	if err := r.s.db.WithContext(ctx).Find(&rows).Error; err != nil {
		return nil, err
	}
	out := make([]domain.Skill, len(rows))
	for i, row := range rows {
		out[i] = skillToDomain(row)
	}
	return out, nil
}

func (r *skillRepo) Get(ctx context.Context, id string) (domain.Skill, error) {
	var row skillModel
	if err := r.s.db.WithContext(ctx).First(&row, "id = ?", id).Error; err != nil {
		return domain.Skill{}, err
	}
	return skillToDomain(row), nil
}

func (r *skillRepo) Upsert(ctx context.Context, sk domain.Skill) error {
	m := skillFromDomain(sk)
	var existing skillModel
	err := r.s.db.WithContext(ctx).First(&existing, "id = ?", sk.ID).Error
	if err != nil {
		return r.s.db.WithContext(ctx).Create(&m).Error
	}
	// Select all columns so zero values (and builtin=true/false) always persist.
	// Plain Updates() skips zero fields and can leave stale builtin/metadata.
	return r.s.db.WithContext(ctx).Model(&existing).Select(
		"Name", "Description", "License", "Compatibility", "MetadataJSON",
		"AllowedTools", "KeywordsJSON", "ToolIDsJSON", "SystemHint", "Body",
		"SourcePath", "Builtin",
	).Updates(&m).Error
}

func (r *skillRepo) Delete(ctx context.Context, id string) error {
	return r.s.db.WithContext(ctx).Delete(&skillModel{}, "id = ?", id).Error
}

// ---- SkillFileRepo ----

type skillFileRepo struct{ s *Store }

func (r *skillFileRepo) ListBySkill(ctx context.Context, skillID string) ([]domain.SkillFile, error) {
	var rows []skillFileModel
	if err := r.s.db.WithContext(ctx).Where("skill_id = ?", skillID).Find(&rows).Error; err != nil {
		return nil, err
	}
	out := make([]domain.SkillFile, len(rows))
	for i, row := range rows {
		out[i] = skillFileToDomain(row)
	}
	return out, nil
}

func (r *skillFileRepo) Get(ctx context.Context, skillID, path string) (domain.SkillFile, error) {
	id := skillID + ":" + path
	var row skillFileModel
	if err := r.s.db.WithContext(ctx).First(&row, "id = ?", id).Error; err != nil {
		return domain.SkillFile{}, err
	}
	return skillFileToDomain(row), nil
}

func (r *skillFileRepo) Upsert(ctx context.Context, f domain.SkillFile) error {
	m := skillFileFromDomain(f)
	var existing skillFileModel
	err := r.s.db.WithContext(ctx).First(&existing, "id = ?", m.ID).Error
	if err != nil {
		return r.s.db.WithContext(ctx).Create(&m).Error
	}
	return r.s.db.WithContext(ctx).Model(&existing).Updates(&m).Error
}

func (r *skillFileRepo) Delete(ctx context.Context, skillID, path string) error {
	id := skillID + ":" + path
	return r.s.db.WithContext(ctx).Delete(&skillFileModel{}, "id = ?", id).Error
}

func (r *skillFileRepo) DeleteBySkill(ctx context.Context, skillID string) error {
	return r.s.db.WithContext(ctx).Delete(&skillFileModel{}, "skill_id = ?", skillID).Error
}

// ---- SessionRepo ----

type sessionRepo struct{ s *Store }

func (r *sessionRepo) Create(ctx context.Context, s domain.Session) error {
	m := sessionFromDomain(s)
	return r.s.db.WithContext(ctx).Create(&m).Error
}

func (r *sessionRepo) Update(ctx context.Context, s domain.Session) error {
	return r.s.db.WithContext(ctx).Model(&sessionModel{}).Where("id = ?", s.ID).Updates(sessionFromDomain(s)).Error
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

// ---- MCPServerRepo ----

type mcpServerRepo struct{ s *Store }

func (r *mcpServerRepo) List(ctx context.Context) ([]domain.MCPServer, error) {
	var rows []mcpServerModel
	if err := r.s.db.WithContext(ctx).Order("rowid asc").Find(&rows).Error; err != nil {
		return nil, err
	}
	out := make([]domain.MCPServer, len(rows))
	for i, row := range rows {
		out[i] = mcpServerToDomain(row)
	}
	return out, nil
}

func (r *mcpServerRepo) Get(ctx context.Context, id string) (domain.MCPServer, error) {
	var row mcpServerModel
	if err := r.s.db.WithContext(ctx).First(&row, "id = ?", id).Error; err != nil {
		return domain.MCPServer{}, err
	}
	return mcpServerToDomain(row), nil
}

func (r *mcpServerRepo) Upsert(ctx context.Context, s domain.MCPServer) error {
	m := mcpServerFromDomain(s)
	var existing mcpServerModel
	if err := r.s.db.WithContext(ctx).First(&existing, "id = ?", s.ID).Error; err != nil {
		return r.s.db.WithContext(ctx).Create(&m).Error
	}
	return r.s.db.WithContext(ctx).Model(&existing).Updates(&m).Error
}

func (r *mcpServerRepo) Delete(ctx context.Context, id string) error {
	return r.s.db.WithContext(ctx).Delete(&mcpServerModel{}, "id = ?", id).Error
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
	if len(q.Scopes) == 0 {
		return nil, nil
	}

	clauses := make([]string, 0, len(q.Scopes))
	args := make([]any, 0, len(q.Scopes)*2)
	for _, ref := range q.Scopes {
		clauses = append(clauses, "(scope = ? AND scope_id = ?)")
		args = append(args, string(ref.Scope), ref.ScopeID)
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
		MetaJSON: string(metaJSON),
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

