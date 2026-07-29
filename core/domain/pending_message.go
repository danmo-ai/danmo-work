package domain

import "time"

// PendingMessageStatus is the lifecycle of a queued user message.
type PendingMessageStatus string

const (
	PendingQueued    PendingMessageStatus = "queued"
	PendingSteering  PendingMessageStatus = "steering" // soft-steer into active turn at next tool→LLM boundary
	PendingSending   PendingMessageStatus = "sending"
	PendingDiscarded PendingMessageStatus = "discarded"
)

// PendingMessage is a user message waiting to become the next turn after the
// active turn ends. It is session-scoped and editable until dequeued.
type PendingMessage struct {
	ID          string              `json:"id"`
	SessionID   string              `json:"sessionId"`
	Content     string              `json:"content"`
	Attachments []UserAttachment    `json:"attachments,omitempty"`
	Position    int                 `json:"position"`
	Status      PendingMessageStatus `json:"status"`
	AgentID     string              `json:"agentId,omitempty"`
	ModelID     string              `json:"modelId,omitempty"`
	CreatedAt   time.Time           `json:"createdAt"`
	UpdatedAt   time.Time           `json:"updatedAt"`
}

// EnqueuePendingRequest creates a queued message for a session.
type EnqueuePendingRequest struct {
	Content     string           `json:"content"`
	Attachments []UserAttachment `json:"attachments,omitempty"`
	AgentID     string           `json:"agentId,omitempty"`
	ModelID     string           `json:"modelId,omitempty"`
}

// UpdatePendingRequest patches a queued message.
type UpdatePendingRequest struct {
	Content     *string           `json:"content,omitempty"`
	Attachments *[]UserAttachment `json:"attachments,omitempty"`
}

// ReorderPendingRequest sets explicit queue order (ids from front to back).
type ReorderPendingRequest struct {
	IDs []string `json:"ids"`
}
