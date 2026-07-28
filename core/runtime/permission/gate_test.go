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

func TestGateAllowlistDoesNotAskNetwork(t *testing.T) {
	g := NewGate(nil)
	r := g.CheckRequest(Request{
		ToolName: "exec_shell",
		Risk:     domain.RiskHigh,
		Command:  "npm install lodash",
		Sandbox:  strongSB(domain.SandboxNetworkAllowlist),
	})
	// Proxy enforces domains; no ReasonNetwork ask (still may ask for other reasons).
	if r.Reason == ReasonNetwork {
		t.Fatalf("allowlist should not ask for network: %+v", r)
	}
	if r.Decision != DecisionAllow {
		t.Fatalf("expected allow for sandboxed npm under allowlist, got %+v", r)
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
