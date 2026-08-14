package port

import (
	"context"
	"time"

	"danmo-work/core/domain"
)

// SandboxRunOptions configures a single sandboxed process invocation.
type SandboxRunOptions struct {
	// Command is the shell command string (passed to sh -c, bash -lc, or cmd /c).
	Command string
	// WorkDir is the project workspace root (bind / write root).
	WorkDir string
	// ProjectID identifies the owning project; container backends key their
	// per-project container on it (default "default").
	ProjectID string
	// Timeout bounds execution; zero means default (30s).
	Timeout time.Duration
	// Env is the child environment. Nil means a filtered copy of the host env.
	Env []string
	// AllowNetwork overrides config network=deny for this invocation (after user approval).
	AllowNetwork bool
	// AllowlistProxy is the host:port of the loopback allowlist proxy. Set by
	// sandbox.Manager when network=allowlist is active; opens OS network and
	// signals backends not to unshare/deny net.
	AllowlistProxy string
	// ExtraDomains are once/turn-scoped Hard allows merged into the proxy for
	// this invocation only (not persisted as session grants).
	ExtraDomains []string
}

// EgressAuthority is Hard outbound-network policy (shared by shell + host HTTP).
type EgressAuthority interface {
	// CheckHost applies Hard egress policy for host-side HTTP tools.
	CheckHost(host string) error
	// ProxyURL returns the allowlist proxy URL (http://host:port) when active.
	ProxyURL() string
	// GrantSessionDomains merges hosts into the Hard allowlist for one session.
	GrantSessionDomains(sessionID string, domains []string)
	// RevokeSessionDomains drops Hard grants for a session (e.g. session end).
	RevokeSessionDomains(sessionID string)
	// GrantTurnDomains merges once-scope domains for the active turn.
	GrantTurnDomains(turnID string, domains []string)
	// ClearTurnDomains drops once-scope domains when the turn ends.
	ClearTurnDomains(turnID string)
}

// ProcessSandbox executes commands under the platform FS/network sandbox.
type ProcessSandbox interface {
	Status() domain.SandboxStatus
	Run(ctx context.Context, opts SandboxRunOptions) ([]byte, error)
	// Configure replaces policy and re-probes the backend (e.g. after config save).
	Configure(cfg domain.ConfigSandboxSection)
	Close() error
}

// Sandbox is ProcessSandbox + EgressAuthority (OS sandbox owns the allowlist proxy).
type Sandbox interface {
	ProcessSandbox
	EgressAuthority
}

// SandboxBackend is the unified backend abstraction. OS sandboxes (seatbelt,
// landlock, bwrap, win-token, wsl2), OCI container engines (podman, docker,
// apple-container), and direct host execution (host-weak) all implement it.
// Tools face only this interface plus the BackendFactory.
type SandboxBackend interface {
	Name() domain.SandboxBackend
	Run(ctx context.Context, opts SandboxRunOptions, cfg domain.ConfigSandboxSection) ([]byte, error)
	Close() error
}

// BackendFactory builds and probes sandbox backends. When the sandbox is
// disabled (or danger-full-access), the factory returns the host-weak backend
// (direct OS execution).
type BackendFactory interface {
	// Available probes all platform-relevant backends for the settings page.
	Available(cfg domain.ConfigSandboxSection) []domain.SandboxBackendInfo
	// Build constructs the backend selected by cfg. Returned values:
	// backend (host-weak when disabled/unknown/unavailable), the effective
	// backend name, degraded flag, degraded reason, and capabilities.
	Build(cfg domain.ConfigSandboxSection, allowlistProxyActive bool) (backend SandboxBackend, name domain.SandboxBackend, degraded bool, reason string, caps []string)
}
