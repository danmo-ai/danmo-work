package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"danmo-work/core/domain"
	"danmo-work/core/port"
)

// AnthropicMessagesClient talks to the Anthropic Messages API
// (POST {baseURL}/messages).
type AnthropicMessagesClient struct {
	baseURL string
	apiKey  string
	timeout time.Duration
	client  *http.Client
}

func NewAnthropicMessagesClient(baseURL, apiKey string) *AnthropicMessagesClient {
	return NewAnthropicMessagesClientWithTimeout(baseURL, apiKey, DefaultChatHTTPTimeout)
}

func NewAnthropicMessagesClientWithTimeout(baseURL, apiKey string, timeout time.Duration) *AnthropicMessagesClient {
	if baseURL == "" {
		baseURL = "https://api.anthropic.com/v1"
	}
	if timeout <= 0 {
		timeout = DefaultChatHTTPTimeout
	}
	return &AnthropicMessagesClient{
		baseURL: strings.TrimRight(baseURL, "/"),
		apiKey:  apiKey,
		timeout: timeout,
		client:  &http.Client{Timeout: timeout},
	}
}

func (p *AnthropicMessagesClient) Chat(ctx context.Context, req port.LLMChatRequest, effort string, effortCfg *EffortConfig) (port.LLMChatResponse, error) {
	model := req.Model
	if model == "" {
		return port.LLMChatResponse{}, fmt.Errorf("model not specified")
	}

	var systemParts []string
	messages := make([]map[string]any, 0, len(req.Messages))
	for _, m := range req.Messages {
		if m.Role == "system" {
			if strings.TrimSpace(m.Content) != "" {
				systemParts = append(systemParts, m.Content)
			}
			continue
		}
		msg := map[string]any{
			"role": m.Role,
		}
		if m.Role == "assistant" && len(m.ToolCalls) > 0 {
			var contents []map[string]any
			// Thinking-enabled requests require the assistant turn that carries
			// tool_use to begin with the original thinking (or redacted_thinking)
			// block, echoed back with its signature. Reconstruct it when present.
			if m.ReasoningSignature != "" && m.ReasoningContent != "" {
				contents = append(contents, map[string]any{
					"type":      "thinking",
					"thinking":  m.ReasoningContent,
					"signature": m.ReasoningSignature,
				})
			} else if m.ReasoningRedacted != "" {
				contents = append(contents, map[string]any{
					"type": "redacted_thinking",
					"data": m.ReasoningRedacted,
				})
			}
			if m.Content != "" {
				contents = append(contents, map[string]any{"type": "text", "text": m.Content})
			}
			for _, tc := range m.ToolCalls {
				contents = append(contents, map[string]any{
					"type":  "tool_use",
					"id":    tc.ID,
					"name":  tc.Name,
					"input": tc.Arguments,
				})
			}
			msg["content"] = contents
		} else if m.Role == "tool" {
			// Anthropic has no "tool" role — tool_result blocks must be carried
			// on a user message, otherwise the API rejects the request.
			msg["role"] = "user"
			msg["content"] = []map[string]any{{
				"type":        "tool_result",
				"tool_use_id": m.ToolCallID,
				"content":     anthropicUserContent(m),
			}}
		} else {
			msg["content"] = anthropicUserContent(m)
		}
		messages = append(messages, msg)
	}

	body := map[string]any{
		"model":    model,
		"messages": messages,
	}
	// Anthropic Messages supports temperature, top_p, max_tokens, stop_sequences (+ thinking).
	// frequency_penalty / presence_penalty are not part of this API — omit.
	applyAnthropicGenParams(body, req.GenParams)
	if _, ok := body["max_tokens"]; !ok {
		body["max_tokens"] = 4096
	}
	if len(systemParts) > 0 {
		blocks := make([]map[string]any, 0, len(systemParts))
		for i, text := range systemParts {
			block := map[string]any{"type": "text", "text": text}
			if i == len(systemParts)-1 {
				block["cache_control"] = map[string]any{"type": "ephemeral"}
			}
			blocks = append(blocks, block)
		}
		body["system"] = blocks
	}
	// Always declare tools when the request has them. A request whose history
	// contains tool_use / tool_result blocks MUST define tools even when the
	// caller wants to suppress further calls; dropping tools returns a 400.
	// ToolChoice="none" is expressed via the tool_choice field instead.
	if len(req.Tools) > 0 {
		var tools []map[string]any
		for i, t := range req.Tools {
			tool := map[string]any{
				"name":         t.Name,
				"description":  t.Description,
				"input_schema": t.Parameters,
			}
			if i == len(req.Tools)-1 {
				tool["cache_control"] = map[string]any{"type": "ephemeral"}
			}
			tools = append(tools, tool)
		}
		body["tools"] = tools
		if req.ToolChoice == "none" {
			body["tool_choice"] = map[string]any{"type": "none"}
		}
	}

	ApplyReasoningEffort(domain.LLMProviderAnthropic, effort, effortCfg, body)
	reconcileAnthropicThinking(body)

	b, err := json.Marshal(body)
	if err != nil {
		return port.LLMChatResponse{}, err
	}

	hReq, err := http.NewRequestWithContext(ctx, http.MethodPost, p.baseURL+"/messages", bytes.NewReader(b))
	if err != nil {
		return port.LLMChatResponse{}, err
	}
	hReq.Header.Set("Content-Type", "application/json")
	hReq.Header.Set("x-api-key", p.apiKey)
	hReq.Header.Set("anthropic-version", "2023-06-01")

	resp, err := p.client.Do(hReq)
	if err != nil {
		return port.LLMChatResponse{}, wrapHTTPTimeout(err, p.timeout)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return port.LLMChatResponse{}, wrapHTTPTimeout(err, p.timeout)
	}
	if resp.StatusCode != http.StatusOK {
		return port.LLMChatResponse{}, classifyHTTPError(resp.StatusCode, respBody)
	}

	var result struct {
		Content []struct {
			Type      string `json:"type"`
			Text      string `json:"text"`
			ID        string `json:"id"`
			Name      string `json:"name"`
			Input     map[string]any
			Thinking  string `json:"thinking"`
			Signature string `json:"signature"`
			Data      string `json:"data"`
		} `json:"content"`
		Usage struct {
			InputTokens              int `json:"input_tokens"`
			OutputTokens             int `json:"output_tokens"`
			CacheCreationInputTokens int `json:"cache_creation_input_tokens"`
			CacheReadInputTokens     int `json:"cache_read_input_tokens"`
		} `json:"usage"`
	}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return port.LLMChatResponse{}, err
	}

	// Anthropic's input_tokens excludes cache read/write tokens, unlike OpenAI's
	// prompt_tokens (which includes cached_tokens). Fold cache tokens into
	// PromptTokens so it reflects true context occupancy — TurnRunner uses it to
	// drive compaction, and cross-provider usage stays comparable.
	usage := &port.LLMUsage{
		PromptTokens:        result.Usage.InputTokens + result.Usage.CacheReadInputTokens + result.Usage.CacheCreationInputTokens,
		CompletionTokens:    result.Usage.OutputTokens,
		TotalTokens:         result.Usage.InputTokens + result.Usage.CacheReadInputTokens + result.Usage.CacheCreationInputTokens + result.Usage.OutputTokens,
		CacheCreationTokens: result.Usage.CacheCreationInputTokens,
		CacheReadTokens:     result.Usage.CacheReadInputTokens,
	}
	if usage.Empty() {
		usage = nil
	}

	content := ""
	reasoning := ""
	signature := ""
	redacted := ""
	var toolCalls []port.ChatToolCall
	for _, c := range result.Content {
		switch c.Type {
		case "text":
			content += c.Text
		case "thinking":
			reasoning += c.Thinking
			if c.Signature != "" {
				signature = c.Signature
			}
		case "redacted_thinking":
			if c.Data != "" {
				redacted = c.Data
			}
		case "tool_use":
			if c.Input == nil {
				return port.LLMChatResponse{}, fmt.Errorf("tool '%s' input is null", c.Name)
			}
			toolCalls = append(toolCalls, port.ChatToolCall{
				ID:        c.ID,
				Name:      c.Name,
				Arguments: c.Input,
			})
		}
	}

	// Preserve assistant text and reasoning even when tool calls are present so
	// the model's pre-call narration is not silently dropped, and so the
	// thinking block can be echoed back on the next request.
	return port.LLMChatResponse{
		Content:            content,
		ReasoningContent:   reasoning,
		ReasoningSignature: signature,
		ReasoningRedacted:  redacted,
		ToolCalls:          toolCalls,
		Usage:              usage,
		Done:               len(toolCalls) == 0,
	}, nil
}

