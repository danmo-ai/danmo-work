package domain

// ConnectorCatalogEntry is a one-click MCP connector preset.
type ConnectorCatalogEntry struct {
	ID          string      `json:"id"`
	Name        string      `json:"name"`
	Description string      `json:"description"`
	Category    string      `json:"category"` // saas | gateway | china | local
	Transport   string      `json:"transport"`
	URL         string      `json:"url,omitempty"`
	Command     string      `json:"command,omitempty"`
	Args        string      `json:"args,omitempty"`
	Auth        MCPAuthMode `json:"auth"`
	// DocsURL points users at setup instructions.
	DocsURL           string `json:"docsUrl,omitempty"`
	OAuthAuthorizeURL string `json:"oauthAuthorizeUrl,omitempty"`
	OAuthTokenURL     string `json:"oauthTokenUrl,omitempty"`
	OAuthScopes       string `json:"oauthScopes,omitempty"`
	Region            string `json:"region,omitempty"` // global | cn
	Tags              []string `json:"tags,omitempty"`
}
