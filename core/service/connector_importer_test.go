package service

import (
	"os"
	"path/filepath"
	"testing"

	"danmo-work/core/domain"
)

func TestConnectorImporterImport(t *testing.T) {
	dir := t.TempDir()
	raw := `{
  "id": "github-mcp",
  "name": "GitHub MCP",
  "description": "Official GitHub remote MCP",
  "category": "saas",
  "transport": "streamable-http",
  "url": "https://api.githubcopilot.com/mcp/",
  "auth": "headers",
  "docsUrl": "https://github.com/github/github-mcp-server",
  "region": "global",
  "tags": ["github"]
}`
	if err := os.WriteFile(filepath.Join(dir, "connector.json"), []byte(raw), 0o644); err != nil {
		t.Fatal(err)
	}
	entry, err := NewConnectorImporter().Import(dir)
	if err != nil {
		t.Fatal(err)
	}
	if entry.ID != "github-mcp" || entry.Transport != "streamable-http" || entry.Auth != domain.MCPAuthHeaders {
		t.Fatalf("unexpected entry: %+v", entry)
	}
	req := InstallCatalogEntry(*entry, "")
	if req.CatalogID != "github-mcp" || req.URL == "" || !req.Enabled {
		t.Fatalf("unexpected upsert req: %+v", req)
	}
}
