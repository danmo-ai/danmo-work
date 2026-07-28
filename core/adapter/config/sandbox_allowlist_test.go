package config

import (
	"os"
	"path/filepath"
	"testing"

	"danmo-work/core/domain"
)

func TestLoadAllowlistDomains(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	yaml := `
runtime:
  sandbox:
    enabled: true
    mode: workspace-write
    network: allowlist
    allowlist_domains:
      - pypi.org
      - "*.pythonhosted.org"
`
	if err := os.WriteFile(path, []byte(yaml), 0600); err != nil {
		t.Fatal(err)
	}
	l := NewLoader(path)
	cfg, err := l.Load(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	sb := cfg.Runtime.Sandbox
	if sb.Network != domain.SandboxNetworkAllowlist {
		t.Fatalf("network=%s", sb.Network)
	}
	if len(sb.AllowlistDomains) != 2 {
		t.Fatalf("domains=%v", sb.AllowlistDomains)
	}
	if sb.AllowlistDomains[0] != "pypi.org" || sb.AllowlistDomains[1] != "*.pythonhosted.org" {
		t.Fatalf("domains=%v", sb.AllowlistDomains)
	}
}

func TestSaveAllowlistDomainsRoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	l := NewLoader(path)
	cfg, err := l.Load(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	cfg.Runtime.Sandbox = domain.ConfigSandboxSection{
		Enabled:          true,
		Mode:             domain.SandboxModeWorkspaceWrite,
		Network:          domain.SandboxNetworkAllowlist,
		AllowlistDomains: []string{"registry.npmjs.org", "*.npmjs.org"},
	}
	if err := l.Save(t.Context(), cfg); err != nil {
		t.Fatal(err)
	}
	cfg2, err := l.Load(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	got := cfg2.Runtime.Sandbox.AllowlistDomains
	if len(got) != 2 || got[0] != "registry.npmjs.org" {
		t.Fatalf("round-trip domains=%v", got)
	}
}
