package builtin

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestEditExactReplace(t *testing.T) {
	dir := t.TempDir()
	f := filepath.Join(dir, "test.go")
	content := "package main\n\nfunc main() {\n\tprintln(\"hello\")\n}\n"
	os.WriteFile(f, []byte(content), 0644)

	ft := setupTracker(dir)
	ft.NoteRead(f)

	h := &Edit{}
	result, err := h.Execute(nil, map[string]any{
		"path":           f,
		"oldString":      "\"hello\"",
		"newString":      "\"world\"",
		"__work_dir":     dir,
		"__file_tracker": ft,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(result.Content, "\"world\"") {
		t.Errorf("expected replacement, got: %s", result.Content)
	}
	if !strings.Contains(result.Content, "--- a/") {
		t.Error("expected unified diff in output")
	}
}

func TestEditReplaceAll(t *testing.T) {
	dir := t.TempDir()
	f := filepath.Join(dir, "test.txt")
	content := "foo bar foo bar foo\n"
	os.WriteFile(f, []byte(content), 0644)

	ft := setupTracker(dir)
	ft.NoteRead(f)

	h := &Edit{}
	result, err := h.Execute(nil, map[string]any{
		"path":           f,
		"oldString":      "foo",
		"newString":      "baz",
		"replaceAll":     true,
		"__work_dir":     dir,
		"__file_tracker": ft,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(result.Content, "3 occurrence") {
		t.Errorf("expected 3 replacements, got: %s", result.Content)
	}
}

func TestEditNoopViaFuzzyMatch(t *testing.T) {
	dir := t.TempDir()
	f := filepath.Join(dir, "test.txt")
	content := "foo  bar\n"
	os.WriteFile(f, []byte(content), 0644)

	ft := setupTracker(dir)
	ft.NoteRead(f)

	before, _ := os.Stat(f)

	h := &Edit{}
	result, err := h.Execute(nil, map[string]any{
		"path":           f,
		"oldString":      "foo bar",  // single space: whitespace-fuzzy match
		"newString":      "foo  bar", // equals the actual file text
		"__work_dir":     dir,
		"__file_tracker": ft,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(result.Content, "No changes made") {
		t.Errorf("expected no-op message, got: %q", result.Content)
	}
	if op, _ := result.Meta["op"].(string); op != "noop" {
		t.Errorf("expected op=noop, got %q", op)
	}
	if changed, ok := result.Meta["changed"].(bool); !ok || changed {
		t.Error("expected changed=false in meta")
	}
	after, _ := os.Stat(f)
	if !after.ModTime().Equal(before.ModTime()) {
		t.Error("no-op edit must not rewrite the file (mtime changed)")
	}
}

func TestEditFuzzyIndent(t *testing.T) {
	dir := t.TempDir()
	f := filepath.Join(dir, "test.go")
	content := "func main() {\n\t\tprintln(\"hello\")\n}\n"
	os.WriteFile(f, []byte(content), 0644)

	ft := setupTracker(dir)
	ft.NoteRead(f)

	h := &Edit{}
	result, err := h.Execute(nil, map[string]any{
		"path":           f,
		"oldString":      "\tprintln(\"hello\")",
		"newString":      "\tprintln(\"world\")",
		"__work_dir":     dir,
		"__file_tracker": ft,
	})
	if err != nil {
		t.Fatalf("fuzzy indent matching failed: %v", err)
	}

	if !strings.Contains(result.Content, "println(\"world\")") {
		t.Errorf("expected fuzzy replacement, got: %s", result.Content)
	}
}

func TestEditFuzzyWhitespace(t *testing.T) {
	dir := t.TempDir()
	f := filepath.Join(dir, "test.txt")
	content := "hello\nworld\nfoo  bar\n"
	os.WriteFile(f, []byte(content), 0644)

	ft := setupTracker(dir)
	ft.NoteRead(f)

	h := &Edit{}
	result, err := h.Execute(nil, map[string]any{
		"path":           f,
		"oldString":      "foo bar",
		"newString":      "baz qux",
		"__work_dir":     dir,
		"__file_tracker": ft,
	})
	if err != nil {
		t.Fatalf("fuzzy whitespace matching failed: %v", err)
	}
	if !strings.Contains(result.Content, "baz qux") {
		t.Errorf("expected fuzzy replacement, got: %s", result.Content)
	}
}

func TestEditRequiresReadFirst(t *testing.T) {
	dir := t.TempDir()
	f := filepath.Join(dir, "test.txt")
	content := "hello world\n"
	os.WriteFile(f, []byte(content), 0644)

	ft := setupTracker(dir)

	h := &Edit{}
	_, err := h.Execute(nil, map[string]any{
		"path":           f,
		"oldString":      "hello",
		"newString":      "goodbye",
		"__work_dir":     dir,
		"__file_tracker": ft,
	})
	if err == nil {
		t.Fatal("expected error for editing without reading first")
	}
	if !strings.Contains(err.Error(), "has not been read yet") {
		t.Errorf("expected read-first error, got: %v", err)
	}
}

func TestEditConsecutiveWithoutReread(t *testing.T) {
	dir := t.TempDir()
	f := filepath.Join(dir, "test.txt")
	os.WriteFile(f, []byte("alpha beta gamma\n"), 0644)

	ft := setupTracker(dir)
	ft.NoteRead(f)

	h := &Edit{}
	input := map[string]any{
		"path":           f,
		"oldString":      "alpha",
		"newString":      "ALPHA",
		"__work_dir":     dir,
		"__file_tracker": ft,
	}
	if _, err := h.Execute(nil, input); err != nil {
		t.Fatalf("first edit: %v", err)
	}

	// Second edit must not fail just because our own first write changed mtime.
	input["oldString"] = "beta"
	input["newString"] = "BETA"
	if _, err := h.Execute(nil, input); err != nil {
		t.Fatalf("second edit without re-read: %v", err)
	}

	saved, _ := os.ReadFile(f)
	if got := string(saved); got != "ALPHA BETA gamma\n" {
		t.Fatalf("unexpected content: %q", got)
	}
}

func TestEditStillDetectsExternalChange(t *testing.T) {
	dir := t.TempDir()
	f := filepath.Join(dir, "test.txt")
	os.WriteFile(f, []byte("hello world\n"), 0644)

	ft := setupTracker(dir)
	ft.NoteRead(f)

	// External modification after the tracked read.
	os.WriteFile(f, []byte("hello world\nchanged\n"), 0644)

	h := &Edit{}
	_, err := h.Execute(nil, map[string]any{
		"path":           f,
		"oldString":      "hello",
		"newString":      "goodbye",
		"__work_dir":     dir,
		"__file_tracker": ft,
	})
	if err == nil {
		t.Fatal("expected error after external change")
	}
	if !strings.Contains(err.Error(), "has changed since it was last read") {
		t.Errorf("expected stale-read error, got: %v", err)
	}
}

func TestEditMultipleExactMatches(t *testing.T) {
	dir := t.TempDir()
	f := filepath.Join(dir, "test.txt")
	content := "foo bar foo baz foo\n"
	os.WriteFile(f, []byte(content), 0644)

	ft := setupTracker(dir)
	ft.NoteRead(f)

	h := &Edit{}
	_, err := h.Execute(nil, map[string]any{
		"path":           f,
		"oldString":      "foo",
		"newString":      "bar",
		"replaceAll":     false,
		"__work_dir":     dir,
		"__file_tracker": ft,
	})
	if err == nil {
		t.Fatal("expected error for multiple matches without replaceAll")
	}
	if !strings.Contains(err.Error(), "set replaceAll=true") {
		t.Errorf("expected replaceAll hint, got: %v", err)
	}
}

func TestEditNotFoundIncludesClosestMatch(t *testing.T) {
	dir := t.TempDir()
	f := filepath.Join(dir, "story.md")
	content := "chapter one\nthe hero walked home\nend\n"
	os.WriteFile(f, []byte(content), 0644)

	ft := setupTracker(dir)
	ft.NoteRead(f)

	h := &Edit{}
	// Near-miss that fails exact/indent/whitespace matchers but is still close enough for hints.
	_, err := h.Execute(nil, map[string]any{
		"path":           f,
		"oldString":      "the hero walked away slowly",
		"newString":      "the hero ran home",
		"__work_dir":     dir,
		"__file_tracker": ft,
	})
	if err == nil {
		t.Fatal("expected not-found error")
	}
	msg := err.Error()
	if !strings.Contains(msg, "Closest match") && !strings.Contains(msg, "No close match") {
		t.Fatalf("expected closest-match guidance, got: %v", err)
	}
	if !strings.Contains(msg, "Tips:") {
		t.Fatalf("expected tips in error, got: %v", err)
	}
}

func TestEditNotFoundLineNumberPrefixHint(t *testing.T) {
	dir := t.TempDir()
	f := filepath.Join(dir, "a.txt")
	os.WriteFile(f, []byte("hello world\n"), 0644)
	ft := setupTracker(dir)
	ft.NoteRead(f)

	h := &Edit{}
	_, err := h.Execute(nil, map[string]any{
		"path":           f,
		"oldString":      "1: hello world",
		"newString":      "1: hello there",
		"__work_dir":     dir,
		"__file_tracker": ft,
	})
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "line-number prefixes") {
		t.Fatalf("expected line-number hint, got: %v", err)
	}
}
