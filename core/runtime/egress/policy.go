// Package egress owns Hard outbound-network policy helpers shared by the OS
// sandbox, OCI execution backend, Soft Gate consumers, and host HTTP tools.
package egress

import (
	"fmt"
	"strings"

	"danmo-work/core/adapter/container"
	"danmo-work/core/domain"
	"danmo-work/core/runtime/sandbox/netproxy"
)

// ProxyEnvKeys lists env vars stripped/replaced when injecting allowlist proxy.
var ProxyEnvKeys = map[string]struct{}{
	"HTTP_PROXY": {}, "HTTPS_PROXY": {}, "ALL_PROXY": {}, "NO_PROXY": {},
	"http_proxy": {}, "https_proxy": {}, "all_proxy": {}, "no_proxy": {},
}

// OSNetworkOpen reports whether OS runners should leave network available
// (no unshare-net / seatbelt deny). Allowlist opens OS net only while the
// host-side proxy address is attached to the run.
func OSNetworkOpen(network domain.SandboxNetwork, allowNetwork bool, allowlistProxy string) bool {
	if allowNetwork {
		return true
	}
	if network == domain.SandboxNetworkAllow {
		return true
	}
	if network == domain.SandboxNetworkAllowlist && strings.TrimSpace(allowlistProxy) != "" {
		return true
	}
	return false
}

// NeedNetDeny reports whether backend selection should prefer network isolation.
func NeedNetDeny(network domain.SandboxNetwork, allowlistProxyActive bool) bool {
	switch network {
	case domain.SandboxNetworkAllow:
		return false
	case domain.SandboxNetworkAllowlist:
		return !allowlistProxyActive
	default:
		return true
	}
}

// ContainerNetworkMode maps sandbox network policy to engine --network value.
// Empty string = engine default (full / vmnet). "none" = no egress. "host" =
// share host net (docker/podman allowlist path to reach loopback proxy).
func ContainerNetworkMode(network domain.SandboxNetwork, allowNetwork bool, engine string) string {
	if allowNetwork {
		return ""
	}
	apple := engine == string(domain.EnvironmentEngineAppleContainer)
	switch network {
	case domain.SandboxNetworkAllow:
		return ""
	case domain.SandboxNetworkAllowlist:
		if apple {
			// Apple Container has no host network — default vmnet + host.container.internal.
			return ""
		}
		return "host"
	default:
		return "none"
	}
}

// CheckHost applies Hard egress for a hostname (no port).
func CheckHost(cfg domain.ConfigSandboxSection, domains []string, host string) error {
	host = strings.ToLower(strings.TrimSpace(host))
	if host == "" {
		return fmt.Errorf("egress: empty host")
	}
	if !cfg.Enabled || cfg.Mode == domain.SandboxModeDangerFullAccess {
		return nil
	}
	switch cfg.Network {
	case domain.SandboxNetworkAllow:
		return nil
	case domain.SandboxNetworkAllowlist:
		if netproxy.Match(host, domains) {
			return nil
		}
		return fmt.Errorf("egress: host %q not in allowlist", host)
	default:
		return fmt.Errorf("egress: network deny blocks host %q", host)
	}
}

// ProxyEnvOpts configures BuildProxyEnv.
type ProxyEnvOpts struct {
	// ProxyAddr is host:port or full http(s):// URL from Sandbox.ProxyURL / ProxyAddr.
	ProxyAddr string
	// Engine is podman|docker|apple-container; apple rewrites loopback to host.container.internal.
	Engine string
	// ForContainer adds localhost / host.container.internal to NO_PROXY.
	ForContainer bool
}

// NormalizeProxyURL returns http://host:port (Apple-rewritten when needed).
func NormalizeProxyURL(proxyAddr, engine string) string {
	u := strings.TrimSpace(proxyAddr)
	if u == "" {
		return ""
	}
	if engine == string(domain.EnvironmentEngineAppleContainer) {
		return container.RewriteProxyForApple(u)
	}
	if !strings.HasPrefix(u, "http://") && !strings.HasPrefix(u, "https://") {
		return "http://" + u
	}
	return u
}

// BuildProxyEnv injects HTTP(S)_PROXY / ALL_PROXY / NO_PROXY into environ.
func BuildProxyEnv(environ []string, opts ProxyEnvOpts) []string {
	proxyURL := NormalizeProxyURL(opts.ProxyAddr, opts.Engine)
	if proxyURL == "" {
		return append([]string{}, environ...)
	}
	noProxy := ""
	if opts.ForContainer {
		noProxy = "localhost,127.0.0.1,host.container.internal"
	}
	out := make([]string, 0, len(environ)+8)
	for _, e := range environ {
		key, _, _ := strings.Cut(e, "=")
		if _, drop := ProxyEnvKeys[key]; drop {
			continue
		}
		out = append(out, e)
	}
	out = append(out,
		"HTTP_PROXY="+proxyURL,
		"HTTPS_PROXY="+proxyURL,
		"ALL_PROXY="+proxyURL,
		"NO_PROXY="+noProxy,
		"http_proxy="+proxyURL,
		"https_proxy="+proxyURL,
		"all_proxy="+proxyURL,
		"no_proxy="+noProxy,
	)
	return out
}

// StripProxyEnv removes proxy-related variables (deny-mode MCP children).
func StripProxyEnv(environ []string) []string {
	out := make([]string, 0, len(environ))
	for _, e := range environ {
		key, _, _ := strings.Cut(e, "=")
		if _, drop := ProxyEnvKeys[key]; drop {
			continue
		}
		out = append(out, e)
	}
	return out
}

// ProxyAddrFromURL strips http(s):// prefix to host:port for runners.
func ProxyAddrFromURL(proxyURL string) string {
	u := strings.TrimSpace(proxyURL)
	u = strings.TrimPrefix(u, "http://")
	u = strings.TrimPrefix(u, "https://")
	return u
}
