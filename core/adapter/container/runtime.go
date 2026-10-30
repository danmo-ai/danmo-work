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
	// InspectPublishedPorts returns containerPort→hostPort for a running container.
	// Optional; engines that cannot publish ports may return nil, nil.
	InspectPublishedPorts(ctx context.Context, name string) (PublishedPorts, error)
}

// Bind mounts an additional host path into the container.
type Bind struct {
	Host      string
	Container string
	ReadOnly  bool
}

// PortPublish maps a container port to the host (loopback by default).
// HostPort 0 asks the engine to allocate an ephemeral host port.
type PortPublish struct {
	HostIP        string // empty → 127.0.0.1
	HostPort      int    // 0 → ephemeral
	ContainerPort int
	Proto         string // empty → tcp
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
	// Ports publishes container ports to the host (desktop profile CDP/noVNC).
	Ports []PortPublish
	// Cmd overrides the image cmd. When empty and KeepImageCmd is false → ["sleep","infinity"].
	Cmd []string
	// Entrypoint overrides image ENTRYPOINT when non-nil (including empty slice to clear).
	Entrypoint []string
	// KeepImageCmd leaves image ENTRYPOINT/CMD unchanged (desktop profile).
	KeepImageCmd bool
	// ShmSize is passed as --shm-size (Chromium benefits from >=256m).
	ShmSize string
}

// PublishedPorts reports host→container port mappings after create/start.
type PublishedPorts map[int]int // containerPort → hostPort
