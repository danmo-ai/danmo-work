package permission

import (
	"testing"

	"danmo-work/core/domain"
)

func strongSB(net domain.SandboxNetwork) domain.SandboxStatus {
	return domain.SandboxStatus{
		Enabled: true,
		Mode:    domain.SandboxModeWorkspaceWrite,
		Network: net,
		Backend: domain.SandboxBackendSeatbelt,
	}
}

func TestGateSandboxedGitStatusAllow(t *testing.T) {
	g := NewGate(nil)
	r := g.CheckRequest(Request{
		ToolName: "exec_shell",
		Risk:     domain.RiskHigh,
		Command:  "git status",
		Sandbox:  strongSB(domain.SandboxNetworkDeny),
	})
	if r.Decision != DecisionAllow {
		t.Fatalf("got %+v", r)
	}
}

func TestGateDangerousAsk(t *testing.T) {
	g := NewGate(nil)
	r := g.CheckRequest(Request{
		ToolName: "exec_shell",
		Risk:     domain.RiskHigh,
		Command:  "rm -rf /tmp/foo",
		Sandbox:  strongSB(domain.SandboxNetworkDeny),
	})
	if r.Decision != DecisionAsk || r.Reason != ReasonDangerousCommand {
		t.Fatalf("got %+v", r)
	}
}

func TestGateNetworkAskAndSessionAllow(t *testing.T) {
	g := NewGate(nil)
	r := g.CheckRequest(Request{
		ToolName: "exec_shell",
		Risk:     domain.RiskHigh,
		Command:  "npm install lodash",
		Sandbox:  strongSB(domain.SandboxNetworkDeny),
	})
	if r.Decision != DecisionAsk || r.Reason != ReasonNetwork {
		t.Fatalf("got %+v", r)
	}
	r2 := g.CheckRequest(Request{
		ToolName:      "exec_shell",
		Risk:          domain.RiskHigh,
		Command:       "npm install lodash",
		Sandbox:       strongSB(domain.SandboxNetworkDeny),
		SessionGrants: map[string]bool{GrantKeyNetwork: true},
	})
	if r2.Decision != DecisionAllow {
		t.Fatalf("got %+v", r2)
	}
}

func TestGateAllowlistAsksUnknownDomain(t *testing.T) {
	g := NewGate(nil)
	sb := strongSB(domain.SandboxNetworkAllowlist)
	sb.AllowlistDomains = []string{"example.com"}
	r := g.CheckRequest(Request{
		ToolName: "exec_shell",
		Risk:     domain.RiskHigh,
		Command:  "npm install lodash",
		Sandbox:  sb,
	})
	if r.Decision != DecisionAsk || r.Reason != ReasonNetworkDomain {
		t.Fatalf("expected network_domain ask, got %+v", r)
	}
	if r.Domain != "registry.npmjs.org" {
		t.Fatalf("domain=%q", r.Domain)
	}
}

func TestGateAllowlistAllowsListedDomain(t *testing.T) {
	g := NewGate(nil)
	sb := strongSB(domain.SandboxNetworkAllowlist)
	sb.AllowlistDomains = []string{"registry.npmjs.org", "*.npmjs.org"}
	r := g.CheckRequest(Request{
		ToolName: "exec_shell",
		Risk:     domain.RiskHigh,
		Command:  "npm install lodash",
		Sandbox:  sb,
	})
	if r.Decision != DecisionAllow {
		t.Fatalf("expected allow, got %+v", r)
	}
}

func TestGateHostEgressDenyAsks(t *testing.T) {
	g := NewGate(nil)
	r := g.CheckRequest(Request{
		ToolName: "web_fetch",
		Risk:     domain.RiskLow,
		URL:      "https://example.com/a",
		Sandbox:  strongSB(domain.SandboxNetworkDeny),
	})
	if r.Decision != DecisionAsk || r.Reason != ReasonNetwork {
		t.Fatalf("got %+v", r)
	}
}

func TestAutoApprovableCeiling(t *testing.T) {
	if AutoApprovable(ReasonDangerousCommand) || AutoApprovable(ReasonUnsandboxed) {
		t.Fatal("dangerous/unsandboxed must not auto-approve")
	}
	if !AutoApprovable(ReasonNetwork) || !AutoApprovable(ReasonNetworkDomain) {
		t.Fatal("network reasons should be auto-approvable")
	}
}

func TestPermissionRuleDeny(t *testing.T) {
	g := NewGate([]domain.PermissionRule{{Pattern: "web_*", Action: domain.PermDeny}})
	r := g.CheckRequest(Request{
		ToolName: "web_fetch",
		Risk:     domain.RiskLow,
		URL:      "https://x.com",
		Sandbox:  strongSB(domain.SandboxNetworkAllow),
	})
	if r.Decision != DecisionDeny || r.Reason != ReasonRuleDeny {
		t.Fatalf("got %+v", r)
	}
}

