package builtin

import (
	"context"
	"strings"
	"testing"

	"danmo-work/core/port"
)

type stubToolRecaller struct {
	byTurn    map[string]map[string]port.RecalledToolResult
	bySession map[string]map[string]port.RecalledToolResult
}

func (s *stubToolRecaller) RecallToolResult(turnID, callID string) (port.RecalledToolResult, bool) {
	if s.byTurn == nil {
		return port.RecalledToolResult{}, false
	}
	if m, ok := s.byTurn[turnID]; ok {
		r, ok := m[callID]
		return r, ok
	}
	return port.RecalledToolResult{}, false
}

func (s *stubToolRecaller) RecallToolResultInSession(sessionID, callID string) (port.RecalledToolResult, bool) {
	if s.bySession == nil {
		return port.RecalledToolResult{}, false
	}
	if m, ok := s.bySession[sessionID]; ok {
		r, ok := m[callID]
		return r, ok
	}
	return port.RecalledToolResult{}, false
}

func TestRecallToolResultExecute(t *testing.T) {
	store := &stubToolRecaller{
		byTurn: map[string]map[string]port.RecalledToolResult{
			"turn-1": {
				"c1": {
					TurnID: "turn-1", CallID: "c1", ToolName: "grep", Output: "full grep output",
				},
			},
		},
		bySession: map[string]map[string]port.RecalledToolResult{
			"sess-1": {
				"c2": {
					TurnID: "turn-0", CallID: "c2", ToolName: "read_file", Output: "session fallback",
				},
			},
		},
	}
	h := &RecallToolResult{Store: store}

	res, err := h.Execute(context.Background(), map[string]any{
		"call_id": "c1", "__turn_id": "turn-1", "__session_id": "sess-1",
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(res.Content, "full grep output") {
		t.Fatalf("unexpected content: %q", res.Content)
	}

	res, err = h.Execute(context.Background(), map[string]any{
		"call_id": "c2", "__session_id": "sess-1",
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(res.Content, "session fallback") {
		t.Fatalf("unexpected session fallback: %q", res.Content)
	}

	if _, err := h.Execute(context.Background(), map[string]any{"call_id": "missing"}); err == nil {
		t.Fatal("expected not found error")
	}
}
