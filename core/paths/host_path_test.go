package paths

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestMergeHostPATHPrependsExistingMissingDirs(t *testing.T) {
	tmp := t.TempDir()
	a := filepath.Join(tmp, "a")
	b := filepath.Join(tmp, "b")
	missing := filepath.Join(tmp, "missing")
	if err := os.MkdirAll(a, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(b, 0o755); err != nil {
		t.Fatal(err)
	}

	cur := "/usr/bin" + string(os.PathListSeparator) + b
	got, ok := mergeHostPATH(cur, []string{a, b, missing})
	if !ok {
		t.Fatal("expected PATH change")
	}
	wantPrefix := a + string(os.PathListSeparator)
	if !strings.HasPrefix(got, wantPrefix) {
		t.Fatalf("PATH=%q want prefix %q", got, wantPrefix)
	}
	if strings.Count(got, b) != 1 {
		t.Fatalf("expected %q once in PATH %q", b, got)
	}
	if strings.Contains(got, missing) {
		t.Fatalf("missing dir should not be prepended: %q", got)
	}
}

func TestMergeHostPATHNoChangeWhenAlreadyPresent(t *testing.T) {
	tmp := t.TempDir()
	a := filepath.Join(tmp, "a")
	if err := os.MkdirAll(a, 0o755); err != nil {
		t.Fatal(err)
	}
	cur := a + string(os.PathListSeparator) + "/usr/bin"
	got, ok := mergeHostPATH(cur, []string{a})
	if ok {
		t.Fatalf("unexpected change: %q", got)
	}
	if got != cur {
		t.Fatalf("got %q want %q", got, cur)
	}
}

func TestHostExtraBinDirsIncludesDarwinDefaults(t *testing.T) {
	if runtime.GOOS != "darwin" && runtime.GOOS != "linux" {
		t.Skip("host extra bins are for unix desktop PATH gaps")
	}
	dirs := hostExtraBinDirs()
	if len(dirs) == 0 {
		t.Fatal("expected extra bin dirs")
	}
	joined := strings.Join(dirs, "\n")
	if runtime.GOOS == "darwin" {
		for _, want := range []string{"/opt/homebrew/bin", "/usr/local/bin", "Docker.app"} {
			if !strings.Contains(joined, want) {
				t.Fatalf("darwin dirs missing %q in %#v", want, dirs)
			}
		}
	}
	if !strings.Contains(joined, DirName+string(os.PathSeparator)+"bin") &&
		!strings.Contains(joined, filepath.Join(DirName, "bin")) {
		// home may be empty in weird envs; still require a danmo bin candidate when home works
		home, err := os.UserHomeDir()
		if err == nil && home != "" {
			t.Fatalf("expected ~/.danmo-work/bin in %#v", dirs)
		}
	}
}
