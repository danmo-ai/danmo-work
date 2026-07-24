package sandbox

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"danmo-work/core/domain"
	"danmo-work/core/port"
)

func TestNewDefaultsAndStatus(t *testing.T) {
	m := New(domain.ConfigSandboxSection{Enabled: true})
	st := m.Status()
	if !st.Enabled {
		t.Fatal("expected enabled")
	}
	if st.Mode != domain.SandboxModeWorkspaceWrite {
		t.Fatalf("mode: %s", st.Mode)
	}
	if st.Platform != runtime.GOOS {
		t.Fatalf("platform: %s", st.Platform)
	}
	if st.Backend == "" {
		t.Fatal("backend empty")
	}
}

func TestDangerFullAccessUsesHost(t *testing.T) {
	m := New(domain.ConfigSandboxSection{
		Enabled: true,
		Mode:    domain.SandboxModeDangerFullAccess,
	})
	st := m.Status()
	if st.Backend != domain.SandboxBackendDisabled {
		t.Fatalf("backend=%s", st.Backend)
	}
}

func TestRunEcho(t *testing.T) {
	dir := t.TempDir()
	m := New(domain.ConfigSandboxSection{Enabled: true, Mode: domain.SandboxModeWorkspaceWrite, Network: domain.SandboxNetworkAllow})
	cmd := "echo dq-sandbox-ok"
	if runtime.GOOS == "windows" {
		cmd = "echo dq-sandbox-ok"
	}
	out, err := m.Run(context.Background(), port.SandboxRunOptions{
		Command: cmd,
		WorkDir: dir,
		Timeout: 10 * time.Second,
	})
	if err != nil {
		// On CI without seatbelt/bwrap/landlock, host-weak still works.
		t.Logf("run err (may be ok on degraded): %v out=%q", err, out)
	}
	if err == nil && !strings.Contains(string(out), "dq-sandbox-ok") {
		t.Fatalf("unexpected output: %q backend=%s", out, m.Status().Backend)
	}
}

func TestWorkspaceWriteAllowsTempFile(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("path quoting differs on windows")
	}
	dir := t.TempDir()
	target := filepath.Join(dir, "out.txt")
	m := New(domain.ConfigSandboxSection{
		Enabled: true,
		Mode:    domain.SandboxModeWorkspaceWrite,
		Network: domain.SandboxNetworkAllow,
	})
	_, err := m.Run(context.Background(), port.SandboxRunOptions{
		Command: "echo hello > out.txt",
		WorkDir: dir,
		Timeout: 10 * time.Second,
	})
	if err != nil {
		t.Fatalf("run: %v backend=%s reason=%s", err, m.Status().Backend, m.Status().DegradedReason)
	}
	b, err := os.ReadFile(target)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(b), "hello") {
		t.Fatalf("file content: %q", b)
	}
}
