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
