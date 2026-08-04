package service

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

func TestRunFirstLaunchStampSkip(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("bash fixture")
	}
	home := t.TempDir()
	prevHome := firstLaunchHomeDir
	firstLaunchHomeDir = func() string { return home }
	t.Cleanup(func() {
		firstLaunchHomeDir = prevHome
		ResetFirstLaunchForTest()
	})
	t.Setenv(envFirstLaunchDisable, "")
	t.Setenv(envFirstLaunchScript, "")

	flDir := filepath.Join(home, firstLaunchDirName)
	if err := os.MkdirAll(flDir, 0o755); err != nil {
		t.Fatal(err)
	}
	script := filepath.Join(flDir, firstLaunchScriptSH)
	marker := filepath.Join(home, "ran")
	body := "#!/bin/sh\ntouch \"$DANMO_HOME/ran\"\n"
	if err := os.WriteFile(script, []byte(body), 0o755); err != nil {
		t.Fatal(err)
	}

	if err := runFirstLaunch(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(marker); err != nil {
		t.Fatal(err)
	}
	stamp, err := os.ReadFile(filepath.Join(flDir, firstLaunchStampName))
	if err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(string(stamp)) == "" {
		t.Fatal("empty stamp")
	}
	_ = os.Remove(marker)
	if err := runFirstLaunch(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(marker); err == nil {
		t.Fatal("should not re-run when stamp matches")
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
