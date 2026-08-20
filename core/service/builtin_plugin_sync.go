package service

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"danmo-work/core/domain"
	"danmo-work/core/paths"
	"danmo-work/core/resource/plugins"
)

const builtinPluginsVersionFile = ".builtin_plugins_version"

// Agents/skills/knowledge that used to live under core/resource/home and are now
// shipped as builtin plugins. Removed from data/ on plugin sync so native FS
// copies cannot shadow plugin components (MergeAgentsByID: native wins).
var migratedBuiltinAgentFiles = []string{
	"github.md",
	"danmo-make.md",
	"novel.md",
	"browser.md",
	"operator.md",
}

var migratedBuiltinSkillDirs = []string{
	"github",
	"danmo-make",
	"novel-writing",
	"browser",
	"computer-use",
}

var migratedBuiltinKnowledgeDirs = []string{
	"kb-novel-craft",
}

// SyncBuiltinPlugins materializes embedded first-party plugins into
// $WORK_HOME/plugins/<name>/ and records them in installed.json (Builtin=true).
// Hash-gated like SyncBuiltinToFS. Also cleans leftovers migrated out of home embed.
func SyncBuiltinPlugins(dataDir string) error {
	hash, err := plugins.BuiltinContentHash()
	if err != nil {
		return fmt.Errorf("builtin plugins content hash: %w", err)
	}

	pluginsDir := filepath.Join(filepath.Dir(dataDir), pluginsDirName)
	if err := os.MkdirAll(pluginsDir, 0o755); err != nil {
		return err
	}

	versionFile := filepath.Join(pluginsDir, builtinPluginsVersionFile)
	storedHash, _ := os.ReadFile(versionFile)
	hashMatches := strings.TrimSpace(string(storedHash)) == hash

	// Always strip migrated leftovers so native agents cannot shadow plugins.
	cleanMigratedBuiltinLeftovers(dataDir)

	if hashMatches {
		return ensureBuiltinInstalledRecords(pluginsDir)
	}

	log.Printf("[builtin-plugins] version changed, syncing...")

	for _, name := range plugins.BuiltinPluginNames() {
		if err := materializeBuiltinPlugin(pluginsDir, name); err != nil {
			return fmt.Errorf("materialize plugin %q: %w", name, err)
		}
	}

	if err := ensureBuiltinInstalledRecords(pluginsDir); err != nil {
		return err
	}

	if err := os.WriteFile(versionFile, []byte(hash+"\n"), 0o644); err != nil {
		return err
	}

	names := plugins.BuiltinPluginNames()
	log.Printf("[builtin-plugins] sync complete (hash=%s, plugins=%d)", hash[:16], len(names))
	return nil
}

func materializeBuiltinPlugin(pluginsDir, name string) error {
	files, err := plugins.FilesForPlugin(name)
	if err != nil {
		return err
	}
	destDir := filepath.Join(pluginsDir, name)
	if err := os.RemoveAll(destDir); err != nil {
		return err
	}
	for _, f := range files {
		rel := strings.TrimPrefix(f.Path, name+"/")
		if rel == f.Path || rel == "" || rel == "." {
			continue
		}
		target := filepath.Join(destDir, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err
		}
		mode := os.FileMode(0o644)
		if strings.HasSuffix(rel, ".sh") {
			mode = 0o755
		}
		if err := os.WriteFile(target, f.Content, mode); err != nil {
			return err
		}
	}
	return nil
}

func ensureBuiltinInstalledRecords(pluginsDir string) error {
	installedFile := filepath.Join(pluginsDir, "installed.json")
	installed := make(map[string]domain.PluginInstalled)

	if data, err := os.ReadFile(installedFile); err == nil {
		var m domain.PluginInstalledManifest
		if json.Unmarshal(data, &m) == nil && m.Plugins != nil {
			installed = m.Plugins
		}
	}

	now := time.Now().UTC().Format(time.RFC3339)
	for _, name := range plugins.BuiltinPluginNames() {
		root := filepath.Join(pluginsDir, name)
		if _, err := os.Stat(filepath.Join(root, "plugin.json")); err != nil {
			continue
		}
		rec := installed[name]
		version := rec.Version
		if version == "" {
			version = "1.0.0"
		}
		if mf, err := loadPluginManifestFile(root); err == nil && mf.Version != "" {
			version = mf.Version
		}
		installedAt := rec.InstalledAt
		if installedAt == "" {
			installedAt = now
		}
		installed[name] = domain.PluginInstalled{
			Name:         name,
			Version:      version,
			RootPath:     root,
			MarketSource: domain.PluginMarketSourceBuiltin,
			Builtin:      true,
			InstalledAt:  installedAt,
		}
	}

	m := domain.PluginInstalledManifest{Plugins: installed}
	data, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(installedFile, append(data, '\n'), 0o644)
}

func loadPluginManifestFile(root string) (*domain.PluginManifest, error) {
	data, err := os.ReadFile(filepath.Join(root, "plugin.json"))
	if err != nil {
		return nil, err
	}
	var mf domain.PluginManifest
	if err := json.Unmarshal(data, &mf); err != nil {
		return nil, err
	}
	return &mf, nil
}

func cleanMigratedBuiltinLeftovers(dataDir string) {
	agentsDir := filepath.Join(dataDir, "agents")
	for _, name := range migratedBuiltinAgentFiles {
		path := filepath.Join(agentsDir, name)
		if _, err := os.Stat(path); err != nil {
			continue
		}
		log.Printf("[builtin-plugins] removing migrated agent leftover: %s", name)
		_ = os.Remove(path)
	}

	skillsDir := filepath.Join(dataDir, "skills")
	for _, id := range migratedBuiltinSkillDirs {
		path := filepath.Join(skillsDir, id)
		if _, err := os.Stat(path); err != nil {
			continue
		}
		log.Printf("[builtin-plugins] removing migrated skill leftover: %s", id)
		_ = os.RemoveAll(path)
	}

	for _, id := range migratedBuiltinKnowledgeDirs {
		path := filepath.Join(paths.KnowledgeDir(), id)
		if _, err := os.Stat(path); err != nil {
			continue
		}
		log.Printf("[builtin-plugins] removing migrated knowledge leftover: %s", id)
		_ = os.RemoveAll(path)
	}

	stale := filepath.Join(dataDir, "knowledge")
	for _, id := range migratedBuiltinKnowledgeDirs {
		path := filepath.Join(stale, id)
		if _, err := os.Stat(path); err != nil {
			continue
		}
		log.Printf("[builtin-plugins] removing stale data-dir knowledge: %s", id)
		_ = os.RemoveAll(path)
	}
}
