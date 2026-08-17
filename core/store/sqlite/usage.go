package sqlite

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"danmo-work/core/domain"
	"danmo-work/core/port"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type usageRollupModel struct {
	Grain               string    `gorm:"primaryKey;size:16"`
	RefID               string    `gorm:"column:ref_id;primaryKey;size:256"`
	ProjectID           string    `gorm:"column:project_id;index;size:128"`
	SessionID           string    `gorm:"column:session_id;index;size:128"`
	Model               string    `gorm:"column:model;index;size:256"`
	AgentID             string    `gorm:"column:agent_id;index;size:128"`
	PromptTokens        int       `gorm:"column:prompt_tokens"`
	CompletionTokens    int       `gorm:"column:completion_tokens"`
	TotalTokens         int       `gorm:"column:total_tokens"`
	CacheReadTokens     int       `gorm:"column:cache_read_tokens"`
	CacheCreationTokens int       `gorm:"column:cache_creation_tokens"`
	CallCount           int       `gorm:"column:call_count"`
	MaxPromptTokens     int       `gorm:"column:max_prompt_tokens"`
	UpdatedAt           time.Time `gorm:"column:updated_at;index"`
}

func (usageRollupModel) TableName() string { return "llm_usage_rollups" }

func usageRollupToDomain(m usageRollupModel) domain.UsageRollup {
	return domain.UsageRollup{
		Grain:               domain.UsageGrain(m.Grain),
		RefID:               m.RefID,
		ProjectID:           m.ProjectID,
		SessionID:           m.SessionID,
		Model:               m.Model,
		AgentID:             m.AgentID,
		PromptTokens:        m.PromptTokens,
		CompletionTokens:    m.CompletionTokens,
		TotalTokens:         m.TotalTokens,
		CacheReadTokens:     m.CacheReadTokens,
		CacheCreationTokens: m.CacheCreationTokens,
		CallCount:           m.CallCount,
		MaxPromptTokens:     m.MaxPromptTokens,
		UpdatedAt:           m.UpdatedAt,
	}
}

type usageRepo struct{ s *Store }

func (s *Store) Usage() port.UsageRepo { return &usageRepo{s} }

func (r *usageRepo) AddDelta(ctx context.Context, turnID, sessionID, projectID string, delta domain.UsageDelta, at time.Time) error {
	delta = delta.Normalize()
	if delta.Empty() {
		return nil
	}
	if at.IsZero() {
		at = time.Now().UTC()
	} else {
		at = at.UTC()
	}

	return r.s.withWrite(func(db *gorm.DB) error {
		return db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
			if turnID != "" && sessionID != "" {
				if err := upsertUsageDelta(tx, domain.UsageGrainTurn, turnID, projectID, sessionID, delta, at); err != nil {
					return err
				}
			}
			if sessionID != "" {
				if err := upsertUsageDelta(tx, domain.UsageGrainSession, sessionID, projectID, sessionID, delta, at); err != nil {
					return err
				}
			}
			if projectID != "" {
				if err := upsertUsageDelta(tx, domain.UsageGrainProject, projectID, projectID, "", delta, at); err != nil {
					return err
				}
			}
			if delta.Model != "" {
				ref := domain.ModelRollupRefID(projectID, delta.Model)
				if err := upsertUsageDelta(tx, domain.UsageGrainModel, ref, projectID, sessionID, delta, at); err != nil {
					return err
				}
				// Always maintain a global (project-empty) model row for cross-project charts.
				if projectID != "" {
					global := delta
					if err := upsertUsageDelta(tx, domain.UsageGrainModel, delta.Model, "", sessionID, global, at); err != nil {
						return err
					}
				}
			}
			if delta.AgentID != "" {
				ref := domain.AgentRollupRefID(projectID, delta.AgentID)
				if err := upsertUsageDelta(tx, domain.UsageGrainAgent, ref, projectID, sessionID, delta, at); err != nil {
					return err
				}
				if projectID != "" {
					if err := upsertUsageDelta(tx, domain.UsageGrainAgent, delta.AgentID, "", sessionID, delta, at); err != nil {
						return err
					}
				}
			}
			return nil
		})
	})
}

