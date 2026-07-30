package textdiff

import (
	"strings"
	"testing"
)

func TestUnifiedBasic(t *testing.T) {
	old := "a\nb\nc\nd\ne\n"
	cur := "a\nB\nc\nD\ne\n"
	patch := Unified("x.md", old, cur)
	if !strings.Contains(patch, "-b") || !strings.Contains(patch, "+B") {
		t.Fatalf("patch:\n%s", patch)
	}
	if !strings.Contains(patch, "-d") || !strings.Contains(patch, "+D") {
		t.Fatalf("patch:\n%s", patch)
	}
	if !strings.Contains(patch, " a") {
		t.Fatalf("expected context a:\n%s", patch)
	}
}

func TestUnifiedIdentical(t *testing.T) {
	if Unified("f", "x\n", "x\n") != "" {
		t.Fatal("expected empty")
	}
}
