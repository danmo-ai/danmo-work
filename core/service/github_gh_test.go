package service

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestResolveGhBinEnvOverride(t *testing.T) {
	dir := t.TempDir()
	name := "gh"
	if runtime.GOOS == "windows" {
		name = "gh.exe"
	}
	bin := filepath.Join(dir, name)
	if err := os.WriteFile(bin, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("WORK_GH_BIN", bin)
	got := ResolveGhBin()
	if got != bin {
		t.Fatalf("ResolveGhBin=%q want %q", got, bin)
	}
}

func TestResolveGhBinHomeDir(t *testing.T) {
	dir := t.TempDir()
	prev := ghHomeBinDir
	ghHomeBinDir = func() string { return dir }
	t.Cleanup(func() { ghHomeBinDir = prev })
	t.Setenv("WORK_GH_BIN", "")

	bin := filepath.Join(dir, ghExecutableName())
	if err := os.WriteFile(bin, []byte("x"), 0o755); err != nil {
		t.Fatal(err)
	}
	got := ResolveGhBin()
	if got != bin {
		t.Fatalf("ResolveGhBin=%q want home %q", got, bin)
	}
}

func TestGitHubGhHint(t *testing.T) {
	missing := GitHubGhHint("")
	if !strings.Contains(missing, "missing") || !strings.Contains(missing, "gh") {
		t.Fatalf("missing hint: %s", missing)
	}
	ready := GitHubGhHint("/usr/bin/gh")
	if !strings.Contains(ready, "ready") || !strings.Contains(ready, "/usr/bin/gh") {
		t.Fatalf("ready hint: %s", ready)
	}
}
