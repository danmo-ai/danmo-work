package domain

import "strings"

// IsolationSource identifies which Hard boundary is enforcing isolation.
type IsolationSource string

const (
	IsolationNone      IsolationSource = "none"
	IsolationOSSandbox IsolationSource = "os-sandbox"
	IsolationContainer IsolationSource = "container"
)

// EffectiveIsolation is the Soft Gate view of Hard isolation.
// Soft decisions must use this — not SandboxStatus alone — so OCI container
// mode is treated as strong even when the host OS sandbox is host-weak.
type EffectiveIsolation struct {
	Strong           bool            `json:"strong"`
	Source           IsolationSource `json:"source"`
	Network          SandboxNetwork  `json:"network"`
	AllowlistDomains []string        `json:"allowlistDomains,omitempty"`
	AllowlistActive  bool            `json:"allowlistActive,omitempty"`
}

// ComputeEffectiveIsolation merges OS sandbox + execution environment status.
func ComputeEffectiveIsolation(sb SandboxStatus, env EnvironmentStatus) EffectiveIsolation {
	out := EffectiveIsolation{
		Network:          sb.Network,
		AllowlistDomains: append([]string{}, sb.AllowlistDomains...),
		AllowlistActive:  sb.AllowlistActive,
	}
	if env.Backend == EnvironmentBackendContainer && !env.Degraded && strings.TrimSpace(env.Engine) != "" {
		out.Strong = true
		out.Source = IsolationContainer
		return out
	}
	// Unified backends: container engines may be reported directly via the
	// sandbox status (podman/docker/apple-container backends).
	if sb.Enabled && IsContainerBackend(sb.Backend) && !sb.Degraded {
		out.Strong = true
		out.Source = IsolationContainer
		return out
	}
	if isStrongOSSandbox(sb) {
		out.Strong = true
		out.Source = IsolationOSSandbox
		return out
	}
	out.Source = IsolationNone
	return out
}

func isStrongOSSandbox(st SandboxStatus) bool {
	if !st.Enabled {
		return false
	}
	switch st.Backend {
	case SandboxBackendHostWeak, SandboxBackendDisabled, "":
		return false
	}
	switch st.Mode {
	case SandboxModeWorkspaceWrite, SandboxModeReadOnly:
		return true
	default:
		return false
	}
}
