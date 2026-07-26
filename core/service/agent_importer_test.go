package service

import (
	"testing"

	"danmo-work/core/domain"
)

func TestParseAgentMD(t *testing.T) {
	md := `---
id: meeting-facilitator
name: Meeting Facilitator
description: Demo
persona: Facilitator
mode: subagent
steps: 8
inherit_ambient: false
mcp_servers:
  - github
skills:
  - meeting-notes
tools:
  - tool_id: read_skill
    risk_level: low
  - mcp_server: notion
    risk_level: external
knowledge: []
---

You facilitate meetings.
`
	imp := NewAgentImporter()
	a, err := imp.ParseAgentMD(md)
	if err != nil {
		t.Fatal(err)
	}
	if a.ID != "meeting-facilitator" || a.Mode != domain.AgentModeSubagent {
		t.Fatalf("unexpected agent: %+v", a)
	}
	if len(a.SkillIDs) != 1 || a.SkillIDs[0] != "meeting-notes" {
		t.Fatalf("skills: %+v", a.SkillIDs)
	}
	if a.SystemPrompt == "" {
		t.Fatal("empty system prompt")
	}
	if a.InheritAmbient == nil || *a.InheritAmbient {
		t.Fatalf("inherit_ambient: %+v", a.InheritAmbient)
	}
	if len(a.Tools) != 1 || a.Tools[0].ToolID != "read_skill" {
		t.Fatalf("tools: %+v", a.Tools)
	}
	if len(a.MCPServers) != 2 || a.MCPServers[0] != "github" || a.MCPServers[1] != "notion" {
		t.Fatalf("mcpServers: %+v", a.MCPServers)
	}
}
