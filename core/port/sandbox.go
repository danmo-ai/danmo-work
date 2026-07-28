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
}

// Sandbox executes commands under the platform sandbox policy.
type Sandbox interface {
	Status() domain.SandboxStatus
	Run(ctx context.Context, opts SandboxRunOptions) ([]byte, error)
	// Configure replaces policy and re-probes the backend (e.g. after config save).
	Configure(cfg domain.ConfigSandboxSection)
	// GrantDomains merges hosts into the runtime allowlist (session grants).
	GrantDomains(domains []string)
	// CheckHost applies Hard egress policy for host-side HTTP tools.
	CheckHost(host string) error
	// ProxyURL returns the allowlist proxy URL (http://host:port) when active.
	ProxyURL() string
}