func upsertUsageDelta(tx *gorm.DB, grain domain.UsageGrain, refID, projectID, sessionID string, delta domain.UsageDelta, at time.Time) error {
	if refID == "" {
		return nil
	}
	model := delta.Model
	agentID := delta.AgentID
	switch grain {
	case domain.UsageGrainModel:
		model = delta.Model
		if i := strings.LastIndex(refID, "\x1f"); i >= 0 {
			model = refID[i+1:]
		} else {
			model = refID
		}
	case domain.UsageGrainAgent:
		agentID = delta.AgentID
		if i := strings.LastIndex(refID, "\x1f"); i >= 0 {
			agentID = refID[i+1:]
		} else {
			agentID = refID
		}
	}
	row := usageRollupModel{
		Grain:               string(grain),
		RefID:               refID,
		ProjectID:           projectID,
		SessionID:           sessionID,
		Model:               model,
		AgentID:             agentID,
		PromptTokens:        delta.PromptTokens,
		CompletionTokens:    delta.CompletionTokens,
		TotalTokens:         delta.TotalTokens,
		CacheReadTokens:     delta.CacheReadTokens,
		CacheCreationTokens: delta.CacheCreationTokens,
		CallCount:           1,
		MaxPromptTokens:     delta.PromptTokens,
		UpdatedAt:           at,
	}
	return tx.Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "grain"}, {Name: "ref_id"}},
		DoUpdates: clause.Assignments(map[string]interface{}{
			"prompt_tokens":         gorm.Expr("prompt_tokens + ?", delta.PromptTokens),
			"completion_tokens":     gorm.Expr("completion_tokens + ?", delta.CompletionTokens),
			"total_tokens":          gorm.Expr("total_tokens + ?", delta.TotalTokens),
			"cache_read_tokens":     gorm.Expr("cache_read_tokens + ?", delta.CacheReadTokens),
			"cache_creation_tokens": gorm.Expr("cache_creation_tokens + ?", delta.CacheCreationTokens),
			"call_count":            gorm.Expr("call_count + 1"),
			"max_prompt_tokens":     gorm.Expr("MAX(max_prompt_tokens, ?)", delta.PromptTokens),
			"updated_at":            at,
			"project_id":            projectID,
			"session_id":            sessionID,
			"model":                 model,
			"agent_id":              agentID,
		}),
	}).Create(&row).Error
}

func (r *usageRepo) Get(ctx context.Context, grain domain.UsageGrain, refID string) (domain.UsageRollup, error) {
	var m usageRollupModel
	err := r.s.db.WithContext(ctx).Where("grain = ? AND ref_id = ?", string(grain), refID).First(&m).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return domain.UsageRollup{}, fmt.Errorf("usage rollup %s/%s: %w", grain, refID, err)
	}
	if err != nil {
		return domain.UsageRollup{}, err
	}
	return usageRollupToDomain(m), nil
}

func (r *usageRepo) HasGrain(ctx context.Context, grain domain.UsageGrain, refID string) (bool, error) {
	var n int64
	err := r.s.db.WithContext(ctx).Model(&usageRollupModel{}).
		Where("grain = ? AND ref_id = ?", string(grain), refID).
		Count(&n).Error
	return n > 0, err
}

func (r *usageRepo) ListBySession(ctx context.Context, sessionID string) ([]domain.UsageRollup, error) {
	var rows []usageRollupModel
	err := r.s.db.WithContext(ctx).
		Where("session_id = ? AND grain = ?", sessionID, string(domain.UsageGrainTurn)).
		Order("updated_at ASC").
		Find(&rows).Error
	if err != nil {
		return nil, err
	}
	out := make([]domain.UsageRollup, len(rows))
	for i, m := range rows {
		out[i] = usageRollupToDomain(m)
	}
	return out, nil
}

func (r *usageRepo) ListByProject(ctx context.Context, projectID string, grain domain.UsageGrain) ([]domain.UsageRollup, error) {
	if grain == "" {
		grain = domain.UsageGrainSession
	}
	var rows []usageRollupModel
	err := r.s.db.WithContext(ctx).
		Where("project_id = ? AND grain = ?", projectID, string(grain)).
		Order("updated_at DESC").
		Find(&rows).Error
	if err != nil {
		return nil, err
	}
	out := make([]domain.UsageRollup, len(rows))
	for i, m := range rows {
		out[i] = usageRollupToDomain(m)
	}
	return out, nil
}

