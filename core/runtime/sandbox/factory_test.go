package sandbox

import (
	"testing"

	"danmo-work/core/adapter/container"
	"danmo-work/core/domain"
)

func TestFactoryDisabledReturnsHostBackend(t *testing.T) {
	f := NewBackendFactory()
	b, name, degraded, reason, caps := f.Build(domain.ConfigSandboxSection{Enabled: false}, false)
	if name != domain.SandboxBackendHostWeak || b.Name() != domain.SandboxBackendHostWeak {
		t.Fatalf("name=%s backend=%s", name, b.Name())
	}
	if !degraded || reason == "" {
		t.Fatalf("degraded=%v reason=%q", degraded, reason)
	}
	if len(caps) == 0 {
		t.Fatal("caps empty")
	}
}

func TestFactoryDangerFullAccessReturnsHostBackend(t *testing.T) {
	f := NewBackendFactory()
	b, name, _, _, _ := f.Build(domain.ConfigSandboxSection{
		Enabled: true,
		Mode:    domain.SandboxModeDangerFullAccess,
	}, false)
	if name != domain.SandboxBackendHostWeak || b.Name() != domain.SandboxBackendHostWeak {
		t.Fatalf("name=%s backend=%s", name, b.Name())
	}
}

func TestFactoryContainerBackendMissingEngineDegrades(t *testing.T) {
	f := NewBackendFactory()
	for _, eng := range []string{"podman", "docker", "apple-container"} {
		if _, err := container.Detect(domain.EnvironmentEngine(eng)); err == nil {
			continue // engine installed on this host; nothing to assert
		}
		b, name, degraded, reason, _ := f.Build(domain.ConfigSandboxSection{
			Enabled: true,
			Backend: eng,
		}, false)
		if name != domain.SandboxBackendHostWeak || b.Name() != domain.SandboxBackendHostWeak {
			t.Fatalf("engine=%s name=%s backend=%s", eng, name, b.Name())
		}
		if !degraded || reason == "" {
			t.Fatalf("engine=%s degraded=%v reason=%q", eng, degraded, reason)
		}
		return
	}
	t.Skip("all container engines available on this host")
}

func TestFactoryAvailableListsBackends(t *testing.T) {
	f := NewBackendFactory()
	infos := f.Available(domain.ConfigSandboxSection{Enabled: true})
	if len(infos) < 3 {
		t.Fatalf("expected at least host-weak + 2 container engines, got %d", len(infos))
	}
	seen := map[domain.SandboxBackend]bool{}
	for _, i := range infos {
		seen[i.Name] = true
	}
	if !seen[domain.SandboxBackendHostWeak] {
		t.Fatal("host-weak should always be listed")
	}
	if !seen[domain.SandboxBackendPodman] || !seen[domain.SandboxBackendDocker] || !seen[domain.SandboxBackendAppleContainer] {
		t.Fatalf("container engines missing from list: %v", infos)
	}
}

func TestFactoryNormalizeBackendAliases(t *testing.T) {
	cases := map[string]string{
		"":           "auto",
		"auto":       "auto",
		"host":       "host-weak",
		"host-weak":  "host-weak",
		"bubblewrap": "bwrap",
		"wsl":        "wsl2",
		"token":      "win-token",
		"apple":      "apple-container",
		"container":  "apple-container",
	}
	for in, want := range cases {
		if got := normalizeBackendName(in); got != want {
			t.Fatalf("normalizeBackendName(%q)=%q want %q", in, got, want)
		}
	}
}
