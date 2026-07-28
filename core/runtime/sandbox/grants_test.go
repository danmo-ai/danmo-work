package sandbox

import (
	"testing"

	"danmo-work/core/domain"
)

func TestSessionAndTurnDomainGrants(t *testing.T) {
	m := New(domain.ConfigSandboxSection{
		Enabled:          true,
		Mode:             domain.SandboxModeWorkspaceWrite,
		Network:          domain.SandboxNetworkAllowlist,
		AllowlistDomains: []string{"config.example.com"},
	})
	defer m.Close()

	if err := m.CheckHost("config.example.com"); err != nil {
		t.Fatal(err)
	}
	if err := m.CheckHost("once.example.com"); err == nil {
		t.Fatal("once should be blocked before grant")
	}

	m.GrantTurnDomains("turn-1", []string{"once.example.com"})
	if err := m.CheckHost("once.example.com"); err != nil {
		t.Fatal(err)
	}
	m.ClearTurnDomains("turn-1")
	if err := m.CheckHost("once.example.com"); err == nil {
		t.Fatal("once should be revoked after clear")
	}

	m.GrantSessionDomains("sess-a", []string{"sess.example.com"})
	if err := m.CheckHost("sess.example.com"); err != nil {
		t.Fatal(err)
	}
	m.RevokeSessionDomains("sess-a")
	if err := m.CheckHost("sess.example.com"); err == nil {
		t.Fatal("session grant should be revoked")
	}
}
