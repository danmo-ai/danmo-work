package domain

type PermAction string

const (
	PermAsk  PermAction = "ask"
	PermDeny PermAction = "deny"
)

type PermissionRule struct {
	Pattern string     `json:"pattern" mapstructure:"pattern" yaml:"pattern"`
	Action  PermAction `json:"action" mapstructure:"action" yaml:"action"`
}

// Soft-gate reason codes that must never be skipped by auto_approve.
const (
	ReasonDangerousCommand = "dangerous_command"
	ReasonUnsandboxed      = "unsandboxed"
)

// AutoApprovableReason reports whether auto_approve may skip waiting for reason.
func AutoApprovableReason(reason string) bool {
	switch reason {
	case ReasonDangerousCommand, ReasonUnsandboxed:
		return false
	default:
		return true
	}
}