func TestGateHostWeakAsk(t *testing.T) {
	g := NewGate(nil)
	r := g.CheckRequest(Request{
		ToolName: "exec_shell",
		Risk:     domain.RiskHigh,
		Command:  "ls",
		Sandbox: domain.SandboxStatus{
			Enabled: true,
			Mode:    domain.SandboxModeWorkspaceWrite,
			Backend: domain.SandboxBackendHostWeak,
		},
	})
	if r.Decision != DecisionAsk || r.Reason != ReasonUnsandboxed {
		t.Fatalf("got %+v", r)
	}
}

func TestGateMediumAllow(t *testing.T) {
	g := NewGate(nil)
	r := g.CheckRequest(Request{ToolName: "write", Risk: domain.RiskMedium})
	if r.Decision != DecisionAllow {
		t.Fatalf("got %+v", r)
	}
}

func TestHeuristics(t *testing.T) {
	if !LooksDangerous("sudo apt install x") {
		t.Fatal("sudo")
	}
	if !LooksLikeNetwork("curl https://example.com") {
		t.Fatal("curl")
	}
	if LooksLikeNetwork("git status") {
		t.Fatal("git status should not need net")
	}
}

func TestEffectiveHTTPRequestRisk(t *testing.T) {
	if got := EffectiveHTTPRequestRisk(domain.RiskMedium, "GET", nil); got != domain.RiskMedium {
		t.Fatalf("GET = %s", got)
	}
	if got := EffectiveHTTPRequestRisk(domain.RiskMedium, "POST", nil); got != domain.RiskExternal {
		t.Fatalf("POST = %s", got)
	}
	if got := EffectiveHTTPRequestRisk(domain.RiskMedium, "GET", map[string]string{"Authorization": "Bearer x"}); got != domain.RiskExternal {
		t.Fatalf("auth header = %s", got)
	}
	if got := EffectiveHTTPRequestRisk(domain.RiskMedium, "GET", map[string]string{"X-Custom": "1"}); got != domain.RiskMedium {
		t.Fatalf("custom header = %s", got)
	}
}

func TestEffectiveFileOpRisk(t *testing.T) {
	if got := EffectiveFileOpRisk(domain.RiskMedium, "move"); got != domain.RiskMedium {
		t.Fatalf("move = %s", got)
	}
	if got := EffectiveFileOpRisk(domain.RiskMedium, "copy"); got != domain.RiskMedium {
		t.Fatalf("copy = %s", got)
	}
	if got := EffectiveFileOpRisk(domain.RiskMedium, "delete"); got != domain.RiskHigh {
		t.Fatalf("delete = %s", got)
	}
	if got := EffectiveFileOpRisk(domain.RiskMedium, " DELETE "); got != domain.RiskHigh {
		t.Fatalf("padded delete = %s", got)
	}
}

func TestGateExternalAsk(t *testing.T) {
	g := NewGate(nil)
	r := g.CheckRequest(Request{ToolName: "mcp_notion_search", Risk: domain.RiskExternal})
	if r.Decision != DecisionAsk || r.Reason != ReasonExternal {
		t.Fatalf("got %+v", r)
	}
}

func TestGateDiscussDenyWrite(t *testing.T) {
	g := NewGate(nil)
	r := g.CheckRequest(Request{ToolName: "write", Risk: domain.RiskMedium, Mode: domain.PermModeDiscuss})
	if r.Decision != DecisionDeny {
		t.Fatalf("got %+v", r)
	}
}

func TestGateComputerAlwaysAsks(t *testing.T) {
	g := NewGate(nil)
	// Interactive mode: ask with desktop_control reason.
	r := g.CheckRequest(Request{ToolName: "computer", Risk: domain.RiskHigh})
	if r.Decision != DecisionAsk || r.Reason != ReasonDesktopControl {
		t.Fatalf("interactive got %+v", r)
	}
	// Auto mode must NOT auto-allow desktop control.
	r2 := g.CheckRequest(Request{ToolName: "computer", Risk: domain.RiskHigh, Mode: domain.PermModeAuto})
	if r2.Decision != DecisionAsk || r2.Reason != ReasonDesktopControl {
		t.Fatalf("auto got %+v", r2)
	}
	if AutoApprovable(r2.Reason) {
		t.Fatal("desktop_control must never be auto-approvable")
	}
}

