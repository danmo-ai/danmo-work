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
	// PromptTokens folds cache read/write into the base input tokens so it
	// reflects true context occupancy (10 + 100 + 8), comparable to OpenAI's
	// prompt_tokens which already includes cached tokens.
	if resp.Usage.PromptTokens != 118 || resp.Usage.CompletionTokens != 2 {
		t.Fatalf("prompt/completion: %+v", resp.Usage)
	}
	if resp.Usage.TotalTokens != 120 {
		t.Fatalf("total: %+v", resp.Usage)
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

func TestAnthropicToolMessageBecomesUserToolResult(t *testing.T) {
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
			{Role: "user", Content: "goal"},
			{Role: "assistant", ToolCalls: []port.ChatToolCall{{ID: "c1", Name: "read_file", Arguments: map[string]any{"path": "a"}}}},
			{Role: "tool", ToolCallID: "c1", Name: "read_file", Content: "file body"},
		},
		Tools:      []domain.ToolSchema{{Name: "read_file", Description: "read", Parameters: map[string]any{"type": "object"}}},
		ToolChoice: "none",
	}, "", nil)
	if err != nil {
		t.Fatalf("chat: %v", err)
	}
	msgs, _ := gotBody["messages"].([]any)
	if len(msgs) != 3 {
		t.Fatalf("messages: %#v", msgs)
	}
	toolMsg, _ := msgs[2].(map[string]any)
	if toolMsg["role"] != "user" {
		t.Fatalf("tool message role must be user, got %#v", toolMsg["role"])
	}
	content, _ := toolMsg["content"].([]any)
	if len(content) != 1 {
		t.Fatalf("tool_result content: %#v", toolMsg["content"])
	}
	block, _ := content[0].(map[string]any)
	if block["type"] != "tool_result" || block["tool_use_id"] != "c1" {
		t.Fatalf("tool_result block: %#v", block)
	}
	// ToolChoice=none must still declare tools and pass tool_choice type none.
	if _, ok := gotBody["tools"].([]any); !ok {
		t.Fatalf("tools must be present with ToolChoice=none: %#v", gotBody["tools"])
	}
	tc, _ := gotBody["tool_choice"].(map[string]any)
	if tc["type"] != "none" {
		t.Fatalf("tool_choice: %#v", gotBody["tool_choice"])
	}
}

func TestAnthropicThinkingBlockReplayAndParse(t *testing.T) {
	var gotBody map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(raw, &gotBody)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"content": []map[string]any{
				{"type": "thinking", "thinking": "let me think", "signature": "sig-abc"},
				{"type": "text", "text": "answer"},
				{"type": "tool_use", "id": "c1", "name": "read_file", "input": map[string]any{"path": "a"}},
			},
			"usage": map[string]any{"input_tokens": 1, "output_tokens": 1},
		})
	}))
	defer server.Close()

	p := NewAnthropicMessagesClient(server.URL, "k")
	resp, err := p.Chat(context.Background(), port.LLMChatRequest{
		Model: "claude-sonnet-4-5",
		Messages: []port.ChatMessage{
			// Assistant turn that carries an echoed thinking block before tool_use.
			{Role: "assistant", ReasoningContent: "prior", ReasoningSignature: "sig-prior",
				ToolCalls: []port.ChatToolCall{{ID: "p1", Name: "read_file", Arguments: map[string]any{}}}},
			{Role: "tool", ToolCallID: "p1", Content: "x"},
		},
	}, "", nil)
	if err != nil {
		t.Fatalf("chat: %v", err)
	}
	// Parse side: thinking + signature + narration text + tool call all survive.
	if resp.ReasoningContent != "let me think" || resp.ReasoningSignature != "sig-abc" {
		t.Fatalf("reasoning not parsed: %+v", resp)
	}
	if resp.Content != "answer" {
		t.Fatalf("assistant text dropped: %q", resp.Content)
	}
	if len(resp.ToolCalls) != 1 {
		t.Fatalf("tool call not parsed: %+v", resp.ToolCalls)
	}
	// Replay side: the assistant message must begin with a thinking block
	// carrying the original signature before tool_use.
	msgs, _ := gotBody["messages"].([]any)
	first, _ := msgs[0].(map[string]any)
	content, _ := first["content"].([]any)
	head, _ := content[0].(map[string]any)
	if head["type"] != "thinking" || head["signature"] != "sig-prior" {
		t.Fatalf("thinking block not replayed first: %#v", content)
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
