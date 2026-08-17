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

// DefaultChatHTTPTimeout covers waiting for the full non-streaming response.
// High-effort reasoning models (e.g. deepseek max) regularly exceed 2 minutes.
const DefaultChatHTTPTimeout = 10 * time.Minute

// OpenAIChatCompletionsClient talks to OpenAI-compatible Chat Completions APIs
// (POST {baseURL}/chat/completions).
type OpenAIChatCompletionsClient struct {
	baseURL string
	apiKey  string
	timeout time.Duration
	client  *http.Client
}

func NewOpenAIChatCompletionsClient(baseURL, apiKey string) *OpenAIChatCompletionsClient {
	return NewOpenAIChatCompletionsClientWithTimeout(baseURL, apiKey, DefaultChatHTTPTimeout)
}

func NewOpenAIChatCompletionsClientWithTimeout(baseURL, apiKey string, timeout time.Duration) *OpenAIChatCompletionsClient {
	if baseURL == "" {
		baseURL = "https://api.openai.com/v1"
	}
	if timeout <= 0 {
		timeout = DefaultChatHTTPTimeout
	}
	return &OpenAIChatCompletionsClient{
		baseURL: strings.TrimRight(baseURL, "/"),
		apiKey:  apiKey,
		timeout: timeout,
		client:  &http.Client{Timeout: timeout},
	}
}

func (p *OpenAIChatCompletionsClient) Chat(ctx context.Context, req port.LLMChatRequest, effort string) (port.LLMChatResponse, error) {
	model := req.Model
	if model == "" {
		return port.LLMChatResponse{}, fmt.Errorf("model not specified")
	}

	dialect := resolveReasoningDialect(req.GenParams, model)
	echoReasoning := dialectEchoesReasoning(dialect)

	messages := make([]map[string]any, 0, len(req.Messages))
	for _, m := range req.Messages {
		msg := map[string]any{
			"role": m.Role,
		}
		// Assistant messages with tool_calls must omit content (or set null)
		// per OpenAI API spec. Empty string causes some providers (e.g. DeepSeek)
		// to not recognize the message as a tool_calls carrier, making subsequent
		// tool messages appear unpaired → 400 error.
		if len(m.Parts) > 0 && len(m.ToolCalls) == 0 {
			msg["content"] = openaiUserContent(m)
		} else if m.Content != "" || len(m.ToolCalls) == 0 {
			msg["content"] = m.Content
		}
		if len(m.ToolCalls) > 0 {
			var tcs []map[string]any
			for _, tc := range m.ToolCalls {
				tcs = append(tcs, map[string]any{
					"id":   tc.ID,
					"type": "function",
					"function": map[string]any{
						"name":      tc.Name,
						"arguments": marshalArgs(tc.Arguments),
					},
				})
			}
			msg["tool_calls"] = tcs
		}
		if m.ToolCallID != "" {
			msg["tool_call_id"] = m.ToolCallID
			msg["name"] = m.Name
		}
		if echoReasoning && m.ReasoningContent != "" {
			msg["reasoning_content"] = m.ReasoningContent
		}
		messages = append(messages, msg)
	}

	body := map[string]any{
		"model":    model,
		"messages": messages,
	}
	applyChatCompletionsGenParams(body, req.GenParams)
	if len(req.Tools) > 0 {
		var tools []map[string]any
		for _, t := range req.Tools {
			tools = append(tools, map[string]any{
				"type": "function",
				"function": map[string]any{
					"name":        t.Name,
					"description": t.Description,
					"parameters":  t.Parameters,
				},
			})
		}
		body["tools"] = tools
		if req.ToolChoice != "" {
			body["tool_choice"] = req.ToolChoice
		} else {
			body["tool_choice"] = "auto"
		}
	}

	applyReasoningDialectRequest(body, dialect, effort, req.GenParams)

	b, err := json.Marshal(body)
	if err != nil {
		return port.LLMChatResponse{}, err
	}

	hReq, err := http.NewRequestWithContext(ctx, http.MethodPost, p.baseURL+"/chat/completions", bytes.NewReader(b))
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

	out, err := parseChatCompletionsResponse(respBody)
	if err != nil {
		return port.LLMChatResponse{}, err
	}
	content, reasoning := normalizeChatCompletionsReasoning(out.Content, out.ReasoningContent, "", dialect)
	out.Content = content
	out.ReasoningContent = reasoning
	return out, nil
}

