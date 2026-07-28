package execution

import (
	"testing"

	"danmo-work/core/domain"
	"danmo-work/core/runtime/egress"
)

func TestNormalizeEnvDefaultsLocal(t *testing.T) {
	cfg := normalizeEnv(domain.ConfigEnvironmentSection{})
	if cfg.Backend != domain.EnvironmentBackendLocal {
		t.Fatalf("backend=%q", cfg.Backend)
	}
	if cfg.Image != defaultImage {
		t.Fatalf("image=%q", cfg.Image)
	}
	if cfg.WorkspaceMount != "" {
		t.Fatalf("mount should stay empty (same-as-host), got %q", cfg.WorkspaceMount)
	}
}

func TestResolveWorkspaceMount(t *testing.T) {
	host := "/Users/me/proj"
	if got := resolveWorkspaceMount("", host); got != host {
		t.Fatalf("empty → %q", got)
	}
	if got := resolveWorkspaceMount("same", host); got != host {
		t.Fatalf("same → %q", got)
	}
	if got := resolveWorkspaceMount("/workspace", host); got != "/workspace" {
		t.Fatalf("explicit → %q", got)
	}
}

func TestNormalizeEnvContainerAliases(t *testing.T) {
	for _, b := range []string{"container", "oci"} {
		cfg := normalizeEnv(domain.ConfigEnvironmentSection{Backend: domain.EnvironmentBackend(b)})
		if cfg.Backend != domain.EnvironmentBackendContainer {
			t.Fatalf("%s → %q", b, cfg.Backend)
		}
	}
	cfg := normalizeEnv(domain.ConfigEnvironmentSection{
		Backend: domain.EnvironmentBackendContainer,
		Engine:  "apple",
	})
	if cfg.Engine != domain.EnvironmentEngineAppleContainer {
		t.Fatalf("engine=%q", cfg.Engine)
	}
}

func TestSanitizeID(t *testing.T) {
	got := sanitizeID("abc_DEF-12")
	if got != "abc_DEF-12" {
		t.Fatalf("got %q", got)
	}
	got = sanitizeID("proj/../x")
	if got != "proj----x" {
		t.Fatalf("got %q", got)
	}
}

func TestStatusDegradesWithoutEngine(t *testing.T) {
	m := New(domain.ConfigEnvironmentSection{Backend: domain.EnvironmentBackendContainer}, domain.ConfigSandboxSection{}, nil)
	st := m.Status()
	if !st.Degraded {
		t.Fatal("expected degraded without podman/docker or tar")
	}
	if st.Backend != domain.EnvironmentBackendLocal {
		t.Fatalf("effective backend=%q", st.Backend)
	}
}

func TestContainerNetwork(t *testing.T) {
	deny := egress.ContainerNetworkMode(domain.SandboxNetworkDeny, false, "podman")
	if deny != "none" {
		t.Fatalf("deny=%q", deny)
	}
	alist := egress.ContainerNetworkMode(domain.SandboxNetworkAllowlist, false, "docker")
	if alist != "host" {
		t.Fatalf("allowlist=%q", alist)
	}
	appleAlist := egress.ContainerNetworkMode(domain.SandboxNetworkAllowlist, false, "apple-container")
	if appleAlist != "" {
		t.Fatalf("apple allowlist=%q", appleAlist)
	}
	allow := egress.ContainerNetworkMode(domain.SandboxNetworkAllow, false, "podman")
	if allow != "" {
		t.Fatalf("allow=%q", allow)
	}
}
