package plugins

import (
	"io/fs"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestBuiltinPluginNames(t *testing.T) {
	names := BuiltinPluginNames()
	want := map[string]bool{"github": true, "danmo-make": true, "novel": true, "browser": true}
	if len(names) != len(want) {
		t.Fatalf("names=%v want %d entries", names, len(want))
	}
	for _, n := range names {
		if !want[n] {
			t.Errorf("unexpected plugin %q", n)
		}
	}
}

func TestBuiltinPluginManifestsAndExperts(t *testing.T) {
	for _, name := range BuiltinPluginNames() {
		data, err := fs.ReadFile(FS, name+"/plugin.json")
		if err != nil {
			t.Fatalf("%s: plugin.json: %v", name, err)
		}
		if !strings.Contains(string(data), `"name": "`+name+`"`) {
			t.Errorf("%s: plugin.json name mismatch", name)
		}
		expertPath := name + "/ai.danmo.work/experts/" + name + ".md"
		if name == "novel" {
			expertPath = name + "/ai.danmo.work/experts/novel.md"
		}
		ed, err := fs.ReadFile(FS, expertPath)
		if err != nil {
			t.Fatalf("%s: expert: %v", name, err)
		}
		var fm struct {
			ID         string   `yaml:"id"`
			Source     string   `yaml:"source"`
			Mode       string   `yaml:"mode"`
			Skills     []string `yaml:"skills"`
			MCPServers []string `yaml:"mcp_servers"`
			Knowledge  []string `yaml:"knowledge"`
		}
		parts := strings.SplitN(string(ed), "---", 3)
		if len(parts) < 3 {
			t.Fatalf("%s: missing frontmatter", name)
		}
		if err := yaml.Unmarshal([]byte(parts[1]), &fm); err != nil {
			t.Fatalf("%s: frontmatter: %v", name, err)
		}
		if fm.ID != name {
			t.Errorf("%s: id=%q", name, fm.ID)
		}
		if fm.Source != "builtin" {
			t.Errorf("%s: source=%q want builtin", name, fm.Source)
		}
		if fm.Mode != "subagent" {
			t.Errorf("%s: mode=%q want subagent", name, fm.Mode)
		}
	}
}

func TestNovelPluginPacksSkillAndCraftKB(t *testing.T) {
	if _, err := fs.ReadFile(FS, "novel/skills/novel-writing/SKILL.md"); err != nil {
		t.Fatal(err)
	}
	if _, err := fs.ReadFile(FS, "novel/skills/novel-writing/references/routes.md"); err != nil {
		t.Fatal(err)
	}
	meta, err := fs.ReadFile(FS, "novel/ai.danmo.work/knowledge/_meta.json")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(meta), `"id": "kb-novel-craft"`) {
		t.Fatalf("novel KB meta missing stable id: %s", meta)
	}
	if _, err := fs.ReadFile(FS, "novel/ai.danmo.work/knowledge/01-pacing-structure.md"); err != nil {
		t.Fatal(err)
	}
}

func TestGitHubAndDanmoMakePluginsShipBoundMCP(t *testing.T) {
	for _, name := range []string{"github", "danmo-make"} {
		data, err := fs.ReadFile(FS, name+"/mcp.json")
		if err != nil {
			t.Fatalf("%s mcp.json: %v", name, err)
		}
		if !strings.Contains(string(data), `"ambientMount": false`) {
			t.Errorf("%s mcp.json must set ambientMount=false", name)
		}
	}
}

func TestBuiltinContentHashStable(t *testing.T) {
	a, err := BuiltinContentHash()
	if err != nil {
		t.Fatal(err)
	}
	b, err := BuiltinContentHash()
	if err != nil {
		t.Fatal(err)
	}
	if a != b || len(a) != 64 {
		t.Fatalf("hash unstable or wrong len: %q / %q", a, b)
	}
}
