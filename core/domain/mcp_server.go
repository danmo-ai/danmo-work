package domain

import (
	"strings"
)

// MCPAuthMode selects how the client authenticates to an MCP server.
type MCPAuthMode string

const (
	MCPAuthNone    MCPAuthMode = "none"
	MCPAuthHeaders MCPAuthMode = "headers"
	MCPAuthOAuth   MCPAuthMode = "oauth"
)

// MCPServer represents a configured MCP (Model Context Protocol) server.
type MCPServer struct {
	ID              string            `json:"id"`
	Name            string            `json:"name"`
	Description     string            `json:"description"`
	Transport       string            `json:"transport"` // stdio | sse | streamable-http
	Command         string            `json:"command"`   // for stdio
	Args            string            `json:"args"`      // space-separated args for stdio
	URL             string            `json:"url"`       // for sse / streamable-http
	Env             string            `json:"env"`       // KEY=value per line
	Headers         map[string]string `json:"headers"`   // for sse / streamable-http (non-secret)
	Auth            MCPAuthMode       `json:"auth"`      // none | headers | oauth
	// SecretHeadersRef maps header names to secret store keys (Phase 1).
	SecretHeadersRef map[string]string `json:"secretHeadersRef,omitempty"`
	// OAuth fields (tokens live in secrets store; only metadata here).
	OAuthClientID     string `json:"oauthClientId,omitempty"`
	OAuthAuthorizeURL string `json:"oauthAuthorizeUrl,omitempty"`
	OAuthTokenURL     string `json:"oauthTokenUrl,omitempty"`
	OAuthScopes       string `json:"oauthScopes,omitempty"`
	OAuthStatus       string `json:"oauthStatus,omitempty"` // disconnected | pending | connected | error
	CatalogID         string `json:"catalogId,omitempty"`   // preset from connector catalog
	EnabledTools      []string     `json:"enabledTools"`    // tool names user enabled
	DiscoveredTools   []MCPToolDef `json:"discoveredTools"` // tools discovered from server
	ToolTimeout       int          `json:"toolTimeout"`     // seconds, default 300
	Status            string       `json:"status"`          // connected | disconnected | error
	Enabled           bool         `json:"enabled"`         // user toggle
}

// MCPToolDef describes a single tool exposed by an MCP server.
type MCPToolDef struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	Enabled     bool           `json:"enabled"`
	InputSchema map[string]any `json:"inputSchema,omitempty"`
}

// MCPToolBinding is a runtime-ready tool registration for the engine catalog.
type MCPToolBinding struct {
	ServerID    string         `json:"serverId"`
	ServerName  string         `json:"serverName"`
	ToolName    string         `json:"toolName"`    // original MCP tool name
	ExposedName string         `json:"exposedName"` // name exposed to the LLM
	Description string         `json:"description"`
	InputSchema map[string]any `json:"inputSchema,omitempty"`
	RiskLevel   RiskLevel      `json:"riskLevel"`
}

// UpsertMCPServerRequest is the payload for creating / updating an MCP server.
type UpsertMCPServerRequest struct {
	Name               string            `json:"name"`
	Description        string            `json:"description"`
	Transport          string            `json:"transport"`
	Command            string            `json:"command"`
	Args               string            `json:"args"`
	URL                string            `json:"url"`
	Env                string            `json:"env"`
	Headers            map[string]string `json:"headers"`
	Auth               MCPAuthMode       `json:"auth"`
	SecretHeadersRef   map[string]string `json:"secretHeadersRef,omitempty"`
	OAuthClientID      string            `json:"oauthClientId,omitempty"`
	OAuthAuthorizeURL  string            `json:"oauthAuthorizeUrl,omitempty"`
	OAuthTokenURL      string            `json:"oauthTokenUrl,omitempty"`
	OAuthScopes        string            `json:"oauthScopes,omitempty"`
	CatalogID          string            `json:"catalogId,omitempty"`
	EnabledTools       []string          `json:"enabledTools"`
	ToolTimeout        int               `json:"toolTimeout"`
	Status             string            `json:"status"`
	Enabled            bool              `json:"enabled"`
	// HeaderSecrets are plaintext header values to store encrypted (Phase 1).
	HeaderSecrets map[string]string `json:"headerSecrets,omitempty"`
}

// ExposedMCPToolName builds a stable LLM-facing tool name for an MCP tool.
func ExposedMCPToolName(serverName, toolName string) string {
	slug := sanitizeMCPName(serverName)
	tool := sanitizeMCPName(toolName)
	if slug == "" {
		slug = "server"
	}
	return "mcp_" + slug + "_" + tool
}

func sanitizeMCPName(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	var b strings.Builder
	lastUnderscore := false
	for _, r := range s {
		ok := (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9')
		if ok {
			b.WriteRune(r)
			lastUnderscore = false
			continue
		}
		if !lastUnderscore {
			b.WriteByte('_')
			lastUnderscore = true
		}
	}
	out := strings.Trim(b.String(), "_")
	if len(out) > 48 {
		out = out[:48]
	}
	return out
}
