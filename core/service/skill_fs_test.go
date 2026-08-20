package service

import (
	"os"
	"path/filepath"
	"testing"

	"danmo-work/core/domain"
)

func writeSkill(t *testing.T, dir, name, body string) {
	t.Helper()
	skillDir := filepath.Join(dir, name)
	if err := os.MkdirAll(filepath.Join(skillDir, "references"), 0o755); err != nil {
		t.Fatal(err)
	}
	md := "---\nname: " + name + "\ndescription: test " + name + "\n---\n\n" + body + "\n"
	if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte(md), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(skillDir, "references", "note.md"), []byte("ref-"+name+"-"+filepath.Base(dir)), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestScanSkillDirsOverridesByOrder(t *testing.T) {
	root := t.TempDir()
	low := filepath.Join(root, "low")
	high := filepath.Join(root, "high")
	_ = os.MkdirAll(low, 0o755)
	_ = os.MkdirAll(high, 0o755)
	writeSkill(t, low, "demo", "from-low")
	writeSkill(t, high, "demo", "from-high")
	writeSkill(t, high, "only-high", "only")

	skills, files := ScanSkillDirs([]string{low, high})
	if len(skills) != 2 {
		t.Fatalf("skills=%d %+v", len(skills), skills)
	}
	found := map[string]string{}
	for _, sk := range skills {
		found[sk.ID] = sk.Body
	}
	if found["demo"] != "from-high" {
		t.Fatalf("demo body=%q", found["demo"])
	}
	if found["only-high"] != "only" {
		t.Fatalf("only-high=%q", found["only-high"])
	}
	refs := files["demo"]
	if len(refs) != 1 {
		t.Fatalf("demo files=%+v", refs)
	}
	if string(refs[0].Content) != "ref-demo-high" {
		t.Fatalf("demo file content=%q", refs[0].Content)
	}
}

func TestScanSkillDirsSetsDirFromID(t *testing.T) {
	root := t.TempDir()
	writeSkill(t, root, "demo", "body")
	skills, _ := ScanSkillDirs([]string{root})
	if len(skills) != 1 {
		t.Fatalf("skills=%d", len(skills))
	}
	want := filepath.Join(root, "demo")
	if skills[0].Dir != want {
		t.Fatalf("Dir=%q want %q", skills[0].Dir, want)
	}
}

func TestScanSkillDirsMissingOK(t *testing.T) {
	skills, files := ScanSkillDirs([]string{filepath.Join(t.TempDir(), "nope")})
	if len(skills) != 0 || len(files) != 0 {
		t.Fatalf("got skills=%v files=%v", skills, files)
	}
}

func TestMergeSkillsByID(t *testing.T) {
	merged := MergeSkillsByID(
		[]domain.Skill{{ID: "a", Body: "1"}, {ID: "b", Body: "1"}},
		[]domain.Skill{{ID: "b", Body: "2"}, {ID: "c", Body: "2"}},
	)
	if len(merged) != 3 {
		t.Fatalf("len=%d", len(merged))
	}
	byID := map[string]string{}
	for _, sk := range merged {
		byID[sk.ID] = sk.Body
	}
	if byID["a"] != "1" || byID["b"] != "2" || byID["c"] != "2" {
		t.Fatalf("%v", byID)
	}
	if merged[0].ID != "a" || merged[1].ID != "b" || merged[2].ID != "c" {
		t.Fatalf("order=%v", merged)
	}
}

func TestBoundSkills(t *testing.T) {
	all := []domain.Skill{{ID: "a"}, {ID: "b"}, {ID: "c"}}
	got := BoundSkills(all, domain.Agent{SkillIDs: []string{"b", "x", "a"}})
	if len(got) != 2 || got[0].ID != "a" || got[1].ID != "b" {
		t.Fatalf("%+v", got)
	}
	if BoundSkills(all, domain.Agent{}) != nil {
		t.Fatal("expected nil for empty skillIds")
	}
}
