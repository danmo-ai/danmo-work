package container

import (
	"os"
	"path/filepath"
	"testing"
)

func TestResolveTarPathOverride(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "danmo-work-env.tar")
	if err := os.WriteFile(p, []byte("fake"), 0o644); err != nil {
		t.Fatal(err)
	}
	got := ResolveTarPath(p)
	if got != p {
		t.Fatalf("got %q want %q", got, p)
	}
}

func TestResolveTarPathWorkEnv(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "custom.tar")
	if err := os.WriteFile(p, []byte("fake"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("WORK_ENV_TAR", p)
	got := ResolveTarPath("")
	if got != p {
		t.Fatalf("got %q want %q", got, p)
	}
}

func TestResolveTarPathMissing(t *testing.T) {
	t.Setenv("WORK_ENV_TAR", "")
	got := ResolveTarPath(filepath.Join(t.TempDir(), "nope.tar"))
	if got != "" {
		t.Fatalf("expected empty, got %q", got)
	}
}
