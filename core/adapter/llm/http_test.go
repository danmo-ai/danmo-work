package llm

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"danmo-work/core/port"
)

func TestHTTPProviderParsesUsage(t *testing.T) {
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
			},
		})
	}))
	defer server.Close()

	p := NewHTTPProvider(server.URL, "")
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
}

func TestHTTPProviderOmitsEmptyUsage(t *testing.T) {
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

	p := NewHTTPProvider(server.URL, "")
	resp, err := p.Chat(context.Background(), port.LLMChatRequest{Model: "gpt-4o"}, "")
	if err != nil {
		t.Fatalf("chat: %v", err)
	}
	if resp.Usage != nil {
		t.Errorf("expected nil usage, got %+v", resp.Usage)
	}
}

func TestHTTPProviderToolCallWithArguments(t *testing.T) {
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

	p := NewHTTPProvider(server.URL, "")
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

func TestHTTPProviderToolCallNullArguments(t *testing.T) {
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

	p := NewHTTPProvider(server.URL, "")
	_, err := p.Chat(context.Background(), port.LLMChatRequest{Model: "test"}, "")
	if err == nil {
		t.Fatal("expected error for null arguments")
	}
	t.Logf("got expected error: %v", err)
}

func TestHTTPProviderToolCallEmptyStringArguments(t *testing.T) {
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

	p := NewHTTPProvider(server.URL, "")
	_, err := p.Chat(context.Background(), port.LLMChatRequest{Model: "test"}, "")
	if err == nil {
		t.Fatal("expected error for empty string arguments")
	}
	t.Logf("got expected error: %v", err)
}
