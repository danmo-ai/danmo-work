package builtin

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestResolveRipgrepBinEnvPrecedence(t *testing.T) {
	dir := t.TempDir()
	custom := filepath.Join(dir, "rg")
	if err := os.WriteFile(custom, []byte("x"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("WORK_RIPGREP_BIN", custom)
	if got := ResolveRipgrepBin(); got != custom {
		t.Fatalf("expected WORK_RIPGREP_BIN to win, got %q", got)
	}
	t.Setenv("WORK_RIPGREP_BIN", filepath.Join(dir, "missing"))
	if got := ResolveRipgrepBin(); got == custom {
		t.Fatal("missing WORK_RIPGREP_BIN should not resolve")
	}
}

func TestRunRipgrepIntegration(t *testing.T) {
	bin := ResolveRipgrepBin()
	if bin == "" {
		t.Skip("ripgrep not available")
	}
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "src"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "src", "app.go"), []byte("package main\nfunc Hello() {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "src", "skip.ts"), []byte("function Hello() {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, ".gitignore"), []byte("src/skip.ts\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	results, count, err := runRipgrep(context.Background(), bin, grepOpts{
		pattern:       "Hello",
		root:          dir,
		include:       "*.go",
		maxResults:    100,
		respectIgnore: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("expected 1 match, got %d (%+v)", count, results)
	}
	if !strings.HasSuffix(results[0].File, "app.go") {
		t.Fatalf("unexpected match file: %s", results[0].File)
	}

	// Ignored files appear when respectIgnore=false.
	results, count, err = runRipgrep(context.Background(), bin, grepOpts{
		pattern:       "Hello",
		root:          dir,
		maxResults:    100,
		respectIgnore: false,
	})
	if err != nil {
		t.Fatal(err)
	}
	if count != 2 {
		t.Fatalf("expected 2 matches with ignore disabled, got %d", count)
	}
}

func TestRunRipgrepContextLines(t *testing.T) {
	bin := ResolveRipgrepBin()
	if bin == "" {
		t.Skip("ripgrep not available")
	}
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "test.txt"), []byte("line1\nline2\nline3 - match\nline4\nline5\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	results, count, err := runRipgrep(context.Background(), bin, grepOpts{
		pattern:       "match",
		root:          dir,
		contextLines:  1,
		maxResults:    100,
		respectIgnore: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if count != 1 || len(results) != 1 {
		t.Fatalf("expected 1 match, got %d (%+v)", count, results)
	}
	joined := strings.Join(results[0].Context, " ")
	if !strings.Contains(joined, "line2") || !strings.Contains(joined, "line4") {
		t.Fatalf("expected context lines, got: %v", results[0].Context)
	}
}

func TestRunRipgrepMaxCap(t *testing.T) {
	bin := ResolveRipgrepBin()
	if bin == "" {
		t.Skip("ripgrep not available")
	}
	dir := t.TempDir()
	var b strings.Builder
	for i := 0; i < 50; i++ {
		b.WriteString("needle line\n")
	}
	if err := os.WriteFile(filepath.Join(dir, "many.txt"), []byte(b.String()), 0o644); err != nil {
		t.Fatal(err)
	}
	results, count, err := runRipgrep(context.Background(), bin, grepOpts{
		pattern:       "needle",
		root:          dir,
		maxResults:    5,
		respectIgnore: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if count > 5 || len(results) > 5 {
		t.Fatalf("cap exceeded: count=%d len=%d", count, len(results))
	}
	if count < 5 {
		t.Fatalf("expected at least 5 matches, got %d", count)
	}
}

func TestGitignoreRules(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, ".gitignore"), []byte("# comment\nbuild/\n*.log\n/top.txt\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	r := loadGitignore(dir)
	if r == nil {
		t.Fatal("expected rules")
	}
	if !r.ignoresDir(filepath.Join(dir, "build")) {
		t.Error("build/ should be ignored as dir")
	}
	if !r.ignoresFile(filepath.Join(dir, "a", "b", "x.log")) {
		t.Error("*.log should match at any depth")
	}
	if !r.ignoresFile(filepath.Join(dir, "top.txt")) {
		t.Error("/top.txt should match at root")
	}
	if r.ignoresFile(filepath.Join(dir, "sub", "top.txt")) {
		t.Error("/top.txt should NOT match in subdirectories")
	}
	if r.ignoresFile(filepath.Join(dir, "keep.go")) {
		t.Error("keep.go should not be ignored")
	}
}
