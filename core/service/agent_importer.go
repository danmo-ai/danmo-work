package service

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"danmo-work/core/domain"

	"gopkg.in/yaml.v3"
)

// AgentImporter parses AGENT.md (or agents-style markdown) into domain.Agent.
type AgentImporter struct{}

func NewAgentImporter() *AgentImporter {
	return &AgentImporter{}
}

type agentFrontmatter struct {
	ID             string            `yaml:"id"`
	Name           string            `yaml:"name"`
	Description    string            `yaml:"description"`
	Persona        string            `yaml:"persona"`
	Mode           string            `yaml:"mode"`
	Steps          int               `yaml:"steps"`
	Skills         []string          `yaml:"skills"`
	Tools          []toolFrontmatter `yaml:"tools"`
	Knowledge      []string          `yaml:"knowledge"`
	CanDelegate    bool              `yaml:"can_delegate"`
	InheritAmbient *bool             `yaml:"inherit_ambient"`
	Metadata       map[string]string `yaml:"metadata"`
}

type toolFrontmatter struct {
	ToolID    string `yaml:"tool_id"`
	MCP       string `yaml:"mcp"`        // legacy alias
	MCPServer string `yaml:"mcp_server"` // preferred: bind by MCP server id or "*"
	RiskLevel string `yaml:"risk_level"`
}

func (i *AgentImporter) Import(dirPath string) (*domain.Agent, error) {
	candidates := []string{
		filepath.Join(dirPath, "AGENT.md"),
		filepath.Join(dirPath, "agent.md"),
	}
	// Also accept a single .md named after the folder.
	base := filepath.Base(dirPath)
	candidates = append(candidates, filepath.Join(dirPath, base+".md"))

	var content []byte
	var err error
	for _, p := range candidates {
		content, err = os.ReadFile(p)
		if err == nil {
			break
		}
	}
	if err != nil {
		return nil, fmt.Errorf("AGENT.md not found in %s", dirPath)
	}
	return i.ParseAgentMD(string(content))
}

func (i *AgentImporter) ParseAgentMD(content string) (*domain.Agent, error) {
	var fm agentFrontmatter
	parts := strings.SplitN(content, "---", 3)
	if len(parts) < 3 {
		return nil, fmt.Errorf("invalid AGENT.md: missing YAML frontmatter")
	}
	if err := yaml.Unmarshal([]byte(strings.TrimSpace(parts[1])), &fm); err != nil {
		return nil, err
	}
	if fm.ID == "" {
		return nil, fmt.Errorf("invalid AGENT.md: id is required in frontmatter")
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
	name := fm.Name
	if name == "" {
		name = fm.ID
	}
	marketSrc := ""
	if fm.Metadata != nil {
		marketSrc = fm.Metadata["market.source"]
	}
	return &domain.Agent{
		ID:             fm.ID,
		Name:           name,
		Description:    fm.Description,
		Persona:        fm.Persona,
		Mode:           parseAgentImportMode(fm.Mode),
		SystemPrompt:   strings.TrimSpace(parts[2]),
		Steps:          fm.Steps,
		SkillIDs:       fm.Skills,
		Tools:          tools,
		KnowledgeIDs:   fm.Knowledge,
		CanDelegate:    fm.CanDelegate,
		InheritAmbient: fm.InheritAmbient,
		MarketSource:   marketSrc,
	}, nil
}

func parseAgentRisk(s string) domain.RiskLevel {
	switch strings.ToLower(s) {
	case "external":
		return domain.RiskExternal
	case "high":
		return domain.RiskHigh
	case "medium":
		return domain.RiskMedium
	default:
		return domain.RiskLow
	}
}

func parseAgentImportMode(s string) domain.AgentMode {
	switch strings.ToLower(s) {
	case "subagent":
		return domain.AgentModeSubagent
	default:
		return domain.AgentModePrimary
	}
}