func TestGateSessionGrantIsGeneric(t *testing.T) {
	g := NewGate(nil)

	// Any tool ask can be remembered for the session under tool:<name>.
	granted := map[string]bool{GrantKeyToolPrefix + "computer": true}
	if r := g.CheckRequest(Request{ToolName: "computer", Risk: domain.RiskHigh, SessionGrants: granted}); r.Decision != DecisionAllow {
		t.Fatalf("computer session grant got %+v", r)
	}
	// The grant is per tool, not a blanket bypass.
	if r := g.CheckRequest(Request{ToolName: "mcp_notion_search", Risk: domain.RiskExternal, SessionGrants: granted}); r.Decision != DecisionAsk {
		t.Fatalf("grant must not leak to another tool, got %+v", r)
	}
	// Same generic key works for an external MCP tool.
	mcpGranted := map[string]bool{GrantKeyToolPrefix + "mcp_notion_search": true}
	if r := g.CheckRequest(Request{ToolName: "mcp_notion_search", Risk: domain.RiskExternal, SessionGrants: mcpGranted}); r.Decision != DecisionAllow {
		t.Fatalf("mcp session grant got %+v", r)
	}
	// Read-only modes still deny even with a stale grant.
	if r := g.CheckRequest(Request{ToolName: "computer", Risk: domain.RiskHigh, Mode: domain.PermModePlan, SessionGrants: granted}); r.Decision != DecisionDeny {
		t.Fatalf("plan + grant got %+v", r)
	}
	// Deny rules win over a session grant.
	denyGate := NewGate([]domain.PermissionRule{{Pattern: "computer", Action: domain.PermDeny}})
	if r := denyGate.CheckRequest(Request{ToolName: "computer", Risk: domain.RiskHigh, SessionGrants: granted}); r.Decision != DecisionDeny {
		t.Fatalf("deny rule + grant got %+v", r)
	}
}

func TestSessionGrantKeyScope(t *testing.T) {
	if got := SessionGrantKey(ReasonDangerousCommand, "", "exec_shell"); got != "" {
		t.Fatalf("dangerous command must not be session-grantable, got %q", got)
	}
	if got := SessionGrantKey(ReasonUnsandboxed, "", "exec_shell"); got != "" {
		t.Fatalf("unsandboxed must not be session-grantable, got %q", got)
	}
	if got := SessionGrantKey(ReasonNetwork, "", "exec_shell"); got != GrantKeyNetwork {
		t.Fatalf("network key = %q", got)
	}
	if got := SessionGrantKey(ReasonNetworkDomain, "api.example.com", "exec_shell"); got != GrantKeyDomainPrefix+"api.example.com" {
		t.Fatalf("domain key = %q", got)
	}
	if got := SessionGrantKey(ReasonDesktopSandbox, "", "computer"); got != GrantKeyToolPrefix+"computer" {
		t.Fatalf("desktop key = %q", got)
	}
	if got := SessionGrantKey(ReasonHighRisk, "", "write"); got != GrantKeyToolPrefix+"write" {
		t.Fatalf("high risk key = %q", got)
	}
}

func TestGateDangerousCommandNeverSessionGranted(t *testing.T) {
	g := NewGate(nil)
	// Even a (hypothetical) stale tool grant must not widen to a dangerous command.
	r := g.CheckRequest(Request{
		ToolName:      "exec_shell",
		Risk:          domain.RiskHigh,
		Command:       "rm -rf /",
		Sandbox:       strongSB(domain.SandboxNetworkAllow),
		SessionGrants: map[string]bool{GrantKeyToolPrefix + "exec_shell": true},
	})
	if r.Decision != DecisionAsk || r.Reason != ReasonDangerousCommand {
		t.Fatalf("got %+v", r)
	}
}

func TestGateComputerDeniedInPlan(t *testing.T) {
	g := NewGate(nil)
	r := g.CheckRequest(Request{ToolName: "computer", Risk: domain.RiskHigh, Mode: domain.PermModePlan})
	if r.Decision != DecisionDeny {
		t.Fatalf("plan got %+v", r)
	}
}

func TestGateContainerIsolationStrong(t *testing.T) {
	g := NewGate(nil)
	weak := domain.SandboxStatus{
		Enabled: true,
		Mode:    domain.SandboxModeWorkspaceWrite,
		Network: domain.SandboxNetworkDeny,
		Backend: domain.SandboxBackendHostWeak,
	}
	iso := domain.ComputeEffectiveIsolation(weak, domain.EnvironmentStatus{
		Backend:  domain.EnvironmentBackendContainer,
		Engine:   "podman",
		Degraded: false,
	})
	if !iso.Strong || iso.Source != domain.IsolationContainer {
		t.Fatalf("iso=%+v", iso)
	}
	r := g.CheckRequest(Request{
		ToolName:  "exec_shell",
		Risk:      domain.RiskHigh,
		Command:   "ls",
		Sandbox:   weak,
		Isolation: iso,
	})
	if r.Decision != DecisionAllow {
		t.Fatalf("container should be strong: %+v", r)
	}
}

func TestGateWebSearchAllowlistAsksProviderHost(t *testing.T) {
	g := NewGate(nil)
	sb := strongSB(domain.SandboxNetworkAllowlist)
	sb.AllowlistDomains = []string{"example.com"}
	r := g.CheckRequest(Request{
		ToolName:       "web_search",
		Risk:           domain.RiskLow,
		SearchProvider: string(domain.SearchProviderTavily),
		Sandbox:        sb,
	})
	if r.Decision != DecisionAsk || r.Reason != ReasonNetworkDomain || r.Domain != "api.tavily.com" {
		t.Fatalf("got %+v", r)
	}
}
