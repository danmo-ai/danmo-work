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

func TestSyncBuiltinMCPDoesNotOverwriteExisting(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	m := NewMCPManager(dir)
	if _, err := m.Create(ctx, domain.UpsertMCPServerRequest{
		ID:           GitHubExpertID,
		Name:         "User GitHub",
		Transport:    "streamable-http",
		URL:          "https://example.invalid/mcp/",
		Auth:         domain.MCPAuthHeaders,
		Enabled:      true,
		AmbientMount: boolPtr(true),
		Headers:      map[string]string{"Authorization": "Bearer ghp_keep"},
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
		t.Fatalf("existing item overwritten: url=%q", gh.URL)
	}
	if gh.Name != "User GitHub" {
		t.Fatalf("existing item overwritten: name=%q", gh.Name)
	}
	if !gh.AmbientMount {
		t.Fatal("existing AmbientMount was changed")
	}
	if gh.Headers["Authorization"] != "Bearer ghp_keep" {
		t.Fatalf("headers not preserved: %+v", gh.Headers)
	}

	// Missing sibling builtin should still be inserted.
	if _, err := m.Get(ctx, "danmo-make"); err != nil {
		t.Fatalf("danmo-make should still be seeded: %v", err)
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
