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

	if err := SyncBuiltinPlugins(dataDir); err != nil {
		t.Fatal(err)
	}

	for _, name := range plugins.BuiltinPluginNames() {
		pluginJSON := filepath.Join(root, "plugins", name, "plugin.json")
		if _, err := os.Stat(pluginJSON); err != nil {
			t.Fatalf("missing %s: %v", pluginJSON, err)
		}
	}
	mcpJSON := filepath.Join(root, "plugins", "danmo-novel", "mcp.json")
	if _, err := os.Stat(mcpJSON); err != nil {
		t.Fatalf("danmo-novel mcp.json should sync: %v", err)
	}
	rawMCP, err := os.ReadFile(mcpJSON)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(rawMCP), "danmo-novel") || !strings.Contains(string(rawMCP), "7804") {
		t.Fatalf("danmo-novel mcp.json should bind danmo-novel :7804: %s", rawMCP)
	}
	if _, err := os.Stat(filepath.Join(root, "plugins", "danmo-novel", "ai.danmo.work", "experts", "danmo-novel.md")); err != nil {
		t.Fatalf("danmo-novel expert should sync: %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, "plugins", "novel")); err == nil {
		t.Fatal("retired novel plugin dir should not be materialized")
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

func TestSyncBuiltinPluginsRemovesRetiredOperatorPlugin(t *testing.T) {
	root := t.TempDir()
	t.Setenv("WORK_HOME", root)
	dataDir := filepath.Join(root, "data")
	pluginsDir := filepath.Join(root, "plugins")
	legacy := filepath.Join(pluginsDir, "operator")
	if err := os.MkdirAll(legacy, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(legacy, "plugin.json"), []byte(`{"name":"operator"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	installedPath := filepath.Join(pluginsDir, "installed.json")
	manifest := domain.PluginInstalledManifest{
		Plugins: map[string]domain.PluginInstalled{
			"operator": {Name: "operator", RootPath: legacy, Builtin: true},
		},
	}
	data, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(installedPath, append(data, '\n'), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := SyncBuiltinPlugins(dataDir); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(legacy); err == nil {
		t.Fatal("retired operator plugin dir should be removed")
	}
	got, err := os.ReadFile(installedPath)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(got), `"operator"`) {
		t.Fatalf("installed.json still references operator: %s", got)
	}
	if _, err := os.Stat(filepath.Join(pluginsDir, "computer", "plugin.json")); err != nil {
		t.Fatalf("computer plugin missing: %v", err)
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

	dn, err := mm.Get(context.Background(), "danmo-novel")
	if err != nil {
		t.Fatalf("danmo-novel mcp: %v", err)
	}
	if dn.AmbientMount {
		t.Fatal("danmo-novel mcp must be bound-only")
	}
	if !strings.Contains(dn.URL, "7804") && !strings.Contains(dn.URL, "/mcp") {
		t.Fatalf("danmo-novel mcp URL=%q", dn.URL)
	}

	list := pm.ListInstalled()
	if len(list) < 12 {
		t.Fatalf("installed plugins=%d want >=12", len(list))
	}
	var dnovel, uiTest *domain.PluginInstalled
	for i := range list {
		switch list[i].Name {
		case "danmo-novel":
			dnovel = &list[i]
		case "ui-test":
			uiTest = &list[i]
		}
	}
	if dnovel == nil {
		t.Fatal("danmo-novel plugin missing from list")
	}
	if uiTest == nil {
		t.Fatal("ui-test plugin missing from list")
	}
	if len(uiTest.Components.Knowledge) != 1 || uiTest.Components.Knowledge[0] != "kb-ui-test" {
		t.Fatalf("ui-test knowledge components=%v", uiTest.Components.Knowledge)
	}
	ut, err := am.Get(context.Background(), "ui-test")
	if err != nil {
		t.Fatalf("ui-test expert: %v", err)
	}
	if !ut.Builtin || ut.Mode != domain.AgentModeSubagent {
		t.Fatalf("ui-test agent: %+v", ut)
	}
	if _, err := sm.Get(context.Background(), "ui-test"); err != nil {
		t.Fatalf("ui-test skill: %v", err)
	}
	for _, name := range list {
		if name.Name == "novel" {
			t.Fatal("retired novel plugin should not be installed")
		}
	}

	dnAgent, err := am.Get(context.Background(), "danmo-novel")
	if err != nil {
		t.Fatalf("danmo-novel expert: %v", err)
	}
	if !dnAgent.Builtin || dnAgent.Mode != domain.AgentModeSubagent {
		t.Fatalf("danmo-novel agent: %+v", dnAgent)
	}

	skill, err := sm.Get(context.Background(), "danmo-novel")
	if err != nil {
		t.Fatalf("danmo-novel skill: %v", err)
	}
	if skill.Dir == "" {
		t.Fatal("danmo-novel skill Dir should be the plugin skill folder")
	}
	if !strings.Contains(skill.Dir, filepath.Join("danmo-novel", "skills", "danmo-novel")) {
		t.Fatalf("danmo-novel skill Dir=%q", skill.Dir)
	}
	skillBody, err := os.ReadFile(filepath.Join(skill.Dir, "SKILL.md"))
	if err != nil {
		t.Fatalf("read danmo-novel SKILL.md: %v", err)
	}
	if !strings.Contains(string(skillBody), "mcp_danmo_novel") {
		t.Fatalf("danmo-novel SKILL.md should reference MCP tools, got %d bytes", len(skillBody))
	}
}
