package service

import (
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
