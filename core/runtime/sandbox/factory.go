package sandbox

import (
	"strings"

	"danmo-work/core/adapter/container"
	"danmo-work/core/domain"
	"danmo-work/core/port"
)

// BackendFactory builds and probes sandbox backends. Seatbelt, landlock,
// bwrap, win-token, wsl2, podman, docker, apple-container, and host-weak are
// all produced here; when the sandbox is disabled the factory returns the
// host-weak backend (direct OS execution).
type BackendFactory struct {
	// containerCache keeps container backend instances alive across reprobes
	// so per-project containers and loaded images survive config saves.
	containerCache map[string]*containerBackend
}

// NewBackendFactory returns a fresh backend factory.
func NewBackendFactory() *BackendFactory {
	return &BackendFactory{containerCache: make(map[string]*containerBackend)}
}

var _ port.BackendFactory = (*BackendFactory)(nil)

// containerEngines lists OCI engines exposed as unified sandbox backends.
var containerEngines = []domain.SandboxBackend{
	domain.SandboxBackendPodman,
	domain.SandboxBackendDocker,
	domain.SandboxBackendAppleContainer,
}

// Available probes all platform-relevant backends for the settings page.
func (f *BackendFactory) Available(cfg domain.ConfigSandboxSection) []domain.SandboxBackendInfo {
	infos := probeOSBackends()
	for _, eng := range containerEngines {
		info := domain.SandboxBackendInfo{
			Name:         eng,
			Container:    true,
			Capabilities: []string{"container-isolation", "fs-isolation", "network-control"},
		}
		if _, err := container.Detect(domain.EnvironmentEngine(eng)); err != nil {
			info.Reason = err.Error()
		} else {
			info.Available = true
		}
		infos = append(infos, info)
	}
	return infos
}

// Build constructs the backend selected by cfg. Disabled sandbox (or
// danger-full-access) returns host-weak; unknown/unavailable selections
// degrade to host-weak with a reason.
func (f *BackendFactory) Build(cfg domain.ConfigSandboxSection, allowlistProxyActive bool) (port.SandboxBackend, domain.SandboxBackend, bool, string, []string) {
	if !cfg.Enabled || cfg.Mode == domain.SandboxModeDangerFullAccess {
		return hostBackend{}, domain.SandboxBackendHostWeak, true, "sandbox disabled; direct host execution", []string{"host"}
	}

	force := normalizeBackendName(cfg.Backend)
	switch force {
	case string(domain.SandboxBackendPodman), string(domain.SandboxBackendDocker), string(domain.SandboxBackendAppleContainer):
		if cached, ok := f.containerCache[force]; ok {
			cached.Configure(cfg)
			return cached, domain.SandboxBackend(force), false, "", []string{force, "container-isolation", "fs-isolation", "network-control"}
		}
		b, err := newContainerBackend(domain.EnvironmentEngine(force), cfg)
		if err != nil {
			return hostBackend{}, domain.SandboxBackendHostWeak, true,
				force + " backend unavailable: " + err.Error(), []string{"host"}
		}
		f.containerCache[force] = b
		return b, domain.SandboxBackend(force), false, "", []string{force, "container-isolation", "fs-isolation", "network-control"}
	}
	return selectOSBackend(force, cfg, allowlistProxyActive)
}

// normalizeBackendName maps config/alias spellings to canonical backend ids.
func normalizeBackendName(raw string) string {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "host", "host-weak", "direct", "none":
		return string(domain.SandboxBackendHostWeak)
	case "seatbelt", "sandbox-exec":
		return string(domain.SandboxBackendSeatbelt)
	case "landlock":
		return string(domain.SandboxBackendLandlock)
	case "bwrap", "bubblewrap":
		return string(domain.SandboxBackendBwrap)
	case "win-token", "token", "win_token":
		return string(domain.SandboxBackendWinToken)
	case "wsl2", "wsl":
		return string(domain.SandboxBackendWSL2)
	case "podman":
		return string(domain.SandboxBackendPodman)
	case "docker":
		return string(domain.SandboxBackendDocker)
	case "apple-container", "apple", "container":
		return string(domain.SandboxBackendAppleContainer)
	case "", "auto":
		return "auto"
	default:
		return raw
	}
}
