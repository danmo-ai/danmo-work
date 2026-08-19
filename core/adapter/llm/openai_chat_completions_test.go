package llm

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"danmo-work/core/domain"
	"danmo-work/core/port"
)

func TestOpenAIChatCompletionsClientParsesUsage(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"choices": []map[string]any{
				{
					"message": map[string]any{
						"content": "hello",
					},
				},
			},
			"usage": map[string]any{
				"prompt_tokens":     10,
				"completion_tokens": 5,
				"total_tokens":      15,
				"prompt_tokens_details": map[string]any{
					"cached_tokens": 7,
				},
			},
		})
	}))
	defer server.Close()

	p := NewOpenAIChatCompletionsClient(server.URL, "")
	resp, err := p.Chat(context.Background(), port.LLMChatRequest{Model: "gpt-4o"}, "")
	if err != nil {
		t.Fatalf("chat: %v", err)
	}
	if resp.Content != "hello" {
		t.Errorf("content: got %q", resp.Content)
	}
	if resp.Usage == nil {
		t.Fatal("usage not parsed")
	}
	if resp.Usage.PromptTokens != 10 {
		t.Errorf("prompt tokens: got %d", resp.Usage.PromptTokens)
	}
	if resp.Usage.CompletionTokens != 5 {
		t.Errorf("completion tokens: got %d", resp.Usage.CompletionTokens)
	}
	if resp.Usage.TotalTokens != 15 {
		t.Errorf("total tokens: got %d", resp.Usage.TotalTokens)
	}
	if resp.Usage.CacheReadTokens != 7 {
		t.Errorf("cache read tokens: got %d", resp.Usage.CacheReadTokens)
	}
}

func TestOpenAIChatCompletionsClientOmitsEmptyUsage(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"choices": []map[string]any{
				{
					"message": map[string]any{
						"content": "hi",
					},
				},
			},
		})
	}))
	defer server.Close()

	p := NewOpenAIChatCompletionsClient(server.URL, "")
	resp, err := p.Chat(context.Background(), port.LLMChatRequest{Model: "gpt-4o"}, "")
	if err != nil {
		t.Fatalf("chat: %v", err)
	}
	if resp.Usage != nil {
		t.Errorf("expected nil usage, got %+v", resp.Usage)
	}
}

func TestOpenAIChatCompletionsClientToolCallWithArguments(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"choices": []map[string]any{
				{
					"message": map[string]any{
						"tool_calls": []map[string]any{
							{
								"id": "call_1",
								"function": map[string]any{
									"name":      "ask_user",
									"arguments": `{"question":"hello?"}`,
								},
							},
						},
					},
				},
			},
		})
	}))
	defer server.Close()

	p := NewOpenAIChatCompletionsClient(server.URL, "")
	resp, err := p.Chat(context.Background(), port.LLMChatRequest{Model: "test"}, "")
	if err != nil {
		t.Fatalf("chat: %v", err)
	}
	if len(resp.ToolCalls) != 1 {
		t.Fatalf("expected 1 tool call, got %d", len(resp.ToolCalls))
	}
	tc := resp.ToolCalls[0]
	if tc.Name != "ask_user" {
		t.Errorf("name: got %q", tc.Name)
	}
	if tc.Arguments["question"] != "hello?" {
		t.Errorf("arguments: got %+v", tc.Arguments)
	}
}

func TestOpenAIChatCompletionsClientToolCallNullArguments(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"choices": []map[string]any{
				{
					"message": map[string]any{
						"tool_calls": []map[string]any{
							{
								"id": "call_2",
								"function": map[string]any{
									"name":      "ask_user",
									"arguments": nil,
								},
							},
						},
					},
				},
			},
		})
	}))
	defer server.Close()

	p := NewOpenAIChatCompletionsClient(server.URL, "")
	_, err := p.Chat(context.Background(), port.LLMChatRequest{Model: "test"}, "")
	if err == nil {
		t.Fatal("expected error for null arguments")
	}
	t.Logf("got expected error: %v", err)
}

func TestOpenAIChatCompletionsClientToolCallRepairsUnescapedQuotes(t *testing.T) {
	// Simulate provider returning arguments as a JSON string that itself contains
	// unescaped quotes (common LLM damage for write/edit content).
	brokenInner := `{"path":"note.txt","content":"quote "x" here"}`
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"choices": []map[string]any{
				{
					"message": map[string]any{
						"tool_calls": []map[string]any{
							{
								"id": "call_repair",
								"function": map[string]any{
									"name":      "write_file",
									"arguments": brokenInner,
								},
							},
						},
					},
				},
			},
		})
	}))
	defer server.Close()

	p := NewOpenAIChatCompletionsClient(server.URL, "")
	resp, err := p.Chat(context.Background(), port.LLMChatRequest{Model: "test"}, "")
	if err != nil {
		t.Fatalf("chat: %v", err)
	}
	if len(resp.ToolCalls) != 1 {
		t.Fatalf("expected 1 tool call, got %d", len(resp.ToolCalls))
	}
	tc := resp.ToolCalls[0]
	if tc.Name != "write_file" {
		t.Errorf("name: got %q", tc.Name)
	}
	if tc.Arguments["path"] != "note.txt" {
		t.Errorf("path: got %#v", tc.Arguments["path"])
	}
	if tc.Arguments["content"] != `quote "x" here` {
		t.Errorf("content: got %#v", tc.Arguments["content"])
	}
	if tc.RepairedFrom != brokenInner {
		t.Errorf("RepairedFrom should carry pre-repair bytes for audit: got %q want %q", tc.RepairedFrom, brokenInner)
	}
}

