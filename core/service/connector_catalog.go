package service

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"danmo-work/core/domain"
)

// DefaultConnectorCatalog returns built-in MCP connector presets.
// Deprecated: prefer market.sources → dq-market kind=connector packages.
// Kept for the legacy GET /mcp/catalog API.
// First-party GitHub / Danmo Make connectors also ship via builtin plugins;
// BuiltinConnectorIDs may additionally seed mcp.json when non-empty.
func DefaultConnectorCatalog() []domain.ConnectorCatalogEntry {
	return []domain.ConnectorCatalogEntry{
		{
			ID:          "composio",
			Name:        "Composio Connect",
			Description: "1000+ apps via meta-tools (search → connect → execute). Managed OAuth.",
			Category:    "gateway",
			Transport:   "streamable-http",
			URL:         "https://connect.composio.dev/mcp",
			Auth:        domain.MCPAuthHeaders,
			DocsURL:     "https://docs.composio.dev/",
			Region:      "global",
			Tags:        []string{"slack", "gmail", "github", "notion", "gateway"},
		},
		{
			ID:          "openconnector",
			Name:        "OpenConnector (self-hosted)",
			Description: "Open-source auth gateway. Point at your local OpenConnector MCP endpoint.",
			Category:    "gateway",
			Transport:   "streamable-http",
			URL:         "http://localhost:3000/mcp",
			Auth:        domain.MCPAuthNone,
			DocsURL:     "https://github.com/oomol-lab/open-connector",
			Region:      "global",
			Tags:        []string{"gateway", "self-hosted"},
		},
		{
			ID:          "notion-mcp",
			Name:        "Notion MCP",
			Description: "Notion workspace via remote MCP (OAuth).",
			Category:    "saas",
			Transport:   "streamable-http",
			URL:         "https://mcp.notion.com/mcp",
			Auth:        domain.MCPAuthOAuth,
			DocsURL:     "https://developers.notion.com/",
			Region:      "global",
			Tags:        []string{"notion", "docs"},
		},
		{
			ID:          "feishu-docs",
			Name:        "Feishu / Lark Docs MCP",
			Description: "Feishu documents & wiki via community/official MCP. Prefer outbound IM channel for chat; use MCP for doc actions.",
			Category:    "china",
			Transport:   "stdio",
			Command:     "npx",
			Args:        "-y @larksuiteoapi/lark-mcp",
			Auth:        domain.MCPAuthHeaders,
			DocsURL:     "https://open.feishu.cn/",
			Region:      "cn",
			Tags:        []string{"feishu", "lark", "docs", "bitable"},
		},
		{
			ID:          "feishu-bitable",
			Name:        "Feishu Bitable (via OpenConnector/Composio)",
			Description: "Multidimensional tables — connect through a gateway MCP after Phase-0 wiring.",
			Category:    "china",
			Transport:   "streamable-http",
			URL:         "http://localhost:3000/mcp",
			Auth:        domain.MCPAuthNone,
			DocsURL:     "https://open.feishu.cn/document/server-docs/docs/bitable-v1/bitable-overview",
			Region:      "cn",
			Tags:        []string{"feishu", "bitable", "sheets"},
		},
		{
			ID:          "filesystem-mcp",
			Name:        "Filesystem MCP (local)",
			Description: "Reference stdio MCP for local files — useful for testing CallTool wiring.",
			Category:    "local",
			Transport:   "stdio",
			Command:     "npx",
			Args:        "-y @modelcontextprotocol/server-filesystem",
			Auth:        domain.MCPAuthNone,
			DocsURL:     "https://github.com/modelcontextprotocol/servers",
			Region:      "global",
			Tags:        []string{"local", "files", "test"},
		},
		DanmoMakeCatalogEntry(),
		GitHubCatalogEntry(),
	}
}

// BuiltinConnectorIDs are product connectors auto-seeded into mcp.json on bootstrap
// when missing. GitHub / Danmo Make now ship via builtin plugins (mcp.json in the
// plugin pack); keep this empty so SyncBuiltinMCP is a no-op unless new product
// seeds are added. Catalog entries remain for the Connectors UI.
// CodeGraph is market-installed (connector deps script + CLI), not product-seeded.
var BuiltinConnectorIDs = []string{}

