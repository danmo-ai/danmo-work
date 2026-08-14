package domain

// SandboxMode controls filesystem/network policy for process execution.
// Aligned with Codex CLI naming.
type SandboxMode string

const (
	SandboxModeReadOnly         SandboxMode = "read-only"
	SandboxModeWorkspaceWrite   SandboxMode = "workspace-write"
	SandboxModeDangerFullAccess SandboxMode = "danger-full-access"
)

// SandboxNetwork controls outbound network for sandboxed processes.
type SandboxNetwork string

const (
	SandboxNetworkDeny      SandboxNetwork = "deny"
	SandboxNetworkAllow     SandboxNetwork = "allow"
	SandboxNetworkAllowlist SandboxNetwork = "allowlist"
)

// SandboxBackend identifies the enforcement mechanism in use. All backends —
// OS sandboxes (seatbelt / landlock / bwrap / win-token / wsl2), OCI container
// engines (podman / docker / apple-container), and direct host execution
// (host-weak) — implement the same backend interface and are produced by the
// sandbox factory.
type SandboxBackend string

const (
	SandboxBackendSeatbelt       SandboxBackend = "seatbelt"
	SandboxBackendLandlock       SandboxBackend = "landlock"
	SandboxBackendBwrap          SandboxBackend = "bwrap"
	SandboxBackendWinToken       SandboxBackend = "win-token"
	SandboxBackendWSL2           SandboxBackend = "wsl2"
	SandboxBackendPodman         SandboxBackend = "podman"
	SandboxBackendDocker         SandboxBackend = "docker"
	SandboxBackendAppleContainer SandboxBackend = "apple-container"
	SandboxBackendHostWeak       SandboxBackend = "host-weak"
	SandboxBackendDisabled       SandboxBackend = "disabled"
)

// IsContainerBackend reports whether the backend runs commands in an OCI
// container engine instead of an OS-level sandbox.
func IsContainerBackend(b SandboxBackend) bool {
	switch b {
	case SandboxBackendPodman, SandboxBackendDocker, SandboxBackendAppleContainer:
		return true
	}
	return false
}

// SandboxShellPreference selects the host shell interpreter for exec_shell.
// Applies to win-token / host-weak paths on Windows; WSL2 backend always uses bash inside WSL.
const (
	SandboxShellAuto = "auto" // Windows: Coreutils+cmd when available, else Git Bash, else cmd; sh on Unix
	SandboxShellBash = "bash" // require Git Bash on Windows (error if missing)
	SandboxShellCmd  = "cmd"  // force cmd.exe on Windows (Coreutils still injected onto PATH when present)
)

// ConfigSandboxSection is persisted under runtime.sandbox in config.yaml.
type ConfigSandboxSection struct {
	Enabled bool           `json:"enabled" mapstructure:"enabled" yaml:"enabled"`
	Mode    SandboxMode    `json:"mode" mapstructure:"mode" yaml:"mode"`
	Network SandboxNetwork `json:"network" mapstructure:"network" yaml:"network"`
	// AllowlistDomains is used when Network=allowlist. Exact hosts or "*.example.com" suffixes.
	// Empty with allowlist fails closed (treated as deny).
	AllowlistDomains []string `json:"allowlistDomains,omitempty" mapstructure:"allowlist_domains" yaml:"allowlist_domains,omitempty"`
	// Backend selects one unified backend: auto (probe best OS sandbox) |
	// seatbelt | landlock | bwrap | win-token | wsl2 | podman | docker |
	// apple-container | host-weak. Container engines are only used when
	// explicitly selected. Empty means auto.
	Backend string `json:"backend,omitempty" mapstructure:"backend" yaml:"backend,omitempty"`
	// Shell selects the Windows host interpreter: auto | bash | cmd. Empty means auto.
	// Ignored for WSL2 (always bash via wsl) and container backends. Unix always uses sh.
	Shell string `json:"shell,omitempty" mapstructure:"shell" yaml:"shell,omitempty"`
	// Container params (only used when Backend is a container engine):
	// Image is the local tag after load (default localhost/danmo-work-env:bundled).
	Image string `json:"image,omitempty" mapstructure:"image" yaml:"image,omitempty"`
	// TarPath overrides discovery of the optional OCI env tar (user-downloaded
	// Release asset, not shipped inside the app package). Empty = auto.
	TarPath string `json:"tarPath,omitempty" mapstructure:"tar_path" yaml:"tar_path,omitempty"`
	// WorkspaceMount is the path inside the container. Empty / "same" = use the
	// host project absolute path so file tools and exec_shell share paths.
	WorkspaceMount string `json:"workspaceMount,omitempty" mapstructure:"workspace_mount" yaml:"workspace_mount,omitempty"`
	// Resources: default unlimited; optional user overrides (Settings / config).
	Resources EnvironmentResources `json:"resources,omitempty" mapstructure:"resources" yaml:"resources,omitempty"`
}

// SandboxBackendInfo describes one probed backend for the settings page.
type SandboxBackendInfo struct {
	Name         SandboxBackend `json:"name"`
	Available    bool           `json:"available"`
	Reason       string         `json:"reason,omitempty"`
	Capabilities []string       `json:"capabilities,omitempty"`
	// Container is true for OCI engine backends (podman/docker/apple-container).
	Container bool `json:"container,omitempty"`
	// AutoPreferred marks the backend the auto probe would select.
	AutoPreferred bool `json:"autoPreferred,omitempty"`
}

// SandboxStatus is the probed runtime sandbox capability surface.
type SandboxStatus struct {
	Enabled        bool           `json:"enabled"`
	Mode           SandboxMode    `json:"mode"`
	Network        SandboxNetwork `json:"network"`
	Backend        SandboxBackend `json:"backend"`
	Degraded       bool           `json:"degraded"`
	DegradedReason string         `json:"degradedReason,omitempty"`
	Platform       string         `json:"platform"`
	Capabilities   []string       `json:"capabilities,omitempty"`
	// AllowlistActive is true when network=allowlist and the host-side proxy is running.
	AllowlistActive bool `json:"allowlistActive,omitempty"`
	// AllowlistProxy is the loopback proxy address (e.g. "127.0.0.1:41234") when active.
	AllowlistProxy string `json:"allowlistProxy,omitempty"`
	// AllowlistDomains echoes the normalized domain rules in effect.
	AllowlistDomains []string `json:"allowlistDomains,omitempty"`
	// Shell is the human-readable interpreter label (e.g. "cmd (Coreutils)", "bash (Git for Windows)", "cmd", "sh").
	Shell string `json:"shell,omitempty"`
	// ShellPath is the absolute path to bash.exe when using Git Bash; empty for cmd/sh/WSL2.
	ShellPath string `json:"shellPath,omitempty"`
	// CoreutilsBin is the Windows Coreutils applet directory on PATH (ls.exe, cat.exe, …); empty when unavailable.
	CoreutilsBin string `json:"coreutilsBin,omitempty"`
	// Backends lists every probed backend available on this host (settings page).
	Backends []SandboxBackendInfo `json:"backends,omitempty"`
}
