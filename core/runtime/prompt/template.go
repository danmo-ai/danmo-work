package prompt

import (
	"fmt"
	"io/fs"
	"strings"

	"danmo-work/core/domain"

	"gopkg.in/yaml.v3"
)

type agentFrontmatter struct {
	ID          string                `yaml:"id"`
	Name        string                `yaml:"name"`
	Description string                `yaml:"description"`
	Persona     string                `yaml:"persona"`
	Mode        string                `yaml:"mode"`
	Steps       int                   `yaml:"steps"`
	Skills      []string              `yaml:"skills"`
	Tools       []toolFrontmatter     `yaml:"tools"`
	Knowledge   []string              `yaml:"knowledge"`
	CanDelegate bool                  `yaml:"can_delegate"`
}

type toolFrontmatter struct {
	ToolID    string `yaml:"tool_id"`
	MCP       string `yaml:"mcp"`
	RiskLevel string `yaml:"risk_level"`
}

func parseRisk(s string) domain.RiskLevel {
	switch strings.ToLower(s) {
	case "high":
		return domain.RiskHigh
	case "medium":
		return domain.RiskMedium
	default:
		return domain.RiskLow
	}
}

func parseAgentMode(s string) domain.AgentMode {
	switch strings.ToLower(s) {
	case "subagent":
		return domain.AgentModeSubagent
	default:
		return domain.AgentModePrimary
	}
}

func parseFrontmatter(content string) (agentFrontmatter, string, error) {
	var fm agentFrontmatter
	parts := strings.SplitN(content, "---", 3)
	if len(parts) < 3 {
		return fm, content, nil
	}
	if err := yaml.Unmarshal([]byte(strings.TrimSpace(parts[1])), &fm); err != nil {
		return fm, content, err
	}
	return fm, strings.TrimSpace(parts[2]), nil
}

type AgentTemplate struct {
	Agent  domain.Agent
	Source string
}

func LoadTemplates() ([]AgentTemplate, error) {
	entries, err := fs.ReadDir(AgentTemplates, "agents")
	if err != nil {
		return nil, err
	}
	var result []AgentTemplate
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".md") {
			continue
		}
		data, err := fs.ReadFile(AgentTemplates, "agents/"+entry.Name())
		if err != nil {
			return nil, err
		}
		fm, body, err := parseFrontmatter(string(data))
		if err != nil {
			return nil, err
		}
		if fm.ID == "" {
			continue
		}
		var tools []domain.ToolBinding
		for _, t := range fm.Tools {
			tools = append(tools, domain.ToolBinding{
				ToolID:    t.ToolID,
				MCPServer: t.MCP,
				RiskLevel: parseRisk(t.RiskLevel),
			})
		}
		agent := domain.Agent{
			ID:           fm.ID,
			Name:         fm.Name,
			Description:  fm.Description,
			Persona:      fm.Persona,
			Mode:         parseAgentMode(fm.Mode),
			SystemPrompt: body,
			Steps:        fm.Steps,
			SkillIDs:     fm.Skills,
			Tools:        tools,
			KnowledgeIDs: fm.Knowledge,
			CanDelegate:  fm.CanDelegate,
		}
		result = append(result, AgentTemplate{Agent: agent, Source: entry.Name()})
	}
	return result, nil
}

func LoadTemplateByID(id string) (*domain.Agent, error) {
	templates, err := LoadTemplates()
	if err != nil {
		return nil, err
	}
	for _, t := range templates {
		if t.Agent.ID == id {
			return &t.Agent, nil
		}
	}
	return nil, fs.ErrNotExist
}

type skillFrontmatter struct {
	Name          string            `yaml:"name"`
	Description   string            `yaml:"description"`
	License       string            `yaml:"license"`
	Compatibility string            `yaml:"compatibility"`
	Metadata      map[string]string `yaml:"metadata"`
	AllowedTools  string            `yaml:"allowed-tools"`
}

type SkillTemplate struct {
	Skill  domain.Skill
	Source string
}

func LoadSkillTemplates() ([]SkillTemplate, error) {
	entries, err := fs.ReadDir(SkillTemplates, "skills")
	if err != nil {
		return nil, err
	}
	var result []SkillTemplate
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		skillDir := "skills/" + entry.Name()
		data, err := fs.ReadFile(SkillTemplates, skillDir+"/SKILL.md")
		if err != nil {
			continue
		}
		skill, err := parseSkill(string(data), entry.Name())
		if err != nil {
			continue
		}
		result = append(result, SkillTemplate{Skill: *skill, Source: entry.Name()})
	}
	return result, nil
}

func LoadSkillTemplateByID(id string) (*domain.Skill, error) {
	templates, err := LoadSkillTemplates()
	if err != nil {
		return nil, err
	}
	for _, t := range templates {
		if t.Skill.ID == id {
			s := t.Skill
			return &s, nil
		}
	}
	return nil, fmt.Errorf("skill template %q not found", id)
}

// LoadBuiltinSkillFiles reads all resource files (scripts/, references/, assets/)
// from the embedded FS for a given skill directory name (including nested files).
func LoadBuiltinSkillFiles(skillID string) ([]domain.SkillFile, error) {
	skillDir := "skills/" + skillID
	var files []domain.SkillFile
	for _, sub := range []string{"scripts", "references", "assets"} {
		subDir := skillDir + "/" + sub
		_ = fs.WalkDir(SkillTemplates, subDir, func(fullPath string, d fs.DirEntry, walkErr error) error {
			if walkErr != nil || d.IsDir() {
				return nil
			}
			relPath := strings.TrimPrefix(fullPath, skillDir+"/")
			data, err := fs.ReadFile(SkillTemplates, fullPath)
			if err != nil {
				return nil
			}
			info, _ := d.Info()
			var size int64
			if info != nil {
				size = info.Size()
			}
			files = append(files, domain.SkillFile{
				ID:      skillID + ":" + relPath,
				SkillID: skillID,
				Path:    relPath,
				Content: data,
				Size:    size,
			})
			return nil
		})
	}
	return files, nil
}

func parseSkill(content, dirName string) (*domain.Skill, error) {
	var fm skillFrontmatter
	parts := strings.SplitN(content, "---", 3)
	if len(parts) < 3 {
		return nil, nil
	}
	if err := yaml.Unmarshal([]byte(strings.TrimSpace(parts[1])), &fm); err != nil {
		return nil, err
	}
	if fm.Name == "" {
		return nil, nil
	}
	return &domain.Skill{
		ID:            fm.Name,
		Name:          fm.Name,
		Description:   fm.Description,
		License:       fm.License,
		Compatibility: fm.Compatibility,
		Metadata:      fm.Metadata,
		AllowedTools:  fm.AllowedTools,
		Body:          strings.TrimSpace(parts[2]),
		SourcePath:    dirName,
	}, nil
}
