package service

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"danmo-work/core/domain"

	"gopkg.in/yaml.v3"
)

type AgentManager struct {
	dataDir          string
	pluginExpertDirs []string
	mu               sync.RWMutex
}

func NewAgentManager(dataDir string) *AgentManager {
	return &AgentManager{dataDir: dataDir}
}

// SetPluginExpertDirs replaces the plugin expert directories list (batch set, called by PluginManager.Init).
func (m *AgentManager) SetPluginExpertDirs(dirs []string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.pluginExpertDirs = append([]string{}, dirs...)
}

// RegisterPluginExpertDir appends a single plugin experts/ directory.
func (m *AgentManager) RegisterPluginExpertDir(dir string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, existing := range m.pluginExpertDirs {
		if existing == dir {
			return
		}
	}
	m.pluginExpertDirs = append(m.pluginExpertDirs, dir)
}

// UnregisterPluginExpertDir removes a plugin experts/ directory.
func (m *AgentManager) UnregisterPluginExpertDir(dir string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	filtered := m.pluginExpertDirs[:0]
	for _, d := range m.pluginExpertDirs {
		if d != dir {
			filtered = append(filtered, d)
		}
	}
	m.pluginExpertDirs = filtered
}

func (m *AgentManager) agentDir() string {
	return filepath.Join(m.dataDir, "agents")
}

func (m *AgentManager) List(ctx context.Context) ([]domain.Agent, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	var plugin []domain.Agent
	for _, dir := range m.pluginExpertDirs {
		agents, _ := LoadAgentsFromFS(dir)
		plugin = append(plugin, agents...)
	}
	native, _ := LoadAgentsFromFS(m.agentDir())
	return MergeAgentsByID(plugin, native), nil
}

func (m *AgentManager) Get(ctx context.Context, id string) (*domain.Agent, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	dir := m.agentDir()
	path := filepath.Join(dir, id+".md")
	data, err := os.ReadFile(path)
	if err == nil {
		return parseAgentMarkdown(string(data))
	}
	for _, pDir := range m.pluginExpertDirs {
		path := filepath.Join(pDir, id+".md")
		data, err := os.ReadFile(path)
		if err == nil {
			return parseAgentMarkdown(string(data))
		}
	}
	return nil, fmt.Errorf("agent %q not found", id)
}

func (m *AgentManager) Upsert(ctx context.Context, a domain.Agent) error {
	domain.NormalizeAgentBindings(&a)
	if a.Source == "" {
		a.Source = "user"
	}
	dir := m.agentDir()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	// Never overwrite builtin agents.
	if existing, err := m.Get(ctx, a.ID); err == nil && existing.Source == "builtin" {
		return fmt.Errorf("cannot modify builtin agent %q", a.ID)
	}
	return writeAgentFile(dir, a)
}

func (m *AgentManager) Delete(ctx context.Context, id string) error {
	a, err := m.Get(ctx, id)
	if err != nil {
		return err
	}
	if a.Source == "builtin" {
		return fmt.Errorf("cannot delete builtin agent %q", id)
	}
	return os.Remove(filepath.Join(m.agentDir(), id+".md"))
}

func (m *AgentManager) ResetFromTemplate(ctx context.Context, id string) (*domain.Agent, error) {
	return nil, fmt.Errorf("ResetFromTemplate is no longer supported: builtin agents are read-only")
}

// LoadAgentsFromFS reads all agent files (*.yaml, *.md) from a directory.
func LoadAgentsFromFS(dir string) ([]domain.Agent, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var result []domain.Agent
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".md") {
			continue
		}
		path := filepath.Join(dir, entry.Name())
		data, err := os.ReadFile(path)
		if err != nil {
			log.Printf("[agents] read %s: %v", path, err)
			continue
		}
		agent, err := parseAgentMarkdown(string(data))
		if err != nil {
			log.Printf("[agents] parse %s: %v", path, err)
			continue
		}
		result = append(result, *agent)
	}
	return result, nil
}

