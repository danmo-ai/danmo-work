package runtime

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"danmo-work/core/adapter/llm"
	"danmo-work/core/domain"
	"danmo-work/core/port"
	"danmo-work/core/runtime/permission"
	"danmo-work/core/runtime/tool"
)

type mockToolHandler struct {
	name  string
	risk  domain.RiskLevel
	calls int
}

func (h *mockToolHandler) Name() string              { return h.name }
func (h *mockToolHandler) RiskLevel() domain.RiskLevel { return h.risk }
func (h *mockToolHandler) Describe(args map[string]any) string { return h.name }
func (h *mockToolHandler) Schema() domain.ToolSchema {
	return domain.ToolSchema{
		Name: h.name, Description: "mock tool",
		Parameters: map[string]any{
			"type": "object", "properties": map[string]any{},
		},
	}
}
func (h *mockToolHandler) Execute(_ context.Context, _ map[string]any) (domain.ToolResult, error) {
	h.calls++
	return domain.ToolResult{Content: "ok"}, nil
}

func TestTrackDoomConsecutiveNotCumulative(t *testing.T) {
	tr := NewTurnRunner(nil, nil, permission.NewGate(nil), tool.NewRegistry(), nil)
	const turnID = "t1"
	const threshold = 3

	// Interleaved todowrite/write should not trip consecutive OR short alternating.
	for i := 0; i < 3; i++ {
		n := tr.trackDoom(turnID, "todowrite", "todowrite", threshold)
		if n >= threshold {
			t.Fatalf("interleaved: unexpected doom after todowrite #%d streak=%d", i+1, n)
		}
		n = tr.trackDoom(turnID, "write", "write", threshold)
		if n >= threshold {
			t.Fatalf("interleaved: unexpected doom on write streak=%d", n)
		}
	}

	// Three consecutive identical calls should trip.
	tr2 := NewTurnRunner(nil, nil, permission.NewGate(nil), tool.NewRegistry(), nil)
	var last int
	for i := 0; i < 3; i++ {
		last = tr2.trackDoom("t2", "todowrite", "todowrite", threshold)
	}
	if last < threshold {
		t.Fatalf("expected consecutive doom streak>=%d got %d", threshold, last)
	}
}

func TestDetectAlternatingLoop(t *testing.T) {
	// Need >= 8 alternating (4 A-B pairs) with threshold 3
	pat := []string{"a", "b", "a", "b", "a", "b", "a", "b"}
	if !detectAlternatingLoop(pat, 3) {
		t.Fatal("expected alternating doom")
	}
	if detectAlternatingLoop([]string{"a", "b", "a", "b", "a", "b"}, 3) {
		t.Fatal("6-long A-B should not trip (min 8)")
	}
	if detectAlternatingLoop([]string{"a", "a", "a"}, 3) {
		t.Fatal("identical streak is not alternating")
	}
}

func TestTurnRunnerDoomLoopMessagesIntegrity(t *testing.T) {
	mockLLM := llm.NewMock()
	for i := 0; i < 5; i++ {
		mockLLM.AddToolCall("todowrite", map[string]any{"todos": []any{}})
	}

	stream := NewStreamEventManager(nil)
	perm := permission.NewGate(nil)
	reg := tool.NewRegistry()
	todowriteTool := &mockToolHandler{name: "todowrite", risk: domain.RiskLow}
	reg.Register(todowriteTool)

	configStore := &testConfigStore{
		cfg: &domain.ConfigFile{
			Runtime: domain.ConfigRuntimeSection{
				Turn: domain.ConfigTurnSection{
					DoomLoopThreshold: 5,
					MaxStepsDefault:   20,
				},
			},
		},
	}

	tr := NewTurnRunner(mockLLM, stream, perm, reg, configStore)
	ctx := context.Background()

	tctx := TurnContext{
		SessionID: "test-session",
		TurnID:    "turn-doom-1",
		Agent:     domain.Agent{ID: "test-agent", Steps: 20},
		Model:     "test-model",
		MaxSteps:  20,
		WorkDir:   "/tmp",
		Messages: []Message{
			{Role: RoleSystem, Content: "You are a test assistant"},
			{Role: RoleUser, Content: "do something"},
		},
	}

	rep, msgs, err := tr.Run(ctx, tctx)
	if err != nil {
		t.Fatalf("turn 1 unexpected error: %v", err)
	}
	if rep.Status != domain.ReportFailed {
		t.Errorf("turn 1: expected ReportFailed, got %v", rep.Status)
	}
	if rep.Summary != "doom loop for todowrite" {
		t.Errorf("turn 1: expected 'doom loop for todowrite', got %q", rep.Summary)
	}
	if todowriteTool.calls < 4 {
		t.Errorf("turn 1: expected at least 4 todowrite calls before doom loop, got %d", todowriteTool.calls)
	}

	validateToolMessagePairs(t, msgs, "turn 1")

	mockLLM2 := llm.NewMock().AddText("all done")
	stream2 := NewStreamEventManager(nil)
	reg2 := tool.NewRegistry()
	reg2.Register(&mockToolHandler{name: "todowrite", risk: domain.RiskLow})
	tr2 := NewTurnRunner(mockLLM2, stream2, perm, reg2, configStore)

	tctx2 := TurnContext{
		SessionID: "test-session",
		TurnID:    "turn-doom-2",
		Agent:     domain.Agent{ID: "test-agent", Steps: 20},
		Model:     "test-model",
		MaxSteps:  20,
		WorkDir:   "/tmp",
		Messages: append(append([]Message(nil), msgs...), Message{Role: RoleUser, Content: "continue"}),
	}

	rep2, msgs2, err2 := tr2.Run(ctx, tctx2)
	if err2 != nil {
		t.Fatalf("turn 2 unexpected error: %v", err2)
	}
	if rep2.Status != domain.ReportDone {
		t.Errorf("turn 2: expected ReportDone, got %v: %s", rep2.Status, rep2.Summary)
	}

	validateToolMessagePairs(t, msgs2, "turn 2")
}

