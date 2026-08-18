//go:build !windows

package builtin

import (
	"os"
	"path/filepath"
	"testing"
)

// TestResolveWritePathBlocksSymlinkEscape guards mutating tools against
// symlinks inside the project that point outside it — the lexical containment
// check in resolvePath alone does not catch those.
func TestResolveWritePathBlocksSymlinkEscape(t *testing.T) {
	work := t.TempDir()
	outside := t.TempDir()

	if err := os.Symlink(outside, filepath.Join(work, "escape")); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}

	// Direct symlink target and a file beneath it must both be rejected.
	if _, err := resolveWritePath(work, "escape"); err == nil {
		t.Fatal("write to symlink pointing outside project should be rejected")
	}
	if _, err := resolveWritePath(work, "escape/inner.txt"); err == nil {
		t.Fatal("write beneath escaping symlink should be rejected")
	}

	// Normal paths inside the project still resolve, including not-yet-existing ones.
	if _, err := resolveWritePath(work, "sub/new.txt"); err != nil {
		t.Fatalf("plain in-project write path should resolve, got %v", err)
	}

	// A symlink pointing inside the project stays allowed.
	if err := os.MkdirAll(filepath.Join(work, "real"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(work, "real"), filepath.Join(work, "alias")); err != nil {
		t.Fatal(err)
	}
	if _, err := resolveWritePath(work, "alias/file.txt"); err != nil {
		t.Fatalf("in-project symlink should be allowed, got %v", err)
	}
}
