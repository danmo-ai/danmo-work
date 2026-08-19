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

	"danmo-work/core/port"
)

// OpenAIResponsesClient talks to the OpenAI Responses API
// (POST {baseURL}/responses). Request/response mapping is independent of
// Chat Completions — do not share message assembly or choice parsing.
type OpenAIResponsesClient struct {
	baseURL string
	apiKey  string
	timeout time.Duration
	client  *http.Client
}

func NewOpenAIResponsesClient(baseURL, apiKey string) *OpenAIResponsesClient {
	return NewOpenAIResponsesClientWithTimeout(baseURL, apiKey, DefaultChatHTTPTimeout)
}

func NewOpenAIResponsesClientWithTimeout(baseURL, apiKey string, timeout time.Duration) *OpenAIResponsesClient {
	if baseURL == "" {
		baseURL = "https://api.openai.com/v1"
	}
	if timeout <= 0 {
		timeout = DefaultChatHTTPTimeout
	}
	return &OpenAIResponsesClient{
		baseURL: strings.TrimRight(baseURL, "/"),
		apiKey:  apiKey,
		timeout: timeout,
		client:  &http.Client{Timeout: timeout},
	}
}

func (p *OpenAIResponsesClient) Chat(ctx context.Context, req port.LLMChatRequest, effort string) (port.LLMChatResponse, error) {
	model := req.Model
	if model == "" {
		return port.LLMChatResponse{}, fmt.Errorf("model not specified")
	}

	body, err := buildResponsesBody(req, effort)
	if err != nil {
		return port.LLMChatResponse{}, err
	}
	b, err := json.Marshal(body)
	if err != nil {
		return port.LLMChatResponse{}, err
	}

	hReq, err := http.NewRequestWithContext(ctx, http.MethodPost, p.baseURL+"/responses", bytes.NewReader(b))
	if err != nil {
		return port.LLMChatResponse{}, err
	}
	hReq.Header.Set("Content-Type", "application/json")
	if p.apiKey != "" {
		hReq.Header.Set("Authorization", "Bearer "+p.apiKey)
	}

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

	return parseResponsesBody(respBody)
}

// buildResponsesBody maps port.LLMChatRequest onto a Responses API payload.
// Exported for tests via the package-private name (same package).
func buildResponsesBody(req port.LLMChatRequest, effort string) (map[string]any, error) {
	instructions := ""
	input := make([]any, 0, len(req.Messages))

	for _, m := range req.Messages {
		switch m.Role {
		case "system":
			if instructions == "" {
				instructions = m.Content
			} else {
				instructions = instructions + "\n\n" + m.Content
			}
		case "tool":
			input = append(input, map[string]any{
				"type":    "function_call_output",
				"call_id": m.ToolCallID,
				"output":  openaiToolOutput(m),
			})
		case "assistant":
			if len(m.ToolCalls) > 0 {
				if m.Content != "" {
					input = append(input, map[string]any{
						"type":    "message",
						"role":    "assistant",
						"content": m.Content,
					})
				}
				for _, tc := range m.ToolCalls {
					input = append(input, map[string]any{
						"type":      "function_call",
						"call_id":   tc.ID,
						"name":      tc.Name,
						"arguments": marshalArgs(tc.Arguments),
					})
				}
			} else {
				input = append(input, map[string]any{
					"type":    "message",
					"role":    "assistant",
					"content": responsesMessageContent(m),
				})
			}
		default: // user / other
			input = append(input, map[string]any{
				"type":    "message",
				"role":    "user",
				"content": responsesMessageContent(m),
			})
		}
	}

	body := map[string]any{
		"model": req.Model,
		"input": input,
		"store": false,
	}
	if instructions != "" {
		body["instructions"] = instructions
	}
	// Responses Create supports temperature, top_p, max_output_tokens (+ reasoning).
	// frequency_penalty / presence_penalty / stop are Chat Completions-only — omit here.
	applyResponsesGenParams(body, req.GenParams)
	if len(req.Tools) > 0 {
		tools := make([]map[string]any, 0, len(req.Tools))
		for _, t := range req.Tools {
			tools = append(tools, map[string]any{
				"type":        "function",
				"name":        t.Name,
				"description": t.Description,
				"parameters":  t.Parameters,
			})
		}
		body["tools"] = tools
		if req.ToolChoice != "" {
			body["tool_choice"] = req.ToolChoice
		} else {
			body["tool_choice"] = "auto"
		}
	}
	if effort != "" && effort != "off" {
		body["reasoning"] = map[string]any{"effort": effort}
	}
	return body, nil
}

