package runtime

import (
	"context"
	"testing"

	"danmo-work/core/domain"
	"danmo-work/core/adapter/llm"
	"danmo-work/core/runtime/permission"
	"danmo-work/core/runtime/tool"
)

func TestSoftSteerInjectedBetweenToolAndNextModelCall(t *testing.T) {
	mockLLM := llm.NewMock().
		AddToolCall("noop_tool", map[string]any{}).
		AddText("steered done")

	reg := tool.NewRegistry()
	reg.Register(&mockToolHandler{name: "noop_tool", risk: domain.RiskLow})
	runner := NewTurnRunner(mockLLM, NewStreamEventManager(nil), permission.NewGate(nil), reg, nil)

	var pending []Message
	pending = []Message{{Role: RoleUser, Content: "please change direction"}}
	drains := 0

	rep, msgs, err := runner.Run(context.Background(), TurnContext{
		SessionID: "s1",
		TurnID:    "t1",
		Agent:     domain.Agent{ID: "a", Steps: 10},
		Model:     "m",
		MaxSteps:  10,
		Messages:  []Message{{Role: RoleUser, Content: "start"}},
		DrainSteers: func() []Message {
			drains++
			// After tools complete, second step start drains the steer.
			if drains >= 2 && len(pending) > 0 {
				out := pending
				pending = nil
				return out
			}
			return nil
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if rep.Status != domain.ReportDone {
		t.Fatalf("status=%s summary=%q", rep.Status, rep.Summary)
	}
	found := false
	for _, m := range msgs {
		if m.Role == RoleUser && m.Content == "please change direction" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("soft steer user message missing from transcript: %+v", msgs)
	}
}

func TestSoftSteerKeepsTurnAliveWhenModelStops(t *testing.T) {
	mockLLM := llm.NewMock().
		AddText("first final").
		AddText("after steer")

	runner := NewTurnRunner(mockLLM, NewStreamEventManager(nil), permission.NewGate(nil), tool.NewRegistry(), nil)

	pending := []Message{{Role: RoleUser, Content: "one more thing"}}
	calls := 0
	rep, msgs, err := runner.Run(context.Background(), TurnContext{
		SessionID: "s1",
		TurnID:    "t1",
		Agent:     domain.Agent{ID: "a", Steps: 10},
		Model:     "m",
		MaxSteps:  10,
		Messages:  []Message{{Role: RoleUser, Content: "start"}},
		DrainSteers: func() []Message {
			// After first final answer, inject steer so the turn continues.
			calls++
			if calls == 2 && len(pending) > 0 {
				out := pending
				pending = nil
				return out
			}
			return nil
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if rep.Status != domain.ReportDone {
		t.Fatalf("status=%s summary=%q", rep.Status, rep.Summary)
	}
	found := false
	for _, m := range msgs {
		if m.Role == RoleUser && m.Content == "one more thing" {
			found = true
		}
	}
	if !found {
		t.Fatal("expected steer message in transcript")
	}
	if rep.Summary != "after steer" {
		t.Fatalf("expected final summary after continued turn, got %q", rep.Summary)
	}
}
