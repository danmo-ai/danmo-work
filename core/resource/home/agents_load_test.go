package home

import (
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

func TestHomeEmbedOnlyKeepsPrimaryTeam(t *testing.T) {
	templates, err := LoadAgentTemplates()
	if err != nil {
		t.Fatalf("LoadAgentTemplates: %v", err)
	}
	if len(templates) != 1 || templates[0].Agent.ID != "team" {
		ids := make([]string, 0, len(templates))
		for _, t := range templates {
			ids = append(ids, t.Agent.ID)
		}
		t.Fatalf("home agents=%v want only [team]", ids)
	}
	if templates[0].Agent.Mode != domain.AgentModePrimary {
		t.Fatal("team must be primary")
	}
	if !templates[0].Agent.CanDelegate {
		t.Fatal("team must can_delegate")
	}
}

func TestKnowledgeDirsEmptyAfterPluginMigration(t *testing.T) {
	if dirs := KnowledgeDirs(); len(dirs) != 0 {
		t.Fatalf("expected no home-embedded knowledge dirs, got %v", dirs)
	}
}

func TestLoadSkillTemplatesIncludesAdaptedPack(t *testing.T) {
	templates, err := LoadSkillTemplates()
	if err != nil {
		t.Fatalf("LoadSkillTemplates: %v", err)
	}

	want := map[string]string{
		"debugging":               "coding",
		"git-workflow":            "coding",
		"test-driven-development": "coding",
		"writing-plans":           "coding",
		"requesting-code-review":  "coding",
		"brainstorming":           "work",
		"deep-research":           "work",
		"document-writing":        "work",
		"playable-slides":         "work",
		"sheet-writing":           "work",
		"skill-creator":           "general",
	}

	got := make(map[string]string, len(templates))
	for _, tmpl := range templates {
		cat := ""
		if tmpl.Skill.Metadata != nil {
			cat = tmpl.Skill.Metadata["category"]
		}
		got[tmpl.Skill.ID] = cat
		if tmpl.Skill.Body == "" {
			t.Errorf("skill %q has empty body", tmpl.Skill.ID)
		}
	}

	for id, cat := range want {
		gotCat, ok := got[id]
		if !ok {
			t.Errorf("missing shared home skill %q", id)
			continue
		}
		if gotCat != cat {
			t.Errorf("skill %q category = %q, want %q", id, gotCat, cat)
		}
	}

	for _, migrated := range []string{"github", "danmo-make", "novel-writing", "browser", "computer-use"} {
		if _, ok := got[migrated]; ok {
			t.Errorf("skill %q should live in its capability plugin, not home", migrated)
		}
	}
}
