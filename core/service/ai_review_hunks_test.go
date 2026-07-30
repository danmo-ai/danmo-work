package service

import (
	"strings"
	"testing"

	"danmo-work/core/textdiff"
)

func TestApplySelectedHunksPartial(t *testing.T) {
	old := "a\nb\nc\nd\ne\n"
	cur := "a\nB\nc\nD\ne\n"
	patch := textdiff.Unified("x.md", old, cur)
	hunks, err := parseUnifiedHunks(patch)
	if err != nil {
		t.Fatal(err)
	}
	if len(hunks) == 0 {
		t.Fatalf("no hunks in patch:\n%s", patch)
	}

	acceptAll, err := applySelectedHunks(old, cur, patch, true, nil)
	if err != nil {
		t.Fatal(err)
	}
	if acceptAll != cur {
		t.Fatalf("accept all:\n got %q\nwant %q", acceptAll, cur)
	}

	rejectAll, err := applySelectedHunks(old, cur, patch, false, nil)
	if err != nil {
		t.Fatal(err)
	}
	if rejectAll != old {
		t.Fatalf("reject all: %q", rejectAll)
	}

	// Accept only first hunk — reverse the rest.
	out, err := applySelectedHunks(old, cur, patch, false, []int{0})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "B") {
		t.Fatalf("expected first change kept: %q\npatch:\n%s", out, patch)
	}
}

func TestParseUnifiedHunks(t *testing.T) {
	patch := "--- a/f\n+++ b/f\n@@ -1,3 +1,3 @@\n a\n-b\n+B\n c\n"
	hunks, err := parseUnifiedHunks(patch)
	if err != nil {
		t.Fatal(err)
	}
	if len(hunks) != 1 {
		t.Fatalf("hunks=%d", len(hunks))
	}
	if len(hunks[0].OldLines) != 3 || hunks[0].OldLines[1] != "b" {
		t.Fatalf("old lines: %#v", hunks[0].OldLines)
	}
	if hunks[0].NewLines[1] != "B" {
		t.Fatalf("new lines: %#v", hunks[0].NewLines)
	}
}
