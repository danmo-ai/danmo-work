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
	"danmo-work/core/runtime/egress"
	"danmo-work/core/runtime/sandbox/netproxy"
)

const (
	defaultTimeout = 30 * time.Second
	// reexecArg triggers landlock apply-then-exec in the same binary (Linux).
	reexecArg = "__dq-sandbox-landlock"
)

// Manager selects and runs the best available sandbox backend.
type Manager struct {
	mu             sync.RWMutex
	cfg            domain.ConfigSandboxSection
	status         domain.SandboxStatus
	runner         runner
	proxy          *netproxy.Server
	sessionDomains map[string][]string // sessionID → Hard grants (scope=session)
	turnDomains    map[string][]string // turnID → once-scope Hard grants
}

type runner interface {
	name() domain.SandboxBackend
	run(ctx context.Context, opts port.SandboxRunOptions, cfg domain.ConfigSandboxSection) ([]byte, error)
}

// New probes the host and returns a Manager. cfg may be partially filled;
// missing fields get safe defaults (enabled, workspace-write, network deny).
func New(cfg domain.ConfigSandboxSection) *Manager {
	cfg = normalizeConfig(cfg)
	m := &Manager{
		cfg:            cfg,
		sessionDomains: make(map[string][]string),
		turnDomains:    make(map[string][]string),
	}
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
	cfg.AllowlistDomains = netproxy.NormalizeDomains(cfg.AllowlistDomains)
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

// Close stops the allowlist proxy if running.
func (m *Manager) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.stopProxyLocked()
}

