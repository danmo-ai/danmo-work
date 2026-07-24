package port

import (
	"context"
	"danmo-work/core/domain"
)

// ChatContentPart is a multimodal content block (text or image) for vision models.
type ChatContentPart struct {
	Type     string // "text" | "image"
	Text     string
	MimeType string // image/* for image parts
	Data     string // raw base64 for image parts
	Name     string
}

type ChatMessage struct {
	Role       string
	Content    string            // text-only shortcut; ignored for content when Parts is set for user msgs with images
	Parts      []ChatContentPart // optional multimodal parts (user images)
	ToolCalls  []ChatToolCall
	ToolCallID string
	Name       string
}

type ChatToolCall struct {
	ID        string
	Name      string
	Arguments map[string]any
}

type LLMChatRequest struct {
	Model      string
	Messages   []ChatMessage
	Tools      []domain.ToolSchema
	ToolChoice string
	GenParams  *ModelGenParams // optional; nil = provider defaults
}

// ModelGenParams carries per-model generation parameters.
// Zero values mean "use provider default" (do not send to API).
type ModelGenParams struct {
	MaxTokens          int
	Temperature        float64
	TopP               float64
	FrequencyPenalty   float64
	PresencePenalty    float64
	Stop               []string
	ThinkingMode       string         // "adaptive" or "enabled"; for Anthropic thinking
	EffortBudgetTokens map[string]int // effort → budget_tokens mapping
}

type LLMChatResponse struct {
	Content          string
	ReasoningContent string // thinking/reasoning trace from reasoning models
	ToolCalls        []ChatToolCall
	Usage            *LLMUsage
	Done             bool
}

type LLMUsage struct {
	PromptTokens     int
	CompletionTokens int
	TotalTokens      int
}

type LLMProvider interface {
	Chat(ctx context.Context, req LLMChatRequest) (LLMChatResponse, error)
}
