package builtin

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"danmo-work/core/runtime/tool"
)

func setupTracker(workDir string) *tool.FileTracker {
	return tool.NewFileTracker(workDir)
}

func TestWriteCreatesDirectory(t *testing.T) {
	dir := t.TempDir()
	newDir := filepath.Join(dir, "newdir")

	h := &Write{}
	result, err := h.Execute(nil, map[string]any{
		"path":       newDir,
		"write_type": "directory",
		"__work_dir": dir,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(result.Content, "Created directory") {
		t.Error("expected directory creation message")
	}

	_, statErr := os.Stat(newDir)
	if statErr != nil {
		t.Errorf("directory was not created: %v", statErr)
	}
}

func TestWriteFileWithDiff(t *testing.T) {
	dir := t.TempDir()
	f := filepath.Join(dir, "test.txt")

	h := &Write{}
	_, err := h.Execute(nil, map[string]any{
		"path":       f,
		"content":    "new content\n",
		"__work_dir": dir,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	saved, _ := os.ReadFile(f)
	if string(saved) != "new content\n" {
		t.Errorf("expected content, got: %q", string(saved))
	}
}

func TestWriteIdenticalContentNoop(t *testing.T) {
	dir := t.TempDir()
	f := filepath.Join(dir, "test.txt")
	const content = "same content\n"

	h := &Write{}
	ft := setupTracker(dir)
	if _, err := h.Execute(nil, map[string]any{
		"path": f, "content": content, "__work_dir": dir, "__file_tracker": ft,
	}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	before, statErr := os.Stat(f)
	if statErr != nil {
		t.Fatalf("stat: %v", statErr)
	}

	result, err := h.Execute(nil, map[string]any{
		"path": f, "content": content, "__work_dir": dir, "__file_tracker": ft,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(result.Content, "No changes written") {
		t.Errorf("expected no-op message, got: %q", result.Content)
	}
	if op, _ := result.Meta["op"].(string); op != "noop" {
		t.Errorf("expected op=noop, got %q", op)
	}
	if changed, ok := result.Meta["changed"].(bool); !ok || changed {
		t.Error("expected changed=false in meta")
	}
	if diff, _ := result.Meta["diff"].(string); diff != "" {
		t.Errorf("expected empty diff, got %q", diff)
	}
	after, _ := os.Stat(f)
	if !after.ModTime().Equal(before.ModTime()) {
		t.Error("no-op write must not rewrite the file (mtime changed)")
	}
}

func TestWriteOverwriteRequiresRead(t *testing.T) {
	dir := t.TempDir()
	f := filepath.Join(dir, "test.txt")
	os.WriteFile(f, []byte("old content\n"), 0644)

	ft := setupTracker(dir)
	// Don't call NoteRead on purpose

	h := &Write{}
	_, err := h.Execute(nil, map[string]any{
		"path":           f,
		"content":        "new content\n",
		"__work_dir":     dir,
		"__file_tracker": ft,
	})
	if err == nil {
		t.Fatal("expected error when overwriting without reading first")
	}
	if !strings.Contains(err.Error(), "has not been read yet") {
		t.Errorf("expected read-first error, got: %v", err)
	}
}

func TestApplyPatchFuzz(t *testing.T) {
	dir := t.TempDir()
	f := filepath.Join(dir, "test.txt")
	content := "line1\nline2\nline3\nline4 - target\nline5\nline6\nline7\n"
	os.WriteFile(f, []byte(content), 0644)

	patch := "--- test.txt\n+++ test.txt\n@@ -3,3 +3,3 @@\n line3\n-line4 - target\n+line4 - replaced\n line5\n"

	h := &ApplyPatch{}
	result, err := h.Execute(nil, map[string]any{
		"patch":      patch,
		"fuzz":       float64(3),
		"__work_dir": dir,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(result.Content, "Patched") {
		t.Errorf("expected successful patch, got: %q", result.Content)
	}

	saved, _ := os.ReadFile(f)
	if !strings.Contains(string(saved), "line4 - replaced") {
		t.Errorf("expected patched content, got: %q", string(saved))
	}
}

func TestApplyPatchNoop(t *testing.T) {
	dir := t.TempDir()
	f := filepath.Join(dir, "test.txt")
	content := "a\nb\nc\n"
	os.WriteFile(f, []byte(content), 0644)

	before, _ := os.Stat(f)

	h := &ApplyPatch{}
	result, err := h.Execute(nil, map[string]any{
		"patch":      "*** Begin Patch\n*** Update File: test.txt\n@@\n a\n-b\n+b\n c\n*** End Patch\n",
		"__work_dir": dir,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(result.Content, "No changes to") {
		t.Errorf("expected no-op message, got: %q", result.Content)
	}
	if fc, ok := result.Meta["file_changes"].([]map[string]any); ok && len(fc) != 0 {
		t.Errorf("expected no file_changes entries for no-op, got %v", fc)
	}
	after, _ := os.Stat(f)
	if !after.ModTime().Equal(before.ModTime()) {
		t.Error("no-op patch must not rewrite the file (mtime changed)")
	}
}

func TestApplyHunksDirect(t *testing.T) {
	lines := []string{"line1", "line2", "line3", "line4 - target", "line5", "line6", "line7"}

	h := hunk{
		oldStart: 3,
		oldCount: 3,
		newStart: 3,
		newCount: 3,
		lines: []hunkLine{
			{op: ' ', text: "line3"},
			{op: '-', text: "line4 - target"},
			{op: '+', text: "line4 - replaced"},
			{op: ' ', text: "line5"},
		},
	}

	result, err := applyHunks(lines, []hunk{h}, 3)
	if err != nil {
		t.Fatalf("applyHunks failed: %v", err)
	}
	t.Logf("applyHunks result: %q", strings.Join(result, "\n"))
	if strings.Join(result, "\n") != "line1\nline2\nline3\nline4 - replaced\nline5\nline6\nline7" {
		t.Errorf("unexpected result: %q", strings.Join(result, "\n"))
	}
}

func TestApplyPatchCreateFile(t *testing.T) {
	dir := t.TempDir()

	patch := "--- /dev/null\n+++ newfile.txt\n@@ -0,0 +1,1 @@\n+hello world\n"

	h := &ApplyPatch{}
	result, err := h.Execute(nil, map[string]any{
		"patch":             patch,
		"create_if_missing": true,
		"__work_dir":        dir,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(result.Content, "Patched") && !strings.Contains(result.Content, "Created") {
		t.Errorf("expected successful patch, got: %s", result.Content)
	}

	newFile := filepath.Join(dir, "newfile.txt")
	saved, err := os.ReadFile(newFile)
	if err != nil {
		t.Fatalf("new file not created: %v", err)
	}
	if !strings.Contains(string(saved), "hello world") {
		t.Errorf("expected content in created file, got: %s", string(saved))
	}
}

func TestApplyBeginPatchUpdate(t *testing.T) {
	dir := t.TempDir()
	f := filepath.Join(dir, "src", "app.go")
	os.MkdirAll(filepath.Dir(f), 0755)
	os.WriteFile(f, []byte("package main\n\nfunc greet() {\n\tprintln(\"hi\")\n}\n"), 0644)

	patch := "*** Begin Patch\n" +
		"*** Update File: src/app.go\n" +
		"@@ func greet()\n" +
		" func greet() {\n" +
		"-\tprintln(\"hi\")\n" +
		"+\tprintln(\"hello\")\n" +
		" }\n" +
		"*** End Patch\n"

	h := &ApplyPatch{}
	result, err := h.Execute(nil, map[string]any{
		"patch":      patch,
		"__work_dir": dir,
	})
	if err != nil {
		t.Fatalf("begin-patch update failed: %v", err)
	}
	if !strings.Contains(result.Content, "Patched") {
		t.Fatalf("unexpected result: %s", result.Content)
	}
	saved, _ := os.ReadFile(f)
	if !strings.Contains(string(saved), `println("hello")`) {
		t.Fatalf("content not updated: %q", string(saved))
	}
}

func TestApplyBeginPatchAddDelete(t *testing.T) {
	dir := t.TempDir()
	old := filepath.Join(dir, "old.txt")
	os.WriteFile(old, []byte("bye\n"), 0644)

	patch := "*** Begin Patch\n" +
		"*** Add File: new.txt\n" +
		"+hello\n" +
		"*** Delete File: old.txt\n" +
		"*** End Patch\n"

	h := &ApplyPatch{}
	if _, err := h.Execute(nil, map[string]any{"patch": patch, "__work_dir": dir}); err != nil {
		t.Fatalf("add/delete failed: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "new.txt")); err != nil {
		t.Fatalf("new.txt missing: %v", err)
	}
	if _, err := os.Stat(old); !os.IsNotExist(err) {
		t.Fatal("old.txt should be deleted")
	}
}

func TestApplyBeginPatchWhitespaceFuzzy(t *testing.T) {
	dir := t.TempDir()
	f := filepath.Join(dir, "a.txt")
	os.WriteFile(f, []byte("foo  bar\nbaz\n"), 0644)

	patch := "*** Begin Patch\n*** Update File: a.txt\n@@\n-foo bar\n+foo qux\n baz\n*** End Patch\n"
	h := &ApplyPatch{}
	if _, err := h.Execute(nil, map[string]any{"patch": patch, "__work_dir": dir}); err != nil {
		t.Fatalf("whitespace fuzzy begin-patch failed: %v", err)
	}
	saved, _ := os.ReadFile(f)
	if string(saved) != "foo qux\nbaz\n" {
		t.Fatalf("got %q", string(saved))
	}
}

func TestApplyPatchErrorIncludesClosest(t *testing.T) {
	dir := t.TempDir()
	f := filepath.Join(dir, "a.txt")
	os.WriteFile(f, []byte("alpha\nbeta\ngamma\n"), 0644)

	patch := "*** Begin Patch\n*** Update File: a.txt\n@@\n-betta\n+beta2\n*** End Patch\n"
	h := &ApplyPatch{}
	_, err := h.Execute(nil, map[string]any{"patch": patch, "__work_dir": dir})
	if err == nil {
		t.Fatal("expected mismatch error")
	}
	if !strings.Contains(err.Error(), "Closest region") && !strings.Contains(err.Error(), "cannot locate") {
		t.Fatalf("expected helpful error, got: %v", err)
	}
}

func TestApplyPatchDeleteFile(t *testing.T) {
	dir := t.TempDir()
	f := filepath.Join(dir, "to_delete.txt")
	os.WriteFile(f, []byte("delete me\n"), 0644)

	patch := "--- to_delete.txt\n+++ /dev/null\n@@ -1,1 +0,0 @@\n-delete me\n"

	h := &ApplyPatch{}
	result, err := h.Execute(nil, map[string]any{
		"patch":      patch,
		"__work_dir": dir,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(result.Content, "Deleted") {
		t.Errorf("expected successful deletion patch, got: %s", result.Content)
	}

	// Verify file was deleted
	_, statErr := os.Stat(f)
	if statErr == nil {
		t.Error("file should have been deleted")
	}
}
