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
	ReasonDesktopControl   = domain.ReasonDesktopControl // computer tool: host GUI control
	ReasonDesktopSandbox   = domain.ReasonDesktopSandbox // computer tool: OCI desktop profile
)

// Request is the structured input for permission checks.
type Request struct {
	ToolName       string
	Risk           domain.RiskLevel
	Command        string // exec_shell command when applicable
	URL            string // host-egress tools (http_request / web_fetch)
	SearchProvider string // web_search configured provider (for Soft allowlist)
	SearchBaseURL  string // web_search base_url (SearXNG / custom DDG)
	Sandbox        domain.SandboxStatus
	Isolation      domain.EffectiveIsolation // preferred over Sandbox for strength
	SessionGrants  map[string]bool           // session-scope approvals (see SessionGrantKey)
	Mode           domain.PermissionMode
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

// Session-scope approval keys. A user decision made with scope=session is
// remembered under the identity of what was approved, so the same ask does not
// prompt again for the rest of the session. This is the only mechanism; tools
// are not special-cased.
const (
	// GrantKeyNetwork is the deny-mode full-egress escape.
	GrantKeyNetwork = "network"
	// GrantKeyDomainPrefix + host grants one allowlisted host.
	GrantKeyDomainPrefix = "domain:"
	// GrantKeyToolPrefix + tool name grants one tool.
	GrantKeyToolPrefix = "tool:"
)

// SessionGrantKey returns the session-scope identity of an Ask result, or ""
// when the reason must always ask again. Reasons with no safe session-wide
// identity (a dangerous command or an unsandboxed shell can be anything) are
// never remembered, so "allow for session" cannot widen into "allow any
// command".
func SessionGrantKey(reason, hostDomain, toolName string) string {
	switch reason {
	case ReasonNetwork:
		return GrantKeyNetwork
	case ReasonNetworkDomain:
		if hostDomain == "" {
			return ""
		}
		return GrantKeyDomainPrefix + hostDomain
	case ReasonDangerousCommand, ReasonUnsandboxed, ReasonRuleDeny, ReasonModeDeny:
		return ""
	}
	if toolName == "" {
		return ""
	}
	return GrantKeyToolPrefix + toolName
}

// grantDomains returns the hosts granted for this session.
func (req Request) grantDomains() []string {
	if len(req.SessionGrants) == 0 {
		return nil
	}
	out := make([]string, 0, len(req.SessionGrants))
	for k, ok := range req.SessionGrants {
		if ok && strings.HasPrefix(k, GrantKeyDomainPrefix) {
			out = append(out, strings.TrimPrefix(k, GrantKeyDomainPrefix))
		}
	}
	return out
}

// Check evaluates tool permission. Prefer CheckRequest for sandbox-aware policy.
func (g *Gate) Check(toolName string, risk domain.RiskLevel) Decision {
	return g.CheckRequest(Request{ToolName: toolName, Risk: risk}).Decision
}

// CheckRequest applies mode + Soft Gate policy (Hard enforcement is separate),
// then lets an explicit session-scope approval override an Ask. Deny always
// wins, so read-only modes and deny rules cannot be bypassed by a stale grant.
func (g *Gate) CheckRequest(req Request) Result {
	res := g.evaluate(req)
	if res.Decision != DecisionAsk {
		return res
	}
	if key := SessionGrantKey(res.Reason, res.Domain, req.ToolName); key != "" && req.SessionGrants[key] {
		return Result{Decision: DecisionAllow}
	}
	return res
}

func (g *Gate) evaluate(req Request) Result {
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
		if req.ToolName == "write" || req.ToolName == "edit" || req.ToolName == "edit_batch" || req.ToolName == "apply_patch" {
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

	// Desktop control always asks at least once; never let auto_approve skip
	// it. The generic session grant (tool:computer) is applied by
	// CheckRequest, so an explicit "allow for session" stops further prompts.
	// Container desktop profile uses a distinct reason so the UI can say
	// "sandbox desktop" instead of "host GUI".
	if req.ToolName == "computer" {
		reason := ReasonDesktopControl
		iso := req.Isolation
		if iso.Source == "" {
			iso = domain.ComputeEffectiveIsolation(req.Sandbox, domain.EnvironmentStatus{})
		}
		if iso.Strong && iso.Source == domain.IsolationContainer {
			reason = domain.ReasonDesktopSandbox
		}
		return Result{Decision: DecisionAsk, Reason: reason}
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
	case "http_request", "web_fetch", "web_search", "browser_navigate":
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
		return mergeDomains(req.Isolation.AllowlistDomains, req.grantDomains())
	}
	return mergeDomains(req.Sandbox.AllowlistDomains, req.grantDomains())
}

func checkShellNetwork(req Request) Result {
	switch networkOf(req) {
	case domain.SandboxNetworkDeny:
		if !LooksLikeNetwork(req.Command) {
			return Result{Decision: DecisionAllow}
		}
		// A session-wide egress grant is applied by CheckRequest.
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
