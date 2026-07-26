package domain

import "testing"

func TestInheritsAmbientDefaults(t *testing.T) {
	primary := Agent{Mode: AgentModePrimary}
	if !primary.InheritsAmbient() {
		t.Fatal("primary should inherit ambient by default")
	}
	sub := Agent{Mode: AgentModeSubagent}
	if sub.InheritsAmbient() {
		t.Fatal("subagent should not inherit ambient by default")
	}
}

func TestInheritsAmbientOverride(t *testing.T) {
	off := false
	primary := Agent{Mode: AgentModePrimary, InheritAmbient: &off}
	if primary.InheritsAmbient() {
		t.Fatal("explicit false should win for primary")
	}
	on := true
	sub := Agent{Mode: AgentModeSubagent, InheritAmbient: &on}
	if !sub.InheritsAmbient() {
		t.Fatal("explicit true should win for subagent")
	}
}