func summaryFromRollup(u domain.UsageRollup) domain.UsageSummary {
	return domain.UsageSummary{
		PromptTokens:        u.PromptTokens,
		CompletionTokens:    u.CompletionTokens,
		TotalTokens:         u.TotalTokens,
		CacheReadTokens:     u.CacheReadTokens,
		CacheCreationTokens: u.CacheCreationTokens,
		CallCount:           u.CallCount,
		MaxPromptTokens:     u.MaxPromptTokens,
	}
}

func (r *usageRepo) countTurns(ctx context.Context, projectID, model string) (int, error) {
	var n int64
	q := r.s.db.WithContext(ctx).Model(&usageRollupModel{}).Where("grain = ?", string(domain.UsageGrainTurn))
	if projectID != "" {
		q = q.Where("project_id = ?", projectID)
	}
	if model != "" {
		q = q.Where("model = ?", model)
	}
	if err := q.Count(&n).Error; err != nil {
		return 0, err
	}
	return int(n), nil
}

func (r *usageRepo) SummarizeScope(ctx context.Context, projectID, model string) (domain.UsageSummary, error) {
	var sum domain.UsageSummary
	switch {
	case model != "":
		ref := domain.ModelRollupRefID(projectID, model)
		u, err := r.Get(ctx, domain.UsageGrainModel, ref)
		if err != nil && !isNotFound(err) {
			return sum, err
		}
		if u.RefID != "" {
			sum = summaryFromRollup(u)
		}
	case projectID != "":
		u, err := r.Get(ctx, domain.UsageGrainProject, projectID)
		if err != nil && !isNotFound(err) {
			return sum, err
		}
		if u.RefID != "" {
			sum = summaryFromRollup(u)
		}
	default:
		var rows []usageRollupModel
		// Global model rows (project_id="") — each model once; sum = all-project totals.
		err := r.s.db.WithContext(ctx).
			Where("grain = ? AND project_id = ?", string(domain.UsageGrainModel), "").
			Find(&rows).Error
		if err != nil {
			return sum, err
		}
		for _, m := range rows {
			sum.PromptTokens += m.PromptTokens
			sum.CompletionTokens += m.CompletionTokens
			sum.TotalTokens += m.TotalTokens
			sum.CacheReadTokens += m.CacheReadTokens
			sum.CacheCreationTokens += m.CacheCreationTokens
			sum.CallCount += m.CallCount
			if m.MaxPromptTokens > sum.MaxPromptTokens {
				sum.MaxPromptTokens = m.MaxPromptTokens
			}
		}
	}
	n, err := r.countTurns(ctx, projectID, model)
	if err != nil {
		return sum, err
	}
	sum.TurnCount = n
	sum.FinalizeAvgTurnTokens()
	return sum, nil
}

func isNotFound(err error) bool {
	return errors.Is(err, gorm.ErrRecordNotFound)
}

func (r *usageRepo) SummarizeSession(ctx context.Context, sessionID string) (domain.UsageBreakdown, error) {
	sess, err := r.Get(ctx, domain.UsageGrainSession, sessionID)
	if err != nil && !isNotFound(err) {
		return domain.UsageBreakdown{}, err
	}
	turns, err := r.ListBySession(ctx, sessionID)
	if err != nil {
		return domain.UsageBreakdown{}, err
	}
	// Session-level model/agent attribution lives on turn rows; group from turns for display.
	bd := domain.UsageBreakdown{Turns: turns}
	if sess.RefID != "" {
		bd.Summary = summaryFromRollup(sess)
	}
	bd.Summary.TurnCount = len(turns)
	bd.Summary.FinalizeAvgTurnTokens()
	projectID := sess.ProjectID
	if projectID == "" && len(turns) > 0 {
		projectID = turns[0].ProjectID
	}
	if projectID != "" {
		models, err := r.ListByProject(ctx, projectID, domain.UsageGrainModel)
		if err != nil {
			return domain.UsageBreakdown{}, err
		}
		bd.Models = models
		agents, err := r.ListByProject(ctx, projectID, domain.UsageGrainAgent)
		if err != nil {
			return domain.UsageBreakdown{}, err
		}
		bd.Agents = agents
	}
	return bd, nil
}

