package service

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParseSkillMDNestedMetadata(t *testing.T) {
	content := `---
name: gifgrep
description: Search GIFs
metadata:
  clawdbot:
    requires:
      bins: [gifgrep]
    install:
      - kind: brew
        formula: steipete/tap/gifgrep
---

Body here.
`
	sk, err := NewSkillImporter().ParseSkillMD(content)
	if err != nil {
		t.Fatal(err)
	}
	if sk.ID != "gifgrep" {
		t.Fatalf("id = %q", sk.ID)
	}
	raw, ok := sk.Metadata["clawdbot"]
	if !ok || raw == "" {
		t.Fatalf("expected clawdbot metadata, got %#v", sk.Metadata)
	}
	if !strings.Contains(raw, `"kind":"brew"`) && !strings.Contains(raw, `"kind": "brew"`) {
		t.Fatalf("expected brew install in metadata: %s", raw)
	}
	tip := compatibilityFromSkillMetadata(sk.Metadata)
	if !strings.Contains(tip, "needs:brew") {
		t.Fatalf("compatibility tip = %q", tip)
	}
}

func TestParseSkillMDFlatMetadata(t *testing.T) {
	content := `---
name: plain
description: d
metadata:
  version: "1.0"
---

x
`
	sk, err := NewSkillImporter().ParseSkillMD(content)
	if err != nil {
		t.Fatal(err)
	}
	if sk.Metadata["version"] != "1.0" {
		t.Fatalf("metadata = %#v", sk.Metadata)
	}
}

func TestImportWalksAllResourceDirs(t *testing.T) {
	dir := t.TempDir()
	mustWrite := func(rel, body string) {
		p := filepath.Join(dir, rel)
		if err := os.MkdirAll(filepath.Dir(p), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(body), 0644); err != nil {
			t.Fatal(err)
		}
	}
	mustWrite("SKILL.md", "---\nname: demo\ndescription: d\n---\n\nBody\n")
	mustWrite("references/a.md", "a")
	mustWrite("templates/b.md", "b")
	mustWrite("rules/c.md", "c")
	mustWrite("_meta.json", `{}`)
	mustWrite(".hidden", "x")

	sk, files, err := NewSkillImporter().Import(dir)
	if err != nil {
		t.Fatal(err)
	}
	if sk.ID != "demo" {
		t.Fatalf("id = %q", sk.ID)
	}
	got := map[string]bool{}
	for _, f := range files {
		got[f.Path] = true
	}
	for _, want := range []string{"references/a.md", "templates/b.md", "rules/c.md"} {
		if !got[want] {
			t.Fatalf("missing file %s in %#v", want, got)
		}
	}
	if got["_meta.json"] || got[".hidden"] || got["SKILL.md"] {
		t.Fatalf("unexpected files: %#v", got)
	}
}