func parseAgentMarkdown(content string) (*domain.Agent, error) {
	parts := strings.SplitN(content, "---", 3)
	if len(parts) < 3 {
		return nil, fmt.Errorf("missing frontmatter")
	}
	var fm struct {
		ID             string `yaml:"id"`
		Name           string `yaml:"name"`
		Description    string `yaml:"description"`
		Source         string `yaml:"source"`
		Persona        string `yaml:"persona"`
		Mode           string `yaml:"mode"`
		Steps          int    `yaml:"steps"`
		Skills         []string          `yaml:"skills"`
		Tools          []toolFrontmatter `yaml:"tools"`
		MCPServers     []string          `yaml:"mcp_servers"`
		Knowledge      []string          `yaml:"knowledge"`
		CanDelegate    bool              `yaml:"can_delegate"`
		InheritAmbient *bool             `yaml:"inherit_ambient"`
	}
	if err := yaml.Unmarshal([]byte(strings.TrimSpace(parts[1])), &fm); err != nil {
		return nil, err
	}
	if fm.ID == "" {
		return nil, fmt.Errorf("agent id required")
	}
	source := fm.Source
	if source == "" {
		source = "builtin"
	}
	mode := domain.AgentModePrimary
	if strings.ToLower(fm.Mode) == "subagent" {
		mode = domain.AgentModeSubagent
	}
	var tools []domain.ToolBinding
	for _, t := range fm.Tools {
		mcp := t.MCPServer
		if mcp == "" {
			mcp = t.MCP
		}
		tools = append(tools, domain.ToolBinding{
			ToolID:    t.ToolID,
			MCPServer: mcp,
			RiskLevel: parseAgentRisk(t.RiskLevel),
		})
	}
	a := domain.Agent{
		ID: fm.ID, Name: fm.Name, Description: fm.Description, Source: source, Builtin: source == "builtin",
		Persona: fm.Persona, Mode: mode, SystemPrompt: strings.TrimSpace(parts[2]),
		Steps: fm.Steps, SkillIDs: fm.Skills, Tools: tools, MCPServers: fm.MCPServers,
		KnowledgeIDs: fm.Knowledge, CanDelegate: fm.CanDelegate,
		InheritAmbient: fm.InheritAmbient,
	}
	domain.NormalizeAgentBindings(&a)
	return &a, nil
}

func writeAgentFile(dir string, a domain.Agent) error {
	type agentFM struct {
		ID             string            `yaml:"id"`
		Name           string            `yaml:"name,omitempty"`
		Description    string            `yaml:"description,omitempty"`
		Source         string            `yaml:"source,omitempty"`
		Persona        string            `yaml:"persona,omitempty"`
		Mode           string            `yaml:"mode,omitempty"`
		Steps          int               `yaml:"steps,omitempty"`
		Skills         []string          `yaml:"skills,omitempty"`
		Tools          []toolFrontmatter `yaml:"tools,omitempty"`
		MCPServers     []string          `yaml:"mcp_servers,omitempty"`
		Knowledge      []string          `yaml:"knowledge,omitempty"`
		CanDelegate    bool              `yaml:"can_delegate,omitempty"`
		InheritAmbient *bool             `yaml:"inherit_ambient,omitempty"`
	}
	var tools []toolFrontmatter
	for _, t := range a.Tools {
		tools = append(tools, toolFrontmatter{ToolID: t.ToolID, RiskLevel: string(t.RiskLevel)})
	}
	fm, _ := yaml.Marshal(agentFM{
		ID: a.ID, Name: a.Name, Description: a.Description, Source: a.Source,
		Persona: a.Persona, Mode: string(a.Mode), Steps: a.Steps,
		Skills: a.SkillIDs, Tools: tools, MCPServers: a.MCPServers,
		Knowledge: a.KnowledgeIDs, CanDelegate: a.CanDelegate,
		InheritAmbient: a.InheritAmbient,
	})

	var b strings.Builder
	b.WriteString("---\n")
	b.WriteString(string(fm))
	b.WriteString("---\n")
	if a.SystemPrompt != "" {
		b.WriteString(a.SystemPrompt)
	}
	path := filepath.Join(dir, a.ID+".md")
	return os.WriteFile(path, []byte(b.String()), 0o644)
}

// MergeAgentsByID merges agent slices; later agents override earlier by ID.
func MergeAgentsByID(layers ...[]domain.Agent) []domain.Agent {
	byID := make(map[string]domain.Agent)
	var order []string
	for _, layer := range layers {
		for _, a := range layer {
			if _, exists := byID[a.ID]; !exists {
				order = append(order, a.ID)
			}
			byID[a.ID] = a
		}
	}
	out := make([]domain.Agent, 0, len(order))
	for _, id := range order {
		out = append(out, byID[id])
	}
	return out
}
