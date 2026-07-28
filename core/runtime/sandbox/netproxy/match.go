// Package netproxy provides a loopback HTTP CONNECT proxy that enforces
// domain allowlists for sandboxed agent shells.
package netproxy

import (
	"net"
	"strings"
)

// NormalizeDomains trims, lowercases, and drops empty rules.
func NormalizeDomains(domains []string) []string {
	out := make([]string, 0, len(domains))
	seen := make(map[string]struct{}, len(domains))
	for _, d := range domains {
		d = strings.ToLower(strings.TrimSpace(d))
		if d == "" {
			continue
		}
		if _, ok := seen[d]; ok {
			continue
		}
		seen[d] = struct{}{}
		out = append(out, d)
	}
	return out
}

// Match reports whether host (without port) is permitted by rules.
// Rules are exact hosts or "*.example.com" suffix wildcards.
// IP literals and empty hosts are always rejected.
func Match(host string, rules []string) bool {
	host = strings.ToLower(strings.TrimSpace(host))
	if host == "" {
		return false
	}
	// Strip brackets from IPv6 literals if present.
	if strings.HasPrefix(host, "[") && strings.HasSuffix(host, "]") {
		host = host[1 : len(host)-1]
	}
	if ip := net.ParseIP(host); ip != nil {
		return false
	}
	if isBlockedHostname(host) {
		return false
	}
	for _, rule := range rules {
		rule = strings.ToLower(strings.TrimSpace(rule))
		if rule == "" {
			continue
		}
		if strings.HasPrefix(rule, "*.") {
			suffix := rule[1:] // ".example.com"
			base := rule[2:]   // "example.com"
			if host == base || strings.HasSuffix(host, suffix) {
				return true
			}
			continue
		}
		if host == rule {
			return true
		}
	}
	return false
}

func isBlockedHostname(host string) bool {
	switch host {
	case "localhost", "metadata.google.internal":
		return true
	}
	if strings.HasSuffix(host, ".localhost") || strings.HasSuffix(host, ".local") {
		return true
	}
	return false
}

// SplitHostPort extracts host from host:port or bare host (defaultPort unused for matching).
func SplitHostPort(authority string) (host string, port string, err error) {
	authority = strings.TrimSpace(authority)
	if authority == "" {
		return "", "", errEmptyHost
	}
	if strings.HasPrefix(authority, "[") {
		// [ipv6]:port or [ipv6]
		host, port, err = net.SplitHostPort(authority)
		if err != nil {
			// bare [ipv6]
			h := strings.TrimPrefix(authority, "[")
			h = strings.TrimSuffix(h, "]")
			if net.ParseIP(h) != nil {
				return h, "", nil
			}
			return "", "", err
		}
		return host, port, nil
	}
	if strings.Count(authority, ":") == 1 {
		return net.SplitHostPort(authority)
	}
	// bare hostname or IPv4 without port
	if net.ParseIP(authority) != nil || !strings.Contains(authority, ":") {
		return authority, "", nil
	}
	return net.SplitHostPort(authority)
}

type emptyHostError struct{}

func (emptyHostError) Error() string { return "empty host" }

var errEmptyHost = emptyHostError{}
