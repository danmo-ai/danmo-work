package service

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"danmo-work/core/domain"
)

func TestSyncBuiltinMCPSeedsMissing(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	m := NewMCPManager(dir)
	if err := m.SyncBuiltinMCP(); err != nil {
		t.Fatal(err)
	}

	gh, err := m.Get(ctx, GitHubExpertID)
	if err != nil {
		t.Fatalf("github missing: %v", err)
	}
	if gh.Auth != domain.MCPAuthHeaders {
		t.Fatalf("github auth=%q want headers", gh.Auth)
	}
	if gh.AmbientMount {
		t.Fatal("github must be bound-only (AmbientMount=false)")
	}
	if gh.CatalogID != GitHubExpertID {
		t.Fatalf("github catalogId=%q", gh.CatalogID)
	}
	if gh.URL != "https://api.githubcopilot.com/mcp/" {
		t.Fatalf("github url=%q", gh.URL)
	}

	dm, err := m.Get(ctx, "danmo-make")
	if err != nil {
		t.Fatalf("danmo-make missing: %v", err)
	}
	if dm.AmbientMount {
		t.Fatal("danmo-make must be bound-only")
	}
	if !strings.Contains(dm.URL, "/mcp/") {
		t.Fatalf("danmo-make url=%q", dm.URL)
	}
}

func TestSyncBuiltinMCPUpdatesStaleBuiltin(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	m := NewMCPManager(dir)
	if _, err := m.Create(ctx, domain.UpsertMCPServerRequest{
		ID:           GitHubExpertID,
		Name:         "stale",
		Transport:    "streamable-http",
		URL:          "https://example.invalid/mcp/",
		Auth:         domain.MCPAuthNone,
		Enabled:      true,
		AmbientMount: boolPtr(true),
	}); err != nil {
		t.Fatal(err)
	}

	if err := m.SyncBuiltinMCP(); err != nil {
		t.Fatal(err)
	}
	gh, err := m.Get(ctx, GitHubExpertID)
	if err != nil {
		t.Fatal(err)
	}
	if gh.URL != "https://api.githubcopilot.com/mcp/" {
		t.Fatalf("url not healed: %q", gh.URL)
	}
	if gh.Auth != domain.MCPAuthHeaders {
		t.Fatalf("auth not healed: %q", gh.Auth)
	}
	if gh.AmbientMount {
		t.Fatal("AmbientMount not healed to bound-only")
	}
	if gh.Name != "GitHub" {
		t.Fatalf("name=%q want GitHub", gh.Name)
	}
}

func TestSyncBuiltinMCPDoesNotOverwriteMarketSource(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	m := NewMCPManager(dir)
	if _, err := m.Create(ctx, domain.UpsertMCPServerRequest{
		ID:           GitHubExpertID,
		Name:         "Market GitHub",
		Transport:    "streamable-http",
		URL:          "https://example.invalid/mcp/",
		Enabled:      true,
		MarketSource: "local",
		AmbientMount: boolPtr(true),
	}); err != nil {
		t.Fatal(err)
	}
	if err := m.SyncBuiltinMCP(); err != nil {
		t.Fatal(err)
	}
	gh, err := m.Get(ctx, GitHubExpertID)
	if err != nil {
		t.Fatal(err)
	}
	if gh.URL != "https://example.invalid/mcp/" {
		t.Fatalf("market-owned entry was overwritten: %q", gh.URL)
	}
}

func TestSyncBuiltinMCPPreservesUserHeaders(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	m := NewMCPManager(dir)
	if _, err := m.Create(ctx, domain.UpsertMCPServerRequest{
		ID:        GitHubExpertID,
		Name:      "GitHub",
		Transport: "streamable-http",
		URL:       "https://example.invalid/mcp/",
		Auth:      domain.MCPAuthHeaders,
		Enabled:   true,
		Headers:   map[string]string{"Authorization": "Bearer ghp_keep"},
	}); err != nil {
		t.Fatal(err)
	}
	if err := m.SyncBuiltinMCP(); err != nil {
		t.Fatal(err)
	}
	gh, err := m.Get(ctx, GitHubExpertID)
	if err != nil {
		t.Fatal(err)
	}
	if gh.Headers["Authorization"] != "Bearer ghp_keep" {
		t.Fatalf("headers not preserved: %+v", gh.Headers)
	}
}

func TestSyncBuiltinMCPPersistsAmbientMountAndAuth(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	m := NewMCPManager(dir)
	if err := m.SyncBuiltinMCP(); err != nil {
		t.Fatal(err)
	}

	raw, err := os.ReadFile(filepath.Join(dir, "mcp.json"))
	if err != nil {
		t.Fatal(err)
	}
	var doc mcpSpecDoc
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatal(err)
	}
	ghSpec, ok := doc.MCPServers[GitHubExpertID]
	if !ok {
		t.Fatal("github missing from mcp.json")
	}
	if ghSpec.AmbientMount == nil || *ghSpec.AmbientMount {
		t.Fatalf("mcp.json ambientMount=%v want false", ghSpec.AmbientMount)
	}
	if ghSpec.Auth != string(domain.MCPAuthHeaders) {
		t.Fatalf("mcp.json auth=%q", ghSpec.Auth)
	}
	if ghSpec.CatalogID != GitHubExpertID {
		t.Fatalf("mcp.json catalogId=%q", ghSpec.CatalogID)
	}

	reloaded := NewMCPManager(dir)
	gh, err := reloaded.Get(ctx, GitHubExpertID)
	if err != nil {
		t.Fatal(err)
	}
	if gh.AmbientMount {
		t.Fatal("AmbientMount lost after reload")
	}
	if gh.Auth != domain.MCPAuthHeaders {
		t.Fatalf("Auth lost after reload: %q", gh.Auth)
	}
	if gh.CatalogID != GitHubExpertID {
		t.Fatalf("CatalogID lost after reload: %q", gh.CatalogID)
	}
}

func TestSyncBuiltinMCPIdempotent(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	m := NewMCPManager(dir)
	if err := m.SyncBuiltinMCP(); err != nil {
		t.Fatal(err)
	}
	if err := m.SyncBuiltinMCP(); err != nil {
		t.Fatal(err)
	}
	gh, err := m.Get(ctx, GitHubExpertID)
	if err != nil {
		t.Fatal(err)
	}
	if gh.AmbientMount || gh.Auth != domain.MCPAuthHeaders {
		t.Fatalf("second sync corrupted builtin: ambient=%v auth=%q", gh.AmbientMount, gh.Auth)
	}
}
