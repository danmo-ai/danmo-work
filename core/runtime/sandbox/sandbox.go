// Package sandbox provides OS-level process isolation for agent tool execution.
// Backends align with mainstream coding agents: Seatbelt (macOS), Landlock/seccomp
// with bubblewrap fallback (Linux), and restricted tokens (Windows).
package sandbox

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"sync"
	"time"

	"danmo-work/core/domain"
	"danmo-work/core/port"
)

const (
	defaultTimeout = 30 * time.Second
	// reexecArg triggers landlock apply-then-exec in the same binary (Linux).
	reexecArg = "__dq-sandbox-landlock"
)

// Manager selects and runs the best available sandbox backend.
type Manager struct {
	mu     sync.RWMutex
	cfg    domain.ConfigSandboxSection
	status domain.SandboxStatus
	runner runner
}

type runner interface {
	name() domain.SandboxBackend
	run(ctx context.Context, opts port.SandboxRunOptions, cfg domain.ConfigSandboxSection) ([]byte, error)
}

// New probes the host and returns a Manager. cfg may be partially filled;
// missing fields get safe defaults (enabled, workspace-write, network deny).
func New(cfg domain.ConfigSandboxSection) *Manager {
	cfg = normalizeConfig(cfg)
	m := &Manager{cfg: cfg}
	m.reprobe()
	return m
}

func normalizeConfig(cfg domain.ConfigSandboxSection) domain.ConfigSandboxSection {
	if cfg.Mode == "" {
		cfg.Mode = domain.SandboxModeWorkspaceWrite
	}
	if cfg.Network == "" {
		cfg.Network = domain.SandboxNetworkDeny
	}
	cfg.Shell = normalizeShellPref(cfg.Shell)
	return cfg
}

// Configure replaces sandbox policy and re-probes the backend.
func (m *Manager) Configure(cfg domain.ConfigSandboxSection) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.cfg = normalizeConfig(cfg)
	m.reprobeLocked()
}

// Status returns the current probed sandbox status.
func (m *Manager) Status() domain.SandboxStatus {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.status
}

// Run executes a shell command under the selected sandbox backend.
func (m *Manager) Run(ctx context.Context, opts port.SandboxRunOptions) ([]byte, error) {
	if opts.Command == "" {
		return nil, fmt.Errorf("sandbox: command is required")
	}
	if opts.Timeout <= 0 {
		opts.Timeout = defaultTimeout
	}
	if opts.Env == nil {
		opts.Env = filterEnv(os.Environ())
	}

	m.mu.RLock()
	cfg := m.cfg
	r := m.runner
	status := m.status
	m.mu.RUnlock()

	if !cfg.Enabled || cfg.Mode == domain.SandboxModeDangerFullAccess || status.Backend == domain.SandboxBackendDisabled {
		return runHost(ctx, opts, cfg, status.Backend)
	}
	if r == nil {
		return runHost(ctx, opts, cfg, status.Backend)
	}
	return r.run(ctx, opts, cfg)
}

func (m *Manager) reprobe() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.reprobeLocked()
}

func (m *Manager) reprobeLocked() {
	cfg := m.cfg
	st := domain.SandboxStatus{
		Enabled:  cfg.Enabled,
		Mode:     cfg.Mode,
		Network:  cfg.Network,
		Platform: runtime.GOOS,
	}

	if !cfg.Enabled {
		st.Backend = domain.SandboxBackendDisabled
		st.Capabilities = []string{"host"}
		applyShellStatus(&st, resolveShell(cfg, st.Backend))
		m.status = st
		m.runner = hostRunner{}
		return
	}
	if cfg.Mode == domain.SandboxModeDangerFullAccess {
		st.Backend = domain.SandboxBackendDisabled
		st.Capabilities = []string{"full-access"}
		applyShellStatus(&st, resolveShell(cfg, st.Backend))
		m.status = st
		m.runner = hostRunner{}
		return
	}

	backend, r, degraded, reason, caps := selectBackend(cfg)
	st.Backend = backend
	st.Degraded = degraded
	st.DegradedReason = reason
	st.Capabilities = caps
	applyShellStatus(&st, resolveShell(cfg, backend))
	m.status = st
	m.runner = r
}

// selectBackend is implemented per-OS in sandbox_*.go files.

func filterEnv(environ []string) []string {
	// Drop secrets that should not leak into sandboxed shells by default.
	denyPrefix := []string{
		"AWS_SECRET", "AWS_ACCESS_KEY", "OPENAI_API_KEY", "ANTHROPIC_API_KEY",
		"WORK_LLM", "GITHUB_TOKEN", "GH_TOKEN", "NPM_TOKEN",
	}
	out := make([]string, 0, len(environ))
	for _, e := range environ {
		key, _, _ := strings.Cut(e, "=")
		upper := strings.ToUpper(key)
		skip := false
		for _, p := range denyPrefix {
			if strings.HasPrefix(upper, p) || upper == p {
				skip = true
				break
			}
		}
		if skip {
			continue
		}
		out = append(out, e)
	}
	return out
}

type hostRunner struct{}

func (hostRunner) name() domain.SandboxBackend { return domain.SandboxBackendHostWeak }

func (hostRunner) run(ctx context.Context, opts port.SandboxRunOptions, cfg domain.ConfigSandboxSection) ([]byte, error) {
	return runHost(ctx, opts, cfg, domain.SandboxBackendHostWeak)
}

func runHost(ctx context.Context, opts port.SandboxRunOptions, cfg domain.ConfigSandboxSection, backend domain.SandboxBackend) ([]byte, error) {
	ctx, cancel := context.WithTimeout(ctx, opts.Timeout)
	defer cancel()
	sh := resolveShell(cfg, backend)
	cmd, err := shellCommandFor(ctx, opts.Command, sh)
	if err != nil {
		return nil, err
	}
	cmd.Dir = opts.WorkDir
	cmd.Env = opts.Env
	if sh.kind == "cmd" {
		cmd.Env = prependCoreutilsPATH(cmd.Env)
	}
	out, err := cmd.CombinedOutput()
	if ctx.Err() == context.DeadlineExceeded {
		return out, fmt.Errorf("sandbox: command timed out after %s", opts.Timeout)
	}
	return out, err
}

func lookPath(name string) bool {
	_, err := exec.LookPath(name)
	return err == nil
}

func networkAllowed(cfg domain.ConfigSandboxSection, opts port.SandboxRunOptions) bool {
	if opts.AllowNetwork {
		return true
	}
	return cfg.Network == domain.SandboxNetworkAllow || cfg.Network == domain.SandboxNetworkAllowlist
}