// reconcileAnthropicThinking resolves constraints that Anthropic enforces once
// extended thinking is enabled: max_tokens must exceed thinking.budget_tokens,
// and temperature / top_p must not be set (only the default temperature=1 is
// allowed). Without this, valid-looking configs return a 400.
func reconcileAnthropicThinking(body map[string]any) {
	thinking, ok := body["thinking"].(map[string]any)
	if !ok {
		return
	}
	delete(body, "temperature")
	delete(body, "top_p")

	budget := 0
	switch v := thinking["budget_tokens"].(type) {
	case int:
		budget = v
	case int64:
		budget = int(v)
	case float64:
		budget = int(v)
	}
	if budget <= 0 {
		return
	}
	maxTokens := 0
	switch v := body["max_tokens"].(type) {
	case int:
		maxTokens = v
	case int64:
		maxTokens = int(v)
	case float64:
		maxTokens = int(v)
	}
	// max_tokens must be strictly greater than the thinking budget; leave room
	// for the visible answer on top of the reasoning budget.
	if maxTokens <= budget {
		body["max_tokens"] = budget + 4096
	}
}

// applyAnthropicGenParams writes Anthropic Messages sampling fields.
// Supported: temperature, top_p, max_tokens, stop_sequences.
// Not supported: frequency_penalty, presence_penalty.
func applyAnthropicGenParams(body map[string]any, gp *port.ModelGenParams) {
	if gp == nil {
		return
	}
	if gp.MaxTokens > 0 {
		body["max_tokens"] = gp.MaxTokens
	}
	if gp.TopP != 0 {
		body["top_p"] = gp.TopP
	}
	if len(gp.Stop) > 0 {
		body["stop_sequences"] = gp.Stop
	}
	if gp.Temperature != 0 {
		body["temperature"] = gp.Temperature
	}
}
