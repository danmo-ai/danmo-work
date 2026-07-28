package domain

// EnvironmentBackend selects where exec_shell runs.
type EnvironmentBackend string

const (
	EnvironmentBackendLocal     EnvironmentBackend = "local"     // OS sandbox on host
	EnvironmentBackendContainer EnvironmentBackend = "container" // bundled OCI tar + per-project container
)

// ConfigEnvironmentSection is the optional OCI / local execution environment.
// Container mode loads a CI-built image tar locally — never registry pull.
type ConfigEnvironmentSection struct {
	// Backend: local (default) | container
	Backend EnvironmentBackend `json:"backend" mapstructure:"backend" yaml:"backend"`
	// Image is the local tag after `podman/docker load` (default localhost/danmo-work-env:bundled).
	Image string `json:"image" mapstructure:"image" yaml:"image"`
	// TarPath overrides discovery of the bundled OCI tar. Empty = auto
	// (WORK_ENV_TAR, ~/.danmo-work/env/, next to binary, out/env/).
	TarPath string `json:"tarPath" mapstructure:"tar_path" yaml:"tar_path"`
	// WorkspaceMount is the path inside the container (default /workspace).
	WorkspaceMount string `json:"workspaceMount" mapstructure:"workspace_mount" yaml:"workspace_mount"`
}

// EnvironmentStatus is exposed via API for Settings / diagnostics.
type EnvironmentStatus struct {
	Backend        EnvironmentBackend `json:"backend"`
	Engine         string             `json:"engine,omitempty"` // podman | docker
	Image          string             `json:"image,omitempty"`
	ImageLoaded    bool               `json:"imageLoaded"`
	TarPath        string             `json:"tarPath,omitempty"`
	WorkspaceMount string             `json:"workspaceMount,omitempty"`
	Degraded       bool               `json:"degraded"`
	DegradedReason string             `json:"degradedReason,omitempty"`
	// ActiveProjects lists project IDs with a running/created container.
	ActiveProjects []string `json:"activeProjects,omitempty"`
}
