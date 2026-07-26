package service

import (
	"danmo-work/core/domain"
)

// DefaultConnectorCatalog returns built-in MCP connector presets.
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
			ID:          "github-mcp",
			Name:        "GitHub MCP",
			Description: "Official GitHub remote MCP (issues, PRs, repos). Use OAuth or PAT header.",
			Category:    "saas",
			Transport:   "streamable-http",
			URL:         "https://api.githubcopilot.com/mcp/",
			Auth:        domain.MCPAuthHeaders,
			DocsURL:     "https://github.com/github/github-mcp-server",
			Region:      "global",
			Tags:        []string{"github", "code"},
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
	}
}

// InstallCatalogEntry builds an UpsertMCPServerRequest from a catalog preset.
func InstallCatalogEntry(entry domain.ConnectorCatalogEntry, name string) domain.UpsertMCPServerRequest {
	if name == "" {
		name = entry.Name
	}
	return domain.UpsertMCPServerRequest{
		Name:              name,
		Description:       entry.Description,
		Transport:         entry.Transport,
		Command:           entry.Command,
		Args:              entry.Args,
		URL:               entry.URL,
		Auth:              entry.Auth,
		OAuthAuthorizeURL: entry.OAuthAuthorizeURL,
		OAuthTokenURL:     entry.OAuthTokenURL,
		OAuthScopes:       entry.OAuthScopes,
		CatalogID:         entry.ID,
		Enabled:           true,
		EnabledTools:      []string{"*"},
		ToolTimeout:       300,
	}
}
