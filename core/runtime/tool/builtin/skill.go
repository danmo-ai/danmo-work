package builtin

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"danmo-work/core/domain"
	"danmo-work/core/paths"
)

const (
	phDanmoWorkHome = paths.PhDanmoWorkHome
	phAgentsHome    = paths.PhAgentsHome
	phProject       = paths.PhProject
	phWorkHome      = paths.PhWorkHome
)

type ReadSkill struct {
	dataDir    string
	projectDir string
}

func (h *ReadSkill) SetRoots(dataDir, projectDir string) {
	h.dataDir = dataDir
	h.projectDir = projectDir
}

func (h *ReadSkill) Name() string                { return "read_skill" }
func (h *ReadSkill) RiskLevel() domain.RiskLevel { return domain.RiskLow }

func (h *ReadSkill) Describe(args map[string]any) string {
	path, _ := args["path"].(string)
	return path
}

func (h *ReadSkill) Schema() domain.ToolSchema {
	return domain.ToolSchema{
		Name: "read_skill",
		Description: "Read a skill's body or resource file.\n" +
			"Use the <path> from <available_skills> directly.\n" +
			"- Body: path=\"<path>\"\n" +
			"- Resource: path=\"<path>/references/file.md\" or /scripts/... or /assets/...",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"path": map[string]any{
					"type":        "string",
					"description": "Skill path from <available_skills><path>, optionally + /references/... or /scripts/... or /assets/...",
				},
			},
			"required": []string{"path"},
		},
	}
}

func (h *ReadSkill) Execute(ctx context.Context, input map[string]any) (domain.ToolResult, error) {
	p, _ := input["path"].(string)
	if p == "" {
		return domain.ToolResult{}, fmt.Errorf("path is required")
	}
	if strings.Contains(p, "..") {
		return domain.ToolResult{}, fmt.Errorf("invalid path")
	}

	resolved := h.resolvePath(p)
	absPath, err := filepath.Abs(resolved)
	if err != nil {
		return domain.ToolResult{}, fmt.Errorf("path resolution: %w", err)
	}

	if !h.isValid(absPath) {
		return domain.ToolResult{}, fmt.Errorf("path not under a valid skill directory")
	}

	info, err := os.Stat(absPath)
	if err != nil {
		return domain.ToolResult{}, fmt.Errorf("path not found: %s", p)
	}
	if info.IsDir() {
		data, err := os.ReadFile(filepath.Join(absPath, "SKILL.md"))
		if err != nil {
			return domain.ToolResult{}, fmt.Errorf("SKILL.md not found in %s", p)
		}
		_, body, ok := splitSkillFrontmatter(string(data))
		if !ok {
			return domain.ToolResult{}, fmt.Errorf("SKILL.md has no frontmatter in %s", p)
		}
		return domain.ToolResult{Content: body}, nil
	}

	skillRoot := h.findSkillRoot(absPath)
	if skillRoot == "" {
		return domain.ToolResult{}, fmt.Errorf("not a skill resource path: %s", p)
	}
	rel, err := filepath.Rel(skillRoot, absPath)
	if err != nil {
		return domain.ToolResult{}, fmt.Errorf("relative path: %w", err)
	}
	rel = filepath.ToSlash(rel)
	if !isValidSkillResourcePath(rel) {
		return domain.ToolResult{}, fmt.Errorf("resource path must be under scripts/, references/, or assets/: %s", rel)
	}

	data, err := os.ReadFile(absPath)
	if err != nil {
		return domain.ToolResult{}, fmt.Errorf("read file: %w", err)
	}
	return domain.ToolResult{Content: string(data)}, nil
}

func (h *ReadSkill) resolvePath(p string) string {
	p = strings.ReplaceAll(p, phDanmoWorkHome, filepath.Join(h.dataDir))
	p = strings.ReplaceAll(p, phAgentsHome, filepath.Join(home(), ".agents"))
	p = strings.ReplaceAll(p, phWorkHome, paths.WorkHomeFromDataDir(h.dataDir))
	if h.projectDir != "" {
		p = strings.ReplaceAll(p, phProject, h.projectDir)
	}
	return p
}

func (h *ReadSkill) isValid(absPath string) bool {
	mappings := paths.SkillRootMappings(h.dataDir, filepath.Join(home(), ".agents"), h.projectDir, paths.PluginSkillDirs(h.dataDir))
	for _, m := range mappings {
		absRoot, err := filepath.Abs(m.AbsRoot)
		if err != nil {
			continue
		}
		if paths.PathUnderRoot(absPath, absRoot) {
			return true
		}
	}
	return false
}

func (h *ReadSkill) findSkillRoot(absPath string) string {
	dir := filepath.Dir(absPath)
	for {
		if _, err := os.Stat(filepath.Join(dir, "SKILL.md")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return ""
		}
		dir = parent
	}
}

// SkillPathForPrompt returns the placeholder form of a skill directory path for the system prompt.
// Plugin skill dirs under {dataDir}/../plugins/*/skills are discovered from disk.
func SkillPathForPrompt(skillDir, dataDir, agentsHome, projectDir string) string {
	return SkillPathForPromptWithPlugins(skillDir, dataDir, agentsHome, projectDir, paths.PluginSkillDirs(dataDir))
}

// SkillPathForPromptWithPlugins is SkillPathForPrompt with an explicit plugin skills/ list.
func SkillPathForPromptWithPlugins(skillDir, dataDir, agentsHome, projectDir string, pluginSkillDirs []string) string {
	return paths.FormatSkillPath(skillDir, paths.SkillRootMappings(dataDir, agentsHome, projectDir, pluginSkillDirs))
}

func home() string {
	h, err := os.UserHomeDir()
	if err != nil {
		return "."
	}
	return h
}

func isValidSkillResourcePath(p string) bool {
	for _, prefix := range []string{"scripts/", "references/", "assets/"} {
		if strings.HasPrefix(p, prefix) {
			return true
		}
	}
	return false
}

func splitSkillFrontmatter(content string) (fmText, body string, ok bool) {
	lines := strings.Split(strings.TrimPrefix(content, "\ufeff"), "\n")
	if len(lines) == 0 || strings.TrimSpace(lines[0]) != "---" {
		return "", content, false
	}
	for i := 1; i < len(lines); i++ {
		if strings.TrimSpace(lines[i]) == "---" {
			return strings.Join(lines[1:i], "\n"), strings.TrimSpace(strings.Join(lines[i+1:], "\n")), true
		}
	}
	return "", "", false
}
