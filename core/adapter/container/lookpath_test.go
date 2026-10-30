package container

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestLookCLIFallsBackToAbsoluteCandidate(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("unix executable-bit check")
	}
	dir := t.TempDir()
	// Unique name so EnsureHostPATH + LookPath cannot resolve a real docker.
	name := "danmo-test-container-cli"
	bin := filepath.Join(dir, name)
	if err := os.WriteFile(bin, []byte("#!/bin/sh\necho ok\n"), 0o755); err != nil {
		t.Fatal(err)
	}

	old := cliCandidatePaths
	cliCandidatePaths = func(n string) []string {
		if n == name {
			return []string{bin}
		}
		return nil
	}
	t.Cleanup(func() { cliCandidatePaths = old })

	// Simulate macOS GUI PATH (no Homebrew /usr/local).
	t.Setenv("PATH", "/usr/bin"+string(os.PathListSeparator)+"/bin")

	got, err := lookCLI(name)
	if err != nil {
		t.Fatalf("lookCLI: %v", err)
	}
	if got != bin {
		t.Fatalf("got %q want %q", got, bin)
	}
}

func TestLookCLIMissing(t *testing.T) {
	name := "danmo-test-container-cli-missing"
	old := cliCandidatePaths
	cliCandidatePaths = func(string) []string {
		return []string{filepath.Join(t.TempDir(), name)}
	}
	t.Cleanup(func() { cliCandidatePaths = old })
	t.Setenv("PATH", "/usr/bin"+string(os.PathListSeparator)+"/bin")

	if _, err := lookCLI(name); err == nil {
		t.Fatal("expected error when binary is absent")
	}
}

func TestIsExecutableFile(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("unix modes")
	}
	dir := t.TempDir()
	execPath := filepath.Join(dir, "x")
	plain := filepath.Join(dir, "y")
	if err := os.WriteFile(execPath, []byte("x"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(plain, []byte("y"), 0o644); err != nil {
		t.Fatal(err)
	}
	if !isExecutableFile(execPath) {
		t.Fatal("expected executable")
	}
	if isExecutableFile(plain) {
		t.Fatal("plain file should not count as executable")
	}
	if isExecutableFile(dir) {
		t.Fatal("directory should not count")
	}
}
