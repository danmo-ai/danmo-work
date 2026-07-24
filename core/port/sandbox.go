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
}

// Sandbox executes commands under the platform sandbox policy.
type Sandbox interface {
	Status() domain.SandboxStatus
	Run(ctx context.Context, opts SandboxRunOptions) ([]byte, error)
	// Configure replaces policy and re-probes the backend (e.g. after config save).
	Configure(cfg domain.ConfigSandboxSection)
}
