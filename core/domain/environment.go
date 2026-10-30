package domain

// EnvironmentBackend selects where exec_shell runs.
type EnvironmentBackend string

const (
	EnvironmentBackendLocal     EnvironmentBackend = "local"     // OS sandbox on host
	EnvironmentBackendContainer EnvironmentBackend = "container" // bundled OCI tar + per-project container
)

// EnvironmentEngine selects the container CLI. Empty / "auto" = probe.
type EnvironmentEngine string

// EnvImagePreinstalled describes the bundled agent image contents (environments/agent-base/Dockerfile).
// Injected into the system prompt so the LLM knows what is available inside the
// container without probing. Keep in sync with the Dockerfile apk list.
const EnvImagePreinstalled = "Alpine Linux (busybox coreutils) with bash, git, curl, jq, ca-certificates preinstalled; install more with: apk add --no-cache <pkg>"

// EnvDesktopImagePreinstalled describes the desktop agent image
// (environments/agent-desktop/Dockerfile). Keep in sync with that Dockerfile.
const EnvDesktopImagePreinstalled = "Ubuntu 24.04 desktop sandbox with Xvfb :1, openbox, xterm, Chromium (CDP http://127.0.0.1:9222), xdotool/scrot/ImageMagick, x11vnc + noVNC (http://127.0.0.1:6080); bash/git/curl/jq preinstalled"

// Default OCI image tags after local tar load (never registry pull).
const (
	EnvImageTagCLI     = "localhost/danmo-work-env:bundled"
	EnvImageTagDesktop = "localhost/danmo-work-env:desktop"
)

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

// ConfigEnvironmentSection is the legacy optional OCI / local execution
// environment section. Deprecated: container engines are now unified sandbox
// backends selected via runtime.sandbox.backend. Legacy values are migrated
// into ConfigSandboxSection on load and this section is cleared on save.
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
	// WorkspaceMount is the path inside the container. Empty / "same" = use the
	// host project absolute path so file tools and exec_shell share paths.
	WorkspaceMount string `json:"workspaceMount" mapstructure:"workspace_mount" yaml:"workspace_mount"`
	// Resources: default unlimited; optional user overrides (Settings / config).
	Resources EnvironmentResources `json:"resources" mapstructure:"resources" yaml:"resources"`
}

// EnvironmentStatus is exposed via API for Settings / diagnostics.
type EnvironmentStatus struct {
	Backend     EnvironmentBackend `json:"backend"`
	Engine      string             `json:"engine,omitempty"` // podman | docker | apple-container
	Profile     string             `json:"profile,omitempty"` // cli | desktop
	Image       string             `json:"image,omitempty"`
	ImageLoaded bool               `json:"imageLoaded"`
	TarPath     string             `json:"tarPath,omitempty"`
	TarPresent  bool               `json:"tarPresent"`
	TarBytes    int64              `json:"tarBytes,omitempty"`
	TarArch     string             `json:"tarArch,omitempty"`
	DownloadURL string             `json:"downloadUrl,omitempty"`
	AssetName   string             `json:"assetName,omitempty"`
	// TarVariants lists linux/amd64 and linux/arm64 download options for the active profile.
	TarVariants []EnvironmentTarVariant `json:"tarVariants,omitempty"`
	// DesktopTarVariants lists desktop-profile download options (always when useful for Settings).
	DesktopTarVariants []EnvironmentTarVariant `json:"desktopTarVariants,omitempty"`
	WorkspaceMount     string                 `json:"workspaceMount,omitempty"`
	Resources          EnvironmentResources   `json:"resources,omitempty"`
	Degraded           bool                   `json:"degraded"`
	DegradedReason     string                 `json:"degradedReason,omitempty"`
	// ActiveProjects lists project IDs with a running/created container.
	ActiveProjects []string `json:"activeProjects,omitempty"`
	// Desktop endpoints (profile=desktop only; populated when a container is running).
	Display       string `json:"display,omitempty"`  // e.g. ":1"
	CDPURL        string `json:"cdpUrl,omitempty"`   // host-mapped Chromium CDP
	NoVNCURL      string `json:"novncUrl,omitempty"` // host-mapped noVNC viewer
	CDPPort       int    `json:"cdpPort,omitempty"`
	NoVNCPort     int    `json:"novncPort,omitempty"`
	DesktopWidth  int    `json:"desktopWidth,omitempty"`
	DesktopHeight int    `json:"desktopHeight,omitempty"`
}

// EnvironmentTarVariant is one arch-specific env image download option.
type EnvironmentTarVariant struct {
	Arch        string `json:"arch"`
	Profile     string `json:"profile,omitempty"` // cli | desktop
	Present     bool   `json:"present"`
	Path        string `json:"path,omitempty"`
	Bytes       int64  `json:"bytes,omitempty"`
	DownloadURL string `json:"downloadUrl"`
	AssetName   string `json:"assetName"`
	Recommended bool   `json:"recommended,omitempty"`
}

// DesktopEndpoints are published host-side URLs for a desktop sandbox container.
type DesktopEndpoints struct {
	Display   string `json:"display,omitempty"`
	CDPURL    string `json:"cdpUrl,omitempty"`
	NoVNCURL  string `json:"novncUrl,omitempty"`
	CDPPort   int    `json:"cdpPort,omitempty"`
	NoVNCPort int    `json:"novncPort,omitempty"`
	Container string `json:"container,omitempty"`
}
