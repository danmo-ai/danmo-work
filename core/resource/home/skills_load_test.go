package home

import (
	"testing"
)

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
			t.Errorf("missing builtin skill %q", id)
			continue
		}
		if gotCat != cat {
			t.Errorf("skill %q category = %q, want %q", id, gotCat, cat)
		}
	}

	for _, migrated := range []string{"github", "danmo-make", "novel-writing", "browser", "computer-use"} {
		if _, ok := got[migrated]; ok {
			t.Errorf("skill %q should live in builtin plugins, not home embed", migrated)
		}
	}
}
