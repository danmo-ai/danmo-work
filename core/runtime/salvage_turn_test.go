package runtime

import (
	"context"
	"strings"
	"testing"
	"unicode/utf8"

	"danmo-work/core/domain"
)

func TestKeepCompleteToolPairs_KeepsCompletedDropsUnpaired(t *testing.T) {
	delta := []Message{
		{Role: RoleUser, Content: "change badge color"},
		{
			Role: RoleAssistant,
			ToolCalls: []ToolCall{
				{ID: "c1", Name: "tool_a", Arguments: map[string]any{"pattern": "**/weather.html"}},
				{ID: "c2", Name: "tool_b", Arguments: map[string]any{"question": "which color?"}},
			},
		},
		{Role: RoleTool, ToolCallID: "c1", Name: "tool_a", Content: "/tmp/weather.html"},
		// tool_b never completed — unpaired
	}

	got := keepCompleteToolPairs(delta)
	if len(got) != 3 {
		t.Fatalf("expected user + assistant(tool_a) + tool(tool_a), got %d msgs: %+v", len(got), got)
	}
	if got[0].Role != RoleUser {
		t.Fatalf("first should be user, got %s", got[0].Role)
	}
	if got[1].Role != RoleAssistant || len(got[1].ToolCalls) != 1 || got[1].ToolCalls[0].ID != "c1" {
		t.Fatalf("assistant should keep only completed tool_a call, got %+v", got[1])
	}
	if got[2].Role != RoleTool || got[2].ToolCallID != "c1" {
		t.Fatalf("tool result for tool_a missing, got %+v", got[2])
	}
}

func TestKeepCompleteToolPairs_KeepsCancelledToolResult(t *testing.T) {
	delta := []Message{
		{Role: RoleUser, Content: "ask me"},
		{Role: RoleAssistant, ToolCalls: []ToolCall{{ID: "c1", Name: "tool_wait"}}},
		{Role: RoleTool, ToolCallID: "c1", Name: "tool_wait", Content: "cancelled"},
	}
	got := keepCompleteToolPairs(delta)
	if len(got) != 3 {
		t.Fatalf("expected full cancelled pair kept, got %d", len(got))
	}
}

// Mirrors session-1784128: user + completed tools, then waiting tool cancelled unpaired.
func TestKeepCompleteToolPairs_CancelMidWaitKeepsUserAndCompletedTools(t *testing.T) {
	delta := []Message{
		{Role: RoleUser, Content: "change badge color"},
		{Role: RoleAssistant, ToolCalls: []ToolCall{
			{ID: "t_glob", Name: "tool_a"},
			{ID: "t_grep", Name: "tool_b"},
			{ID: "t_read", Name: "tool_c"},
			{ID: "t_wait", Name: "tool_d"},
		}},
		{Role: RoleTool, ToolCallID: "t_glob", Name: "tool_a", Content: "weather.html"},
		{Role: RoleTool, ToolCallID: "t_grep", Name: "tool_b", Content: ".air-badge"},
		{Role: RoleTool, ToolCallID: "t_read", Name: "tool_c", Content: "css snippet"},
		// tool_d waiting — no result
	}
	got := keepCompleteToolPairs(delta)

	var sawUser, sawGlob, sawGrep, sawRead bool
	for _, m := range got {
		if m.Role == RoleUser && m.Content == "change badge color" {
			sawUser = true
		}
		if m.Role == RoleTool {
			switch m.ToolCallID {
			case "t_glob":
				sawGlob = true
			case "t_grep":
				sawGrep = true
			case "t_read":
				sawRead = true
			case "t_wait":
				t.Fatal("unpaired waiting tool should not be kept")
			}
		}
		if m.Role == RoleAssistant {
			for _, tc := range m.ToolCalls {
				if tc.ID == "t_wait" {
					t.Fatal("unpaired waiting tool_call should not remain on assistant")
				}
			}
		}
	}
	if !sawUser || !sawGlob || !sawGrep || !sawRead {
		t.Fatalf("missing salvaged context: user=%v a=%v b=%v c=%v got=%+v", sawUser, sawGlob, sawGrep, sawRead, got)
	}
}

