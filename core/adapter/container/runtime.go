package container

import (
	"context"

	"danmo-work/core/domain"
)

// Runtime is a pluggable container engine (Podman, Docker, Apple Container, …).
// Implementations must never registry-pull; only load local tar / use local tags.
type Runtime interface {
	// Name is a stable id: podman | docker | apple-container
	Name() string
	ImageExists(ctx context.Context, image string) (bool, error)
	LoadTar(ctx context.Context, tarPath string) error
	EnsureTag(ctx context.Context, image string) error
	ContainerInspectState(ctx context.Context, name string) (string, error)
	CreateDetached(ctx context.Context, opts CreateOpts) error
	Start(ctx context.Context, name string) error
	Stop(ctx context.Context, name string) error
	Rm(ctx context.Context, name string) error
	Exec(ctx context.Context, name, workdir, command string, env []string) ([]byte, error)
}

// Bind mounts an additional host path into the container.
type Bind struct {
	Host      string
	Container string
	ReadOnly  bool
}

// CreateOpts configures a long-lived per-project container.
type CreateOpts struct {
	Name      string
	Image     string
	WorkDir   string // host path
	Mount     string // container path
	Network   string // engine-specific; "" = default bridge/nat
	Env       []string
	Binds     []Bind
	Resources domain.EnvironmentResources
}
