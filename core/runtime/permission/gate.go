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
)

// Request is the structured input for permission checks.
type Request struct {
	ToolName            string
	Risk                domain.RiskLevel
	Command             string // exec_shell command when applicable
	Sandbox             domain.SandboxStatus
	SessionAllowNetwork bool
}

// Result is the gate outcome.
type Result struct {
	Decision Decision
	Reason   string
}

type Gate struct {
	Rules []domain.PermissionRule
}

func NewGate(rules []domain.PermissionRule) *Gate {
	return &Gate{Rules: rules}
}

// Check evaluates tool permission. Prefer CheckRequest for sandbox-aware policy.
func (g *Gate) Check(toolName string, risk domain.RiskLevel) Decision {
	return g.CheckRequest(Request{ToolName: toolName, Risk: risk}).Decision
}

// CheckRequest applies mainstream-aligned policy:
// non-high → allow (unless rules deny/ask);
// exec_shell + strong sandbox → allow unless dangerous / network-denied;
// otherwise high → ask.
func (g *Gate) CheckRequest(req Request) Result {
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

	if req.Risk != domain.RiskHigh {
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
		return Result{Decision: DecisionAllow}
	}

	if req.ToolName == "exec_shell" {
		return Result{Decision: DecisionAsk, Reason: ReasonUnsandboxed}
	}
	return Result{Decision: DecisionAsk, Reason: ReasonHighRisk}
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
