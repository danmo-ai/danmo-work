package permission

import (
	"path/filepath"
	"strings"

	"danmo-work/core/domain"
)

type Decision string

const (
	DecisionAllow Decision = "allow"
	DecisionAsk   Decision = "ask"
	DecisionDeny  Decision = "deny"
)

// Reason codes for Ask decisions (stable for UI / session memory).
const (
	ReasonNone              = ""
	ReasonDangerousCommand  = "dangerous_command"
	ReasonNetwork           = "network"
	ReasonUnsandboxed       = "unsandboxed"
	ReasonRuleAsk           = "rule_ask"
	ReasonHighRisk          = "high_risk"
	ReasonExternal          = "external"
	ReasonModeDeny          = "mode_deny"
)

// Request is the structured input for permission checks.
type Request struct {
	ToolName            string
	Risk                domain.RiskLevel
	Command             string // exec_shell command when applicable
	Sandbox             domain.SandboxStatus
	SessionAllowNetwork bool
	Mode                domain.PermissionMode
}

// Result is the gate outcome.
type Result struct {
	Decision Decision
	Reason   string
}

type Gate struct {
	Rules []domain.PermissionRule
	Mode  domain.PermissionMode
}

func NewGate(rules []domain.PermissionRule) *Gate {
	return &Gate{Rules: rules, Mode: domain.PermModeInteractive}
}

// WithMode returns a shallow copy with permission mode set.
func (g *Gate) WithMode(mode domain.PermissionMode) *Gate {
	if g == nil {
		return NewGate(nil).WithMode(mode)
	}
	out := *g
	if mode == "" {
		mode = domain.PermModeInteractive
	}
	out.Mode = mode
	return &out
}

// Check evaluates tool permission. Prefer CheckRequest for sandbox-aware policy.
func (g *Gate) Check(toolName string, risk domain.RiskLevel) Decision {
	return g.CheckRequest(Request{ToolName: toolName, Risk: risk}).Decision
}

// CheckRequest applies mode + mainstream-aligned policy:
// discuss/plan deny write/exec/external;
// interactive asks on high/external;
// auto allows medium writes but still asks dangerous shell / network-deny.
func (g *Gate) CheckRequest(req Request) Result {
	mode := req.Mode
	if mode == "" && g != nil {
		mode = g.Mode
	}
	if mode == "" {
		mode = domain.PermModeInteractive
	}

	for _, r := range g.Rules {
		if !matchPattern(r.Pattern, req.ToolName) {
			continue
		}
		if r.Action == domain.PermDeny {
			return Result{Decision: DecisionDeny, Reason: ReasonRuleAsk}
		}
		if r.Action == domain.PermAsk {
			return Result{Decision: DecisionAsk, Reason: ReasonRuleAsk}
		}
	}

	if mode == domain.PermModeDiscuss || mode == domain.PermModePlan {
		if isConsequentialRisk(req.Risk) || req.ToolName == "exec_shell" || strings.HasPrefix(req.ToolName, "mcp_") {
			return Result{Decision: DecisionDeny, Reason: ReasonModeDeny}
		}
		if req.ToolName == "write" || req.ToolName == "edit" || req.ToolName == "apply_patch" {
			return Result{Decision: DecisionDeny, Reason: ReasonModeDeny}
		}
		return Result{Decision: DecisionAllow}
	}

	if !isAskRisk(req.Risk) {
		return Result{Decision: DecisionAllow}
	}

	if req.ToolName == "exec_shell" && isStrongSandbox(req.Sandbox) {
		if LooksDangerous(req.Command) {
			return Result{Decision: DecisionAsk, Reason: ReasonDangerousCommand}
		}
		if req.Sandbox.Network == domain.SandboxNetworkDeny && LooksLikeNetwork(req.Command) {
			if req.SessionAllowNetwork {
				return Result{Decision: DecisionAllow, Reason: ReasonNone}
			}
			return Result{Decision: DecisionAsk, Reason: ReasonNetwork}
		}
		if mode == domain.PermModeAuto {
			return Result{Decision: DecisionAllow}
		}
		return Result{Decision: DecisionAllow}
	}

	if req.ToolName == "exec_shell" {
		return Result{Decision: DecisionAsk, Reason: ReasonUnsandboxed}
	}

	if mode == domain.PermModeAuto && req.Risk != domain.RiskExternal {
		return Result{Decision: DecisionAllow}
	}

	if req.Risk == domain.RiskExternal || strings.HasPrefix(req.ToolName, "mcp_") {
		return Result{Decision: DecisionAsk, Reason: ReasonExternal}
	}
	return Result{Decision: DecisionAsk, Reason: ReasonHighRisk}
}

func isAskRisk(risk domain.RiskLevel) bool {
	return risk == domain.RiskHigh || risk == domain.RiskExternal
}

func isConsequentialRisk(risk domain.RiskLevel) bool {
	return risk == domain.RiskHigh || risk == domain.RiskExternal || risk == domain.RiskMedium
}

func isStrongSandbox(st domain.SandboxStatus) bool {
	if !st.Enabled {
		return false
	}
	switch st.Backend {
	case domain.SandboxBackendHostWeak, domain.SandboxBackendDisabled, "":
		return false
	}
	switch st.Mode {
	case domain.SandboxModeWorkspaceWrite, domain.SandboxModeReadOnly:
		return true
	default:
		return false
	}
}

func matchPattern(pattern, name string) bool {
	if pattern == "*" {
		return true
	}
	ok, _ := filepath.Match(pattern, name)
	return ok || strings.Contains(name, strings.Trim(pattern, "*"))
}