func parseChatCompletionsResponse(respBody []byte) (port.LLMChatResponse, error) {
	var result struct {
		Choices []struct {
			Message struct {
				Content          string `json:"content"`
				ReasoningContent string `json:"reasoning_content"`
				Reasoning        string `json:"reasoning"`
				ToolCalls        []struct {
					ID       string `json:"id"`
					Function struct {
						Name      string          `json:"name"`
						Arguments json.RawMessage `json:"arguments"`
					} `json:"function"`
				} `json:"tool_calls"`
			} `json:"message"`
		} `json:"choices"`
		Usage struct {
			PromptTokens        int `json:"prompt_tokens"`
			CompletionTokens    int `json:"completion_tokens"`
			TotalTokens         int `json:"total_tokens"`
			PromptTokensDetails struct {
				CachedTokens int `json:"cached_tokens"`
			} `json:"prompt_tokens_details"`
		} `json:"usage"`
	}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return port.LLMChatResponse{}, err
	}
	if len(result.Choices) == 0 {
		return port.LLMChatResponse{}, fmt.Errorf("no choices in llm response")
	}

	usage := &port.LLMUsage{
		PromptTokens:     result.Usage.PromptTokens,
		CompletionTokens: result.Usage.CompletionTokens,
		TotalTokens:      result.Usage.TotalTokens,
		CacheReadTokens:  result.Usage.PromptTokensDetails.CachedTokens,
	}
	if usage.Empty() {
		usage = nil
	}

	choice := result.Choices[0].Message
	reasoning := choice.ReasoningContent
	if reasoning == "" {
		reasoning = choice.Reasoning
	}
	if len(choice.ToolCalls) > 0 {
		var tcs []port.ChatToolCall
		for _, tc := range choice.ToolCalls {
			args, err := parseArgs(tc.Function.Arguments)
			if err != nil {
				return port.LLMChatResponse{}, fmt.Errorf("tool '%s' arguments: %w (raw: %s)", tc.Function.Name, err, string(tc.Function.Arguments))
			}
			tcs = append(tcs, port.ChatToolCall{
				ID:        tc.ID,
				Name:      tc.Function.Name,
				Arguments: args,
			})
		}
		return port.LLMChatResponse{
			ToolCalls:        tcs,
			ReasoningContent: reasoning,
			Usage:            usage,
		}, nil
	}

	return port.LLMChatResponse{
		Content:          choice.Content,
		ReasoningContent: reasoning,
		Usage:            usage,
		Done:             true,
	}, nil
}

// applyChatCompletionsGenParams writes Chat Completions sampling fields.
// Supported: temperature, top_p, frequency_penalty, presence_penalty, stop, max_tokens.
// Zero values mean "omit / use provider default".
func applyChatCompletionsGenParams(body map[string]any, gp *port.ModelGenParams) {
	if gp == nil {
		return
	}
	if gp.Temperature != 0 {
		body["temperature"] = gp.Temperature
	}
	if gp.TopP != 0 {
		body["top_p"] = gp.TopP
	}
	if gp.FrequencyPenalty != 0 {
		body["frequency_penalty"] = gp.FrequencyPenalty
	}
	if gp.PresencePenalty != 0 {
		body["presence_penalty"] = gp.PresencePenalty
	}
	if len(gp.Stop) > 0 {
		body["stop"] = gp.Stop
	}
	if gp.MaxTokens > 0 {
		body["max_tokens"] = gp.MaxTokens
	}
}

func marshalArgs(args map[string]any) string {
	b, _ := json.Marshal(args)
	return string(b)
}

// parseArgs parses tool call arguments from an OpenAI-compatible API response.
// arguments may be a JSON string or a JSON object; both are handled.
// When the model damages JSON structure (unescaped quotes / raw newlines inside
// string values), a best-effort repair is attempted before failing.
func parseArgs(raw json.RawMessage) (map[string]any, error) {
	if len(raw) == 0 || string(raw) == "null" {
		return nil, fmt.Errorf("arguments is null")
	}
	// OpenAI-compatible APIs return arguments as a JSON string, not an object.
	var str string
	if err := json.Unmarshal(raw, &str); err == nil && str != "" {
		raw = json.RawMessage(str)
	}
	var args map[string]any
	if err := json.Unmarshal(raw, &args); err != nil {
		repaired, rerr := repairJSONObject(raw)
		if rerr != nil {
			return nil, err
		}
		if err := json.Unmarshal(repaired, &args); err != nil {
			return nil, err
		}
	}
	if args == nil {
		return nil, fmt.Errorf("arguments parsed to nil")
	}
	return args, nil
}

// classifyHTTPError returns a user-friendly error message for common HTTP
// error codes from LLM APIs.
func classifyHTTPError(statusCode int, body []byte) error {
	switch statusCode {
	case http.StatusUnauthorized:
		return fmt.Errorf("authentication failed (401): check your API key")
	case http.StatusForbidden:
		return fmt.Errorf("access forbidden (403): %s", truncate(body, 200))
	case http.StatusTooManyRequests:
		return fmt.Errorf("rate limit exceeded (429): please retry after a short wait")
	case http.StatusRequestEntityTooLarge:
		return fmt.Errorf("request too large (413): context length exceeded")
	case http.StatusBadRequest:
		// Detect context-length errors from OpenAI-style APIs.
		bodyStr := string(body)
		if strings.Contains(bodyStr, "context_length") || strings.Contains(bodyStr, "maximum context") {
			return fmt.Errorf("context length exceeded: reduce input or use a model with larger context")
		}
		return fmt.Errorf("bad request (400): %s", truncate(body, 200))
	case http.StatusInternalServerError:
		return fmt.Errorf("provider internal error (500): %s", truncate(body, 200))
	default:
		return fmt.Errorf("llm http %d: %s", statusCode, truncate(body, 200))
	}
}

func wrapHTTPTimeout(err error, timeout time.Duration) error {
	if err == nil {
		return nil
	}
	msg := err.Error()
	if strings.Contains(msg, "Client.Timeout") || strings.Contains(msg, "context deadline exceeded") {
		sec := int(timeout / time.Second)
		if sec <= 0 {
			sec = int(DefaultChatHTTPTimeout / time.Second)
		}
		return fmt.Errorf("LLM request timed out after %ds (raise runtime.turn.llm_http_timeout_sec for long reasoning): %w", sec, err)
	}
	return err
}

func truncate(b []byte, maxLen int) string {
	s := string(b)
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
