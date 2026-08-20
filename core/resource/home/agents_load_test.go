package home

import (
	"fmt"
	"testing"

	"danmo-work/core/domain"
)

func TestBuiltinAgentsDoNotBindCoreTools(t *testing.T) {
	templates, err := LoadAgentTemplates()
	if err != nil {
		t.Fatalf("LoadAgentTemplates: %v", err)
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

func TestBuiltinAgentsEditImpliesApplyPatch(t *testing.T) {
	templates, err := LoadAgentTemplates()
	if err != nil {
		t.Fatalf("LoadAgentTemplates: %v", err)
	}
	for _, tmpl := range templates {
		hasEdit, hasPatch := false, false
		for _, b := range tmpl.Agent.Tools {
			switch b.ToolID {
			case "edit":
				hasEdit = true
			case "apply_patch":
				hasPatch = true
			}
		}
		if hasEdit && !hasPatch {
			t.Errorf("agent %q binds edit without apply_patch", tmpl.Agent.ID)
		}
	}
}

func TestKnowledgeDirsEmptyAfterPluginMigration(t *testing.T) {
	if dirs := KnowledgeDirs(); len(dirs) != 0 {
		t.Fatalf("expected no home-embedded knowledge dirs after novel plugin migration, got %v", dirs)
	}
	if NovelCraftKnowledgeBaseID != "kb-novel-craft" {
		t.Fatalf("NovelCraftKnowledgeBaseID=%q", NovelCraftKnowledgeBaseID)
	}
}

func TestBuiltinOperatorExpertOwnsComputerTool(t *testing.T) {
	templates, err := LoadAgentTemplates()
	if err != nil {
		t.Fatalf("LoadAgentTemplates: %v", err)
	}
	var op *AgentTemplate
	for i := range templates {
		if templates[i].Agent.ID == "operator" {
			op = &templates[i]
			break
		}
	}
	if op == nil {
		t.Fatal("missing builtin operator agent template")
	}
	if op.Agent.Mode != domain.AgentModeSubagent {
		t.Fatalf("operator mode=%s, want subagent", op.Agent.Mode)
	}
	if op.Agent.InheritAmbient == nil || *op.Agent.InheritAmbient {
		t.Fatalf("operator should set inherit_ambient=false, got %v", op.Agent.InheritAmbient)
	}
	hasSkill := false
	for _, id := range op.Agent.SkillIDs {
		if id == "computer-use" {
			hasSkill = true
		}
	}
	if !hasSkill {
		t.Fatalf("operator needs skill computer-use; skills=%v", op.Agent.SkillIDs)
	}
	hasComputer := false
	for _, b := range op.Agent.Tools {
		if b.ToolID == "computer" {
			hasComputer = true
		}
	}
	if !hasComputer {
		t.Fatalf("operator must bind computer tool; tools=%v", op.Agent.Tools)
	}
	if _, err := loadSkillByID("computer-use"); err != nil {
		t.Fatalf("computer-use skill must load: %v", err)
	}
}

func loadSkillByID(id string) (*domain.Skill, error) {
	templates, err := LoadSkillTemplates()
	if err != nil {
		return nil, err
	}
	for _, t := range templates {
		if t.Skill.ID == id {
			s := t.Skill
			return &s, nil
		}
	}
	return nil, fmt.Errorf("skill %q not found", id)
}
