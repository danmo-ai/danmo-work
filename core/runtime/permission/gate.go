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
	ReasonRuleDeny         = "rule_deny"
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
	SearchProvider       string // web_search configured provider (for Soft allowlist)
	SearchBaseURL        string // web_search base_url (SearXNG / custom DDG)
	Sandbox              domain.SandboxStatus
	Isolation            domain.EffectiveIsolation // preferred over Sandbox for strength
	SessionAllowNetwork  bool                      // deny-mode full open for session
	SessionAllowDomains  []string                  // allowlist session grants
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
			return Result{Decision: DecisionDeny, Reason: ReasonRuleDeny}
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

	if req.ToolName == "exec_shell" && isStrongIsolation(req) {
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

func networkOf(req Request) domain.SandboxNetwork {
	if req.Isolation.Network != "" {
		return req.Isolation.Network
	}
	return req.Sandbox.Network
}

func allowlistDomainsOf(req Request) []string {
	if len(req.Isolation.AllowlistDomains) > 0 {
		return mergeDomains(req.Isolation.AllowlistDomains, req.SessionAllowDomains)
	}
	return mergeDomains(req.Sandbox.AllowlistDomains, req.SessionAllowDomains)
}

func checkShellNetwork(req Request) Result {
	switch networkOf(req) {
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
		domains := allowlistDomainsOf(req)
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
	hosts := []string{}
	if h := hostFromURL(req.URL); h != "" {
		hosts = append(hosts, h)
	}
	if req.ToolName == "web_search" && len(hosts) == 0 {
		hosts = SearchProviderHosts(req.SearchProvider, req.SearchBaseURL)
	}
	if len(hosts) == 0 && req.ToolName == "web_search" {
		// Unknown provider endpoint — still subject to deny mode.
		switch networkOf(req) {
		case domain.SandboxNetworkDeny:
			if req.SessionAllowNetwork {
				return Result{Decision: DecisionAllow}
			}
			return Result{Decision: DecisionAsk, Reason: ReasonNetwork}
		default:
			return Result{Decision: DecisionAllow}
		}
	}
	if len(hosts) == 0 {
		return Result{Decision: DecisionAllow}
	}
	switch networkOf(req) {
	case domain.SandboxNetworkDeny:
		if req.SessionAllowNetwork {
			return Result{Decision: DecisionAllow}
		}
		return Result{Decision: DecisionAsk, Reason: ReasonNetwork}
	case domain.SandboxNetworkAllowlist:
		domains := allowlistDomainsOf(req)
		for _, host := range hosts {
			if !netproxy.Match(host, domains) {
				return Result{Decision: DecisionAsk, Reason: ReasonNetworkDomain, Domain: host}
			}
		}
		return Result{Decision: DecisionAllow}
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

func isStrongIsolation(req Request) bool {
	if req.Isolation.Source != "" || req.Isolation.Strong {
		return req.Isolation.Strong
	}
	return domain.ComputeEffectiveIsolation(req.Sandbox, domain.EnvironmentStatus{}).Strong
}

func matchPattern(pattern, name string) bool {
	if pattern == "*" {
		return true
	}
	ok, _ := filepath.Match(pattern, name)
	if ok {
		return true
	}
	// Exact suffix/prefix wildcards only — avoid bare substring false positives.
	p := strings.TrimSpace(pattern)
	if strings.HasPrefix(p, "*") && strings.HasSuffix(p, "*") && len(p) > 2 {
		return strings.Contains(name, p[1:len(p)-1])
	}
	if strings.HasPrefix(p, "*") {
		return strings.HasSuffix(name, strings.TrimPrefix(p, "*"))
	}
	if strings.HasSuffix(p, "*") {
		return strings.HasPrefix(name, strings.TrimSuffix(p, "*"))
	}
	return false
}
