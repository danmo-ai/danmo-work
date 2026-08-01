package builtin

import (
	"testing"

	"danmo-work/core/domain"
)

func TestTurnPathFromInput_UsesInjectedPath(t *testing.T) {
	path := []domain.TurnPathEntry{
		{TurnID: "turn-0", AgentID: "team"},
		{TurnID: "turn-1", AgentID: "researcher"},
	}
	got := turnPathFromInput(map[string]any{
		"__turn_id":   "turn-1",
		"__agent_id":  "researcher",
		"__turn_path": path,
	}, "turn-1")
	if len(got) != 2 || got[0].AgentID != "team" || got[1].TurnID != "turn-1" {
		t.Fatalf("got %+v", got)
	}
}

func TestTurnPathFromInput_FallbackSynthesizesFrame(t *testing.T) {
	got := turnPathFromInput(map[string]any{
		"__turn_id":  "turn-0",
		"__agent_id": "team",
	}, "turn-0")
	if len(got) != 1 || got[0].TurnID != "turn-0" || got[0].AgentID != "team" {
		t.Fatalf("got %+v", got)
	}
}
