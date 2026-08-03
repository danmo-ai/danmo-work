package service

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"danmo-work/core/domain"
)

func TestResolveGhBinEnvOverride(t *testing.T) {
	dir := t.TempDir()
	name := "gh"
	if runtime.GOOS == "windows" {
		name = "gh.exe"
	}
	bin := filepath.Join(dir, name)
	if err := os.WriteFile(bin, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("WORK_GH_BIN", bin)
	got := ResolveGhBin()
	if got != bin {
		t.Fatalf("ResolveGhBin=%q want %q", got, bin)
	}
}

func TestResolveGhBinHomeDir(t *testing.T) {
	dir := t.TempDir()
	prev := ghHomeBinDir
	ghHomeBinDir = func() string { return dir }
	t.Cleanup(func() { ghHomeBinDir = prev })
	t.Setenv("WORK_GH_BIN", "")

	bin := filepath.Join(dir, ghExecutableName())
	if err := os.WriteFile(bin, []byte("x"), 0o755); err != nil {
		t.Fatal(err)
	}
	got := ResolveGhBin()
	if got != bin {
		t.Fatalf("ResolveGhBin=%q want home %q", got, bin)
	}
}

func TestResolveGitBinEnvOverride(t *testing.T) {
	dir := t.TempDir()
	bin := filepath.Join(dir, "git")
	if runtime.GOOS == "windows" {
		bin += ".exe"
	}
	if err := os.WriteFile(bin, []byte("x"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("WORK_GIT_BIN", bin)
	got := ResolveGitBin()
	if got != bin {
		t.Fatalf("ResolveGitBin=%q want %q", got, bin)
	}
}

func TestGitHubAccessHintPriority(t *testing.T) {
	mcp := GitHubAccessHint(true, "/usr/bin/gh", "/usr/bin/git")
	if !strings.Contains(mcp, "github-access: mcp") || !strings.Contains(mcp, "mcp_github_") {
		t.Fatalf("mcp hint: %s", mcp)
	}
	gh := GitHubAccessHint(false, "/usr/bin/gh", "/usr/bin/git")
	if !strings.Contains(gh, "github-access: gh") {
		t.Fatalf("gh hint: %s", gh)
	}
	git := GitHubAccessHint(false, "", "/usr/bin/git")
	if !strings.Contains(git, "github-access: git") || !strings.Contains(git, "git") {
		t.Fatalf("git hint: %s", git)
	}
	none := GitHubAccessHint(false, "", "")
	if !strings.Contains(none, "github-access: none") {
		t.Fatalf("none hint: %s", none)
	}
}

func TestGitHubCatalogEntryBoundOnlyBuiltin(t *testing.T) {
	entry := GitHubCatalogEntry()
	if entry.ID != GitHubExpertID {
		t.Fatalf("id=%q want %q", entry.ID, GitHubExpertID)
	}
	if entry.AmbientMount == nil || *entry.AmbientMount {
		t.Fatal("github connector must be bound-only (AmbientMount=false)")
	}
	if !IsProductBuiltinConnector(GitHubExpertID) || !IsProductBuiltinConnector(GitHubLegacyMarketConnectorID) {
		t.Fatal("builtin / legacy market ids should be filtered")
	}
	if CatalogEntryByID(GitHubExpertID) == nil {
		t.Fatal("CatalogEntryByID(github) nil")
	}
}

func TestGitHubMCPReadyRequiresAuth(t *testing.T) {
	ctx := context.Background()
	mcp := NewMCPManager(newMemMCPServerRepo())
	if mcp.GitHubMCPReady(ctx) {
		t.Fatal("missing server should not be ready")
	}
	if _, err := mcp.Create(ctx, domain.UpsertMCPServerRequest{
		ID:        GitHubExpertID,
		Name:      "GitHub",
		Transport: "streamable-http",
		URL:       "https://api.githubcopilot.com/mcp/",
		Auth:      domain.MCPAuthHeaders,
		Enabled:   true,
	}); err != nil {
		t.Fatal(err)
	}
	if mcp.GitHubMCPReady(ctx) {
		t.Fatal("unconfigured auth should not be ready")
	}
	if _, err := mcp.Update(ctx, GitHubExpertID, domain.UpsertMCPServerRequest{
		Name:      "GitHub",
		Transport: "streamable-http",
		URL:       "https://api.githubcopilot.com/mcp/",
		Auth:      domain.MCPAuthHeaders,
		Enabled:   true,
		Headers:   map[string]string{"Authorization": "Bearer ghp_test"},
	}); err != nil {
		t.Fatal(err)
	}
	if !mcp.GitHubMCPReady(ctx) {
		t.Fatal("expected ready after Authorization header")
	}
}
