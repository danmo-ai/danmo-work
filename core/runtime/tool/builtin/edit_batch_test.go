package builtin

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestEditBatchSameFileInOrder(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "a.go"), []byte("alpha beta\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	ft := setupTracker(dir)
	ft.NoteRead(filepath.Join(dir, "a.go"))

	h := &EditBatch{}
	result, err := h.Execute(nil, map[string]any{
		"__work_dir":     dir,
		"__file_tracker": ft,
		"edits": []any{
			map[string]any{"path": "a.go", "oldString": "alpha", "newString": "ALPHA"},
			map[string]any{"path": "a.go", "oldString": "ALPHA beta", "newString": "ALPHA BETA"},
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	got, err := os.ReadFile(filepath.Join(dir, "a.go"))
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "ALPHA BETA\n" {
		t.Fatalf("file = %q", got)
	}
	if !strings.Contains(result.Content, "1. a.go: replaced 1") || !strings.Contains(result.Content, "2. a.go: replaced 1") {
		t.Fatalf("result: %s", result.Content)
	}
	changes, _ := result.Meta["file_changes"].([]map[string]any)
	if len(changes) != 1 || changes[0]["path"] != "a.go" {
		t.Fatalf("file_changes: %#v", result.Meta["file_changes"])
	}
}

func TestEditBatchFailureWritesNothing(t *testing.T) {
	dir := t.TempDir()
	orig := "alpha beta\n"
	if err := os.WriteFile(filepath.Join(dir, "a.go"), []byte(orig), 0o644); err != nil {
		t.Fatal(err)
	}
	ft := setupTracker(dir)
	ft.NoteRead(filepath.Join(dir, "a.go"))

	h := &EditBatch{}
	_, err := h.Execute(nil, map[string]any{
		"__work_dir":     dir,
		"__file_tracker": ft,
		"edits": []any{
			map[string]any{"path": "a.go", "oldString": "alpha", "newString": "ALPHA"},
			map[string]any{"path": "a.go", "oldString": "missing", "newString": "nope"},
		},
	})
	if err == nil || !strings.Contains(err.Error(), "edit 2/2") || !strings.Contains(err.Error(), "No files were written") {
		t.Fatalf("error: %v", err)
	}
	got, readErr := os.ReadFile(filepath.Join(dir, "a.go"))
	if readErr != nil {
		t.Fatal(readErr)
	}
	if string(got) != orig {
		t.Fatalf("file changed on failure: %q", got)
	}
}

func TestEditBatchTwoFiles(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "a.txt"), []byte("a\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "b.txt"), []byte("b\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	ft := setupTracker(dir)
	ft.NoteRead(filepath.Join(dir, "a.txt"))
	ft.NoteRead(filepath.Join(dir, "b.txt"))

	h := &EditBatch{}
	_, err := h.Execute(nil, map[string]any{
		"__work_dir":     dir,
		"__file_tracker": ft,
		"edits": []any{
			map[string]any{"path": "a.txt", "oldString": "a", "newString": "A"},
			map[string]any{"path": "b.txt", "oldString": "b", "newString": "B"},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	a, _ := os.ReadFile(filepath.Join(dir, "a.txt"))
	b, _ := os.ReadFile(filepath.Join(dir, "b.txt"))
	if string(a) != "A\n" || string(b) != "B\n" {
		t.Fatalf("a=%q b=%q", a, b)
	}
}