func TestOpenAIChatCompletionsClientToolCallEmptyStringArguments(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"choices": []map[string]any{
				{
					"message": map[string]any{
						"tool_calls": []map[string]any{
							{
								"id": "call_3",
								"function": map[string]any{
									"name":      "grep",
									"arguments": "",
								},
							},
						},
					},
				},
			},
		})
	}))
	defer server.Close()

	p := NewOpenAIChatCompletionsClient(server.URL, "")
	_, err := p.Chat(context.Background(), port.LLMChatRequest{Model: "test"}, "")
	if err == nil {
		t.Fatal("expected error for empty string arguments")
	}
	t.Logf("got expected error: %v", err)
}

func TestApplyChatCompletionsGenParams(t *testing.T) {
	body := map[string]any{}
	applyChatCompletionsGenParams(body, &port.ModelGenParams{
		Temperature:      0.4,
		TopP:             0.8,
		FrequencyPenalty: 0.2,
		PresencePenalty:  0.1,
		Stop:             []string{"END"},
		MaxTokens:        512,
	})
	if body["temperature"] != 0.4 || body["top_p"] != 0.8 {
		t.Errorf("sampling: %v", body)
	}
	if body["frequency_penalty"] != 0.2 || body["presence_penalty"] != 0.1 {
		t.Errorf("penalties: %v", body)
	}
	stop, _ := body["stop"].([]string)
	if len(stop) != 1 || stop[0] != "END" {
		t.Errorf("stop: %v", body["stop"])
	}
	if body["max_tokens"] != 512 {
		t.Errorf("max_tokens: %v", body["max_tokens"])
	}
}

func TestOpenAIChatCompletionsClient_SendsGenParams(t *testing.T) {
	var got map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&got)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"choices": []map[string]any{{"message": map[string]any{"content": "ok"}}},
		})
	}))
	defer server.Close()

	p := NewOpenAIChatCompletionsClient(server.URL, "")
	_, err := p.Chat(context.Background(), port.LLMChatRequest{
		Model: "gpt-4o",
		GenParams: &port.ModelGenParams{
			Temperature:      0.3,
			FrequencyPenalty: 0.5,
			Stop:             []string{"###"},
			MaxTokens:        100,
		},
	}, "")
	if err != nil {
		t.Fatalf("chat: %v", err)
	}
	if got["temperature"] != 0.3 || got["frequency_penalty"] != 0.5 || got["max_tokens"] != float64(100) {
		t.Errorf("body: %v", got)
	}
	stop, ok := got["stop"].([]any)
	if !ok || len(stop) != 1 || stop[0] != "###" {
		t.Errorf("stop: %v", got["stop"])
	}
}

func TestOpenAIChatCompletionsClient_QwenDialectRequest(t *testing.T) {
	var got map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&got)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"choices": []map[string]any{{
				"message": map[string]any{
					"content":           "ok",
					"reasoning_content": "think",
				},
			}},
		})
	}))
	defer server.Close()

	p := NewOpenAIChatCompletionsClient(server.URL, "")
	resp, err := p.Chat(context.Background(), port.LLMChatRequest{
		Model: "qwen3-max",
		GenParams: &port.ModelGenParams{
			ReasoningDialect: domain.ReasoningDialectQwen,
		},
		Messages: []port.ChatMessage{
			{Role: "assistant", ReasoningContent: "prev", ToolCalls: []port.ChatToolCall{
				{ID: "1", Name: "bash", Arguments: map[string]any{"command": "ls"}},
			}},
			{Role: "tool", ToolCallID: "1", Content: "a"},
		},
	}, "high")
	if err != nil {
		t.Fatalf("chat: %v", err)
	}
	if got["enable_thinking"] != true {
		t.Fatalf("enable_thinking: %v", got)
	}
	if _, ok := got["reasoning_effort"]; ok {
		t.Fatal("unexpected reasoning_effort")
	}
	msgs := got["messages"].([]any)
	first := msgs[0].(map[string]any)
	if first["reasoning_content"] != "prev" {
		t.Fatalf("echo reasoning_content: %v", first)
	}
	if resp.ReasoningContent != "think" {
		t.Fatalf("resp reasoning: %q", resp.ReasoningContent)
	}
}