// applyResponsesGenParams writes Responses API sampling fields only.
// Supported: temperature, top_p, max_output_tokens.
// Not supported by Responses Create (silently skipped): frequency_penalty, presence_penalty, stop.
func applyResponsesGenParams(body map[string]any, gp *port.ModelGenParams) {
	if gp == nil {
		return
	}
	if gp.Temperature != 0 {
		body["temperature"] = gp.Temperature
	}
	if gp.TopP != 0 {
		body["top_p"] = gp.TopP
	}
	if gp.MaxTokens > 0 {
		body["max_output_tokens"] = gp.MaxTokens
	}
}

func responsesMessageContent(m port.ChatMessage) any {
	if len(m.Parts) == 0 {
		return m.Content
	}
	parts := make([]map[string]any, 0, len(m.Parts))
	for _, p := range m.Parts {
		switch p.Type {
		case "image":
			mime := p.MimeType
			if mime == "" {
				mime = "image/png"
			}
			parts = append(parts, map[string]any{
				"type":      "input_image",
				"image_url": fmt.Sprintf("data:%s;base64,%s", mime, p.Data),
			})
		default:
			text := p.Text
			if text == "" {
				text = m.Content
			}
			parts = append(parts, map[string]any{
				"type": "input_text",
				"text": text,
			})
		}
	}
	if len(parts) == 0 {
		return m.Content
	}
	return parts
}

func parseResponsesBody(respBody []byte) (port.LLMChatResponse, error) {
	var result struct {
		Output []struct {
			Type   string `json:"type"`
			Status string `json:"status"`
			Role   string `json:"role"`
			// message content parts
			Content []struct {
				Type string `json:"type"`
				Text string `json:"text"`
			} `json:"content"`
			// function_call fields
			CallID    string `json:"call_id"`
			Name      string `json:"name"`
			Arguments string `json:"arguments"`
			// reasoning summary (optional)
			Summary []struct {
				Type string `json:"type"`
				Text string `json:"text"`
			} `json:"summary"`
		} `json:"output"`
		Usage struct {
			InputTokens        int `json:"input_tokens"`
			OutputTokens       int `json:"output_tokens"`
			TotalTokens        int `json:"total_tokens"`
			InputTokensDetails struct {
				CachedTokens int `json:"cached_tokens"`
			} `json:"input_tokens_details"`
		} `json:"usage"`
	}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return port.LLMChatResponse{}, err
	}
	if len(result.Output) == 0 {
		return port.LLMChatResponse{}, fmt.Errorf("no output in responses api result")
	}

	usage := &port.LLMUsage{
		PromptTokens:     result.Usage.InputTokens,
		CompletionTokens: result.Usage.OutputTokens,
		TotalTokens:      result.Usage.TotalTokens,
		CacheReadTokens:  result.Usage.InputTokensDetails.CachedTokens,
	}
	if usage.Empty() {
		usage = nil
	}

	var content strings.Builder
	var reasoning strings.Builder
	var tcs []port.ChatToolCall

	for _, item := range result.Output {
		switch item.Type {
		case "message":
			for _, part := range item.Content {
				if part.Type == "output_text" || part.Type == "text" {
					content.WriteString(part.Text)
				}
			}
		case "function_call":
			args, repairedFrom, err := parseArgs(json.RawMessage(item.Arguments))
			if err != nil {
				return port.LLMChatResponse{}, fmt.Errorf("tool '%s' arguments: %w (raw: %s)", item.Name, err, item.Arguments)
			}
			tcs = append(tcs, port.ChatToolCall{
				ID:           item.CallID,
				Name:         item.Name,
				Arguments:    args,
				RepairedFrom: repairedFrom,
			})
		case "reasoning":
			for _, s := range item.Summary {
				if s.Text != "" {
					if reasoning.Len() > 0 {
						reasoning.WriteByte('\n')
					}
					reasoning.WriteString(s.Text)
				}
			}
		}
	}

	if len(tcs) > 0 {
		return port.LLMChatResponse{
			Content:          content.String(),
			ToolCalls:        tcs,
			ReasoningContent: reasoning.String(),
			Usage:            usage,
		}, nil
	}

	return port.LLMChatResponse{
		Content:          content.String(),
		ReasoningContent: reasoning.String(),
		Usage:            usage,
		Done:             true,
	}, nil
}
