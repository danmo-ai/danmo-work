package builtin

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestReadSkillBody(t *testing.T) {
	dir := t.TempDir()
	skillDir := filepath.Join(dir, "skills", "demo")
	if err := os.MkdirAll(skillDir, 0o755); err != nil {
		t.Fatal(err)
	}
	md := "---\nname: demo\ndescription: test\n---\n\nhello world"
	if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte(md), 0o644); err != nil {
		t.Fatal(err)
	}

	h := &ReadSkill{dataDir: dir}
	got, err := h.Execute(context.Background(), map[string]any{"path": skillDir})
	if err != nil {
		t.Fatal(err)
	}
	if got.Content != "hello world" {
		t.Fatalf("body=%q", got.Content)
	}
}

func TestReadSkillResourceFile(t *testing.T) {
	dir := t.TempDir()
	skillDir := filepath.Join(dir, "skills", "demo")
	if err := os.MkdirAll(skillDir, 0o755); err != nil {
		t.Fatal(err)
	}
	md := "---\nname: demo\n---\n\nbody"
	if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte(md), 0o644); err != nil {
		t.Fatal(err)
	}
	refDir := filepath.Join(skillDir, "references")
	if err := os.MkdirAll(refDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(refDir, "note.md"), []byte("ref-content"), 0o644); err != nil {
		t.Fatal(err)
	}

	h := &ReadSkill{dataDir: dir}
	got, err := h.Execute(context.Background(), map[string]any{"path": filepath.Join(skillDir, "references", "note.md")})
	if err != nil {
		t.Fatal(err)
	}
	if got.Content != "ref-content" {
		t.Fatalf("ref=%q", got.Content)
	}
}

func TestReadSkillPlaceholderPath(t *testing.T) {
	dir := t.TempDir()
	skillDir := filepath.Join(dir, "skills", "demo")
	if err := os.MkdirAll(skillDir, 0o755); err != nil {
		t.Fatal(err)
	}
	md := "---\nname: demo\n---\n\nhello"
	if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte(md), 0o644); err != nil {
		t.Fatal(err)
	}

	h := &ReadSkill{dataDir: dir}
	p := "{danmo_work_home}/skills/demo"
	got, err := h.Execute(context.Background(), map[string]any{"path": p})
	if err != nil {
		t.Fatal(err)
	}
	if got.Content != "hello" {
		t.Fatalf("body=%q", got.Content)
	}
}

func TestReadSkillInvalidPath(t *testing.T) {
	h := &ReadSkill{dataDir: "/tmp/valid-root"}
	_, err := h.Execute(context.Background(), map[string]any{"path": "/etc/passwd"})
	if err == nil {
		t.Fatal("expected error for path outside skill root")
	}
	if !strings.Contains(err.Error(), "valid skill directory") {
		t.Fatalf("error = %v", err)
	}
}

func TestReadSkillSchema(t *testing.T) {
	s := (&ReadSkill{}).Schema()
	if !strings.Contains(s.Description, "<available_skills>") {
		t.Fatalf("schema: %s", s.Description)
	}
}

func TestSkillPathForPrompt(t *testing.T) {
	tests := []struct {
		skillDir, dataDir, agentsHome, projectDir, want string
	}{
		{"/home/u/.danmo-work/skills/debugging", "/home/u/.danmo-work", "/home/u/.agents", "", "{danmo_work_home}/skills/debugging"},
		{"/home/u/.danmo-work/data/skills/debugging", "/home/u/.danmo-work/data", "/home/u/.agents", "", "{danmo_work_home}/skills/debugging"},
		{"/home/u/.agents/skills/foo", "/home/u/.danmo-work", "/home/u/.agents", "", "{agents_home}/skills/foo"},
		{"/work/proj/.danmo-work/skills/bar", "/home/u/.danmo-work", "/home/u/.agents", "/work/proj", "{project}/.danmo-work/skills/bar"},
		{"/work/proj/.agents/skills/baz", "/home/u/.danmo-work", "/home/u/.agents", "/work/proj", "{project}/.agents/skills/baz"},
	}
	for _, tt := range tests {
		got := SkillPathForPrompt(tt.skillDir, tt.dataDir, tt.agentsHome, tt.projectDir)
		if got != tt.want {
			t.Errorf("SkillPathForPrompt(%q, %q, %q, %q) = %q, want %q",
				tt.skillDir, tt.dataDir, tt.agentsHome, tt.projectDir, got, tt.want)
		}
	}
}

func TestSkillPathForPromptPlugin(t *testing.T) {
	pluginDir := "/home/u/.danmo-work/plugins/ext/skills"
	got := SkillPathForPromptWithPlugins(
		filepath.Join(pluginDir, "plug"),
		"/home/u/.danmo-work/data",
		"/home/u/.agents",
		"",
		[]string{pluginDir},
	)
	want := "{work_home}/plugins/ext/skills/plug"
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestReadSkillWorkHomePlaceholderPath(t *testing.T) {
	root := t.TempDir()
	dataDir := filepath.Join(root, "data")
	skillDir := filepath.Join(root, "plugins", "ext", "skills", "plug")
	if err := os.MkdirAll(skillDir, 0o755); err != nil {
		t.Fatal(err)
	}
	md := "---\nname: plug\n---\n\nplugin-body"
	if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte(md), 0o644); err != nil {
		t.Fatal(err)
	}

	h := &ReadSkill{dataDir: dataDir}
	p := "{work_home}/plugins/ext/skills/plug"
	got, err := h.Execute(context.Background(), map[string]any{"path": p})
	if err != nil {
		t.Fatal(err)
	}
	if got.Content != "plugin-body" {
		t.Fatalf("body=%q", got.Content)
	}
}
