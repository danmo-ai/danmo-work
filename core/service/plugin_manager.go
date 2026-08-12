package service

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"danmo-work/core/domain"
)

const pluginsDirName = "plugins"

// PluginManager manages plugin lifecycle: install, uninstall, and startup registration.
type PluginManager struct {
	pluginsDir    string
	installedFile string

	skillManager     *SkillManager
	agentManager     *AgentManager
	mcpManager       *MCPManager
	knowledgeManager *KnowledgeManager

	installed map[string]domain.PluginInstalled
	mu        sync.RWMutex
}

func NewPluginManager(
	dataDir string,
	sm *SkillManager,
	am *AgentManager,
	mm *MCPManager,
	km *KnowledgeManager,
) *PluginManager {
	pluginsDir := filepath.Join(dataDir, "..", pluginsDirName)
	installedFile := filepath.Join(pluginsDir, "installed.json")
	_ = os.MkdirAll(pluginsDir, 0o755)

	return &PluginManager{
		pluginsDir:       pluginsDir,
		installedFile:    installedFile,
		skillManager:     sm,
		agentManager:     am,
		mcpManager:       mm,
		knowledgeManager: km,
		installed:        make(map[string]domain.PluginInstalled),
	}
}

// Init scans installed plugins and registers their components with each manager.
func (pm *PluginManager) Init(ctx context.Context) error {
	if err := pm.loadInstalled(); err != nil {
		return err
	}

	var skillDirs []string
	var expertDirs []string
	var mcpFiles []string
	var kbRoots []string

	for _, p := range pm.installed {
		root := p.RootPath
		if root == "" {
			continue
		}

		if _, err := pm.LoadPluginManifest(root); err != nil {
			log.Printf("[plugins] %s: invalid manifest: %v", p.Name, err)
			continue
		}

		sd := filepath.Join(root, "skills")
		if st, err := os.Stat(sd); err == nil && st.IsDir() {
			skillDirs = append(skillDirs, sd)
		}

		mcpPath := filepath.Join(root, "mcp.json")
		if st, err := os.Stat(mcpPath); err == nil && !st.IsDir() {
			mcpFiles = append(mcpFiles, mcpPath)
		}

		extDir := filepath.Join(root, domain.DanmoWorkExtensionKey)
		if st, err := os.Stat(extDir); err == nil && st.IsDir() {
			ed := filepath.Join(extDir, "experts")
			if st, err := os.Stat(ed); err == nil && st.IsDir() {
				expertDirs = append(expertDirs, ed)
			}

			kd := filepath.Join(extDir, "knowledge")
			if st, err := os.Stat(kd); err == nil && st.IsDir() {
				kbRoots = append(kbRoots, kd)
			}
		}
	}

	pm.skillManager.SetPluginSkillDirs(skillDirs)
	pm.agentManager.SetPluginExpertDirs(expertDirs)
	pm.knowledgeManager.SetPluginKBRoots(kbRoots)

	if err := pm.mcpManager.SetPluginMCPFiles(mcpFiles); err != nil {
		log.Printf("[plugins] mcp register: %v", err)
	}

	if err := pm.knowledgeManager.ScanPluginBases(ctx); err != nil {
		log.Printf("[plugins] knowledge register: %v", err)
	}

	log.Printf("[plugins] init: %d installed, %d skill dirs, %d expert dirs, %d mcp files, %d kb roots",
		len(pm.installed), len(skillDirs), len(expertDirs), len(mcpFiles), len(kbRoots))
	return nil
}

