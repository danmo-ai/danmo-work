package prompt

import (
	"crypto/sha256"
	"fmt"
	"io/fs"
	"strings"

	"danmo-work/core/domain"

	"gopkg.in/yaml.v3"
)

type agentFrontmatter struct {
	ID             string            `yaml:"id"`
	Name           string            `yaml:"name"`
	Description    string            `yaml:"description"`
	Source         string            `yaml:"source"`
	Persona        string            `yaml:"persona"`
	Mode           string            `yaml:"mode"`
	Steps          int               `yaml:"steps"`
	Skills         []string          `yaml:"skills"`
	Tools          []toolFrontmatter `yaml:"tools"`
	MCPServers     []string          `yaml:"mcp_servers"`
	Knowledge      []string          `yaml:"knowledge"`
	CanDelegate    bool              `yaml:"can_delegate"`
	InheritAmbient *bool             `yaml:"inherit_ambient"`
}

type toolFrontmatter struct {
	ToolID    string `yaml:"tool_id"`
	MCP       string `yaml:"mcp"`
	MCPServer string `yaml:"mcp_server"`
	RiskLevel string `yaml:"risk_level"`
}

func parseRisk(s string) domain.RiskLevel {
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

func LoadAgentTemplates() ([]AgentTemplate, error) {
	return loadAgentTemplatesFromFS(BuiltinFS, "builtin/agents")
}

func loadAgentTemplatesFromFS(fsys fs.FS, dir string) ([]AgentTemplate, error) {
	entries, err := fs.ReadDir(fsys, dir)
	if err != nil {
		return nil, err
	}
	var result []AgentTemplate
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".md") {
			continue
		}
		data, err := fs.ReadFile(fsys, dir+"/"+entry.Name())
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
			mcp := t.MCPServer
			if mcp == "" {
				mcp = t.MCP
			}
			tools = append(tools, domain.ToolBinding{
				ToolID:    t.ToolID,
				MCPServer: mcp,
				RiskLevel: parseRisk(t.RiskLevel),
			})
		}
		source := fm.Source
		if source == "" {
			source = "builtin"
		}
		agent := domain.Agent{
			ID:             fm.ID,
			Name:           fm.Name,
			Description:    fm.Description,
			Source:         source,
			Builtin:        source == "builtin",
			Persona:        fm.Persona,
			Mode:           parseAgentMode(fm.Mode),
			SystemPrompt:   body,
			Steps:          fm.Steps,
			SkillIDs:       fm.Skills,
			Tools:          tools,
			MCPServers:     fm.MCPServers,
			KnowledgeIDs:   fm.Knowledge,
			CanDelegate:    fm.CanDelegate,
			InheritAmbient: fm.InheritAmbient,
		}
		domain.NormalizeAgentBindings(&agent)
		result = append(result, AgentTemplate{Agent: agent, Source: entry.Name()})
	}
	return result, nil
}

type skillFrontmatter struct {
	Name          string            `yaml:"name"`
	Description   string            `yaml:"description"`
	Source        string            `yaml:"source"`
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
	return loadSkillTemplatesFromFS(BuiltinFS, "builtin/skills")
}

func loadSkillTemplatesFromFS(fsys fs.FS, dir string) ([]SkillTemplate, error) {
	entries, err := fs.ReadDir(fsys, dir)
	if err != nil {
		return nil, err
	}
	var result []SkillTemplate
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		skillDir := dir + "/" + entry.Name()
		data, err := fs.ReadFile(fsys, skillDir+"/SKILL.md")
		if err != nil {
			continue
		}
		skill, err := parseSkill(string(data), entry.Name())
		if err != nil || skill == nil {
			continue
		}
		result = append(result, SkillTemplate{Skill: *skill, Source: entry.Name()})
	}
	return result, nil
}

// BuiltinFile represents a file to be copied from embedded FS to the filesystem.
type BuiltinFile struct {
	Path    string
	Content []byte
}

// LoadBuiltinFiles returns all embedded builtin agent and skill files as a flat list.
func LoadBuiltinFiles() ([]BuiltinFile, error) {
	var files []BuiltinFile
	if err := fs.WalkDir(BuiltinFS, "builtin", func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}
		data, err := fs.ReadFile(BuiltinFS, path)
		if err != nil {
			return nil
		}
		relPath := strings.TrimPrefix(path, "builtin/")
		files = append(files, BuiltinFile{Path: relPath, Content: data})
		return nil
	}); err != nil {
		return nil, err
	}
	return files, nil
}

// BuiltinManifestHash returns the SHA256 hash of the embedded manifest.yaml content.
func BuiltinManifestHash() (string, error) {
	data, err := fs.ReadFile(BuiltinFS, "builtin/manifest.yaml")
	if err != nil {
		return "", err
	}
	h := sha256.Sum256(data)
	return fmt.Sprintf("%x", h), nil
}

// loadBuiltinSkillFiles reads resource files for a skill from embedded FS.
func loadBuiltinSkillFiles(fsys fs.FS, dir string, skillID string) ([]domain.SkillFile, error) {
	skillDir := dir + "/" + skillID
	var files []domain.SkillFile
	for _, sub := range []string{"scripts", "references", "assets"} {
		subDir := skillDir + "/" + sub
		_ = fs.WalkDir(fsys, subDir, func(fullPath string, d fs.DirEntry, walkErr error) error {
			if walkErr != nil || d.IsDir() {
				return nil
			}
			relPath := strings.TrimPrefix(fullPath, skillDir+"/")
			data, err := fs.ReadFile(fsys, fullPath)
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
	fmText, body, ok := splitFrontmatter(content)
	if !ok {
		return nil, fmt.Errorf("skill %q: missing YAML frontmatter", dirName)
	}
	var fm skillFrontmatter
	if err := yaml.Unmarshal([]byte(fmText), &fm); err != nil {
		return nil, fmt.Errorf("skill %q: parse frontmatter: %w", dirName, err)
	}
	source := fm.Source
	if source == "" {
		source = "builtin"
	}
	builtin := source == "builtin"
	name := fm.Name
	if name == "" {
		name = dirName
	}
	return &domain.Skill{
		ID:            dirName,
		Name:          name,
		Description:   fm.Description,
		Source:        source,
		Builtin:       builtin,
		License:       fm.License,
		Compatibility: fm.Compatibility,
		Metadata:      fm.Metadata,
		AllowedTools:  fm.AllowedTools,
		Body:          strings.TrimSpace(body),
		SourcePath:    dirName,
	}, nil
}

func splitFrontmatter(content string) (fmText, body string, ok bool) {
	content = strings.TrimPrefix(content, "\ufeff")
	lines := strings.Split(content, "\n")
	if len(lines) == 0 || strings.TrimSpace(lines[0]) != "---" {
		return "", "", false
	}
	for i := 1; i < len(lines); i++ {
		if strings.TrimSpace(lines[i]) == "---" {
			return strings.Join(lines[1:i], "\n"), strings.Join(lines[i+1:], "\n"), true
		}
	}
	return "", "", false
}
