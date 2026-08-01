package runtime

import (
	"context"
	"errors"
	"strings"
	"testing"
	"unicode/utf8"

	"danmo-work/core/domain"
)

func TestSalvagePairedTurnDelta_KeepsCompletedDropsUnpaired(t *testing.T) {
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

	got := salvagePairedTurnDelta(delta)
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

func TestSalvagePairedTurnDelta_KeepsCancelledToolResult(t *testing.T) {
	delta := []Message{
		{Role: RoleUser, Content: "ask me"},
		{Role: RoleAssistant, ToolCalls: []ToolCall{{ID: "c1", Name: "tool_wait"}}},
		{Role: RoleTool, ToolCallID: "c1", Name: "tool_wait", Content: "cancelled"},
	}
	got := salvagePairedTurnDelta(delta)
	if len(got) != 3 {
		t.Fatalf("expected full cancelled pair kept, got %d", len(got))
	}
}

func TestCommitTurnMessages_CancelKeepsPriorTurnAndSalvage(t *testing.T) {
	e := &Engine{turnMessages: make(map[string][]Message)}
	sessionID := "s1"
	prev := []Message{
		{Role: RoleUser, Content: "turn1"},
		{Role: RoleAssistant, Content: "done turn1"},
	}
	full := append([]Message{{Role: RoleSystem, Content: "sys"}}, prev...)
	full = append(full,
		Message{Role: RoleUser, Content: "turn2 goal"},
		Message{Role: RoleAssistant, ToolCalls: []ToolCall{
			{ID: "a", Name: "tool_a"},
			{ID: "b", Name: "tool_b"},
		}},
		Message{Role: RoleTool, ToolCallID: "a", Name: "tool_a", Content: "file body"},
	)
	userIdx := 3 // index of turn2 user in full

	e.commitTurnMessages(sessionID, prev, full, userIdx, context.Canceled)

	got := e.turnMessages[sessionID]
	if len(got) < 4 {
		t.Fatalf("expected prev + salvaged turn2, got %d: %+v", len(got), got)
	}
	if got[0].Content != "turn1" || got[1].Content != "done turn1" {
		t.Fatalf("prev turn lost: %+v", got)
	}
	if got[2].Content != "turn2 goal" {
		t.Fatalf("turn2 user lost: %+v", got[2])
	}
	foundA := false
	for _, m := range got {
		if m.Role == RoleTool && m.ToolCallID == "a" {
			foundA = true
		}
		if m.Role == RoleAssistant {
			for _, tc := range m.ToolCalls {
				if tc.ID == "b" {
					t.Fatal("unpaired tool_b should not be committed")
				}
			}
		}
	}
	if !foundA {
		t.Fatal("completed tool_a result was not salvaged")
	}
}

func TestCommitTurnMessages_SuccessStoresDeltaOnly(t *testing.T) {
	e := &Engine{turnMessages: make(map[string][]Message)}
	sessionID := "s1"
	prev := []Message{{Role: RoleUser, Content: "t1"}}
	full := []Message{
		{Role: RoleSystem, Content: "sys"},
		{Role: RoleUser, Content: "t1"},
		{Role: RoleUser, Content: "t2"},
		{Role: RoleAssistant, Content: "ok"},
	}
	e.commitTurnMessages(sessionID, prev, full, 2, nil)
	got := e.turnMessages[sessionID]
	if len(got) != 3 {
		t.Fatalf("expected prev(1)+delta(2)=3, got %d: %+v", len(got), got)
	}
	if got[0].Content != "t1" || got[1].Content != "t2" || got[2].Content != "ok" {
		t.Fatalf("unexpected messages: %+v", got)
	}
	t1Count := 0
	for _, m := range got {
		if m.Role == RoleUser && m.Content == "t1" {
			t1Count++
		}
	}
	if t1Count != 1 {
		t.Fatalf("t1 duplicated: count=%d", t1Count)
	}
}

func TestCommitTurnMessages_CancelErrorUsesSalvage(t *testing.T) {
	e := &Engine{turnMessages: make(map[string][]Message)}
	err := errors.New("boom")
	full := []Message{
		{Role: RoleUser, Content: "u"},
		{Role: RoleAssistant, ToolCalls: []ToolCall{{ID: "x", Name: "tool_x"}}},
	}
	e.commitTurnMessages("s", nil, full, 0, err)
	got := e.turnMessages["s"]
	if len(got) != 1 || got[0].Role != RoleUser {
		t.Fatalf("expected only user after failed unpaired turn, got %+v", got)
	}
}

// Mirrors session-1784128: user + completed tools, then waiting tool cancelled unpaired.
func TestCommitTurnMessages_CancelMidWaitKeepsUserAndCompletedTools(t *testing.T) {
	e := &Engine{turnMessages: make(map[string][]Message)}
	prev := []Message{
		{Role: RoleUser, Content: "write a game"},
		{Role: RoleAssistant, Content: "done"},
	}
	full := []Message{
		{Role: RoleSystem, Content: "sys"},
		{Role: RoleUser, Content: "write a game"},
		{Role: RoleAssistant, Content: "done"},
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
	userIdx := 3
	e.commitTurnMessages("sess", prev, full, userIdx, context.Canceled)
	got := e.turnMessages["sess"]

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
				t.Fatal("unpaired waiting tool should not be committed")
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

func TestTruncateToolResults_BySizeNotName(t *testing.T) {
	long := make([]byte, turnToolTextMaxChars+50)
	for i := range long {
		long[i] = 'x'
	}
	msgs := []Message{
		{Role: RoleTool, Name: "any_tool", Content: string(long)},
		{Role: RoleTool, Name: "read_file", Content: "short"},
	}
	// keepRecentSteps=0 → truncate all
	out := truncateToolResults(msgs, turnToolTextMaxChars, 0)
	if len(out[0].Content) <= turnToolTextMaxChars {
		t.Fatalf("expected truncation marker on long content, len=%d", len(out[0].Content))
	}
	if out[0].Content[:turnToolTextMaxChars] != string(long[:turnToolTextMaxChars]) {
		t.Fatal("truncated prefix mismatch")
	}
	if out[1].Content != "short" {
		t.Fatalf("short content should be unchanged, got %q", out[1].Content)
	}
}

func TestTruncateToolResults_KeepsRecentSteps(t *testing.T) {
	long := strings.Repeat("x", turnToolTextMaxChars+100)
	msgs := []Message{
		{Role: RoleAssistant, ToolCalls: []ToolCall{{ID: "old-1", Name: "delegate_agent"}}},
		{Role: RoleTool, ToolCallID: "old-1", Name: "delegate_agent", Content: long},
		{Role: RoleAssistant, ToolCalls: []ToolCall{{ID: "mid-1", Name: "web_search"}}},
		{Role: RoleTool, ToolCallID: "mid-1", Name: "web_search", Content: long},
		{Role: RoleAssistant, ToolCalls: []ToolCall{
			{ID: "new-1", Name: "delegate_agent"},
			{ID: "new-2", Name: "delegate_agent"},
		}},
		{Role: RoleTool, ToolCallID: "new-1", Name: "delegate_agent", Content: long},
		{Role: RoleTool, ToolCallID: "new-2", Name: "delegate_agent", Content: long},
	}
	// Keep last 2 steps (mid + new); truncate old only.
	out := truncateToolResults(msgs, turnToolTextMaxChars, 2)
	if !strings.Contains(out[1].Content, "[truncated") {
		t.Fatalf("old step should be truncated, len=%d", len(out[1].Content))
	}
	if out[3].Content != long {
		t.Fatalf("mid step should stay full, len=%d", len(out[3].Content))
	}
	if out[5].Content != long || out[6].Content != long {
		t.Fatalf("newest step tool results should stay full")
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