// InstallPlugin installs a plugin from a package directory.
// depsScript is the relative deps script path (from catalog item deps).
func (pm *PluginManager) InstallPlugin(ctx context.Context, packageDir, depsScript string) error {
	manifest, err := pm.LoadPluginManifest(packageDir)
	if err != nil {
		return err
	}
	if err := domain.ValidatePluginName(manifest.Name); err != nil {
		return err
	}

	destDir := filepath.Join(pm.pluginsDir, manifest.Name)
	if _, err := os.Stat(destDir); err == nil {
		_ = os.RemoveAll(destDir)
	}
	if err := copyDir(packageDir, destDir); err != nil {
		return fmt.Errorf("copy plugin to %s: %w", destDir, err)
	}

	if depsScript != "" {
		scriptPath := filepath.Join(destDir, filepath.FromSlash(depsScript))
		if logOut, err := RunConnectorDepsScript(ctx, destDir, scriptPath, manifest.Name); err != nil {
			return fmt.Errorf("install deps %s: %w\ndeps output:\n%s", depsScript, err, logOut)
		}
	}

	sd := filepath.Join(destDir, "skills")
	if st, err := os.Stat(sd); err == nil && st.IsDir() {
		pm.skillManager.RegisterPluginSkillDir(sd)
	}

	mcpPath := filepath.Join(destDir, "mcp.json")
	if st, err := os.Stat(mcpPath); err == nil && !st.IsDir() {
		if err := pm.mcpManager.RegisterPluginMCP(mcpPath); err != nil {
			log.Printf("[plugins] %s: mcp register: %v", manifest.Name, err)
		}
	}

	extDir := filepath.Join(destDir, domain.DanmoWorkExtensionKey)
	if st, err := os.Stat(extDir); err == nil && st.IsDir() {
		ed := filepath.Join(extDir, "experts")
		if st, err := os.Stat(ed); err == nil && st.IsDir() {
			pm.agentManager.RegisterPluginExpertDir(ed)
		}
		kd := filepath.Join(extDir, "knowledge")
		if st, err := os.Stat(kd); err == nil && st.IsDir() {
			pm.knowledgeManager.RegisterPluginKB(kd)
		}
	}

	pm.mu.Lock()
	pm.installed[manifest.Name] = domain.PluginInstalled{
		Name:         manifest.Name,
		Version:      manifest.Version,
		RootPath:     destDir,
		MarketSource: "",
		InstalledAt:  time.Now().UTC().Format(time.RFC3339),
	}
	pm.mu.Unlock()

	if err := pm.saveInstalled(); err != nil {
		return err
	}

	log.Printf("[plugins] installed %s v%s", manifest.Name, manifest.Version)
	return nil
}

// UninstallPlugin removes a plugin by name.
// depsScript is the relative uninstall deps script path (from catalog item uninstallDeps).
func (pm *PluginManager) UninstallPlugin(ctx context.Context, name, depsScript string) error {
	pm.mu.RLock()
	p, ok := pm.installed[name]
	pm.mu.RUnlock()
	if !ok {
		return fmt.Errorf("plugin %q not installed", name)
	}

	if depsScript != "" {
		scriptPath := filepath.Join(p.RootPath, filepath.FromSlash(depsScript))
		if logOut, err := RunConnectorDepsScript(ctx, p.RootPath, scriptPath, name); err != nil {
			log.Printf("[plugins] %s: uninstall deps %s: %v\ndeps output:\n%s", name, depsScript, err, logOut)
		}
	}

	pluginRoot := p.RootPath

	sd := filepath.Join(pluginRoot, "skills")
	if st, err := os.Stat(sd); err == nil && st.IsDir() {
		pm.skillManager.UnregisterPluginSkillDir(sd)
	}

	pm.mcpManager.UnregisterPluginMCP(name)

	extDir := filepath.Join(pluginRoot, domain.DanmoWorkExtensionKey)
	if st, err := os.Stat(extDir); err == nil && st.IsDir() {
		pm.agentManager.UnregisterPluginExpertDir(filepath.Join(extDir, "experts"))
		kd := filepath.Join(extDir, "knowledge")
		if st, err := os.Stat(kd); err == nil && st.IsDir() {
			_ = pm.knowledgeManager.UnregisterPluginKB(ctx, kd)
		}
	}

	_ = os.RemoveAll(pluginRoot)

	pm.mu.Lock()
	delete(pm.installed, name)
	pm.mu.Unlock()

	if err := pm.saveInstalled(); err != nil {
		return err
	}

	log.Printf("[plugins] uninstalled %s", name)
	return nil
}

// ListInstalled returns all installed plugins enriched with manifest metadata and components.
func (pm *PluginManager) ListInstalled() []domain.PluginInstalled {
	pm.mu.RLock()
	defer pm.mu.RUnlock()
	out := make([]domain.PluginInstalled, 0, len(pm.installed))
	for _, p := range pm.installed {
		enriched := p
		pm.enrichFromManifest(&enriched)
		pm.scanComponents(&enriched)
		out = append(out, enriched)
	}
	return out
}

