package sandbox

import (
	"path/filepath"
	"runtime"
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

func TestEnsureUTF8Env(t *testing.T) {
	got := ensureUTF8Env([]string{"PATH=/bin", "PYTHONIOENCODING=ascii"})
	m := map[string]string{}
	for _, e := range got {
		k, v, ok := strings.Cut(e, "=")
		if ok {
			m[k] = v
		}
	}
	if m["PYTHONUTF8"] != "1" {
		t.Fatalf("PYTHONUTF8=%q", m["PYTHONUTF8"])
	}
	if m["PYTHONIOENCODING"] != "utf-8" {
		t.Fatalf("PYTHONIOENCODING=%q want utf-8 (override)", m["PYTHONIOENCODING"])
	}
	if runtime.GOOS != "windows" {
		if m["LC_ALL"] != "C.UTF-8" || m["LANG"] != "C.UTF-8" {
			t.Fatalf("locale env=%v", m)
		}
	}
	// Preserve existing LANG
	got2 := ensureUTF8Env([]string{"LANG=en_US.UTF-8"})
	m2 := map[string]string{}
	for _, e := range got2 {
		k, v, ok := strings.Cut(e, "=")
		if ok {
			m2[k] = v
		}
	}
	if runtime.GOOS != "windows" && m2["LANG"] != "en_US.UTF-8" {
		t.Fatalf("LANG overwritten: %v", m2)
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
