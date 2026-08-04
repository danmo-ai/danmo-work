package service

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

func writeTestCodeGraphTarGz(path, name string, payload []byte) error {
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	hdr := &tar.Header{
		Name: name,
		Mode: 0o755,
		Size: int64(len(payload)),
	}
	if err := tw.WriteHeader(hdr); err != nil {
		return err
	}
	if _, err := tw.Write(payload); err != nil {
		return err
	}
	if err := tw.Close(); err != nil {
		return err
	}
	if err := gz.Close(); err != nil {
		return err
	}
	return os.WriteFile(path, buf.Bytes(), 0o644)
}

func TestResolveCodeGraphBinEnvOverride(t *testing.T) {
	dir := t.TempDir()
	name := "codegraph"
	if runtime.GOOS == "windows" {
		name = "codegraph.exe"
	}
	bin := filepath.Join(dir, name)
	if err := os.WriteFile(bin, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("WORK_CODEGRAPH_BIN", bin)
	got := ResolveCodeGraphBin()
	if got != bin {
		t.Fatalf("got %q want %q", got, bin)
	}
}

func TestEnsureCodeGraphIndexMissingBinary(t *testing.T) {
	empty := t.TempDir()
	prev := codeGraphHomeBinDir
	codeGraphHomeBinDir = func() string { return empty }
	t.Cleanup(func() { codeGraphHomeBinDir = prev })
	t.Setenv("WORK_CODEGRAPH_BIN", "")
	t.Setenv("PATH", empty)

	dir := t.TempDir()
	st := EnsureCodeGraphIndex(dir)
	if st != CodeGraphFailed {
		t.Fatalf("state=%s want failed", st)
	}
	if codeGraphIndexReady(dir) {
		t.Fatal("should not create .codegraph without binary")
	}
}

func TestResolveCodeGraphBinExtractsArchive(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("tar.gz fixture")
	}
	home := t.TempDir()
	prev := codeGraphHomeBinDir
	codeGraphHomeBinDir = func() string { return home }
	t.Cleanup(func() { codeGraphHomeBinDir = prev })
	t.Setenv("WORK_CODEGRAPH_BIN", "")
	t.Setenv("PATH", home)

	// Not a shebang — real Mach-O/ELF also won't start with #!.
	payload := []byte("CGfake-binary-payload")
	archive := filepath.Join(home, "codegraph.tar.gz")
	if err := writeTestCodeGraphTarGz(archive, "codegraph", payload); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(home, "CODEGRAPH_VERSION"), []byte("0.0.0-test\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	got := ResolveCodeGraphBin()
	want := filepath.Join(home, "codegraph")
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
	b, err := os.ReadFile(want)
	if err != nil {
		t.Fatal(err)
	}
	if string(b) != string(payload) {
		t.Fatalf("extracted contents mismatch")
	}
	// Second resolve should reuse the unpacked binary.
	if again := ResolveCodeGraphBin(); again != want {
		t.Fatalf("second resolve %q want %q", again, want)
	}
}

func TestEnsureCodeGraphIndexReadyWhenPresent(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, ".codegraph"), 0o755); err != nil {
		t.Fatal(err)
	}
	st := EnsureCodeGraphIndex(dir)
	if st != CodeGraphReady {
		t.Fatalf("state=%s want ready", st)
	}
}

func TestEnsureCodeGraphIndexSingleFlight(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell stub")
	}
	dir := t.TempDir()
	script := filepath.Join(dir, "fake-codegraph")
	body := "#!/bin/sh\n# args: init <workdir>\nmkdir -p \"$2/.codegraph\"\nsleep 0.2\n"
	if err := os.WriteFile(script, []byte(body), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("WORK_CODEGRAPH_BIN", script)

	work := filepath.Join(dir, "proj")
	if err := os.MkdirAll(work, 0o755); err != nil {
		t.Fatal(err)
	}

	// Reset job map entry for this path from prior tests in-process.
	codeGraphMu.Lock()
	delete(codeGraphJobs, filepath.Clean(work))
	codeGraphMu.Unlock()

	st1 := EnsureCodeGraphIndex(work)
	st2 := EnsureCodeGraphIndex(work)
	if st1 != CodeGraphIndexing {
		t.Fatalf("first=%s want indexing", st1)
	}
	if st2 != CodeGraphIndexing {
		t.Fatalf("second=%s want indexing (single-flight)", st2)
	}

	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if CodeGraphIndexStatus(work) == CodeGraphReady {
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for ready; status=%s", CodeGraphIndexStatus(work))
}

func TestCodeGraphIndexHint(t *testing.T) {
	h := CodeGraphIndexHint(CodeGraphIndexing, "/tmp/p")
	if !strings.Contains(h, "indexing") || !strings.Contains(h, "degrade") {
		t.Fatalf("hint=%q", h)
	}
}

func TestCodeGraphMCPEnv(t *testing.T) {
	env := CodeGraphMCPEnv()
	if !strings.Contains(env, "CODEGRAPH_MCP_TOOLS=") {
		t.Fatalf("env=%q", env)
	}
	for _, tool := range []string{"explore", "impact", "callers", "status"} {
		if !strings.Contains(env, tool) {
			t.Fatalf("env missing %q: %q", tool, env)
		}
	}
}
