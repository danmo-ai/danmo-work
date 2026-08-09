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

func TestBuiltinAgentsEditImpliesApplyPatch(t *testing.T) {
	templates, err := LoadTemplates()
	if err != nil {
		t.Fatalf("LoadTemplates: %v", err)
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

func TestBuiltinNovelExpertAggregatesSkillAndCraftKB(t *testing.T) {
	templates, err := LoadTemplates()
	if err != nil {
		t.Fatalf("LoadTemplates: %v", err)
	}
	var novel *AgentTemplate
	for i := range templates {
		if templates[i].Agent.ID == "novel" {
			novel = &templates[i]
			break
		}
	}
	if novel == nil {
		t.Fatal("missing builtin novel agent template")
	}
	if novel.Agent.Mode != domain.AgentModeSubagent {
		t.Fatalf("novel mode=%s, want subagent", novel.Agent.Mode)
	}
	hasNovelSkill, hasBrainstorm := false, false
	for _, id := range novel.Agent.SkillIDs {
		switch id {
		case "novel-writing":
			hasNovelSkill = true
		case "brainstorming":
			hasBrainstorm = true
		}
	}
	if !hasNovelSkill || !hasBrainstorm {
		t.Fatalf("novel skills=%v, want novel-writing + brainstorming", novel.Agent.SkillIDs)
	}
	if len(novel.Agent.KnowledgeIDs) != 1 || novel.Agent.KnowledgeIDs[0] != NovelCraftKnowledgeBaseID {
		t.Fatalf("novel knowledge=%v, want [%s]", novel.Agent.KnowledgeIDs, NovelCraftKnowledgeBaseID)
	}
	need := map[string]bool{
		"read_file": false, "write": false, "edit": false, "apply_patch": false,
		"web_search": false, "todowrite": false,
	}
	for _, b := range novel.Agent.Tools {
		if _, ok := need[b.ToolID]; ok {
			need[b.ToolID] = true
		}
		if b.ToolID == "exec_shell" {
			t.Fatal("novel expert must not bind exec_shell")
		}
	}
	for id, ok := range need {
		if !ok {
			t.Fatalf("novel expert missing tool %q; tools=%v", id, novel.Agent.Tools)
		}
	}
	if novel.Agent.CanDelegate {
		t.Fatal("novel expert should not can_delegate in MVP")
	}
}

func TestBuiltinNovelWritingSkillHasReferences(t *testing.T) {
	sk, err := LoadSkillTemplateByID("novel-writing")
	if err != nil {
		t.Fatalf("LoadSkillTemplateByID: %v", err)
	}
	if sk.ID != "novel-writing" || sk.Description == "" {
		t.Fatalf("unexpected skill: %+v", sk)
	}
	files, err := LoadBuiltinSkillFiles("novel-writing")
	if err != nil {
		t.Fatalf("LoadBuiltinSkillFiles: %v", err)
	}
	want := map[string]bool{
		"references/routes.md":                 false,
		"references/init.md":                   false,
		"references/chapter-contract.md":       false,
		"references/chapter-write.md":          false,
		"references/review-gates.md":           false,
		"references/continuity-commit.md":      false,
		"references/table-schema.md":           false,
		"assets/templates/novel-state.yaml":    false,
		"assets/templates/chapter-contract.yaml": false,
	}
	for _, f := range files {
		if _, ok := want[f.Path]; ok {
			want[f.Path] = true
		}
	}
	for path, ok := range want {
		if !ok {
			t.Fatalf("missing skill file %q (got %d files)", path, len(files))
		}
	}
}

func TestLoadNovelCraftDocs(t *testing.T) {
	docs, err := LoadNovelCraftDocs()
	if err != nil {
		t.Fatalf("LoadNovelCraftDocs: %v", err)
	}
	if len(docs) < 9 {
		t.Fatalf("expected >=9 craft docs, got %d", len(docs))
	}
	seen := map[string]bool{}
	for _, d := range docs {
		if d.SeedKey == "" || d.Title == "" || d.Content == "" {
			t.Fatalf("incomplete craft doc: %+v", d)
		}
		if seen[d.SeedKey] {
			t.Fatalf("duplicate seed key %q", d.SeedKey)
		}
		seen[d.SeedKey] = true
	}
}

func TestBuiltinGitHubExpertOwnsConnectorAndGh(t *testing.T) {
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
	if len(github.Agent.MCPServers) != 1 || github.Agent.MCPServers[0] != "github" {
		t.Fatalf("github expert must bind mcp_servers=[github], got %v", github.Agent.MCPServers)
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
