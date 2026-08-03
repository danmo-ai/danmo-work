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

func TestBuildResponsesBody_MapsMessagesAndTools(t *testing.T) {
	req := port.LLMChatRequest{
		Model: "gpt-4o",
		Messages: []port.ChatMessage{
			{Role: "system", Content: "You are helpful."},
			{Role: "user", Content: "list files"},
			{
				Role: "assistant",
				ToolCalls: []port.ChatToolCall{
					{ID: "call_1", Name: "bash", Arguments: map[string]any{"command": "ls"}},
				},
			},
			{Role: "tool", ToolCallID: "call_1", Content: "a.txt\nb.txt"},
		},
		Tools: []domain.ToolSchema{
			{Name: "bash", Description: "run shell", Parameters: map[string]any{"type": "object"}},
		},
		GenParams: &port.ModelGenParams{MaxTokens: 1024, Temperature: 0.2},
	}
	body, err := buildResponsesBody(req, "high")
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	if body["model"] != "gpt-4o" {
		t.Errorf("model: %v", body["model"])
	}
	if body["instructions"] != "You are helpful." {
		t.Errorf("instructions: %v", body["instructions"])
	}
	if body["store"] != false {
		t.Errorf("store should be false, got %v", body["store"])
	}
	if body["max_output_tokens"] != 1024 {
		t.Errorf("max_output_tokens: %v", body["max_output_tokens"])
	}
	reasoning, ok := body["reasoning"].(map[string]any)
	if !ok || reasoning["effort"] != "high" {
		t.Errorf("reasoning: %v", body["reasoning"])
	}
	input, ok := body["input"].([]any)
	if !ok || len(input) != 3 {
		t.Fatalf("input len: %v", body["input"])
	}
	user := input[0].(map[string]any)
	if user["role"] != "user" || user["content"] != "list files" {
		t.Errorf("user item: %v", user)
	}
	fc := input[1].(map[string]any)
	if fc["type"] != "function_call" || fc["call_id"] != "call_1" || fc["name"] != "bash" {
		t.Errorf("function_call: %v", fc)
	}
	out := input[2].(map[string]any)
	if out["type"] != "function_call_output" || out["call_id"] != "call_1" {
		t.Errorf("function_call_output: %v", out)
	}
	tools, ok := body["tools"].([]map[string]any)
	if !ok || len(tools) != 1 || tools[0]["type"] != "function" || tools[0]["name"] != "bash" {
		t.Errorf("tools: %v", body["tools"])
	}
	// Ensure Chat Completions nesting is NOT used.
	if _, hasFn := tools[0]["function"]; hasFn {
		t.Fatal("responses tools must not nest under function")
	}
}

func TestBuildResponsesBody_SkipsUnsupportedGenParams(t *testing.T) {
	body, err := buildResponsesBody(port.LLMChatRequest{
		Model: "gpt-4o",
		Messages: []port.ChatMessage{
			{Role: "user", Content: "hi"},
		},
		GenParams: &port.ModelGenParams{
			Temperature:      0.5,
			TopP:             0.9,
			MaxTokens:        256,
			FrequencyPenalty: 0.4,
			PresencePenalty:  0.3,
			Stop:             []string{"###"},
		},
	}, "")
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	if body["temperature"] != 0.5 || body["top_p"] != 0.9 || body["max_output_tokens"] != 256 {
		t.Errorf("supported fields: %v", body)
	}
	for _, key := range []string{"frequency_penalty", "presence_penalty", "stop", "max_tokens"} {
		if _, ok := body[key]; ok {
			t.Errorf("responses body must not include %q", key)
		}
	}
}

func TestParseResponsesBody_TextAndUsage(t *testing.T) {
	raw := []byte(`{
		"output": [
			{"type":"message","role":"assistant","content":[{"type":"output_text","text":"hello"}]}
		],
		"usage": {"input_tokens": 11, "output_tokens": 2, "total_tokens": 13}
	}`)
	resp, err := parseResponsesBody(raw)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if resp.Content != "hello" {
		t.Errorf("content: %q", resp.Content)
	}
	if !resp.Done {
		t.Error("expected Done")
	}
	if resp.Usage == nil || resp.Usage.PromptTokens != 11 || resp.Usage.CompletionTokens != 2 {
		t.Errorf("usage: %+v", resp.Usage)
	}
}

func TestParseResponsesBody_FunctionCall(t *testing.T) {
	raw := []byte(`{
		"output": [
			{"type":"function_call","call_id":"c1","name":"bash","arguments":"{\"command\":\"pwd\"}"}
		],
		"usage": {"input_tokens": 1, "output_tokens": 1, "total_tokens": 2}
	}`)
	resp, err := parseResponsesBody(raw)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(resp.ToolCalls) != 1 {
		t.Fatalf("tool calls: %+v", resp.ToolCalls)
	}
	if resp.ToolCalls[0].ID != "c1" || resp.ToolCalls[0].Name != "bash" {
		t.Errorf("tc: %+v", resp.ToolCalls[0])
	}
	if resp.ToolCalls[0].Arguments["command"] != "pwd" {
		t.Errorf("args: %+v", resp.ToolCalls[0].Arguments)
	}
}

func TestOpenAIResponsesClient_Chat(t *testing.T) {
	var gotPath string
	var gotBody map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		_ = json.NewDecoder(r.Body).Decode(&gotBody)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"output": []map[string]any{
				{"type": "message", "role": "assistant", "content": []map[string]any{
					{"type": "output_text", "text": "ok"},
				}},
			},
			"usage": map[string]any{"input_tokens": 3, "output_tokens": 1, "total_tokens": 4},
		})
	}))
	defer server.Close()

	p := NewOpenAIResponsesClient(server.URL+"/v1", "sk-test")
	resp, err := p.Chat(context.Background(), port.LLMChatRequest{
		Model:    "gpt-4o",
		Messages: []port.ChatMessage{{Role: "user", Content: "hi"}},
	}, "")
	if err != nil {
		t.Fatalf("chat: %v", err)
	}
	if gotPath != "/v1/responses" {
		t.Errorf("path: %s", gotPath)
	}
	if gotBody["store"] != false {
		t.Errorf("store: %v", gotBody["store"])
	}
	if resp.Content != "ok" {
		t.Errorf("content: %q", resp.Content)
	}
}
