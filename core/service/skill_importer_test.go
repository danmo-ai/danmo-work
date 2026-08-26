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

func TestParseSkillMDBuiltinFlag(t *testing.T) {
	content := `---
name: sheet-writing
source: builtin
description: tables
---

Body
`
	sk, err := NewSkillImporter().ParseSkillMD(content)
	if err != nil {
		t.Fatal(err)
	}
	if !sk.Builtin || sk.Source != "builtin" {
		t.Fatalf("builtin=%v source=%q", sk.Builtin, sk.Source)
	}
}

func TestParseSkillMDDescriptionContainsFence(t *testing.T) {
	content := `---
name: playable-slides
source: builtin
description: "Marp Markdown (` + "`---`" + ` page breaks) for slides"
---

# Playable Slides
`
	sk, err := NewSkillImporter().ParseSkillMD(content)
	if err != nil {
		t.Fatal(err)
	}
	if sk.ID != "playable-slides" {
		t.Fatalf("id = %q", sk.ID)
	}
	if !sk.Builtin {
		t.Fatal("expected builtin")
	}
	if !strings.Contains(sk.Description, "page breaks") {
		t.Fatalf("description = %q", sk.Description)
	}
	if sk.Body != "# Playable Slides" {
		t.Fatalf("body = %q", sk.Body)
	}
}

func TestImportAllNestedSkillID(t *testing.T) {
	root := t.TempDir()
	nested := filepath.Join(root, "team", "planner")
	if err := os.MkdirAll(nested, 0o755); err != nil {
		t.Fatal(err)
	}
	md := "---\nname: planner\ndescription: d\n---\n\nBody\n"
	if err := os.WriteFile(filepath.Join(nested, "SKILL.md"), []byte(md), 0o644); err != nil {
		t.Fatal(err)
	}
	skills, _, err := NewSkillImporter().ImportAll(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(skills) != 1 || skills[0].ID != "team/planner" {
		t.Fatalf("got %+v", skills)
	}
	if skills[0].Metadata[SkillMetaRealPath] != nested {
		t.Fatalf("real_path=%q", skills[0].Metadata[SkillMetaRealPath])
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
