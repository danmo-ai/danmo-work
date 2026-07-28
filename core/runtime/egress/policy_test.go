package egress

import (
	"strings"
	"testing"

	"danmo-work/core/domain"
)

func TestOSNetworkOpen(t *testing.T) {
	if OSNetworkOpen(domain.SandboxNetworkDeny, false, "") {
		t.Fatal("deny should be closed")
	}
	if !OSNetworkOpen(domain.SandboxNetworkDeny, true, "") {
		t.Fatal("AllowNetwork override")
	}
	if !OSNetworkOpen(domain.SandboxNetworkAllow, false, "") {
		t.Fatal("allow")
	}
	if OSNetworkOpen(domain.SandboxNetworkAllowlist, false, "") {
		t.Fatal("allowlist without proxy")
	}
	if !OSNetworkOpen(domain.SandboxNetworkAllowlist, false, "127.0.0.1:9") {
		t.Fatal("allowlist with proxy")
	}
}

func TestContainerNetworkMode(t *testing.T) {
	if got := ContainerNetworkMode(domain.SandboxNetworkDeny, false, "podman"); got != "none" {
		t.Fatalf("deny=%q", got)
	}
	if got := ContainerNetworkMode(domain.SandboxNetworkAllowlist, false, "docker"); got != "host" {
		t.Fatalf("docker allowlist=%q", got)
	}
	if got := ContainerNetworkMode(domain.SandboxNetworkAllowlist, false, "apple-container"); got != "" {
		t.Fatalf("apple allowlist=%q", got)
	}
	if got := ContainerNetworkMode(domain.SandboxNetworkAllow, false, "podman"); got != "" {
		t.Fatalf("allow=%q", got)
	}
}

func TestBuildProxyEnvUnifiedNOProxy(t *testing.T) {
	local := BuildProxyEnv(nil, ProxyEnvOpts{ProxyAddr: "127.0.0.1:1", ForContainer: false})
	joined := strings.Join(local, "\n")
	if !strings.Contains(joined, "NO_PROXY=") || strings.Contains(joined, "NO_PROXY=localhost") {
		t.Fatalf("local NO_PROXY should be empty, got %v", local)
	}
	ctr := BuildProxyEnv(nil, ProxyEnvOpts{ProxyAddr: "127.0.0.1:1", ForContainer: true, Engine: "podman"})
	joined = strings.Join(ctr, "\n")
	if !strings.Contains(joined, "host.container.internal") {
		t.Fatalf("container NO_PROXY missing host alias: %v", ctr)
	}
}

func TestCheckHostAllowlist(t *testing.T) {
	cfg := domain.ConfigSandboxSection{
		Enabled: true,
		Mode:    domain.SandboxModeWorkspaceWrite,
		Network: domain.SandboxNetworkAllowlist,
	}
	if err := CheckHost(cfg, []string{"example.com"}, "example.com"); err != nil {
		t.Fatal(err)
	}
	if err := CheckHost(cfg, []string{"example.com"}, "evil.com"); err == nil {
		t.Fatal("expected deny")
	}
}
