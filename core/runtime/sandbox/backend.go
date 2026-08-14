package sandbox

import (
	"context"

	"danmo-work/core/domain"
	"danmo-work/core/port"
)

// Backend is the unified sandbox backend. OS sandboxes (seatbelt, landlock,
// bwrap, win-token, wsl2), OCI container engines (podman, docker,
// apple-container), and direct host execution (host-weak) all implement it.
type Backend interface {
	port.SandboxBackend
}

// containerState is implemented by container backends so the Manager can
// surface the container execution view (image/tar status, active projects).
type containerState interface {
	ImageReady() bool
	ActiveProjects() []string
	RuntimeName() string
	Degraded() bool
	DegradedReason() string
	NotifyTarInstalled()
	Teardown(ctx context.Context, projectID string) error
}

type hostBackend struct{}

func (hostBackend) Name() domain.SandboxBackend { return domain.SandboxBackendHostWeak }

func (hostBackend) Run(ctx context.Context, opts port.SandboxRunOptions, cfg domain.ConfigSandboxSection) ([]byte, error) {
	return runHost(ctx, opts, cfg, domain.SandboxBackendHostWeak)
}

func (hostBackend) Close() error { return nil }
