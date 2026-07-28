package permission

import (
	"net/url"
	"path/filepath"
	"strings"

	"danmo-work/core/domain"
	"danmo-work/core/runtime/sandbox/netproxy"
)

type Decision string

const (
	DecisionAllow Decision = "allow"
	DecisionAsk   Decision = "ask"
	DecisionDeny  Decision = "deny"
)

// Reason codes for Ask decisions (stable for UI / session memory).
const (
	ReasonNone             = ""
	ReasonDangerousCommand = "dangerous_command"
	ReasonNetwork          = "network"        // deny mode: full egress ask
	ReasonNetworkDomain    = "network_domain" // allowlist: add one domain
	ReasonUnsandboxed      = "unsandboxed"
	ReasonRuleAsk          = "rule_ask"
	ReasonHighRisk         = "high_risk"
	ReasonExternal         = "external"
	ReasonModeDeny         = "mode_deny"
)

// Request is the structured input for permission checks.
type Request struct {
	ToolName             string
	Risk                 domain.RiskLevel
	Command              string // exec_shell command when applicable
	URL                  string // host-egress tools (http_request / web_fetch)
	Sandbox              domain.SandboxStatus
	SessionAllowNetwork  bool     // deny-mode full open for session
	SessionAllowDomains  []string // allowlist session grants
	Mode                 domain.PermissionMode
}

// Result is the gate outcome.
type Result struct {
	Decision Decision
	Reason   string
	Domain   string // set when ReasonNetworkDomain
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

// WithRules returns a shallow copy with rules replaced.
func (g *Gate) WithRules(rules []domain.PermissionRule) *Gate {
	if g == nil {
		return NewGate(rules)
	}
	out := *g
	out.Rules = rules
	return &out
}

// AutoApprovable reports whether auto_approve may skip waiting for this reason.
func AutoApprovable(reason string) bool {
	return domain.AutoApprovableReason(reason)
}

// Check evaluates tool permission. Prefer CheckRequest for sandbox-aware policy.
func (g *Gate) Check(toolName string, risk domain.RiskLevel) Decision {
	return g.CheckRequest(Request{ToolName: toolName, Risk: risk}).Decision
}

// CheckRequest applies mode + Soft Gate policy (Hard enforcement is separate).
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

	// Host egress tools share sandbox network policy (M4).
	if isHostEgressTool(req.ToolName) {
		if r := checkHostEgress(req); r.Decision != DecisionAllow || r.Reason != "" {
			return r
		}
	}

	if !isAskRisk(req.Risk) && req.ToolName != "exec_shell" {
		return Result{Decision: DecisionAllow}
	}

	if req.ToolName == "exec_shell" && isStrongSandbox(req.Sandbox) {
		if LooksDangerous(req.Command) {
			return Result{Decision: DecisionAsk, Reason: ReasonDangerousCommand}
		}
		if r := checkShellNetwork(req); r.Decision != DecisionAllow || r.Reason != "" {
			return r
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

func isHostEgressTool(name string) bool {
	switch name {
	case "http_request", "web_fetch", "web_search":
		return true
	default:
		return false
	}
}

func checkShellNetwork(req Request) Result {
	switch req.Sandbox.Network {
	case domain.SandboxNetworkDeny:
		if !LooksLikeNetwork(req.Command) {
			return Result{Decision: DecisionAllow}
		}
		if req.SessionAllowNetwork {
			return Result{Decision: DecisionAllow}
		}
		return Result{Decision: DecisionAsk, Reason: ReasonNetwork}
	case domain.SandboxNetworkAllowlist:
		hosts := ExtractHosts(req.Command)
		domains := mergeDomains(req.Sandbox.AllowlistDomains, req.SessionAllowDomains)
		for _, h := range hosts {
			if !netproxy.Match(h, domains) {
				return Result{Decision: DecisionAsk, Reason: ReasonNetworkDomain, Domain: h}
			}
		}
		return Result{Decision: DecisionAllow}
	default:
		return Result{Decision: DecisionAllow}
	}
}

func checkHostEgress(req Request) Result {
	host := hostFromURL(req.URL)
	if host == "" && req.ToolName == "web_search" {
		// Provider endpoint is configured separately; still subject to network mode.
		switch req.Sandbox.Network {
		case domain.SandboxNetworkDeny:
			if req.SessionAllowNetwork {
				return Result{Decision: DecisionAllow}
			}
			return Result{Decision: DecisionAsk, Reason: ReasonNetwork}
		case domain.SandboxNetworkAllowlist:
			// web_search uses configured provider; allow Soft, Hard Match happens in client if URL known.
			return Result{Decision: DecisionAllow}
		default:
			return Result{Decision: DecisionAllow}
		}
	}
	if host == "" {
		return Result{Decision: DecisionAllow}
	}
	switch req.Sandbox.Network {
	case domain.SandboxNetworkDeny:
		if req.SessionAllowNetwork {
			return Result{Decision: DecisionAllow}
		}
		return Result{Decision: DecisionAsk, Reason: ReasonNetwork}
	case domain.SandboxNetworkAllowlist:
		domains := mergeDomains(req.Sandbox.AllowlistDomains, req.SessionAllowDomains)
		if netproxy.Match(host, domains) {
			return Result{Decision: DecisionAllow}
		}
		return Result{Decision: DecisionAsk, Reason: ReasonNetworkDomain, Domain: host}
	default:
		return Result{Decision: DecisionAllow}
	}
}

func hostFromURL(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	if !strings.Contains(raw, "://") {
		raw = "https://" + raw
	}
	u, err := url.Parse(raw)
	if err != nil {
		return ""
	}
	return strings.ToLower(u.Hostname())
}

func mergeDomains(a, b []string) []string {
	return netproxy.NormalizeDomains(append(append([]string{}, a...), b...))
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
