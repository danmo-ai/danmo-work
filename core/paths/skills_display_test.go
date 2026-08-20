package paths

import (
	"os"
	"path/filepath"
	"testing"
)

func TestFormatSkillPath(t *testing.T) {
	pluginDir := "/home/u/.danmo-work/plugins/ext/skills"
	mappings := SkillRootMappings(
		"/home/u/.danmo-work/data",
		"/home/u/.agents",
		"/work/proj",
		[]string{pluginDir},
	)
	tests := []struct {
		skillDir, want string
	}{
		{"/home/u/.danmo-work/data/skills/debugging", "{danmo_work_home}/skills/debugging"},
		{"/home/u/.agents/skills/foo", "{agents_home}/skills/foo"},
		{"/work/proj/.danmo-work/skills/bar", "{project}/.danmo-work/skills/bar"},
		{"/work/proj/.agents/skills/baz", "{project}/.agents/skills/baz"},
		{filepath.Join(pluginDir, "plug"), "{work_home}/plugins/ext/skills/plug"},
		{"/tmp/outside/skill", "/tmp/outside/skill"},
	}
	for _, tt := range tests {
		got := FormatSkillPath(tt.skillDir, mappings)
		if got != tt.want {
			t.Errorf("FormatSkillPath(%q) = %q, want %q", tt.skillDir, got, tt.want)
		}
	}
}

func TestFormatSkillPathLongestPrefix(t *testing.T) {
	mappings := []SkillRootMapping{
		{AbsRoot: "/home/u/.danmo-work/data/skills", PlaceholderPrefix: "{danmo_work_home}/skills"},
		{AbsRoot: "/home/u/.danmo-work/data/skills/nested", PlaceholderPrefix: "{nested}"},
	}
	got := FormatSkillPath("/home/u/.danmo-work/data/skills/nested/foo", mappings)
	if got != "{nested}/foo" {
		t.Fatalf("got %q", got)
	}
}

func TestPluginSkillDirs(t *testing.T) {
	root := t.TempDir()
	dataDir := filepath.Join(root, "data")
	pluginSkills := filepath.Join(root, "plugins", "ext", "skills")
	if err := os.MkdirAll(pluginSkills, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, "plugins", "empty"), 0o755); err != nil {
		t.Fatal(err)
	}
	got := PluginSkillDirs(dataDir)
	if len(got) != 1 || got[0] != pluginSkills {
		t.Fatalf("PluginSkillDirs = %v, want [%s]", got, pluginSkills)
	}
}

func TestPathUnderRoot(t *testing.T) {
	if !PathUnderRoot("/a/b/c", "/a/b") {
		t.Fatal("expected under")
	}
	if PathUnderRoot("/a/b-extra/c", "/a/b") {
		t.Fatal("prefix without separator must not match")
	}
	if PathUnderRoot("", "/a") || PathUnderRoot("/a", "") {
		t.Fatal("empty should be false")
	}
}
