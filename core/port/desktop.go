package port

import (
	"context"

	"danmo-work/core/domain"
)

// DesktopEnvironment is implemented by the sandbox manager when a container
// backend can host a virtual Linux desktop (profile=desktop). Computer and
// browser tools use it to drive DISPLAY/CDP inside the sandbox instead of the host.
type DesktopEnvironment interface {
	// DesktopProfile returns cli|desktop for the active sandbox config.
	DesktopProfile() string
	// EnsureDesktop starts (or reuses) the project's desktop container and
	// returns published CDP/noVNC endpoints.
	EnsureDesktop(ctx context.Context, opts SandboxRunOptions) (domain.DesktopEndpoints, error)
	// ExecDesktop runs a shell command inside the desktop container with DISPLAY set.
	ExecDesktop(ctx context.Context, opts SandboxRunOptions, command string, env []string) ([]byte, error)
	// DesktopEndpoints returns the last-known published endpoints for a project.
	DesktopEndpoints(projectID string) (domain.DesktopEndpoints, bool)
}
