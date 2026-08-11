package domain

import "time"

type SessionStatus string

const (
	SessionStatusActive    SessionStatus = "active"
	SessionStatusArchived  SessionStatus = "archived"
	SessionStatusCompleted SessionStatus = "completed"
	SessionStatusFailed    SessionStatus = "failed"
)

type Session struct {
	ID        string        `json:"id"`
	Title     string        `json:"title,omitempty"`
	ProjectID string        `json:"projectId,omitempty"`
	AgentID   string        `json:"agentId,omitempty"`
	ModelID   string        `json:"modelId,omitempty"`
	PlanMode  bool          `json:"planMode,omitempty"`
	Content   string        `json:"content"`
	Status    SessionStatus `json:"status"`
	CreatedAt time.Time     `json:"createdAt"`
	UpdatedAt time.Time     `json:"updatedAt"`
}

// UserAttachment is a multimodal part attached to a user message (vision images).
type UserAttachment struct {
	Type     string `json:"type"`               // "image"
	Name     string `json:"name,omitempty"`
	MimeType string `json:"mimeType,omitempty"` // image/png|jpeg|webp|gif
	Data     string `json:"data,omitempty"`     // raw base64 (data: URL prefix allowed)
}

type CreateSessionRequest struct {
	Content     string           `json:"content"`
	AgentID     string           `json:"agentId,omitempty"`
	ProjectID   string           `json:"projectId,omitempty"`
	ModelID     string           `json:"modelId,omitempty"`
	Title       string           `json:"title,omitempty"`
	PlanMode    bool             `json:"planMode,omitempty"`
	// SkipAutoTitle disables the async LLM title generator (e.g. WeChat ingress).
	SkipAutoTitle bool             `json:"skipAutoTitle,omitempty"`
	Attachments   []UserAttachment `json:"attachments,omitempty"`
}

type SendMessageRequest struct {
	UserInput   string           `json:"userInput"`
	AgentID     string           `json:"agentId,omitempty"`
	ModelID     string           `json:"modelId,omitempty"`
	Attachments []UserAttachment `json:"attachments,omitempty"`
	// SnapshotPaths are project-relative files to snapshot before tools run (AI review).
	SnapshotPaths []string `json:"snapshotPaths,omitempty"`
}

type UpdateSessionRequest struct {
	Title     *string        `json:"title,omitempty"`
	ProjectID *string        `json:"projectId,omitempty"`
	Status    *SessionStatus `json:"status,omitempty"`
	ModelID   *string        `json:"modelId,omitempty"`
	AgentID   *string        `json:"agentId,omitempty"`
	PlanMode  *bool          `json:"planMode,omitempty"`
}
