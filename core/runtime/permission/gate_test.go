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
		ToolName:            "exec_shell",
		Risk:                domain.RiskHigh,
		Command:             "npm install lodash",
		Sandbox:             strongSB(domain.SandboxNetworkDeny),
		SessionAllowNetwork: true,
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
