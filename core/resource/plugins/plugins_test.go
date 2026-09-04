package plugins

import (
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
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
	for _, skill := range []string{"novel-setup", "novel-plan", "novel-write", "novel-review"} {
		if _, err := fs.ReadFile(FS, "novel/skills/"+skill+"/SKILL.md"); err != nil {
			t.Fatalf("%s: %v", skill, err)
		}
	}
	if _, err := fs.Stat(FS, "novel/skills/novel-writing"); err == nil {
		t.Fatal("novel-writing should be replaced by setup/plan/write/review")
	}
	for _, p := range []string{
		"novel/skills/novel-setup/assets/templates/world.md",
		"novel/skills/novel-setup/assets/templates/cast-card.md",
		"novel/skills/novel-setup/assets/templates/book-bible.md",
		"novel/skills/novel-setup/assets/templates/novel-state.yaml",
		"novel/skills/novel-setup/assets/templates/author-lore.md",
		"novel/skills/novel-setup/assets/templates/ledger.md",
		"novel/skills/novel-write/assets/templates/chapter-outline.yaml",
		"novel/skills/novel-write/assets/templates/style-fingerprint.md",
		"novel/skills/novel-plan/assets/templates/book-outline.md",
		"novel/skills/novel-plan/assets/templates/volume-outline.md",
		"novel/ai.danmo.work/knowledge/09-lore-tracks.md",
		"novel/skills/novel-setup/scripts/novel_gate.py",
		"novel/skills/novel-setup/references/gate.md",
	} {
		if _, err := fs.ReadFile(FS, p); err != nil {
			t.Fatalf("%s: %v", p, err)
		}
	}
	outline, err := fs.ReadFile(FS, "novel/skills/novel-write/assets/templates/chapter-outline.yaml")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(outline), "unit_id:") {
		t.Fatal("chapter-outline.yaml must require unit_id")
	}
	cast, err := fs.ReadFile(FS, "novel/skills/novel-setup/assets/templates/cast-card.md")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(cast), "视觉") || !strings.Contains(string(cast), "语言") || !strings.Contains(string(cast), "行为") {
		t.Fatal("cast-card.md must include 三锚点")
	}
	expert, err := fs.ReadFile(FS, "novel/ai.danmo.work/experts/novel.md")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(expert), "tool_id: novel_gate") {
		t.Fatal("novel expert must not bind a Go novel_gate tool")
	}
	if !strings.Contains(string(expert), "novel_gate.py") {
		t.Fatal("novel expert must invoke novel-setup/scripts/novel_gate.py")
	}
	if !strings.Contains(string(expert), "exec_shell") {
		t.Fatal("novel expert must bind exec_shell for the gate script")
	}
	gateDoc, err := fs.ReadFile(FS, "novel/skills/novel-setup/references/gate.md")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(gateDoc), "${WORK_HOME}/plugins/novel/skills/novel-setup/scripts/novel_gate.py") {
		t.Fatal("gate.md must run the plugin script via ${WORK_HOME}")
	}
	if strings.Contains(string(gateDoc), "python3 \"$HOME") {
		t.Fatal("gate.md must not exec $HOME (wrong inside containers)")
	}
	if _, err := fs.Stat(FS, "novel/skills/novel-plan/assets/templates/goldfinger-card.md"); err == nil {
		t.Fatal("goldfinger-card.md belongs under novel-setup, not novel-plan")
	}
	meta, err := fs.ReadFile(FS, "novel/ai.danmo.work/knowledge/_meta.json")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(meta), `"id": "kb-novel-craft"`) {
		t.Fatalf("novel KB meta missing stable id: %s", meta)
	}
}

func TestDocumentPluginPacksOfficeIRKnowledge(t *testing.T) {
	meta, err := fs.ReadFile(FS, "document/ai.danmo.work/knowledge/_meta.json")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(meta), `"id": "kb-office-ir"`) {
		t.Fatalf("document KB meta missing stable id: %s", meta)
	}
	for _, p := range []string{
		"document/ai.danmo.work/knowledge/01-format-matrix.md",
		"document/ai.danmo.work/knowledge/02-slides-ir.md",
		"document/ai.danmo.work/knowledge/03-sheet-ir.md",
		"document/ai.danmo.work/knowledge/04-doc-ir.md",
	} {
		if _, err := fs.ReadFile(FS, p); err != nil {
			t.Fatalf("%s: %v", p, err)
		}
	}
	doc, err := fs.ReadFile(FS, "document/ai.danmo.work/experts/document.md")
	if err != nil {
		t.Fatal(err)
	}
	s := string(doc)
	if !strings.Contains(s, "kb-office-ir") {
		t.Fatal("document expert must bind kb-office-ir")
	}
	if !strings.Contains(s, "search_kb") {
		t.Fatal("document expert must expose search_kb for IR KB")
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

func TestNovelGatePythonScript(t *testing.T) {
	py, err := exec.LookPath("python3")
	if err != nil {
		t.Skip("python3 not found")
	}
	dir := t.TempDir()
	for _, name := range []string{"novel_gate.py", "novel_gate_test.py"} {
		data, err := fs.ReadFile(FS, "novel/skills/novel-setup/scripts/"+name)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, name), data, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	cmd := exec.Command(py, "-m", "unittest", "novel_gate_test")
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("python unittest: %v\n%s", err, out)
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
