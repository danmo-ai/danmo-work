package service

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"danmo-work/core/domain"
)

const (
	// GitHubExpertID is the builtin GitHub expert / skill / connector id.
	GitHubExpertID = "github"
	// GitHubLegacyMarketConnectorID is the former dq-market connector id (removed; superseded by builtin).
	GitHubLegacyMarketConnectorID = "github-mcp"
	ghBinName                     = "gh"
)

var ghHomeBinDir = userHomeDanmoBin

// ResolveGhBin returns the path to the local GitHub CLI.
// Order: WORK_GH_BIN → ~/.danmo-work/bin/gh → PATH.
func ResolveGhBin() string {
	if p := strings.TrimSpace(os.Getenv("WORK_GH_BIN")); p != "" {
		if st, err := os.Stat(p); err == nil && !st.IsDir() {
			return p
		}
	}
	homeBin := filepath.Join(ghHomeBinDir(), ghExecutableName())
	if st, err := os.Stat(homeBin); err == nil && !st.IsDir() {
		return homeBin
	}
	if p, err := exec.LookPath(ghBinName); err == nil {
		return p
	}
	return ""
}

func ghExecutableName() string {
	if runtime.GOOS == "windows" {
		return ghBinName + ".exe"
	}
	return ghBinName
}

// GitHubMCPReady reports whether the builtin github connector has usable auth configured.
func (m *MCPManager) GitHubMCPReady(ctx context.Context) bool {
	if m == nil {
		return false
	}
	srv, err := m.Get(ctx, GitHubExpertID)
	if err != nil || !srv.Enabled {
		return false
	}
	return mcpAuthConfigured(ctx, m, srv)
}

func mcpAuthConfigured(ctx context.Context, m *MCPManager, srv domain.MCPServer) bool {
	if srv.OAuthStatus == "connected" {
		if hasAuthHeader(m.resolveAuthHeaders(ctx, srv)) {
			return true
		}
	}
	if len(srv.SecretHeadersRef) > 0 && hasAuthHeader(m.resolveAuthHeaders(ctx, srv)) {
		return true
	}
	return hasAuthHeader(srv.Headers)
}

func hasAuthHeader(headers map[string]string) bool {
	for k, v := range headers {
		if strings.TrimSpace(v) == "" {
			continue
		}
		if strings.EqualFold(k, "Authorization") || strings.EqualFold(k, "X-MCP-Github-Pat") {
			return true
		}
		// Any configured secret header counts as "configured" for headers auth.
		if strings.Contains(strings.ToLower(k), "authorization") || strings.Contains(strings.ToLower(k), "token") {
			return true
		}
	}
	// Non-empty secret-backed headers that aren't named above still count via resolveAuthHeaders values:
	for _, v := range headers {
		if strings.TrimSpace(v) != "" {
			return true
		}
	}
	return false
}

// IsProductBuiltinConnector hides/blocks market packages that are product-seeded.
func IsProductBuiltinConnector(id string) bool {
	id = strings.TrimSpace(id)
	if id == "" {
		return false
	}
	if id == GitHubLegacyMarketConnectorID {
		return true
	}
	for _, bid := range BuiltinConnectorIDs {
		if bid == id {
			return true
		}
	}
	return false
}

// GitHubAccessHint prepends path-selection context for the github expert child turn.
// Prefer bound MCP when auth is configured; otherwise fall back to local gh.
func GitHubAccessHint(mcpReady bool, ghBin string) string {
	ghBin = strings.TrimSpace(ghBin)
	switch {
	case mcpReady:
		msg := "[github-access: mcp] bound github MCP auth is configured — prefer mcp_github_* tools."
		if ghBin != "" {
			msg += " gh fallback bin=" + ghBin + "."
		} else {
			msg += " gh CLI not on PATH (MCP only)."
		}
		return msg
	case ghBin != "":
		return "[github-access: gh] MCP not configured — use exec_shell → gh (bin=" + ghBin + "); verify with gh auth status before mutating. Configure the builtin github connector (PAT/OAuth) to enable MCP."
	default:
		return "[github-access: none] Neither github MCP auth nor gh CLI is available. Configure the builtin github connector and/or install gh + gh auth login. Do not invent GitHub results."
	}
}

// GitHubGhHint is kept for tests/compat; prefer GitHubAccessHint.
func GitHubGhHint(binPath string) string {
	return GitHubAccessHint(false, binPath)
}
