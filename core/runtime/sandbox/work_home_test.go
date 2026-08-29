package sandbox

import (
	"path/filepath"
	"strings"
	"testing"

	"danmo-work/core/paths"
)

func TestEnsureWorkHomeEnv(t *testing.T) {
	root := t.TempDir()
	t.Setenv("WORK_HOME", root)
	home, err := filepath.Abs(paths.Home())
	if err != nil {
		t.Fatal(err)
	}

	got := ensureWorkHomeEnv([]string{"PATH=/bin", "WORK_HOME=/old"})
	var saw string
	for _, e := range got {
		if strings.HasPrefix(e, "WORK_HOME=") {
			if saw != "" {
				t.Fatalf("duplicate WORK_HOME: %v", got)
			}
			saw = strings.TrimPrefix(e, "WORK_HOME=")
		}
	}
	if saw != home {
		t.Fatalf("WORK_HOME=%q want %q env=%v", saw, home, got)
	}
}

func TestWorkHomeBindSkipsWorkspaceRoot(t *testing.T) {
	root := t.TempDir()
	t.Setenv("WORK_HOME", root)
	home, err := filepath.Abs(paths.Home())
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := workHomeBind(home); ok {
		t.Fatal("must not remount WORK_HOME when it is the workdir")
	}
	bind, ok := workHomeBind(t.TempDir())
	if !ok || bind.Host != home || bind.Container != home || !bind.ReadOnly {
		t.Fatalf("bind=%+v ok=%v want ro %s", bind, ok, home)
	}
}
