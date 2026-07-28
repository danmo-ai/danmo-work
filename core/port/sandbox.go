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
	// Timeout bounds execution; zero means default (30s).
	Timeout time.Duration
	// Env is the child environment. Nil means a filtered copy of the host env.
	Env []string
	// AllowNetwork overrides config network=deny for this invocation (after user approval).
	AllowNetwork bool
	// AllowlistProxy is the host:port of the loopback allowlist proxy. Set by
	// sandbox.Manager when network=allowlist is active; opens OS network and
	// signals runners not to unshare/deny net.
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
