package llm

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"danmo-work/core/domain"
	"danmo-work/core/port"
)

func TestAnthropicMessagesClientConcatenatesSystemAndCacheControl(t *testing.T) {
	var gotBody map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body)
		if err := json.Unmarshal(raw, &gotBody); err != nil {
			t.Errorf("unmarshal request: %v", err)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"content": []map[string]any{{"type": "text", "text": "ok"}},
			"usage": map[string]any{
				"input_tokens":                10,
				"output_tokens":               2,
				"cache_creation_input_tokens": 8,
				"cache_read_input_tokens":     100,
			},
		})
	}))
	defer server.Close()

	p := NewAnthropicMessagesClient(server.URL, "k")
	resp, err := p.Chat(context.Background(), port.LLMChatRequest{
		Model: "claude-sonnet-4-5",
		Messages: []port.ChatMessage{
			{Role: "system", Content: "persona and policies"},
			{Role: "user", Content: "goal"},
			{Role: "user", Content: "<turn-context>\nnow\n</turn-context>"},
		},
		Tools: []domain.ToolSchema{
			{Name: "read_file", Description: "read", Parameters: map[string]any{"type": "object"}},
			{Name: "write", Description: "write", Parameters: map[string]any{"type": "object"}},
		},
	}, "", nil)
	if err != nil {
		t.Fatalf("chat: %v", err)
	}
	if resp.Usage == nil {
		t.Fatal("usage not parsed")
	}
	if resp.Usage.CacheReadTokens != 100 || resp.Usage.CacheCreationTokens != 8 {
		t.Fatalf("cache tokens: %+v", resp.Usage)
	}
	if resp.Usage.PromptTokens != 10 || resp.Usage.CompletionTokens != 2 {
		t.Fatalf("prompt/completion: %+v", resp.Usage)
	}

	sys, ok := gotBody["system"].([]any)
	if !ok || len(sys) != 1 {
		t.Fatalf("system blocks: %#v", gotBody["system"])
	}
	block, _ := sys[0].(map[string]any)
	if block["text"] != "persona and policies" {
		t.Fatalf("system text: %#v", block)
	}
	cc, _ := block["cache_control"].(map[string]any)
	if cc["type"] != "ephemeral" {
		t.Fatalf("system cache_control: %#v", block["cache_control"])
	}

	tools, _ := gotBody["tools"].([]any)
	if len(tools) != 2 {
		t.Fatalf("tools: %#v", gotBody["tools"])
	}
	last, _ := tools[1].(map[string]any)
	tcc, _ := last["cache_control"].(map[string]any)
	if tcc["type"] != "ephemeral" {
		t.Fatalf("last tool cache_control: %#v", last)
	}
	first, _ := tools[0].(map[string]any)
	if _, ok := first["cache_control"]; ok {
		t.Fatal("only last tool should have cache_control")
	}

	msgs, _ := gotBody["messages"].([]any)
	for _, m := range msgs {
		mm, _ := m.(map[string]any)
		if mm["role"] == "system" {
			t.Fatal("Anthropic messages must not include system role")
		}
	}
	if len(msgs) != 2 {
		t.Fatalf("expected 2 non-system messages, got %#v", msgs)
	}
}

func TestAnthropicMessagesClientDoesNotFoldTurnContextIntoSystem(t *testing.T) {
	var gotBody map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(raw, &gotBody)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"content": []map[string]any{{"type": "text", "text": "ok"}},
			"usage":   map[string]any{"input_tokens": 1, "output_tokens": 1},
		})
	}))
	defer server.Close()

	p := NewAnthropicMessagesClient(server.URL, "k")
	_, err := p.Chat(context.Background(), port.LLMChatRequest{
		Model: "claude-sonnet-4-5",
		Messages: []port.ChatMessage{
			{Role: "system", Content: "persona"},
			{Role: "user", Content: "<turn-context>clock</turn-context>"},
		},
	}, "", nil)
	if err != nil {
		t.Fatalf("chat: %v", err)
	}
	raw, err := json.Marshal(gotBody["system"])
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(raw), "<turn-context>") {
		t.Fatalf("turn-context must not be folded into system: %s", raw)
	}
	if !strings.Contains(string(raw), "persona") {
		t.Fatalf("system should keep persona: %s", raw)
	}
}
