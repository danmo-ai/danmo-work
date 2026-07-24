package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"danmo-work/core/domain"
	"danmo-work/core/port"
)

type AnthropicProvider struct {
	baseURL string
	apiKey  string
	client  *http.Client
}

func NewAnthropicProvider(baseURL, apiKey string) *AnthropicProvider {
	if baseURL == "" {
		baseURL = "https://api.anthropic.com/v1"
	}
	return &AnthropicProvider{
		baseURL: baseURL,
		apiKey:  apiKey,
		client:  &http.Client{Timeout: 120 * time.Second},
	}
}

func (p *AnthropicProvider) Chat(ctx context.Context, req port.LLMChatRequest, effort string, effortCfg *EffortConfig) (port.LLMChatResponse, error) {
	model := req.Model
	if model == "" {
		return port.LLMChatResponse{}, fmt.Errorf("model not specified")
	}

	system := ""
	messages := make([]map[string]any, 0, len(req.Messages))
	for _, m := range req.Messages {
		if m.Role == "system" {
			system = m.Content
			continue
		}
		msg := map[string]any{
			"role": m.Role,
		}
		if m.Role == "assistant" && len(m.ToolCalls) > 0 {
			var contents []map[string]any
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
			msg["content"] = []map[string]any{{
				"type":        "tool_result",
				"tool_use_id": m.ToolCallID,
				"content":     m.Content,
			}}
		} else {
			msg["content"] = anthropicUserContent(m)
		}
		messages = append(messages, msg)
	}

	body := map[string]any{
		"model":      model,
		"messages":   messages,
	}
	if req.GenParams != nil {
		gp := req.GenParams
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
	if _, ok := body["max_tokens"]; !ok {
		body["max_tokens"] = 4096
	}
	if system != "" {
		body["system"] = system
	}
	if len(req.Tools) > 0 && req.ToolChoice != "none" {
		var tools []map[string]any
		for _, t := range req.Tools {
			tools = append(tools, map[string]any{
				"name":         t.Name,
				"description":  t.Description,
				"input_schema": t.Parameters,
			})
		}
		body["tools"] = tools
	}

	ApplyReasoningEffort(domain.LLMProviderAnthropic, effort, effortCfg, body)

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
		return port.LLMChatResponse{}, err
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return port.LLMChatResponse{}, err
	}
	if resp.StatusCode != http.StatusOK {
		return port.LLMChatResponse{}, classifyHTTPError(resp.StatusCode, respBody)
	}

	var result struct {
		Content []struct {
			Type   string `json:"type"`
			Text   string `json:"text"`
			ID     string `json:"id"`
			Name   string `json:"name"`
			Input  map[string]any
		} `json:"content"`
		Usage struct {
			InputTokens  int `json:"input_tokens"`
			OutputTokens int `json:"output_tokens"`
		} `json:"usage"`
	}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return port.LLMChatResponse{}, err
	}

	usage := &port.LLMUsage{
		PromptTokens:     result.Usage.InputTokens,
		CompletionTokens: result.Usage.OutputTokens,
		TotalTokens:      result.Usage.InputTokens + result.Usage.OutputTokens,
	}

	content := ""
	var toolCalls []port.ChatToolCall
	for _, c := range result.Content {
		switch c.Type {
		case "text":
			content += c.Text
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

	if len(toolCalls) > 0 {
		return port.LLMChatResponse{ToolCalls: toolCalls, Usage: usage}, nil
	}

	return port.LLMChatResponse{Content: content, Usage: usage, Done: true}, nil
}
