package sandbox

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPrependPathEnv(t *testing.T) {
	env := []string{"FOO=1", "PATH=/usr/bin", "BAR=2"}
	got := prependPathEnv(env, "/opt/coreutils/bin")
	wantPrefix := "PATH=/opt/coreutils/bin" + string(os.PathListSeparator) + "/usr/bin"
	found := false
	for _, e := range got {
		if e == wantPrefix {
			found = true
		}
		if strings.HasPrefix(e, "PATH=") && e != wantPrefix {
			t.Fatalf("unexpected PATH entry %q", e)
		}
	}
	if !found {
		t.Fatalf("missing prepended PATH in %#v", got)
	}
}

func TestPrependPathEnvIdempotent(t *testing.T) {
	dir := filepath.Clean("/opt/coreutils/bin")
	env := []string{"PATH=" + dir + string(os.PathListSeparator) + "/usr/bin"}
	got := prependPathEnv(env, dir)
	if len(got) != 1 || got[0] != env[0] {
		t.Fatalf("got %#v", got)
	}
}

func TestEnsureCoreutilsHardlinks(t *testing.T) {
	root := t.TempDir()
	exe := filepath.Join(root, "coreutils.exe")
	payload := []byte("fake-coreutils-binary")
	if err := os.WriteFile(exe, payload, 0o755); err != nil {
		t.Fatal(err)
	}
	binDir := filepath.Join(root, "bin")
	if err := ensureCoreutilsHardlinks(exe, binDir); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"ls.exe", "cat.exe", "grep.exe", "find.exe"} {
		p := filepath.Join(binDir, name)
		b, err := os.ReadFile(p)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if string(b) != string(payload) {
			t.Fatalf("%s content mismatch", name)
		}
	}
	if !isCoreutilsBinDir(binDir) {
		t.Fatal("expected isCoreutilsBinDir")
	}
}

func TestPrepareCoreutilsFromExe(t *testing.T) {
	srcRoot := t.TempDir()
	src := filepath.Join(srcRoot, "coreutils.exe")
	if err := os.WriteFile(src, []byte("payload"), 0o755); err != nil {
		t.Fatal(err)
	}
	home := t.TempDir()
	t.Setenv("HOME", home)
	// UserHomeDir on Windows uses USERPROFILE; set both.
	t.Setenv("USERPROFILE", home)

	bin, err := prepareCoreutilsFromExe(src)
	if err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(home, ".danmo-work", "bin", "coreutils", "bin")
	if bin != want {
		t.Fatalf("bin=%q want %q", bin, want)
	}
	if !fileExists(filepath.Join(want, "ls.exe")) {
		t.Fatal("missing ls.exe hardlink")
	}
}
