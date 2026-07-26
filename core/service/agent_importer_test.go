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
inherit_ambient: true
skills:
  - meeting-notes
tools:
  - tool_id: read_skill
    risk_level: low
  - mcp_server: github
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
	if a.InheritAmbient == nil || !*a.InheritAmbient {
		t.Fatalf("inherit_ambient: %+v", a.InheritAmbient)
	}
	if len(a.Tools) != 2 || a.Tools[1].MCPServer != "github" {
		t.Fatalf("tools: %+v", a.Tools)
	}
}
