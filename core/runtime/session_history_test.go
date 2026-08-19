package runtime

import (
	"testing"

	"danmo-work/core/port"
)

func TestChatMessagesToRuntime(t *testing.T) {
	in := []port.ChatMessage{
		{Role: "user", Content: "u"},
		{Role: "assistant", Content: "a", ToolCalls: []port.ChatToolCall{{ID: "1", Name: "read_file", Arguments: map[string]any{"path": "x"}}}},
		{Role: "tool", ToolCallID: "1", Name: "read_file", Content: "out"},
	}
	out := chatMessagesToRuntime(in)
	if len(out) != 3 {
		t.Fatalf("len=%d", len(out))
	}
	if out[1].Role != RoleAssistant || len(out[1].ToolCalls) != 1 || out[1].ToolCalls[0].Name != "read_file" {
		t.Fatalf("assistant: %+v", out[1])
	}
	if out[2].Role != RoleTool || out[2].Content != "out" {
		t.Fatalf("tool: %+v", out[2])
	}
}

func TestHistoryHasUserGoal(t *testing.T) {
	h := []Message{{Role: RoleUser, Content: "goal-a"}, {Role: RoleAssistant, Content: "ok"}}
	if !historyHasUserGoal(h, "goal-a") {
		t.Fatal("expected to find goal-a")
	}
	if historyHasUserGoal(h, "other") {
		t.Fatal("should not find other")
	}
}
