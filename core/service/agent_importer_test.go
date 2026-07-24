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
skills:
  - meeting-notes
tools:
  - tool_id: read_skill
    risk_level: low
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
}
