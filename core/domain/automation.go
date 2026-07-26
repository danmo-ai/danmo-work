package domain

// AutomationTrigger selects how an automation is started.
type AutomationTrigger string

const (
	AutomationTriggerSchedule AutomationTrigger = "schedule"
	AutomationTriggerEvent    AutomationTrigger = "event"
	AutomationTriggerWebhook  AutomationTrigger = "webhook"
	AutomationTriggerManual   AutomationTrigger = "manual"
)

// Automation is a durable job that starts a session turn on a schedule or webhook.
type Automation struct {
	ID          string            `json:"id"`
	Name        string            `json:"name"`
	Description string            `json:"description"`
	Enabled     bool              `json:"enabled"`
	Trigger     AutomationTrigger `json:"trigger"`
	Schedule    string            `json:"schedule,omitempty"` // cron expression
	EventType   string            `json:"eventType,omitempty"`
	WebhookPath string            `json:"webhookPath,omitempty"`
	AgentID     string            `json:"agentId,omitempty"`
	ProjectID   string            `json:"projectId,omitempty"`
	ModelID     string            `json:"modelId,omitempty"`
	Prompt      string            `json:"prompt"`
	LastRunAt   string            `json:"lastRunAt,omitempty"`
	NextRunAt   string            `json:"nextRunAt,omitempty"`
	LastTurnID  string            `json:"lastTurnId,omitempty"`
	LastStatus  string            `json:"lastStatus,omitempty"`
	CreatedAt   string            `json:"createdAt,omitempty"`
	UpdatedAt   string            `json:"updatedAt,omitempty"`
}

// UpsertAutomationRequest creates or updates an automation.
type UpsertAutomationRequest struct {
	Name        string            `json:"name"`
	Description string            `json:"description"`
	Enabled     bool              `json:"enabled"`
	Trigger     AutomationTrigger `json:"trigger"`
	Schedule    string            `json:"schedule,omitempty"`
	EventType   string            `json:"eventType,omitempty"`
	WebhookPath string            `json:"webhookPath,omitempty"`
	AgentID     string            `json:"agentId,omitempty"`
	ProjectID   string            `json:"projectId,omitempty"`
	ModelID     string            `json:"modelId,omitempty"`
	Prompt      string            `json:"prompt"`
}
