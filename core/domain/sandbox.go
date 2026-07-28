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

// SandboxBackend identifies the OS enforcement mechanism in use.
type SandboxBackend string

const (
	SandboxBackendSeatbelt SandboxBackend = "seatbelt"
	SandboxBackendLandlock SandboxBackend = "landlock"
	SandboxBackendBwrap    SandboxBackend = "bwrap"
	SandboxBackendWinToken SandboxBackend = "win-token"
	SandboxBackendWSL2     SandboxBackend = "wsl2"
	SandboxBackendHostWeak SandboxBackend = "host-weak"
	SandboxBackendDisabled SandboxBackend = "disabled"
)

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
	// Backend forces a backend when non-empty (e.g. "bwrap", "wsl2", "host-weak").
	// Empty means auto-probe.
	Backend string `json:"backend,omitempty" mapstructure:"backend" yaml:"backend,omitempty"`
	// Shell selects the Windows host interpreter: auto | bash | cmd. Empty means auto.
	// Ignored for WSL2 backend (always bash via wsl). Unix always uses sh.
	Shell string `json:"shell,omitempty" mapstructure:"shell" yaml:"shell,omitempty"`
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
}