func TestCloseUnfinishedToolCalls_ClosesOnlyMissing(t *testing.T) {
	var published []string
	stream := &captureStream{onPublish: func(typ string, payload any) {
		if typ == domain.EventToolError {
			if tp, ok := payload.(domain.ToolPart); ok {
				published = append(published, tp.CallID)
			}
		}
	}}
	p := &TurnRunner{Stream: stream}
	tctx := TurnContext{SessionID: "s", TurnID: "t"}
	calls := []ToolCall{
		{ID: "a", Name: "tool_a"},
		{ID: "b", Name: "tool_b"},
		{ID: "c", Name: "tool_c"},
	}
	msgs := []Message{
		{Role: RoleAssistant, ToolCalls: calls},
		{Role: RoleTool, ToolCallID: "a", Name: "tool_a", Content: "ok"},
	}
	out := p.closeUnfinishedToolCalls(tctx, msgs, calls)
	ids := map[string]bool{}
	for _, m := range out {
		if m.Role == RoleTool {
			ids[m.ToolCallID] = true
			if m.ToolCallID != "a" && m.Content != "cancelled" {
				t.Fatalf("expected cancelled content for %s, got %q", m.ToolCallID, m.Content)
			}
		}
	}
	if !ids["a"] || !ids["b"] || !ids["c"] {
		t.Fatalf("expected results for a,b,c got %v", ids)
	}
	if len(published) != 2 || published[0] != "b" || published[1] != "c" {
		t.Fatalf("expected tool.error for b,c only, got %v", published)
	}
}

type captureStream struct {
	onPublish func(typ string, payload any)
}

func (c *captureStream) Publish(ctx context.Context, sessionID, turnID, typ string, payload any) domain.StreamEvent {
	if c.onPublish != nil {
		c.onPublish(typ, payload)
	}
	return domain.StreamEvent{Type: typ, SessionID: sessionID, TurnID: turnID}
}
func (c *captureStream) Subscribe(sessionID string) chan domain.StreamEvent { return nil }
func (c *captureStream) Unsubscribe(sessionID string, ch chan domain.StreamEvent) {
}
func (c *captureStream) ListSince(sessionID string, since int64) []domain.StreamEvent { return nil }

func TestPruneToolResults_OverThreshold(t *testing.T) {
	long := make([]byte, defaultToolTruncateChars+50)
	for i := range long {
		long[i] = 'x'
	}
	msgs := []Message{
		{Role: RoleTool, Name: "any_tool", Content: string(long)},
		{Role: RoleTool, Name: "read_file", Content: "short"},
	}
	out := pruneToolResults(msgs, defaultToolTruncateChars, nil)
	if !strings.Contains(out[0].Content, toolResultPruneMarker) {
		t.Fatalf("expected prune marker on long content, len=%d", len(out[0].Content))
	}
	if len(out[0].Content) > defaultToolTruncateChars {
		t.Fatalf("pruned content must fit threshold, len=%d", len(out[0].Content))
	}
	if out[1].Content != "short" {
		t.Fatalf("short content should be unchanged, got %q", out[1].Content)
	}
}

func TestPruneToolResults_Idempotent(t *testing.T) {
	long := strings.Repeat("x", defaultToolTruncateChars+100)
	msgs := []Message{{Role: RoleTool, Name: "read_file", Content: long}}
	once := pruneToolResults(msgs, defaultToolTruncateChars, nil)
	twice := pruneToolResults(once, defaultToolTruncateChars, nil)
	if once[0].Content != twice[0].Content {
		t.Fatal("second prune pass must be a no-op")
	}
}

func TestLimitToolOutput_HardCap(t *testing.T) {
	short := "hello"
	if got := limitToolOutput(short, 10); got != short {
		t.Fatalf("short content should be unchanged, got %q", got)
	}
	if got := limitToolOutput(short, 0); got != short {
		t.Fatalf("maxChars<=0 should be a no-op, got %q", got)
	}

	long := strings.Repeat("a", 100)
	got := limitToolOutput(long, 20)
	if !strings.HasPrefix(got, strings.Repeat("a", 20)) {
		t.Fatalf("expected 20-char prefix, got %q", got[:min(40, len(got))])
	}
	if !strings.Contains(got, "truncated, 100 total chars") {
		t.Fatalf("expected truncation marker, got %q", got)
	}

	// Multi-byte rune near the cut boundary must stay valid UTF-8.
	utf := "你好世界你好世界" // each rune is 3 bytes
	got = limitToolOutput(utf, 5)
	if !utf8.ValidString(got) {
		t.Fatalf("truncated UTF-8 content must stay valid, got %q", got)
	}
	if !strings.Contains(got, "truncated") {
		t.Fatalf("expected truncation marker for UTF-8 cut, got %q", got)
	}
}
