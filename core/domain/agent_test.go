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

func TestNormalizeAgentBindingsSplitsMCP(t *testing.T) {
	a := Agent{
		MCPServers: []string{"notion", "*", "notion", ""},
		Tools: []ToolBinding{
			{ToolID: "read_file", RiskLevel: RiskLow},
			{MCPServer: "github", RiskLevel: RiskExternal},
			{ToolID: "", MCPServer: "*"},
		},
	}
	NormalizeAgentBindings(&a)
	if len(a.MCPServers) != 2 || a.MCPServers[0] != "notion" || a.MCPServers[1] != "github" {
		t.Fatalf("mcpServers: %+v", a.MCPServers)
	}
	if len(a.Tools) != 1 || a.Tools[0].ToolID != "read_file" || a.Tools[0].MCPServer != "" {
		t.Fatalf("tools: %+v", a.Tools)
	}
}
