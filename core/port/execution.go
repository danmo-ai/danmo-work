package port

import (
	"context"
	"time"

	"danmo-work/core/domain"
)

// ExecRunOptions configures a single command under the active ExecutionBackend.
type ExecRunOptions struct {
	ProjectID string
	Command   string
	WorkDir   string
	Timeout   time.Duration
	Env       []string
	// AllowNetwork / AllowlistProxy mirror SandboxRunOptions for container net policy.
	AllowNetwork   bool
	AllowlistProxy string
}

// ExecutionBackend runs shell commands either on the host sandbox or in a
// per-project OCI container loaded from a user-downloaded tar (no registry pull).
type ExecutionBackend interface {
	Status() domain.EnvironmentStatus
	// StatusWithTar fills download/install metadata for Settings (version = app version).
	StatusWithTar(version string) domain.EnvironmentStatus
	Configure(cfg domain.ConfigEnvironmentSection, sandboxCfg domain.ConfigSandboxSection)
	NotifyTarInstalled()
	Run(ctx context.Context, opts ExecRunOptions) ([]byte, error)
	Teardown(ctx context.Context, projectID string) error
}
