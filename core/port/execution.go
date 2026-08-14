package port

import (
	"context"

	"danmo-work/core/domain"
)

// EnvironmentInspector exposes Settings / diagnostics for the execution env.
// Implemented by the unified sandbox manager; the container view is only
// meaningful when a container backend (podman/docker/apple-container) is active.
type EnvironmentInspector interface {
	// EnvironmentStatus reports the container execution view (empty/local when
	// no container backend is selected).
	EnvironmentStatus() domain.EnvironmentStatus
	// StatusWithTar fills download/install metadata for Settings (version = app version).
	StatusWithTar(version string) domain.EnvironmentStatus
	NotifyTarInstalled()
}

// ExecutionBackend manages the execution environment lifecycle (per-project
// container teardown, env tar status). The command path itself goes through
// the unified Sandbox backends.
type ExecutionBackend interface {
	EnvironmentInspector
	Teardown(ctx context.Context, projectID string) error
}
