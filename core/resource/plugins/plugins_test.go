package plugins

import (
	"io/fs"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestBuiltinPluginNames(t *testing.T) {
	names := BuiltinPluginNames()
	want := map[string]bool{
		"github": true, "danmo-make": true, "novel": true,
		"browser": true, "computer": true,
		"implementer": true, "explorer": true, "reviewer": true,
		"researcher": true, "document": true, "data": true,
	}
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
		ed, err := fs.ReadFile(FS, expertPath)
		if err != nil {
			t.Fatalf("%s: expert: %v", name, err)
		}
		var fm struct {
			ID     string `yaml:"id"`
			Source string `yaml:"source"`
			Mode   string `yaml:"mode"`
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

func TestDocumentPluginAbsorbedComms(t *testing.T) {
	ed, err := fs.ReadFile(FS, "document/ai.danmo.work/experts/document.md")
	if err != nil {
		t.Fatal(err)
	}
	body := string(ed)
	for _, needle := range []string{"email", "notification", "workplace writing"} {
		if !strings.Contains(strings.ToLower(body), strings.ToLower(needle)) {
			t.Errorf("document expert missing comms coverage for %q", needle)
		}
	}
	if _, err := fs.Stat(FS, "comms/plugin.json"); err == nil {
		t.Fatal("comms should not be a separate plugin after merge")
	}
}

func TestNovelPluginPacksSkillAndCraftKB(t *testing.T) {
	if _, err := fs.ReadFile(FS, "novel/skills/novel-writing/SKILL.md"); err != nil {
		t.Fatal(err)
	}
	meta, err := fs.ReadFile(FS, "novel/ai.danmo.work/knowledge/_meta.json")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(meta), `"id": "kb-novel-craft"`) {
		t.Fatalf("novel KB meta missing stable id: %s", meta)
	}
}

func TestBrowserAndComputerPluginsPackSkills(t *testing.T) {
	if _, err := fs.ReadFile(FS, "browser/skills/browser/SKILL.md"); err != nil {
		t.Fatal(err)
	}
	if _, err := fs.ReadFile(FS, "computer/skills/computer-use/SKILL.md"); err != nil {
		t.Fatal(err)
	}
}

func TestThinCodingPluginsRelyOnHomeSkills(t *testing.T) {
	// implementer should not ship a private copy of debugging/TDD — those stay in home.
	entries, err := fs.ReadDir(FS, "implementer")
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		if e.Name() == "skills" {
			t.Fatal("implementer plugin should not embed skills/; use shared home skills")
		}
	}
}

func TestBuiltinPluginCategories(t *testing.T) {
	want := map[string]string{
		"implementer": "coding", "explorer": "coding", "reviewer": "coding", "github": "coding",
		"researcher": "research", "browser": "research", "computer": "research",
		"document": "office", "data": "office",
		"novel": "creative", "danmo-make": "creative",
	}
	for name, cat := range want {
		ed, err := fs.ReadFile(FS, name+"/ai.danmo.work/experts/"+name+".md")
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		var fm struct {
			Category string `yaml:"category"`
		}
		parts := strings.SplitN(string(ed), "---", 3)
		if len(parts) < 3 {
			t.Fatalf("%s: missing frontmatter", name)
		}
		if err := yaml.Unmarshal([]byte(parts[1]), &fm); err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if fm.Category != cat {
			t.Errorf("%s category=%q want %q", name, fm.Category, cat)
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
