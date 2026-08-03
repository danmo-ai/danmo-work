package prompt

import (
	"testing"

	"danmo-work/core/domain"
)

func TestBuiltinAgentsDoNotBindCoreTools(t *testing.T) {
	templates, err := LoadTemplates()
	if err != nil {
		t.Fatalf("LoadTemplates: %v", err)
	}
	if len(templates) == 0 {
		t.Fatal("expected embedded agent templates")
	}
	for _, tmpl := range templates {
		for _, b := range tmpl.Agent.Tools {
			if domain.IsCoreTool(b.ToolID) {
				t.Errorf("agent %q binds Core tool %q (must not be in tools[])", tmpl.Agent.ID, b.ToolID)
			}
		}
	}
}

func TestBuiltinGitHubExpertIsSkillPlusGh(t *testing.T) {
	templates, err := LoadTemplates()
	if err != nil {
		t.Fatalf("LoadTemplates: %v", err)
	}
	var github *AgentTemplate
	for i := range templates {
		if templates[i].Agent.ID == "github" {
			github = &templates[i]
			break
		}
	}
	if github == nil {
		t.Fatal("missing builtin github agent template")
	}
	if len(github.Agent.MCPServers) != 0 {
		t.Fatalf("github expert must not bind MCP, got %v", github.Agent.MCPServers)
	}
	hasSkill, hasShell := false, false
	for _, id := range github.Agent.SkillIDs {
		if id == "github" {
			hasSkill = true
		}
	}
	for _, b := range github.Agent.Tools {
		if b.ToolID == "exec_shell" {
			hasShell = true
		}
	}
	if !hasSkill || !hasShell {
		t.Fatalf("github expert needs skill=github + exec_shell; skills=%v tools=%v",
			github.Agent.SkillIDs, github.Agent.Tools)
	}
	if github.Agent.InheritAmbient == nil || *github.Agent.InheritAmbient {
		t.Fatal("github expert should set inherit_ambient: false")
	}
}
