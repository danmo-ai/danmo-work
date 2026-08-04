package service

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

func TestRunFirstLaunchExtractsViaScript(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("bash fixture")
	}
	home := t.TempDir()
	prevHome := firstLaunchHomeDir
	prevBin := codeGraphHomeBinDir
	firstLaunchHomeDir = func() string { return home }
	codeGraphHomeBinDir = func() string { return filepath.Join(home, "bin") }
	t.Cleanup(func() {
		firstLaunchHomeDir = prevHome
		codeGraphHomeBinDir = prevBin
		ResetFirstLaunchForTest()
	})
	t.Setenv(envFirstLaunchDisable, "")
	t.Setenv(envFirstLaunchScript, "")

	binDir := filepath.Join(home, "bin")
	flDir := filepath.Join(home, firstLaunchDirName)
	if err := os.MkdirAll(binDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(flDir, 0o755); err != nil {
		t.Fatal(err)
	}

	payload := []byte("CGfake-from-script")
	archive := filepath.Join(binDir, "codegraph.tar.gz")
	if err := writeTestCodeGraphTarGz(archive, "codegraph", payload); err != nil {
		t.Fatal(err)
	}

	script := filepath.Join(flDir, firstLaunchScriptSH)
	body := `#!/usr/bin/env bash
set -euo pipefail
BIN_DIR="$DANMO_HOME/bin"
tmp="$(mktemp -d)"
tar -xzf "$BIN_DIR/codegraph.tar.gz" -C "$tmp"
cp -f "$(find "$tmp" -type f -name codegraph | head -n 1)" "$BIN_DIR/codegraph"
chmod +x "$BIN_DIR/codegraph"
rm -rf "$tmp"
echo script-ok
`
	if err := os.WriteFile(script, []byte(body), 0o755); err != nil {
		t.Fatal(err)
	}

	if err := runFirstLaunch(); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(filepath.Join(binDir, "codegraph"))
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(payload) {
		t.Fatalf("payload mismatch")
	}
	stamp, err := os.ReadFile(filepath.Join(flDir, firstLaunchStampName))
	if err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(string(stamp)) == "" {
		t.Fatal("empty stamp")
	}

	// Second run should skip (stamp match) even if we delete the binary.
	_ = os.Remove(filepath.Join(binDir, "codegraph"))
	if err := runFirstLaunch(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(binDir, "codegraph")); err == nil {
		t.Fatal("should not re-extract when stamp matches")
	}
}

func TestStartFirstLaunchAsync(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("bash fixture")
	}
	home := t.TempDir()
	prevHome := firstLaunchHomeDir
	firstLaunchHomeDir = func() string { return home }
	ResetFirstLaunchForTest()
	t.Cleanup(func() {
		firstLaunchHomeDir = prevHome
		ResetFirstLaunchForTest()
	})
	t.Setenv(envFirstLaunchDisable, "")

	flDir := filepath.Join(home, firstLaunchDirName)
	if err := os.MkdirAll(flDir, 0o755); err != nil {
		t.Fatal(err)
	}
	script := filepath.Join(flDir, firstLaunchScriptSH)
	marker := filepath.Join(home, "async-marker")
	body := "#!/bin/sh\ntouch \"$DANMO_HOME/async-marker\"\n"
	if err := os.WriteFile(script, []byte(body), 0o755); err != nil {
		t.Fatal(err)
	}

	done := make(chan struct{})
	StartFirstLaunchAsync(func() { close(done) })
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("timeout waiting for async first-launch")
	}
	if _, err := os.Stat(marker); err != nil {
		t.Fatalf("marker missing: %v", err)
	}
}
