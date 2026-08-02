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
	if req.ToolTimeout != 300 || req.AmbientMount == nil || !*req.AmbientMount {
		t.Fatalf("defaults: timeout=%d ambient=%v", req.ToolTimeout, req.AmbientMount)
	}
}

func TestInstallCatalogEntryBoundOnlyAndTimeout(t *testing.T) {
	dir := t.TempDir()
	raw := `{
  "id": "creative-local",
  "name": "Creative Local",
  "transport": "streamable-http",
  "url": "http://127.0.0.1:7800/mcp",
  "auth": "none",
  "toolTimeout": 900,
  "ambientMount": false
}`
	if err := os.WriteFile(filepath.Join(dir, "connector.json"), []byte(raw), 0o644); err != nil {
		t.Fatal(err)
	}
	entry, err := NewConnectorImporter().Import(dir)
	if err != nil {
		t.Fatal(err)
	}
	req := InstallCatalogEntry(*entry, "")
	if req.ToolTimeout != 900 {
		t.Fatalf("toolTimeout=%d", req.ToolTimeout)
	}
	if req.AmbientMount == nil || *req.AmbientMount {
		t.Fatalf("ambientMount should be false, got %v", req.AmbientMount)
	}
}