func TestTurnRunnerApprovalRejectContinues(t *testing.T) {
	mockLLM := llm.NewMock().
		AddToolCall("exec_shell", map[string]any{"command": "ls"}).
		AddText("understood, will use a safer approach")

	stream := NewStreamEventManager(nil)
	perm := permission.NewGate(nil)
	reg := tool.NewRegistry()
	reg.Register(&mockToolHandler{name: "exec_shell", risk: domain.RiskHigh})

	configStore := &testConfigStore{
		cfg: &domain.ConfigFile{
			Runtime: domain.ConfigRuntimeSection{
				Turn: domain.ConfigTurnSection{
					DoomLoopThreshold: 5,
					MaxStepsDefault:   20,
				},
			},
		},
	}

	tr := NewTurnRunner(mockLLM, stream, perm, reg, configStore)

	approved := make(chan ApprovalOutcome, 1)
	approved <- ApprovalOutcome{Approved: false, Scope: "once"}
	tr.Approval = &mockApprovalGate{result: approved}

	ctx := context.Background()
	tctx := TurnContext{
		SessionID: "test-session",
		TurnID:    "turn-approval-1",
		Agent:     domain.Agent{ID: "test-agent", Steps: 20},
		Model:     "test-model",
		MaxSteps:  20,
		WorkDir:   "/tmp",
		Messages: []Message{
			{Role: RoleSystem, Content: "You are a test assistant"},
			{Role: RoleUser, Content: "run ls"},
		},
	}

	rep, msgs, err := tr.Run(ctx, tctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rep.Status != domain.ReportDone {
		t.Fatalf("expected ReportDone after soft reject, got %v: %s", rep.Status, rep.Summary)
	}
	foundReject := false
	for _, m := range msgs {
		if m.Role == RoleTool && strings.Contains(m.Content, "rejected") {
			foundReject = true
		}
	}
	if !foundReject {
		t.Fatal("expected tool message containing rejection")
	}
	validateToolMessagePairs(t, msgs, "approval soft reject")
}

func TestTurnRunnerPublishesThinkingNotInHistory(t *testing.T) {
	const thinking = "I should reason carefully about the answer"
	const answer = "final answer text"

	mockLLM := llm.NewMock().
		AddToolCallWithReasoning("todowrite", map[string]any{"todos": []any{}}, "planning tool use").
		AddTextWithReasoning(answer, thinking)

	var published []struct {
		typ     string
		payload any
	}
	stream := &captureStream{onPublish: func(typ string, payload any) {
		published = append(published, struct {
			typ     string
			payload any
		}{typ, payload})
	}}
	perm := permission.NewGate(nil)
	reg := tool.NewRegistry()
	reg.Register(&mockToolHandler{name: "todowrite", risk: domain.RiskLow})

	configStore := &testConfigStore{
		cfg: &domain.ConfigFile{
			Runtime: domain.ConfigRuntimeSection{
				Turn: domain.ConfigTurnSection{
					DoomLoopThreshold: 5,
					MaxStepsDefault:   20,
				},
			},
		},
	}

	var logTypes []string
	tr := NewTurnRunner(mockLLM, stream, perm, reg, configStore)
	tr.Log = func(typ string, _ map[string]any) {
		logTypes = append(logTypes, typ)
	}

	ctx := context.Background()
	tctx := TurnContext{
		SessionID: "test-session",
		TurnID:    "turn-thinking-1",
		Agent:     domain.Agent{ID: "test-agent", Steps: 20},
		Model:     "test-model",
		MaxSteps:  20,
		WorkDir:   "/tmp",
		Messages: []Message{
			{Role: RoleSystem, Content: "You are a test assistant"},
			{Role: RoleUser, Content: "think then answer"},
		},
	}

	rep, msgs, err := tr.Run(ctx, tctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rep.Status != domain.ReportDone {
		t.Fatalf("expected ReportDone, got %v: %s", rep.Status, rep.Summary)
	}

	var thinkingTexts []string
	var messageTexts []string
	for _, ev := range published {
		switch ev.typ {
		case domain.EventAgentThinking:
			p, ok := ev.payload.(domain.AgentThinkingPayload)
			if !ok {
				t.Fatalf("agent.thinking payload type %T", ev.payload)
			}
			thinkingTexts = append(thinkingTexts, p.Text)
		case domain.EventAgentMessage:
			p, ok := ev.payload.(domain.AgentMessagePayload)
			if !ok {
				t.Fatalf("agent.message payload type %T", ev.payload)
			}
			messageTexts = append(messageTexts, p.Text)
		}
	}
	if len(thinkingTexts) != 2 {
		t.Fatalf("expected 2 agent.thinking events, got %d: %v", len(thinkingTexts), thinkingTexts)
	}
	if thinkingTexts[0] != "planning tool use" {
		t.Errorf("first thinking event = %q", thinkingTexts[0])
	}
	if thinkingTexts[1] != thinking {
		t.Errorf("second thinking event = %q", thinkingTexts[1])
	}
	if len(messageTexts) != 1 || messageTexts[0] != answer {
		t.Fatalf("expected one agent.message with answer, got %v", messageTexts)
	}

	for _, m := range msgs {
		if strings.Contains(m.Content, thinking) || strings.Contains(m.Content, "planning tool use") {
			t.Errorf("history message unexpectedly contains thinking content: role=%s content=%q", m.Role, m.Content)
		}
	}

	for _, typ := range logTypes {
		switch typ {
		case "assistant", "tool_result", "user":
			// expected LLM-reconstructable types
		default:
			t.Errorf("turn log got unexpected type %q", typ)
		}
	}

	for _, req := range mockLLM.Requests {
		for _, m := range req.Messages {
			if strings.Contains(m.Content, thinking) || strings.Contains(m.Content, "planning tool use") {
				t.Errorf("LLM request unexpectedly includes thinking content: role=%s content=%q", m.Role, m.Content)
			}
		}
	}

	validateToolMessagePairs(t, msgs, "thinking")
}

type mockApprovalGate struct {
	result chan ApprovalOutcome
}

func (g *mockApprovalGate) WaitApproval(_ context.Context, _ string) (ApprovalOutcome, error) {
	return <-g.result, nil
}

func (g *mockApprovalGate) CreateApproval(_, _, _, _, _ string) string {
	return "approval-1"
}

func validateToolMessagePairs(t *testing.T, msgs []Message, label string) {
	t.Helper()

	toolByID := make(map[string]bool)
	assistantIDs := make(map[string]int)

	for _, m := range msgs {
		if m.Role == RoleTool && m.ToolCallID != "" {
			toolByID[m.ToolCallID] = true
		}
		if m.Role == RoleAssistant && len(m.ToolCalls) > 0 {
			for _, tc := range m.ToolCalls {
				assistantIDs[tc.ID]++
			}
		}
	}

	for id := range assistantIDs {
		if !toolByID[id] {
			t.Errorf("%s: assistant tool_calls ID %q has no corresponding tool message", label, id)
		}
	}

	for id := range toolByID {
		if _, ok := assistantIDs[id]; !ok {
			t.Errorf("%s: orphan tool message for call ID %q (no matching assistant tool_calls)", label, id)
		}
	}
}

func TestEnforceToolPairingDropsOrphanAssistantToolCalls(t *testing.T) {
	tr := NewTurnRunner(nil, nil, nil, nil, nil)
	msgs := []Message{
		{Role: RoleSystem, Content: "sys"},
		{Role: RoleUser, Content: "hi"},
		{Role: RoleAssistant, ToolCalls: []ToolCall{{ID: "c1", Name: "read_file"}}},
		{Role: RoleTool, ToolCallID: "c1", Name: "read_file", Content: "ok"},
		// Orphan: assistant with tool_calls but no matching tool result
		{Role: RoleAssistant, ToolCalls: []ToolCall{{ID: "c2", Name: "write_file"}}},
		{Role: RoleUser, Content: "next"},
	}
	out := tr.enforceToolPairing(msgs)

	// The orphan assistant(tool_calls) for c2 must be dropped.
	for _, m := range out {
		if m.Role == RoleAssistant {
			for _, tc := range m.ToolCalls {
				if tc.ID == "c2" {
					t.Fatal("orphan assistant tool_call c2 should have been dropped")
				}
			}
		}
	}
	// The complete pair (c1) must survive.
	foundC1 := false
	for _, m := range out {
		if m.Role == RoleTool && m.ToolCallID == "c1" {
			foundC1 = true
		}
	}
	if !foundC1 {
		t.Fatal("complete pair c1 should have been kept")
	}
	validateToolMessagePairs(t, out, "enforceToolPairing orphan assistant")
}

func TestEnforceToolPairingKeepsPartialAssistantWithSomeResults(t *testing.T) {
	tr := NewTurnRunner(nil, nil, nil, nil, nil)
	// Assistant with 2 tool_calls, only 1 has a result — should be kept
	// (the missing one will be handled by closeUnfinishedToolCalls elsewhere).
	msgs := []Message{
		{Role: RoleSystem, Content: "sys"},
		{Role: RoleUser, Content: "hi"},
		{Role: RoleAssistant, ToolCalls: []ToolCall{
			{ID: "c1", Name: "read_file"},
			{ID: "c2", Name: "write_file"},
		}},
		{Role: RoleTool, ToolCallID: "c1", Name: "read_file", Content: "ok"},
	}
	out := tr.enforceToolPairing(msgs)

	// Assistant should be kept because c1 has a result.
	foundAssistant := false
	for _, m := range out {
		if m.Role == RoleAssistant && len(m.ToolCalls) > 0 {
			foundAssistant = true
		}
	}
	if !foundAssistant {
		t.Fatal("assistant with at least one matching result should be kept")
	}
}

func TestSnipHeadPreservesLastUserMessage(t *testing.T) {
	tr := NewTurnRunner(nil, nil, nil, nil, nil)
	// Very small budget — should remove history but NOT the last user message.
	msgs := []Message{
		{Role: RoleSystem, Content: "sys"},
		{Role: RoleUser, Content: "old history message 1"},
		{Role: RoleAssistant, Content: "old response 1"},
		{Role: RoleUser, Content: "old history message 2"},
		{Role: RoleAssistant, Content: "old response 2"},
		{Role: RoleUser, Content: "current goal — must survive"},
	}
	out := tr.snipHead(msgs, 10) // budget=10 tokens, very small

	foundLastUser := false
	for _, m := range out {
		if m.Role == RoleUser && m.Content == "current goal — must survive" {
			foundLastUser = true
		}
	}
	if !foundLastUser {
		t.Fatal("snipHead must not remove the last user message (current turn goal)")
	}
}

type failingLLM struct {
	calls int
}

func (f *failingLLM) Chat(_ context.Context, _ port.LLMChatRequest) (port.LLMChatResponse, error) {
	f.calls++
	return port.LLMChatResponse{}, fmt.Errorf("bad request (400): unknown variant image_url")
}

func TestTurnRunnerStopsAfterMaxLLMFailures(t *testing.T) {
	failing := &failingLLM{}
	stream := NewStreamEventManager(nil)
	perm := permission.NewGate(nil)
	reg := tool.NewRegistry()
	configStore := &testConfigStore{
		cfg: &domain.ConfigFile{
			Runtime: domain.ConfigRuntimeSection{
				Turn: domain.ConfigTurnSection{
					DoomLoopThreshold: 10,
					MaxStepsDefault:   50,
					MaxLLMFailures:    3,
				},
			},
		},
	}
	tr := NewTurnRunner(failing, stream, perm, reg, configStore)
	report, _, err := tr.Run(context.Background(), TurnContext{
		SessionID: "s1",
		TurnID:    "t1",
		Agent:     domain.Agent{ID: "a", Steps: 50},
		Model:     "mock/x",
		MaxSteps:  50,
		Messages:  []Message{{Role: RoleUser, Content: "hi"}},
	})
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if failing.calls != 3 {
		t.Fatalf("expected 3 LLM calls, got %d", failing.calls)
	}
	if report.Status != domain.ReportFailed {
		t.Fatalf("expected failed report, got %s", report.Status)
	}
	if !strings.Contains(report.Summary, "3 times") {
		t.Fatalf("expected failure summary mentioning 3 times, got %q", report.Summary)
	}
}
