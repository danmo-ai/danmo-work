package port

import (
	"context"
	"time"

	"danmo-work/core/domain"
)

// ExecRunOptions configures a single command under the active ExecutionBackend.
// Embeds SandboxRunOptions so local and container paths share one RunSpec.
type ExecRunOptions struct {
	SandboxRunOptions
	ProjectID string
}

// EnvironmentInspector exposes Settings / diagnostics for the execution env.
type EnvironmentInspector interface {
	Status() domain.EnvironmentStatus
	// StatusWithTar fills download/install metadata for Settings (version = app version).
	StatusWithTar(version string) domain.EnvironmentStatus
	Configure(cfg domain.ConfigEnvironmentSection, sandboxCfg domain.ConfigSandboxSection)
	NotifyTarInstalled()
}

// CommandRunner runs shell commands and manages per-project container lifecycle.
type CommandRunner interface {
	Run(ctx context.Context, opts ExecRunOptions) ([]byte, error)
	Teardown(ctx context.Context, projectID string) error
	// Close tears down all active project containers (process shutdown).
	Close() error
}

// ExecutionBackend runs shell commands either on the host sandbox or in a
// per-project OCI container loaded from a user-downloaded tar (no registry pull).
type ExecutionBackend interface {
	EnvironmentInspector
	CommandRunner
}

// Ensure ExecRunOptions timeout helper stays discoverable for callers migrating
// from the former flat struct (Timeout lived on the outer type).
func (o ExecRunOptions) EffectiveTimeout(def time.Duration) time.Duration {
	if o.Timeout > 0 {
		return o.Timeout
	}
	return def
}
