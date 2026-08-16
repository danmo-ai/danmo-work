package builtin

import (
	"os"
	"path/filepath"
	"testing"
)

func execFileOp(t *testing.T, workDir string, action, path, dest string, recursive bool) error {
	t.Helper()
	h := &FileOp{}
	input := map[string]any{"action": action, "path": path, "__work_dir": workDir}
	if dest != "" {
		input["destination"] = dest
	}
	if recursive {
		input["recursive"] = true
	}
	_, err := h.Execute(nil, input)
	return err
}

func mustRead(t *testing.T, p string) string {
	t.Helper()
	data, err := os.ReadFile(p)
	if err != nil {
		t.Fatalf("read %s: %v", p, err)
	}
	return string(data)
}

func TestFileOpMoveFile(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "a.txt")
	dst := filepath.Join(dir, "sub", "b.txt")
	if err := os.WriteFile(src, []byte("hello"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := execFileOp(t, dir, "move", "a.txt", "sub/b.txt", false); err != nil {
		t.Fatalf("move: %v", err)
	}
	if _, err := os.Stat(src); !os.IsNotExist(err) {
		t.Fatal("source should not exist after move")
	}
	if got := mustRead(t, dst); got != "hello" {
		t.Fatalf("dest content = %q", got)
	}
}

func TestFileOpMoveDir(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "old", "nested"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "old", "nested", "f.txt"), []byte("x"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := execFileOp(t, dir, "move", "old", "new", false); err != nil {
		t.Fatalf("move dir: %v", err)
	}
	if got := mustRead(t, filepath.Join(dir, "new", "nested", "f.txt")); got != "x" {
		t.Fatalf("moved content = %q", got)
	}
}

func TestFileOpMoveDestExists(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "a.txt"), []byte("a"), 0644)
	os.WriteFile(filepath.Join(dir, "b.txt"), []byte("b"), 0644)
	if err := execFileOp(t, dir, "move", "a.txt", "b.txt", false); err == nil {
		t.Fatal("expected error when destination exists")
	}
	if got := mustRead(t, filepath.Join(dir, "b.txt")); got != "b" {
		t.Fatalf("existing dest must not be overwritten, got %q", got)
	}
}

func TestFileOpCopyFile(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "a.txt")
	os.WriteFile(src, []byte("hello"), 0644)
	if err := execFileOp(t, dir, "copy", "a.txt", "sub/b.txt", false); err != nil {
		t.Fatalf("copy: %v", err)
	}
	if got := mustRead(t, src); got != "hello" {
		t.Fatal("source must be unchanged")
	}
	if got := mustRead(t, filepath.Join(dir, "sub", "b.txt")); got != "hello" {
		t.Fatalf("copy content = %q", got)
	}
}

func TestFileOpCopyDir(t *testing.T) {
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, "old", "nested"), 0755)
	os.WriteFile(filepath.Join(dir, "old", "nested", "f.txt"), []byte("x"), 0644)
	if err := execFileOp(t, dir, "copy", "old", "new", false); err != nil {
		t.Fatalf("copy dir: %v", err)
	}
	if got := mustRead(t, filepath.Join(dir, "new", "nested", "f.txt")); got != "x" {
		t.Fatalf("copied content = %q", got)
	}
	if got := mustRead(t, filepath.Join(dir, "old", "nested", "f.txt")); got != "x" {
		t.Fatal("source must be unchanged")
	}
}

func TestFileOpCopyDirRefusesSymlink(t *testing.T) {
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, "old"), 0755)
	os.WriteFile(filepath.Join(dir, "target.txt"), []byte("t"), 0644)
	if err := os.Symlink(filepath.Join(dir, "target.txt"), filepath.Join(dir, "old", "link.txt")); err != nil {
		t.Fatal(err)
	}
	if err := execFileOp(t, dir, "copy", "old", "new", false); err == nil {
		t.Fatal("expected error copying dir containing symlink")
	}
}

func TestFileOpDeleteFile(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "a.txt")
	os.WriteFile(p, []byte("hello"), 0644)
	if err := execFileOp(t, dir, "delete", "a.txt", "", false); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if _, err := os.Stat(p); !os.IsNotExist(err) {
		t.Fatal("file should be gone")
	}
}

func TestFileOpDeleteDirRequiresRecursive(t *testing.T) {
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, "d"), 0755)
	if err := execFileOp(t, dir, "delete", "d", "", false); err == nil {
		t.Fatal("expected error deleting directory without recursive")
	}
	if _, err := os.Stat(filepath.Join(dir, "d")); err != nil {
		t.Fatal("directory must remain")
	}
}

func TestFileOpDeleteDirRecursive(t *testing.T) {
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, "d", "nested"), 0755)
	os.WriteFile(filepath.Join(dir, "d", "nested", "f.txt"), []byte("x"), 0644)
	if err := execFileOp(t, dir, "delete", "d", "", true); err != nil {
		t.Fatalf("recursive delete: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "d")); !os.IsNotExist(err) {
		t.Fatal("directory should be gone")
	}
}

func TestFileOpDeleteRefusesSymlink(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "target.txt")
	os.WriteFile(target, []byte("t"), 0644)
	if err := os.Symlink(target, filepath.Join(dir, "link.txt")); err != nil {
		t.Fatal(err)
	}
	if err := execFileOp(t, dir, "delete", "link.txt", "", false); err == nil {
		t.Fatal("expected error deleting symlink")
	}
	if got := mustRead(t, target); got != "t" {
		t.Fatal("target must be unchanged")
	}
}

func TestFileOpEscapeProjectRoot(t *testing.T) {
	dir := t.TempDir()
	if err := execFileOp(t, dir, "move", "../outside.txt", "inside.txt", false); err == nil {
		t.Fatal("expected error for path outside project root")
	}
	if err := execFileOp(t, dir, "move", "a.txt", "../outside.txt", false); err == nil {
		t.Fatal("expected error for destination outside project root")
	}
	if err := execFileOp(t, dir, "delete", "../outside.txt", "", false); err == nil {
		t.Fatal("expected error for delete outside project root")
	}
}

func TestFileOpProjectRootRefused(t *testing.T) {
	dir := t.TempDir()
	if err := execFileOp(t, dir, "delete", ".", "", true); err == nil {
		t.Fatal("expected error operating on project root")
	}
	if err := execFileOp(t, dir, "move", ".", "x", false); err == nil {
		t.Fatal("expected error moving project root")
	}
}

func TestFileOpMissingSource(t *testing.T) {
	dir := t.TempDir()
	if err := execFileOp(t, dir, "move", "nope.txt", "x.txt", false); err == nil {
		t.Fatal("expected error for missing source")
	}
}

func TestFileOpUnknownAction(t *testing.T) {
	dir := t.TempDir()
	if err := execFileOp(t, dir, "fly", "a.txt", "b.txt", false); err == nil {
		t.Fatal("expected error for unknown action")
	}
}

func TestFileOpDescribe(t *testing.T) {
	h := &FileOp{}
	got := h.Describe(map[string]any{"action": "move", "path": "a.go", "destination": "b.go"})
	if got != "move a.go -> b.go" {
		t.Fatalf("describe move = %q", got)
	}
	got = h.Describe(map[string]any{"action": "delete", "path": "a.go"})
	if got != "delete a.go" {
		t.Fatalf("describe delete = %q", got)
	}
}
