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

func TestHomeEmbedHasNoToolBoundCapabilityExperts(t *testing.T) {
	templates, err := LoadAgentTemplates()
	if err != nil {
		t.Fatalf("LoadAgentTemplates: %v", err)
	}
	for _, tmpl := range templates {
		switch tmpl.Agent.ID {
		case "browser", "operator", "github", "danmo-make", "novel":
			t.Errorf("agent %q should live in builtin plugins, not home embed", tmpl.Agent.ID)
		}
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