// GrantSessionDomains merges hosts into the Hard allowlist for one session.
func (m *Manager) GrantSessionDomains(sessionID string, domains []string) {
	sessionID = strings.TrimSpace(sessionID)
	domains = netproxy.NormalizeDomains(domains)
	if sessionID == "" || len(domains) == 0 {
		return
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	cur := append([]string{}, m.sessionDomains[sessionID]...)
	m.sessionDomains[sessionID] = netproxy.NormalizeDomains(append(cur, domains...))
	m.refreshAllowlistLocked()
}

// RevokeSessionDomains drops Hard grants for a session.
func (m *Manager) RevokeSessionDomains(sessionID string) {
	sessionID = strings.TrimSpace(sessionID)
	if sessionID == "" {
		return
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.sessionDomains[sessionID]; !ok {
		return
	}
	delete(m.sessionDomains, sessionID)
	m.refreshAllowlistLocked()
}

// GrantTurnDomains merges once-scope domains for the active turn.
func (m *Manager) GrantTurnDomains(turnID string, domains []string) {
	turnID = strings.TrimSpace(turnID)
	domains = netproxy.NormalizeDomains(domains)
	if turnID == "" || len(domains) == 0 {
		return
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	cur := append([]string{}, m.turnDomains[turnID]...)
	m.turnDomains[turnID] = netproxy.NormalizeDomains(append(cur, domains...))
	m.refreshAllowlistLocked()
}

// ClearTurnDomains drops once-scope domains when the turn ends.
func (m *Manager) ClearTurnDomains(turnID string) {
	turnID = strings.TrimSpace(turnID)
	if turnID == "" {
		return
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.turnDomains[turnID]; !ok {
		return
	}
	delete(m.turnDomains, turnID)
	m.refreshAllowlistLocked()
}

func (m *Manager) refreshAllowlistLocked() {
	if m.cfg.Network != domain.SandboxNetworkAllowlist {
		return
	}
	_, _ = m.syncProxyLocked(m.cfg)
	if m.proxy != nil {
		m.proxy.UpdateDomains(m.effectiveDomainsLocked())
	}
	m.status.AllowlistDomains = m.effectiveDomainsLocked()
	if m.proxy != nil {
		m.status.AllowlistActive = true
		m.status.AllowlistProxy = m.proxy.Addr()
	}
}

func (m *Manager) effectiveDomainsLocked() []string {
	out := append([]string{}, m.cfg.AllowlistDomains...)
	for _, ds := range m.sessionDomains {
		out = append(out, ds...)
	}
	for _, ds := range m.turnDomains {
		out = append(out, ds...)
	}
	return netproxy.NormalizeDomains(out)
}

// CheckHost applies Hard egress for host-side HTTP tools.
func (m *Manager) CheckHost(host string) error {
	m.mu.RLock()
	cfg := m.cfg
	domains := m.effectiveDomainsLocked()
	m.mu.RUnlock()
	return egress.CheckHost(cfg, domains, host)
}

// ProxyURL returns http://addr when allowlist proxy is active.
func (m *Manager) ProxyURL() string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.proxy == nil || !m.status.AllowlistActive {
		return ""
	}
	return "http://" + m.proxy.Addr()
}

// ApplyMCPEnv returns env for an MCP stdio child under sandbox network policy (S2).
func (m *Manager) ApplyMCPEnv(base []string, networkOverride string) (env []string, wrapBwrapNetDeny bool) {
	m.mu.RLock()
	cfg := m.cfg
	proxyAddr := ""
	if m.proxy != nil && m.status.AllowlistActive {
		proxyAddr = m.proxy.Addr()
	}
	m.mu.RUnlock()

	netMode := cfg.Network
	switch strings.ToLower(strings.TrimSpace(networkOverride)) {
	case "deny", "allow", "allowlist":
		netMode = domain.SandboxNetwork(strings.ToLower(strings.TrimSpace(networkOverride)))
	}

	env = append([]string{}, base...)
	switch netMode {
	case domain.SandboxNetworkAllowlist:
		if proxyAddr != "" {
			env = egress.BuildProxyEnv(env, egress.ProxyEnvOpts{ProxyAddr: proxyAddr, ForContainer: false})
		}
	case domain.SandboxNetworkDeny:
		wrapBwrapNetDeny = lookPath("bwrap")
		// Strip proxies so the child cannot widen via host proxy.
		env = egress.StripProxyEnv(env)
	}
	return env, wrapBwrapNetDeny
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
	proxyAddr := ""
	if status.AllowlistActive && m.proxy != nil {
		proxyAddr = m.proxy.Addr()
	}
	m.mu.RUnlock()

	if !opts.AllowNetwork && proxyAddr != "" && cfg.Network == domain.SandboxNetworkAllowlist {
		opts.AllowlistProxy = proxyAddr
		opts.Env = egress.BuildProxyEnv(opts.Env, egress.ProxyEnvOpts{ProxyAddr: proxyAddr, ForContainer: false})
	}
	if len(opts.ExtraDomains) > 0 && cfg.Network == domain.SandboxNetworkAllowlist {
		// Merge once extras into proxy for this Run (also covered if GrantTurnDomains already ran).
		m.mu.Lock()
		extra := netproxy.NormalizeDomains(opts.ExtraDomains)
		combined := netproxy.NormalizeDomains(append(m.effectiveDomainsLocked(), extra...))
		if m.proxy != nil {
			m.proxy.UpdateDomains(combined)
		}
		m.mu.Unlock()
		defer func() {
			m.mu.Lock()
			if m.proxy != nil {
				m.proxy.UpdateDomains(m.effectiveDomainsLocked())
			}
			m.mu.Unlock()
		}()
	}

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

	allowlistActive, allowlistReason := m.syncProxyLocked(cfg)
	if cfg.Network == domain.SandboxNetworkAllowlist {
		st.AllowlistDomains = m.effectiveDomainsLocked()
		if allowlistActive && m.proxy != nil {
			st.AllowlistActive = true
			st.AllowlistProxy = m.proxy.Addr()
		}
	}

	if !cfg.Enabled {
		_ = m.stopProxyLocked()
		st.AllowlistActive = false
		st.AllowlistProxy = ""
		st.Backend = domain.SandboxBackendDisabled
		st.Capabilities = []string{"host"}
		applyShellStatus(&st, resolveShell(cfg, st.Backend))
		m.status = st
		m.runner = hostRunner{}
		return
	}
	if cfg.Mode == domain.SandboxModeDangerFullAccess {
		_ = m.stopProxyLocked()
		st.AllowlistActive = false
		st.AllowlistProxy = ""
		st.Backend = domain.SandboxBackendDisabled
		st.Capabilities = []string{"full-access"}
		applyShellStatus(&st, resolveShell(cfg, st.Backend))
		m.status = st
		m.runner = hostRunner{}
		return
	}

	backend, r, degraded, reason, caps := selectBackend(cfg, allowlistActive)
	st.Backend = backend
	st.Degraded = degraded
	st.DegradedReason = reason
	st.Capabilities = caps
	if allowlistReason != "" {
		st.Degraded = true
		if st.DegradedReason != "" {
			st.DegradedReason = st.DegradedReason + "; " + allowlistReason
		} else {
			st.DegradedReason = allowlistReason
		}
	}
	if allowlistActive {
		st.Capabilities = append(st.Capabilities, "allowlist-proxy")
	}
	applyShellStatus(&st, resolveShell(cfg, backend))
	m.status = st
	m.runner = r
}

// syncProxyLocked starts/stops the allowlist proxy. Caller holds m.mu.
// Returns whether the proxy is active and an optional degraded reason.
func (m *Manager) syncProxyLocked(cfg domain.ConfigSandboxSection) (active bool, degradedReason string) {
	want := cfg.Enabled &&
		cfg.Mode != domain.SandboxModeDangerFullAccess &&
		cfg.Network == domain.SandboxNetworkAllowlist

	if !want {
		_ = m.stopProxyLocked()
		return false, ""
	}
	domains := m.effectiveDomainsLocked()
	if len(domains) == 0 {
		_ = m.stopProxyLocked()
		return false, "allowlist requested but allowlist_domains is empty; failing closed to deny"
	}

	// Restart if domains changed or proxy missing.
	if m.proxy != nil {
		same := sameStringSlice(m.proxy.Domains(), domains)
		if same {
			return true, ""
		}
		m.proxy.UpdateDomains(domains)
		return true, ""
	}
	srv, err := netproxy.Start(domains)
	if err != nil {
		_ = m.stopProxyLocked()
		return false, "allowlist proxy failed to start; failing closed to deny: " + err.Error()
	}
	m.proxy = srv
	return true, ""
}

func (m *Manager) stopProxyLocked() error {
	if m.proxy == nil {
		return nil
	}
	err := m.proxy.Close()
	m.proxy = nil
	return err
}

func sameStringSlice(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
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

func networkAllowed(cfg domain.ConfigSandboxSection, opts port.SandboxRunOptions) bool {
	return egress.OSNetworkOpen(cfg.Network, opts.AllowNetwork, opts.AllowlistProxy)
}

// needNetDeny reports whether backend selection should prefer network isolation.
func needNetDeny(cfg domain.ConfigSandboxSection, allowlistProxyActive bool) bool {
	return egress.NeedNetDeny(cfg.Network, allowlistProxyActive)
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
