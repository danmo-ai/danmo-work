package domain

import (
	"encoding/json"
	"time"
)

const (
	EventTurnStarted       = "turn.started"
	EventTurnEnded         = "turn.ended"
	EventTurnFailed        = "turn.failed"
	EventStepStarted       = "step.started"
	EventStepEnded         = "step.ended"
	EventToolPending       = "tool.pending"
	EventToolRunning       = "tool.running"
	EventToolCompleted     = "tool.completed"
	EventToolError         = "tool.error"
	EventToolArgsRepaired  = "tool.args_repaired"
	EventCapabilityActive  = "capability.activated"
	EventPermissionAsk     = "permission.ask"
	EventPermissionDecided = "permission.decided"
	EventContextCompacted  = "context.compacted"
	EventDelegateStarted   = "delegate.started"
	EventDelegateCompleted = "delegate.completed"
	EventAskUserPending    = "ask_user.pending"
	EventReport            = "report"
	EventUserMessage       = "user.message"
	EventAgentMessage      = "agent.message"
	EventAgentThinking     = "agent.thinking"
	EventLLMUsage          = "llm.usage"
	EventError             = "error"
	EventSessionCompleted  = "session.completed"
)

type StreamEvent struct {
	Seq       int64           `json:"seq"`
	Type      string          `json:"type"`
	SessionID string          `json:"sessionId"`
	TurnID    string          `json:"turnId,omitempty"`
	Payload   json.RawMessage `json:"payload"`
	CreatedAt time.Time       `json:"createdAt"`
}
