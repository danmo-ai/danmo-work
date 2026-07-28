package service

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestImportNormalizesBodyRefs(t *testing.T) {
	dir := t.TempDir()
	skillDir := filepath.Join(dir, "demo-skill")
	if err := os.MkdirAll(filepath.Join(skillDir, "references"), 0o755); err != nil {
		t.Fatal(err)
	}
	md := "---\nname: demo-skill\ndescription: d\n---\n\nSee `references/note.md`.\n"
	if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte(md), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(skillDir, "references", "note.md"), []byte("hi"), 0o644); err != nil {
		t.Fatal(err)
	}

	sk, files, err := NewSkillImporter().Import(skillDir)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(sk.Body, "`demo-skill/references/note.md`") {
		t.Fatalf("body not normalized: %q", sk.Body)
	}
	if len(files) != 1 || files[0].Path != "references/note.md" {
		t.Fatalf("files = %+v", files)
	}
}
