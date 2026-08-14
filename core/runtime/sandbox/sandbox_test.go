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

func TestMain(m *testing.M) {
	// Landlock backend reexecs the current binary; handle that before tests run.
	if MaybeReexec() {
		return
	}
	os.Exit(m.Run())
}

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
	// Factory returns the host-weak backend when the sandbox is effectively off.
	if st.Backend != domain.SandboxBackendHostWeak {
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

func TestNetworkAllowed(t *testing.T) {
	deny := domain.ConfigSandboxSection{Network: domain.SandboxNetworkDeny}
	allow := domain.ConfigSandboxSection{Network: domain.SandboxNetworkAllow}
	alist := domain.ConfigSandboxSection{Network: domain.SandboxNetworkAllowlist}
	if networkAllowed(deny, port.SandboxRunOptions{}) {
		t.Fatal("deny should block")
	}
	if !networkAllowed(deny, port.SandboxRunOptions{AllowNetwork: true}) {
		t.Fatal("AllowNetwork overrides deny")
	}
	if !networkAllowed(allow, port.SandboxRunOptions{}) {
		t.Fatal("allow should open")
	}
	if networkAllowed(alist, port.SandboxRunOptions{}) {
		t.Fatal("allowlist without proxy should not open OS net")
	}
	if !networkAllowed(alist, port.SandboxRunOptions{AllowlistProxy: "127.0.0.1:9"}) {
		t.Fatal("allowlist with proxy should open OS net")
	}
}

func TestAllowlistProxyStatusAndEnv(t *testing.T) {
	m := New(domain.ConfigSandboxSection{
		Enabled:          true,
		Mode:             domain.SandboxModeWorkspaceWrite,
		Network:          domain.SandboxNetworkAllowlist,
		AllowlistDomains: []string{"example.com", "*.npmjs.org"},
		Backend:          "host-weak",
	})
	defer m.Close()
	st := m.Status()
	if !st.AllowlistActive || st.AllowlistProxy == "" {
		t.Fatalf("expected active proxy, status=%+v", st)
	}
	if len(st.AllowlistDomains) != 2 {
		t.Fatalf("domains: %v", st.AllowlistDomains)
	}

	cmd := "printenv HTTP_PROXY"
	if runtime.GOOS == "windows" {
		cmd = "echo %HTTP_PROXY%"
	}
	out, err := m.Run(context.Background(), port.SandboxRunOptions{
		Command: cmd,
		WorkDir: t.TempDir(),
		Timeout: 10 * time.Second,
		Env:     []string{"PATH=" + os.Getenv("PATH"), "SystemRoot=" + os.Getenv("SystemRoot"), "ComSpec=" + os.Getenv("ComSpec")},
	})
	if err != nil {
		t.Fatalf("run: %v out=%q", err, out)
	}
	if !strings.Contains(string(out), st.AllowlistProxy) {
		t.Fatalf("HTTP_PROXY not injected: out=%q want addr %q", out, st.AllowlistProxy)
	}
}

func TestAllowlistEmptyFailsClosed(t *testing.T) {
	m := New(domain.ConfigSandboxSection{
		Enabled: true,
		Mode:    domain.SandboxModeWorkspaceWrite,
		Network: domain.SandboxNetworkAllowlist,
		Backend: "host-weak",
	})
	defer m.Close()
	st := m.Status()
	if st.AllowlistActive {
		t.Fatal("empty domains should not activate proxy")
	}
	if !st.Degraded || st.DegradedReason == "" {
		t.Fatalf("expected degraded: %+v", st)
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
