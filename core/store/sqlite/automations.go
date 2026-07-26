package sqlite

import (
	"context"
	"time"

	"danmo-work/core/domain"
	"danmo-work/core/port"
)

type automationModel struct {
	ID          string `gorm:"primaryKey"`
	Name        string
	Description string
	Enabled     bool
	Trigger     string
	Schedule    string
	EventType   string `gorm:"column:event_type"`
	WebhookPath string `gorm:"column:webhook_path"`
	AgentID     string `gorm:"column:agent_id"`
	ProjectID   string `gorm:"column:project_id"`
	ModelID     string `gorm:"column:model_id"`
	Prompt      string
	LastRunAt   string `gorm:"column:last_run_at"`
	NextRunAt   string `gorm:"column:next_run_at"`
	LastTurnID  string `gorm:"column:last_turn_id"`
	LastStatus  string `gorm:"column:last_status"`
	CreatedAt   int64  `gorm:"column:created_at"`
	UpdatedAt   int64  `gorm:"column:updated_at"`
}

func (automationModel) TableName() string { return "automations" }

type automationRepo struct{ s *Store }

var _ port.AutomationRepo = (*automationRepo)(nil)

func automationToDomain(m automationModel) domain.Automation {
	return domain.Automation{
		ID:          m.ID,
		Name:        m.Name,
		Description: m.Description,
		Enabled:     m.Enabled,
		Trigger:     domain.AutomationTrigger(m.Trigger),
		Schedule:    m.Schedule,
		EventType:   m.EventType,
		WebhookPath: m.WebhookPath,
		AgentID:     m.AgentID,
		ProjectID:   m.ProjectID,
		ModelID:     m.ModelID,
		Prompt:      m.Prompt,
		LastRunAt:   m.LastRunAt,
		NextRunAt:   m.NextRunAt,
		LastTurnID:  m.LastTurnID,
		LastStatus:  m.LastStatus,
		CreatedAt:   formatUnix(m.CreatedAt),
		UpdatedAt:   formatUnix(m.UpdatedAt),
	}
}

func automationFromDomain(a domain.Automation) automationModel {
	now := time.Now().Unix()
	created := now
	if a.CreatedAt != "" {
		if t, err := time.Parse(time.RFC3339, a.CreatedAt); err == nil {
			created = t.Unix()
		}
	}
	return automationModel{
		ID:          a.ID,
		Name:        a.Name,
		Description: a.Description,
		Enabled:     a.Enabled,
		Trigger:     string(a.Trigger),
		Schedule:    a.Schedule,
		EventType:   a.EventType,
		WebhookPath: a.WebhookPath,
		AgentID:     a.AgentID,
		ProjectID:   a.ProjectID,
		ModelID:     a.ModelID,
		Prompt:      a.Prompt,
		LastRunAt:   a.LastRunAt,
		NextRunAt:   a.NextRunAt,
		LastTurnID:  a.LastTurnID,
		LastStatus:  a.LastStatus,
		CreatedAt:   created,
		UpdatedAt:   now,
	}
}

func formatUnix(ts int64) string {
	if ts <= 0 {
		return ""
	}
	return time.Unix(ts, 0).UTC().Format(time.RFC3339)
}

func (r *automationRepo) List(ctx context.Context) ([]domain.Automation, error) {
	var rows []automationModel
	if err := r.s.db.WithContext(ctx).Order("updated_at desc").Find(&rows).Error; err != nil {
		return nil, err
	}
	out := make([]domain.Automation, len(rows))
	for i, m := range rows {
		out[i] = automationToDomain(m)
	}
	return out, nil
}

func (r *automationRepo) ListEnabled(ctx context.Context) ([]domain.Automation, error) {
	var rows []automationModel
	if err := r.s.db.WithContext(ctx).Where("enabled = ?", true).Find(&rows).Error; err != nil {
		return nil, err
	}
	out := make([]domain.Automation, len(rows))
	for i, m := range rows {
		out[i] = automationToDomain(m)
	}
	return out, nil
}

func (r *automationRepo) Get(ctx context.Context, id string) (domain.Automation, error) {
	var m automationModel
	if err := r.s.db.WithContext(ctx).Where("id = ?", id).First(&m).Error; err != nil {
		return domain.Automation{}, err
	}
	return automationToDomain(m), nil
}

func (r *automationRepo) Upsert(ctx context.Context, a domain.Automation) error {
	m := automationFromDomain(a)
	var existing automationModel
	err := r.s.db.WithContext(ctx).Where("id = ?", a.ID).First(&existing).Error
	if err == nil {
		m.CreatedAt = existing.CreatedAt
	}
	return r.s.db.WithContext(ctx).Save(&m).Error
}

func (r *automationRepo) Delete(ctx context.Context, id string) error {
	return r.s.db.WithContext(ctx).Where("id = ?", id).Delete(&automationModel{}).Error
}
