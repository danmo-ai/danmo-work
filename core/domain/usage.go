package domain

import "time"

// UsageGrain identifies the aggregation level of a usage rollup row.
type UsageGrain string

const (
	UsageGrainTurn    UsageGrain = "turn"
	UsageGrainSession UsageGrain = "session"
	UsageGrainProject UsageGrain = "project"
	UsageGrainModel   UsageGrain = "model"
	UsageGrainAgent   UsageGrain = "agent"
)

// UsageDelta is a single LLM call's token contribution to rollups.
type UsageDelta struct {
	PromptTokens     int
	CompletionTokens int
	TotalTokens      int
	Model            string
	AgentID          string
}

// Normalize fills TotalTokens from prompt+completion when the API omitted it.
func (d UsageDelta) Normalize() UsageDelta {
	if d.TotalTokens <= 0 {
		d.TotalTokens = d.PromptTokens + d.CompletionTokens
	}
	return d
}

// UsageRollup is one accumulated row (turn / session / project / model / agent).
type UsageRollup struct {
	Grain            UsageGrain `json:"grain"`
	RefID            string     `json:"refId"`
	ProjectID        string     `json:"projectId,omitempty"`
	SessionID        string     `json:"sessionId,omitempty"`
	Model            string     `json:"model,omitempty"`
	AgentID          string     `json:"agentId,omitempty"`
	PromptTokens     int        `json:"promptTokens"`
	CompletionTokens int        `json:"completionTokens"`
	TotalTokens      int        `json:"totalTokens"`
	CallCount        int        `json:"callCount"`
	MaxPromptTokens  int        `json:"maxPromptTokens,omitempty"`
	UpdatedAt        time.Time  `json:"updatedAt"`
}

// UsageSummary is aggregated token totals.
type UsageSummary struct {
	PromptTokens     int `json:"promptTokens"`
	CompletionTokens int `json:"completionTokens"`
	TotalTokens      int `json:"totalTokens"`
	CallCount        int `json:"callCount"`
	MaxPromptTokens  int `json:"maxPromptTokens,omitempty"`
	TurnCount        int `json:"turnCount"`
	AvgTurnTokens    int `json:"avgTurnTokens,omitempty"` // TotalTokens / TurnCount when TurnCount > 0
}

// FinalizeAvgTurnTokens fills AvgTurnTokens from TotalTokens / TurnCount.
func (s *UsageSummary) FinalizeAvgTurnTokens() {
	if s.TurnCount > 0 {
		s.AvgTurnTokens = s.TotalTokens / s.TurnCount
	} else {
		s.AvgTurnTokens = 0
	}
}

// UsageBreakdown is a parent summary plus child rollups.
type UsageBreakdown struct {
	Summary  UsageSummary  `json:"summary"`
	Turns    []UsageRollup `json:"turns,omitempty"`
	Sessions []UsageRollup `json:"sessions,omitempty"`
	Models   []UsageRollup `json:"models,omitempty"`
	Agents   []UsageRollup `json:"agents,omitempty"`
}

// UsagePeriod is the chart bucket size.
type UsagePeriod string

const (
	UsagePeriodDay   UsagePeriod = "day"
	UsagePeriodWeek  UsagePeriod = "week"
	UsagePeriodMonth UsagePeriod = "month"
)

// UsageSeriesFilter selects chart data. Buckets use rollup UpdatedAt.
type UsageSeriesFilter struct {
	Period    UsagePeriod
	ProjectID string
	Model     string
	AgentID   string
	From      time.Time
	To        time.Time
	// Grain defaults to session (sessions SUM'd into period buckets).
	// Use UsageGrainModel to chart per-model rows (filter.Model optional).
	Grain UsageGrain
}

// UsageSeriesPoint is one period bucket.
type UsageSeriesPoint struct {
	PeriodStart      time.Time `json:"periodStart"`
	PromptTokens     int       `json:"promptTokens"`
	CompletionTokens int       `json:"completionTokens"`
	TotalTokens      int       `json:"totalTokens"`
	CallCount        int       `json:"callCount"`
	Model            string    `json:"model,omitempty"`
	AgentID          string    `json:"agentId,omitempty"`
}

// ModelRollupRefID builds the rollup ref for a model row (project-scoped when projectID set).
func ModelRollupRefID(projectID, model string) string {
	if model == "" {
		return ""
	}
	if projectID == "" {
		return model
	}
	return projectID + "\x1f" + model
}

// AgentRollupRefID builds the rollup ref for an agent row (project-scoped when projectID set).
func AgentRollupRefID(projectID, agentID string) string {
	if agentID == "" {
		return ""
	}
	if projectID == "" {
		return agentID
	}
	return projectID + "\x1f" + agentID
}
