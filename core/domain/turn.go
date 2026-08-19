package domain

type TurnStatus string

const (
	TurnRunning   TurnStatus = "running"
	TurnCompleted TurnStatus = "completed"
	TurnFailed    TurnStatus = "failed"
	TurnCancelled TurnStatus = "cancelled"
	TurnTimeout   TurnStatus = "timeout"
)

type TurnLog struct {
	ID        string     `json:"id"`
	SessionID string     `json:"sessionId"`
	ProjectID string     `json:"projectId,omitempty"`
	Status    TurnStatus `json:"status"`
	AgentID   string     `json:"agentId"`
	Goal      string     `json:"goal"`
	// Nested marks a nested tool-run turn (e.g. delegate_agent execution).
	// Nested turns are debug/zip artifacts, never parent-session LLM history,
	// and are excluded from replay and auto-resume.
	Nested bool `json:"nested,omitempty"`
}

// TurnPathEntry is one frame on the root→current delegation path.
// Lead turn is depth 0; each nested delegate_agent appends a frame.
type TurnPathEntry struct {
	TurnID  string `json:"turnId"`
	AgentID string `json:"agentId"`
}
