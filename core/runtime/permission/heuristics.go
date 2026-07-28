package permission

import (
	"net/http"
	"regexp"
	"strings"

	"danmo-work/core/domain"
)

var (
	reRmRf     = regexp.MustCompile(`(?i)\brm\s+(-[a-zA-Z]*r[a-zA-Z]*f|-rf|-fr)\b`)
	reSudo     = regexp.MustCompile(`(?i)\bsudo\b`)
	reMkfs     = regexp.MustCompile(`(?i)\bmkfs(\.\w+)?\b`)
	reDdIf     = regexp.MustCompile(`(?i)\bdd\s+.*\bif=`)
	reCurlSh   = regexp.MustCompile(`(?i)\b(curl|wget)\b[^|;]*\|\s*(ba)?sh\b`)
	reChmod777 = regexp.MustCompile(`(?i)\bchmod\s+(-R\s+)?777\b`)
	reSSHWrite = regexp.MustCompile(`(?i)(~|/home/[^/\s]+)/\.ssh\b`)
)

// LooksDangerous reports commands that should always ask even inside a strong sandbox.
func LooksDangerous(cmd string) bool {
	c := strings.TrimSpace(cmd)
	if c == "" {
		return false
	}
	switch {
	case reSudo.MatchString(c),
		reRmRf.MatchString(c),
		reMkfs.MatchString(c),
		reDdIf.MatchString(c),
		reCurlSh.MatchString(c),
		reChmod777.MatchString(c),
		reSSHWrite.MatchString(c) && looksLikeWrite(c):
		return true
	default:
		return false
	}
}

func looksLikeWrite(cmd string) bool {
	lower := strings.ToLower(cmd)
	for _, tok := range []string{" >", ">>", "tee ", "cp ", "mv ", "install ", "cat >"} {
		if strings.Contains(lower, tok) {
			return true
		}
	}
	return false
}

var networkPatterns = []string{
	"curl ", "curl\t", "wget ", "wget\t",
	"npm install", "npm i ", "npx ",
	"yarn add", "pnpm install", "pnpm i ",
	"pip install", "pip3 install", "uv pip",
	"go get ", "go install ",
	"cargo install",
	"git push", "git pull", "git fetch", "git clone",
	"docker pull", "docker push",
	"apt-get ", "apt install", "brew install",
}

// LooksLikeNetwork reports commands that typically need outbound network.
func LooksLikeNetwork(cmd string) bool {
	lower := strings.ToLower(strings.TrimSpace(cmd))
	if lower == "" {
		return false
	}
	for _, p := range networkPatterns {
		if strings.Contains(lower, p) {
			return true
		}
	}
	// bare curl/wget as first token
	fields := strings.Fields(lower)
	if len(fields) > 0 {
		switch fields[0] {
		case "curl", "wget":
			return true
		}
	}
	return false
}

// sensitiveHTTPHeaderNames elevate http_request risk when present.
var sensitiveHTTPHeaderNames = map[string]struct{}{
	"authorization":       {},
	"proxy-authorization": {},
	"cookie":              {},
	"set-cookie":          {},
	"x-api-key":           {},
	"api-key":             {},
	"x-auth-token":        {},
}

// EffectiveHTTPRequestRisk raises risk for mutating methods or credential headers.
// Base schema risk stays medium; callers pass handler.RiskLevel() as base.
func EffectiveHTTPRequestRisk(base domain.RiskLevel, method string, headers map[string]string) domain.RiskLevel {
	if base == domain.RiskHigh || base == domain.RiskExternal {
		return domain.RiskExternal
	}
	m := strings.ToUpper(strings.TrimSpace(method))
	if m == "" {
		m = http.MethodGet
	}
	switch m {
	case http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete:
		return domain.RiskExternal
	}
	for k := range headers {
		if _, ok := sensitiveHTTPHeaderNames[strings.ToLower(strings.TrimSpace(k))]; ok {
			return domain.RiskExternal
		}
	}
	if base == "" {
		return domain.RiskMedium
	}
	return base
}

var reURLHost = regexp.MustCompile(`(?i)https?://([a-z0-9]([a-z0-9-]*[a-z0-9])?\.)+[a-z]{2,}(:\d+)?`)
var reHostToken = regexp.MustCompile(`(?i)\b([a-z0-9]([a-z0-9-]*[a-z0-9])?\.)+(com|org|net|io|dev|ai|co|cn|app|cloud)\b`)

// ExtractHosts finds likely outbound hostnames in a shell command for allowlist checks.
func ExtractHosts(cmd string) []string {
	cmd = strings.TrimSpace(cmd)
	if cmd == "" {
		return nil
	}
	seen := map[string]struct{}{}
	var out []string
	add := func(h string) {
		h = strings.ToLower(strings.TrimSpace(h))
		if h == "" {
			return
		}
		if _, ok := seen[h]; ok {
			return
		}
		seen[h] = struct{}{}
		out = append(out, h)
	}
	for _, m := range reURLHost.FindAllStringSubmatch(cmd, -1) {
		if len(m) > 0 {
			u := m[0]
			if i := strings.Index(u, "://"); i >= 0 {
				rest := u[i+3:]
				if j := strings.IndexAny(rest, "/:?"); j >= 0 {
					rest = rest[:j]
				}
				add(rest)
			}
		}
	}
	// Package managers without explicit URL still need registry hosts — caller
	// may Ask with a conventional domain when LooksLikeNetwork but ExtractHosts empty.
	if LooksLikeNetwork(cmd) && len(out) == 0 {
		lower := strings.ToLower(cmd)
		switch {
		case strings.Contains(lower, "npm ") || strings.Contains(lower, "npx ") || strings.Contains(lower, "yarn ") || strings.Contains(lower, "pnpm "):
			add("registry.npmjs.org")
		case strings.Contains(lower, "pip ") || strings.Contains(lower, "pip3 ") || strings.Contains(lower, "uv pip"):
			add("pypi.org")
		case strings.Contains(lower, "go get") || strings.Contains(lower, "go install") || strings.Contains(lower, "go mod"):
			add("proxy.golang.org")
		case strings.Contains(lower, "git clone") || strings.Contains(lower, "git pull") || strings.Contains(lower, "git fetch") || strings.Contains(lower, "git push"):
			add("github.com")
		default:
			for _, m := range reHostToken.FindAllString(cmd, -1) {
				add(m)
			}
		}
	}
	return out
}

// ParseHTTPHeadersFromArgs extracts string headers from tool-call arguments.
func ParseHTTPHeadersFromArgs(args map[string]any) map[string]string {
	if args == nil {
		return nil
	}
	raw, ok := args["headers"]
	if !ok || raw == nil {
		return nil
	}
	switch h := raw.(type) {
	case map[string]string:
		return h
	case map[string]any:
		out := make(map[string]string, len(h))
		for k, v := range h {
			if s, ok := v.(string); ok {
				out[k] = s
			}
		}
		return out
	default:
		return nil
	}
}
