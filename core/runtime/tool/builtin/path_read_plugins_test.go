package builtin

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestResolveReadPathAllowsWorkHomePlugins(t *testing.T) {
	home := t.TempDir()
	t.Setenv("WORK_HOME", home)

	plugins := filepath.Join(home, "plugins", "danmo-novel", "skills", "danmo-novel")
	if err := os.MkdirAll(plugins, 0o755); err != nil {
		t.Fatal(err)
	}
	gate := filepath.Join(plugins, "SKILL.md")
	if err := os.WriteFile(gate, []byte("---\nname: danmo-novel\n---\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	work := t.TempDir()
	if err := os.WriteFile(filepath.Join(work, "in-project.txt"), []byte("x\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	got, err := resolveReadPath(work, gate)
	if err != nil {
		t.Fatalf("absolute plugin path: %v", err)
	}
	if got != filepath.Clean(gate) {
		t.Fatalf("got %q want %q", got, gate)
	}

	got, err = resolveReadPath(work, "$WORK_HOME/plugins/danmo-novel/skills/danmo-novel/SKILL.md")
	if err != nil {
		t.Fatalf("$WORK_HOME plugin path: %v", err)
	}
	if got != filepath.Clean(gate) {
		t.Fatalf("$WORK_HOME expand: got %q want %q", got, gate)
	}

	got, err = resolveReadPath(work, "in-project.txt")
	if err != nil {
		t.Fatalf("project path: %v", err)
	}
	if got != filepath.Join(work, "in-project.txt") {
		t.Fatalf("project resolve: got %q", got)
	}

	if _, err := resolveReadPath(work, filepath.Join(home, "config.yaml")); err == nil {
		t.Fatal("expected rejection for non-plugins WORK_HOME path")
	}

	if _, err := resolvePath(work, gate); err == nil {
		t.Fatal("mutating resolvePath must still reject plugin paths")
	}
	if _, err := resolveWritePath(work, gate); err == nil {
		t.Fatal("resolveWritePath must still reject plugin paths")
	}
}

func TestResolveReadPathRejectsOutside(t *testing.T) {
	home := t.TempDir()
	t.Setenv("WORK_HOME", home)
	work := t.TempDir()
	outside := t.TempDir()

	_, err := resolveReadPath(work, filepath.Join(outside, "secret.txt"))
	if err == nil {
		t.Fatal("expected outside path rejection")
	}
	if !strings.Contains(err.Error(), "outside project directory") {
		t.Fatalf("unexpected error: %v", err)
	}
}