// DanmoMakeCatalogEntry is the built-in local creative MCP preset.
func DanmoMakeCatalogEntry() domain.ConnectorCatalogEntry {
	return domain.ConnectorCatalogEntry{
		ID:           "danmo-make",
		Name:         "Danmo Make",
		Description:  "Local image/video/audio generation via Danmo Make MCP (bound-only; use with danmo-make expert).",
		Category:     "local",
		Transport:    "streamable-http",
		URL:          ResolveDanmoMakeMCPURL(),
		Auth:         domain.MCPAuthNone,
		DocsURL:      "https://github.com/danmo-ai/danmo-make",
		Region:       "global",
		Tags:         []string{"danmo-make", "image", "video", "audio", "generation", "local"},
		ToolTimeout:  900,
		AmbientMount: boolPtr(false),
	}
}

// CodeGraphCatalogEntry is the built-in local code-intelligence MCP (bound-only).
func CodeGraphCatalogEntry() domain.ConnectorCatalogEntry {
	cmd := ResolveCodeGraphBin()
	if cmd == "" {
		cmd = codeGraphBinName // seed placeholder; bootstrap refreshes when binary appears
	}
	return domain.ConnectorCatalogEntry{
		ID:           CodeGraphServerID,
		Name:         "CodeGraph",
		Description:  "Local code knowledge graph via CodeGraph-Rust MCP (bound-only; use with codegraph expert).",
		Category:     "local",
		Transport:    "stdio",
		Command:      cmd,
		Args:         "serve --mcp",
		Auth:         domain.MCPAuthNone,
		DocsURL:      "https://github.com/sunerpy/codegraph-rust",
		Region:       "global",
		Tags:         []string{"codegraph", "code", "symbols", "impact", "local"},
		ToolTimeout:  120,
		AmbientMount: boolPtr(false),
	}
}

// GitHubCatalogEntry is the built-in GitHub remote MCP (bound-only; github expert).
// Primary workflow remains local `gh`; this connector is the hosted MCP fallback / OAuth path.
func GitHubCatalogEntry() domain.ConnectorCatalogEntry {
	return domain.ConnectorCatalogEntry{
		ID:           GitHubExpertID,
		Name:         "GitHub",
		Description:  "Official GitHub remote MCP (bound-only; use with github expert + local gh CLI).",
		Category:     "saas",
		Transport:    "streamable-http",
		URL:          "https://api.githubcopilot.com/mcp/",
		Auth:         domain.MCPAuthHeaders,
		DocsURL:      "https://github.com/github/github-mcp-server",
		Region:       "global",
		Tags:         []string{"github", "code", "mcp", "bound"},
		ToolTimeout:  120,
		AmbientMount: boolPtr(false),
	}
}

// ResolveDanmoMakeMCPURL reads ~/.danmo-make/api.port when present, else :7800.
// Trailing slash is required for FastMCP streamable-http mounts.
func ResolveDanmoMakeMCPURL() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return "http://127.0.0.1:7800/mcp/"
	}
	raw, err := os.ReadFile(filepath.Join(home, ".danmo-make", "api.port"))
	if err != nil {
		return "http://127.0.0.1:7800/mcp/"
	}
	port := strings.TrimSpace(string(raw))
	if port == "" {
		return "http://127.0.0.1:7800/mcp/"
	}
	return fmt.Sprintf("http://127.0.0.1:%s/mcp/", port)
}

func boolPtr(v bool) *bool { return &v }

// CatalogEntryByID returns a DefaultConnectorCatalog entry or nil.
func CatalogEntryByID(id string) *domain.ConnectorCatalogEntry {
	id = strings.TrimSpace(id)
	if id == "" {
		return nil
	}
	for _, e := range DefaultConnectorCatalog() {
		if e.ID == id {
			cp := e
			return &cp
		}
	}
	return nil
}

// InstallCatalogEntry builds an UpsertMCPServerRequest from a catalog preset.
func InstallCatalogEntry(entry domain.ConnectorCatalogEntry, name string) domain.UpsertMCPServerRequest {
	if name == "" {
		name = entry.Name
	}
	timeout := 300
	if entry.ToolTimeout > 0 {
		timeout = entry.ToolTimeout
	}
	ambient := true
	if entry.AmbientMount != nil {
		ambient = *entry.AmbientMount
	}
	return domain.UpsertMCPServerRequest{
		Name:              name,
		Description:       entry.Description,
		Transport:         entry.Transport,
		Command:           entry.Command,
		Args:              entry.Args,
		URL:               entry.URL,
		Env:               entry.Env,
		Auth:              entry.Auth,
		OAuthAuthorizeURL: entry.OAuthAuthorizeURL,
		OAuthTokenURL:     entry.OAuthTokenURL,
		OAuthScopes:       entry.OAuthScopes,
		CatalogID:         entry.ID,
		Enabled:           true,
		EnabledTools:      []string{"*"},
		ToolTimeout:       timeout,
		AmbientMount:      &ambient,
	}
}