func (pm *PluginManager) enrichFromManifest(p *domain.PluginInstalled) {
	if p.RootPath == "" {
		return
	}
	mf, err := pm.LoadPluginManifest(p.RootPath)
	if err != nil || mf == nil {
		return
	}
	p.Version = mf.Version
	p.Description = mf.Description
	p.Author = mf.Author
	p.Homepage = mf.Homepage
	p.Repository = mf.Repository
	p.License = mf.License
	p.Keywords = mf.Keywords
}

func (pm *PluginManager) scanComponents(p *domain.PluginInstalled) {
	if p.RootPath == "" {
		return
	}
	root := p.RootPath

	sd := filepath.Join(root, "skills")
	if entries, err := os.ReadDir(sd); err == nil {
		for _, e := range entries {
			if e.IsDir() {
				p.Components.Skills = append(p.Components.Skills, e.Name())
			}
		}
	}

	mcpPath := filepath.Join(root, "mcp.json")
	if data, err := os.ReadFile(mcpPath); err == nil {
		var doc mcpSpecDoc
		if json.Unmarshal(data, &doc) == nil {
			for id := range doc.MCPServers {
				p.Components.MCP = append(p.Components.MCP, id)
			}
		}
	}

	extDir := filepath.Join(root, domain.DanmoWorkExtensionKey)
	if st, err := os.Stat(extDir); err == nil && st.IsDir() {
		ed := filepath.Join(extDir, "experts")
		if entries, err := os.ReadDir(ed); err == nil {
			for _, e := range entries {
				if !e.IsDir() && strings.HasSuffix(e.Name(), ".md") {
					id := strings.TrimSuffix(e.Name(), ".md")
					p.Components.Experts = append(p.Components.Experts, id)
				}
			}
		}

		kd := filepath.Join(extDir, "knowledge")
		if st, err := os.Stat(kd); err == nil && st.IsDir() {
			p.Components.Knowledge = []string{p.Name}
		}
	}
}

// LoadPluginManifest reads and validates plugin.json from a directory.
func (pm *PluginManager) LoadPluginManifest(rootPath string) (*domain.PluginManifest, error) {
	manifestPath := filepath.Join(rootPath, "plugin.json")
	data, err := os.ReadFile(manifestPath)
	if err != nil {
		return nil, fmt.Errorf("read plugin.json: %w", err)
	}
	var m domain.PluginManifest
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, fmt.Errorf("parse plugin.json: %w", err)
	}
	if m.Name == "" {
		return nil, fmt.Errorf("plugin.json: name is required")
	}
	return &m, nil
}

func (pm *PluginManager) loadInstalled() error {
	data, err := os.ReadFile(pm.installedFile)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	var m domain.PluginInstalledManifest
	if err := json.Unmarshal(data, &m); err != nil {
		return err
	}
	pm.installed = m.Plugins
	if pm.installed == nil {
		pm.installed = make(map[string]domain.PluginInstalled)
	}
	return nil
}

func (pm *PluginManager) saveInstalled() error {
	pm.mu.RLock()
	defer pm.mu.RUnlock()
	m := domain.PluginInstalledManifest{Plugins: pm.installed}
	data, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(pm.installedFile, append(data, '\n'), 0o644)
}

func copyDir(src, dst string) error {
	srcInfo, err := os.Stat(src)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(dst, srcInfo.Mode()); err != nil {
		return err
	}
	entries, err := os.ReadDir(src)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		srcPath := filepath.Join(src, entry.Name())
		dstPath := filepath.Join(dst, entry.Name())
		if entry.IsDir() {
			if err := copyDir(srcPath, dstPath); err != nil {
				return err
			}
		} else {
			data, err := os.ReadFile(srcPath)
			if err != nil {
				return err
			}
			info, err := entry.Info()
			if err != nil {
				return err
			}
			if err := os.WriteFile(dstPath, data, info.Mode()); err != nil {
				return err
			}
		}
	}
	return nil
}
