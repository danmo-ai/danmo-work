package home

import (
	"testing"
)

func TestBuiltinContentHashStableAndDiffersFromManifest(t *testing.T) {
	a, err := BuiltinContentHash()
	if err != nil {
		t.Fatal(err)
	}
	b, err := BuiltinContentHash()
	if err != nil {
		t.Fatal(err)
	}
	if a != b {
		t.Fatalf("content hash not stable: %s vs %s", a, b)
	}
	if len(a) != 64 {
		t.Fatalf("content hash length %d, want 64 hex chars", len(a))
	}
	manifest, err := BuiltinManifestHash()
	if err != nil {
		t.Fatal(err)
	}
	if a == manifest {
		t.Fatal("content hash should include agents/skills/knowledge, not just manifest.yaml")
	}
}

func TestHashBuiltinFilesChangesWithContent(t *testing.T) {
	base := []BuiltinFile{
		{Path: "manifest.yaml", Content: []byte("version: \"1\"\n")},
		{Path: "skills/debugging/SKILL.md", Content: []byte("debug v1")},
	}
	h1 := hashBuiltinFiles(base)
	changed := []BuiltinFile{
		{Path: "manifest.yaml", Content: []byte("version: \"1\"\n")},
		{Path: "skills/debugging/SKILL.md", Content: []byte("debug v2")},
	}
	h2 := hashBuiltinFiles(changed)
	if h1 == h2 {
		t.Fatal("hash should change when a skill file changes even if manifest is identical")
	}
	reordered := []BuiltinFile{
		{Path: "skills/debugging/SKILL.md", Content: []byte("debug v1")},
		{Path: "manifest.yaml", Content: []byte("version: \"1\"\n")},
	}
	if hashBuiltinFiles(reordered) != h1 {
		t.Fatal("hash should be order-independent")
	}
}
