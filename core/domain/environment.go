package domain

// EnvironmentBackend selects where exec_shell runs.
type EnvironmentBackend string

const (
	EnvironmentBackendLocal     EnvironmentBackend = "local"     // OS sandbox on host
	EnvironmentBackendContainer EnvironmentBackend = "container" // bundled OCI tar + per-project container
)

// EnvironmentEngine selects the container CLI. Empty / "auto" = probe.
type EnvironmentEngine string

const (
	EnvironmentEngineAuto           EnvironmentEngine = "auto"
	EnvironmentEnginePodman         EnvironmentEngine = "podman"
	EnvironmentEngineDocker         EnvironmentEngine = "docker"
	EnvironmentEngineAppleContainer EnvironmentEngine = "apple-container"
)

// EnvironmentResources limits applied at container create time.
// Empty fields mean unlimited (engine defaults). Values are NOT inferred from host RAM/CPU.
type EnvironmentResources struct {
	// CPUs is passed as --cpus (docker/podman/apple), e.g. "2" or "1.5". Empty = no limit.
	CPUs string `json:"cpus,omitempty" mapstructure:"cpus" yaml:"cpus"`
	// Memory is passed as --memory, e.g. "2g" or "512m". Empty = no limit.
	Memory string `json:"memory,omitempty" mapstructure:"memory" yaml:"memory"`
	// Pids is --pids-limit for docker/podman (0 = omit / unlimited). Ignored by Apple Container if unsupported.
	Pids int `json:"pids,omitempty" mapstructure:"pids" yaml:"pids"`
}

// ConfigEnvironmentSection is the optional OCI / local execution environment.
// Container mode loads a user-downloaded CI image tar locally — never registry
// pull, and the tar is not embedded in app release packages.
type ConfigEnvironmentSection struct {
	// Backend: local (default) | container
	Backend EnvironmentBackend `json:"backend" mapstructure:"backend" yaml:"backend"`
	// Engine: auto | podman | docker | apple-container
	Engine EnvironmentEngine `json:"engine" mapstructure:"engine" yaml:"engine"`
	// Image is the local tag after load (default localhost/danmo-work-env:bundled).
	Image string `json:"image" mapstructure:"image" yaml:"image"`
	// TarPath overrides discovery of the optional OCI env tar (user-downloaded
	// Release asset, not shipped inside the app package). Empty = auto
	// (WORK_ENV_TAR, ~/.danmo-work/env/, out/env/ for local builds).
	TarPath string `json:"tarPath" mapstructure:"tar_path" yaml:"tar_path"`
	// WorkspaceMount is the path inside the container (default /workspace).
	WorkspaceMount string `json:"workspaceMount" mapstructure:"workspace_mount" yaml:"workspace_mount"`
	// Resources: default unlimited; optional user overrides (Settings / config).
	Resources EnvironmentResources `json:"resources" mapstructure:"resources" yaml:"resources"`
}

// EnvironmentStatus is exposed via API for Settings / diagnostics.
type EnvironmentStatus struct {
	Backend        EnvironmentBackend   `json:"backend"`
	Engine         string               `json:"engine,omitempty"` // podman | docker | apple-container
	Image          string               `json:"image,omitempty"`
	ImageLoaded    bool                 `json:"imageLoaded"`
	TarPath        string               `json:"tarPath,omitempty"`
	TarPresent     bool                 `json:"tarPresent"`
	TarBytes       int64                `json:"tarBytes,omitempty"`
	TarArch        string               `json:"tarArch,omitempty"`
	DownloadURL    string               `json:"downloadUrl,omitempty"`
	AssetName      string               `json:"assetName,omitempty"`
	WorkspaceMount string               `json:"workspaceMount,omitempty"`
	Resources      EnvironmentResources `json:"resources,omitempty"`
	Degraded       bool                 `json:"degraded"`
	DegradedReason string               `json:"degradedReason,omitempty"`
	// ActiveProjects lists project IDs with a running/created container.
	ActiveProjects []string `json:"activeProjects,omitempty"`
}