func (r *usageRepo) SummarizeProject(ctx context.Context, projectID string) (domain.UsageBreakdown, error) {
	proj, err := r.Get(ctx, domain.UsageGrainProject, projectID)
	if err != nil && !isNotFound(err) {
		return domain.UsageBreakdown{}, err
	}
	sessions, err := r.ListByProject(ctx, projectID, domain.UsageGrainSession)
	if err != nil {
		return domain.UsageBreakdown{}, err
	}
	models, err := r.ListByProject(ctx, projectID, domain.UsageGrainModel)
	if err != nil {
		return domain.UsageBreakdown{}, err
	}
	agents, err := r.ListByProject(ctx, projectID, domain.UsageGrainAgent)
	if err != nil {
		return domain.UsageBreakdown{}, err
	}
	bd := domain.UsageBreakdown{Sessions: sessions, Models: models, Agents: agents}
	if proj.RefID != "" {
		bd.Summary = summaryFromRollup(proj)
	}
	n, err := r.countTurns(ctx, projectID, "")
	if err != nil {
		return domain.UsageBreakdown{}, err
	}
	bd.Summary.TurnCount = n
	bd.Summary.FinalizeAvgTurnTokens()
	return bd, nil
}

func (r *usageRepo) Series(ctx context.Context, filter domain.UsageSeriesFilter) ([]domain.UsageSeriesPoint, error) {
	grain := filter.Grain
	if grain == "" {
		grain = domain.UsageGrainSession
	}
	period := filter.Period
	if period == "" {
		period = domain.UsagePeriodDay
	}

	var rows []usageRollupModel
	q := r.s.db.WithContext(ctx).Model(&usageRollupModel{}).Where("grain = ?", string(grain))
	if filter.ProjectID != "" {
		q = q.Where("project_id = ?", filter.ProjectID)
	} else if grain == domain.UsageGrainModel || grain == domain.UsageGrainAgent {
		q = q.Where("project_id = ?", "")
	}
	if filter.Model != "" {
		q = q.Where("model = ?", filter.Model)
	}
	if filter.AgentID != "" {
		q = q.Where("agent_id = ?", filter.AgentID)
	}
	if !filter.From.IsZero() {
		q = q.Where("updated_at >= ?", filter.From.UTC())
	}
	if !filter.To.IsZero() {
		q = q.Where("updated_at < ?", filter.To.UTC())
	}
	if err := q.Find(&rows).Error; err != nil {
		return nil, err
	}

	// For model grain without a single model filter, keep one series point set
	// aggregated across models per period (UI can request grain=model + list separately).
	buckets := map[int64]*domain.UsageSeriesPoint{}
	var keys []int64
	for _, m := range rows {
		start := periodStart(m.UpdatedAt, period)
		k := start.Unix()
		p, ok := buckets[k]
		if !ok {
			p = &domain.UsageSeriesPoint{PeriodStart: start}
			buckets[k] = p
			keys = append(keys, k)
		}
		p.PromptTokens += m.PromptTokens
		p.CompletionTokens += m.CompletionTokens
		p.TotalTokens += m.TotalTokens
		p.CacheReadTokens += m.CacheReadTokens
		p.CacheCreationTokens += m.CacheCreationTokens
		p.CallCount += m.CallCount
		if filter.Model != "" {
			p.Model = m.Model
		}
		if filter.AgentID != "" {
			p.AgentID = m.AgentID
		}
	}
	sort.Slice(keys, func(i, j int) bool { return keys[i] < keys[j] })
	out := make([]domain.UsageSeriesPoint, 0, len(keys))
	for _, k := range keys {
		out = append(out, *buckets[k])
	}
	return out, nil
}

func periodStart(t time.Time, period domain.UsagePeriod) time.Time {
	t = t.UTC()
	y, m, d := t.Date()
	switch period {
	case domain.UsagePeriodWeek:
		wd := int(t.Weekday())
		if wd == 0 {
			wd = 7
		}
		day := time.Date(y, m, d, 0, 0, 0, 0, time.UTC)
		return day.AddDate(0, 0, -(wd - 1))
	case domain.UsagePeriodMonth:
		return time.Date(y, m, 1, 0, 0, 0, 0, time.UTC)
	default:
		return time.Date(y, m, d, 0, 0, 0, 0, time.UTC)
	}
}
