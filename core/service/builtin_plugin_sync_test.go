package service

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"danmo-work/core/domain"
	"danmo-work/core/resource/plugins"
)

func TestSyncBuiltinPluginsMaterializesPacks(t *testing.T) {
	root := t.TempDir()
	t.Setenv("WORK_HOME", root)
	dataDir := filepath.Join(root, "data")
	if err := os.MkdirAll(filepath.Join(dataDir, "agents"), 0o755); err != nil {
		t.Fatal(err)
	}
	// Leftover from pre-migration home sync must be removed.
	leftover := filepath.Join(dataDir, "agents", "github.md")
	if err := os.WriteFile(leftover, []byte("old"), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := SyncBuiltinPlugins(dataDir); err != nil {
		t.Fatal(err)
	}

	for _, name := range plugins.BuiltinPluginNames() {
		pluginJSON := filepath.Join(root, "plugins", name, "plugin.json")
		if _, err := os.Stat(pluginJSON); err != nil {
			t.Fatalf("missing %s: %v", pluginJSON, err)
		}
	}
	if _, err := os.Stat(leftover); err == nil {
		t.Fatal("migrated github.md leftover was not cleaned")
	}

	installedPath := filepath.Join(root, "plugins", "installed.json")
	data, err := os.ReadFile(installedPath)
	if err != nil {
		t.Fatal(err)
	}
	var m domain.PluginInstalledManifest
	if err := json.Unmarshal(data, &m); err != nil {
		t.Fatal(err)
	}
	for _, name := range plugins.BuiltinPluginNames() {
		p, ok := m.Plugins[name]
		if !ok || !p.Builtin || p.MarketSource != domain.PluginMarketSourceBuiltin {
			t.Fatalf("installed %s = %+v", name, p)
		}
	}

	ver, err := os.ReadFile(filepath.Join(root, "plugins", builtinPluginsVersionFile))
	if err != nil {
		t.Fatal(err)
	}
	hash, err := plugins.BuiltinContentHash()
	if err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(string(ver)) != hash {
		t.Fatalf("version=%q want %q", strings.TrimSpace(string(ver)), hash)
	}
}

func TestSyncBuiltinPluginsIdempotent(t *testing.T) {
	root := t.TempDir()
	t.Setenv("WORK_HOME", root)
	dataDir := filepath.Join(root, "data")
	if err := SyncBuiltinPlugins(dataDir); err != nil {
		t.Fatal(err)
	}
	marker := filepath.Join(root, "plugins", "github", "plugin.json")
	if err := os.WriteFile(marker, []byte(`{"name":"github","version":"tampered"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := SyncBuiltinPlugins(dataDir); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(marker)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(got), "tampered") {
		t.Fatal("matching hash should skip rematerialize")
	}
}

func TestPluginManagerRejectsBuiltinUninstall(t *testing.T) {
	root := t.TempDir()
	t.Setenv("WORK_HOME", root)
	dataDir := filepath.Join(root, "data")
	if err := SyncBuiltinPlugins(dataDir); err != nil {
		t.Fatal(err)
	}

	pm := NewPluginManager(dataDir, NewSkillManager(dataDir), NewAgentManager(dataDir), NewMCPManager(dataDir), nil)
	if err := pm.Init(context.Background()); err != nil {
		t.Fatal(err)
	}
	err := pm.UninstallPlugin(context.Background(), "github", "")
	if err == nil || !strings.Contains(err.Error(), "cannot uninstall builtin") {
		t.Fatalf("expected builtin uninstall reject, got %v", err)
	}
}

func TestBuiltinPluginsRegisterExpertsAndMCP(t *testing.T) {
	root := t.TempDir()
	t.Setenv("WORK_HOME", root)
	dataDir := filepath.Join(root, "data")
	if err := SyncBuiltinPlugins(dataDir); err != nil {
		t.Fatal(err)
	}

	sm := NewSkillManager(dataDir)
	am := NewAgentManager(dataDir)
	mm := NewMCPManager(dataDir)
	pm := NewPluginManager(dataDir, sm, am, mm, nil)
	if err := pm.Init(context.Background()); err != nil {
		t.Fatal(err)
	}

	gh, err := am.Get(context.Background(), "github")
	if err != nil {
		t.Fatalf("github expert: %v", err)
	}
	if !gh.Builtin || gh.Mode != domain.AgentModeSubagent {
		t.Fatalf("github agent: %+v", gh)
	}

	srv, err := mm.Get(context.Background(), "github")
	if err != nil {
		t.Fatalf("github mcp: %v", err)
	}
	if srv.AmbientMount {
		t.Fatal("github mcp must be bound-only")
	}
	if !strings.HasPrefix(srv.MarketSource, "plugin:") {
		t.Fatalf("github mcp MarketSource=%q", srv.MarketSource)
	}

	dm, err := mm.Get(context.Background(), "danmo-make")
	if err != nil {
		t.Fatalf("danmo-make mcp: %v", err)
	}
	if dm.AmbientMount {
		t.Fatal("danmo-make mcp must be bound-only")
	}

	list := pm.ListInstalled()
	if len(list) < 4 {
		t.Fatalf("installed plugins=%d", len(list))
	}
	var novel *domain.PluginInstalled
	for i := range list {
		if list[i].Name == "novel" {
			novel = &list[i]
			break
		}
	}
	if novel == nil {
		t.Fatal("novel plugin missing from list")
	}
	if len(novel.Components.Knowledge) != 1 || novel.Components.Knowledge[0] != "kb-novel-craft" {
		t.Fatalf("novel knowledge components=%v", novel.Components.Knowledge)
	}
}
