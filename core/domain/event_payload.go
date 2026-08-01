package domain

// StreamEvent payload structs used by the runtime to emit a stable UI
// timeline. Keeping payloads typed avoids ad-hoc map[string]any drift and
// makes the event vocabulary explicit for the frontend session workspace.

type TurnStartedPayload struct {
	TurnID  string `json:"turnId"`
	AgentID string `json:"agentId"`
	Goal    string `json:"goal"`
}

type TurnEndedPayload struct {
	TurnID  string `json:"turnId"`
	Status  string `json:"status"`
	Summary string `json:"summary,omitempty"`
}

type UserMessageAttachment struct {
	Type     string `json:"type"`
	Name     string `json:"name,omitempty"`
	MimeType string `json:"mimeType,omitempty"`
	DataURL  string `json:"dataUrl,omitempty"` // data:<mime>;base64,... for UI preview
}

type UserMessagePayload struct {
	Content     string                  `json:"content"`
	Attachments []UserMessageAttachment `json:"attachments,omitempty"`
}

type AgentMessagePayload struct {
	Text string `json:"text"`
}

type AgentThinkingPayload struct {
	Text string `json:"text"`
}

type StepPayload struct {
	Step  int    `json:"step"`
	Title string `json:"title,omitempty"`
}

type PermissionAskPayload struct {
	ApprovalID   string   `json:"approvalId"`
	CallID       string   `json:"callId,omitempty"`
	Tool         string   `json:"tool"`
	Description  string   `json:"description"`
	Reason       string   `json:"reason,omitempty"`
	Domain       string   `json:"domain,omitempty"`       // for network_domain asks
	ScopeOptions []string `json:"scopeOptions,omitempty"` // e.g. ["once","session"]
}

type PermissionDecidedPayload struct {
	ApprovalID string `json:"approvalId"`
	Approved   bool   `json:"approved"`
	Scope      string `json:"scope,omitempty"` // once | session
}

type ErrorPayload struct {
	Message string `json:"message"`
	Kind    string `json:"kind,omitempty"`
}

type LLMUsagePayload struct {
	PromptTokens     int `json:"promptTokens,omitempty"`
	CompletionTokens int `json:"completionTokens,omitempty"`
	TotalTokens      int `json:"totalTokens,omitempty"`
}

type SessionCompletedPayload struct {
	Summary string `json:"summary"`
	Status  string `json:"status"`
}

type DelegateStartedPayload struct {
	AgentID     string `json:"agentId"`
	Goal        string `json:"goal"`
	ChildTurnID string `json:"childTurnId"`
	CallID      string `json:"callId,omitempty"`
}

type DelegateCompletedPayload struct {
	AgentID string `json:"agentId"`
	Status  string `json:"status"`
	Summary string `json:"summary,omitempty"`
}

type CapabilityActivatedPayload struct {
	Name string `json:"name"`
	Kind string `json:"kind,omitempty"`
}

type AskUserPayload struct {
	AskID      string            `json:"askId"`
	CallID     string            `json:"callId"`
	Question   string            `json:"question"`
	Options    []string          `json:"options,omitempty"`
	DefaultOpt string            `json:"defaultOption,omitempty"`
	FormFields []AskUserFormField `json:"formFields,omitempty"`
}

// AskUserFormField is a single field in a structured form presented to the user.
type AskUserFormField struct {
	Name        string   `json:"name"`
	Label       string   `json:"label"`
	Type        string   `json:"type"`                  // "text" | "number" | "select" | "boolean"
	Required    bool     `json:"required,omitempty"`
	Default     any      `json:"default,omitempty"`
	Options     []string `json:"options,omitempty"`     // for select type
	Placeholder string   `json:"placeholder,omitempty"`
}

type ContextCompactedPayload struct {
	FilePath       string `json:"filePath"`
	TurnsCompacted int    `json:"turnsCompacted"`
	TokensBefore   int    `json:"tokensBefore"`
	TokensAfter    int    `json:"tokensAfter"`
}
