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
	dataDir       string
	projectDir    string
	projectLookup func(projectID string) string
}

func (h *ReadSkill) SetRoots(dataDir, projectDir string) {
	h.dataDir = dataDir
	h.projectDir = projectDir
}

func (h *ReadSkill) SetProjectLookup(fn func(projectID string) string) {
	h.projectLookup = fn
}

func (h *ReadSkill) Name() string                { return "read_skill" }
func (h *ReadSkill) RiskLevel() domain.RiskLevel { return domain.RiskLow }

func (h *ReadSkill) Describe(args map[string]any) string {
	if id, _ := args["id"].(string); strings.TrimSpace(id) != "" {
		return strings.TrimSpace(id)
	}
	path, _ := args["path"].(string)
	return path
}

func (h *ReadSkill) Schema() domain.ToolSchema {
	return domain.ToolSchema{
		Name: "read_skill",
		Description: "Read a skill's body or resource file by skill id from <available_skills>.\n" +
			"- Body: id=\"<id>\" (or path=\"<id>\")\n" +
			"- Resource: path=\"<id>/references/file.md\" or /scripts/... or /assets/...\n" +
			"- Optional project_id: search that project's skill dirs first, then plugins, then Home.",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"id": map[string]any{
					"type":        "string",
					"description": "Skill id from <available_skills><id> (relative path under a skill root).",
				},
				"path": map[string]any{
					"type":        "string",
					"description": "Alias of id; may append /references/..., /scripts/..., or /assets/...",
				},
				"project_id": map[string]any{
					"type":        "string",
					"description": "Optional project id. Project skills override plugin and Home skills with the same id.",
				},
			},
		},
	}
}

func (h *ReadSkill) Execute(ctx context.Context, input map[string]any) (domain.ToolResult, error) {
	p := strings.TrimSpace(skillStringArg(input, "id"))
	if p == "" {
		p = strings.TrimSpace(skillStringArg(input, "path"))
	}
	if p == "" {
		return domain.ToolResult{}, fmt.Errorf("id or path is required")
	}
	if strings.Contains(p, "..") {
		return domain.ToolResult{}, fmt.Errorf("invalid path")
	}

	projectDir := h.projectDir
	if pid := strings.TrimSpace(skillStringArg(input, "project_id")); pid != "" && h.projectLookup != nil {
		if d := h.projectLookup(pid); d != "" {
			projectDir = d
		}
	}

	if dir, rest := h.lookupByID(p, projectDir); dir != "" {
		return h.readAt(dir, rest, p)
	}

	var lastErr error
	seen := make(map[string]struct{})
	for _, cand := range h.fallbackCandidates(p) {
		absPath, err := filepath.Abs(cand)
		if err != nil {
			lastErr = fmt.Errorf("path resolution: %w", err)
			continue
		}
		if _, ok := seen[absPath]; ok {
			continue
		}
		seen[absPath] = struct{}{}
		if !h.isValid(absPath) {
			if lastErr == nil {
				lastErr = fmt.Errorf("path not under a valid skill directory")
			}
			continue
		}
		result, err := h.readAbs(absPath, p)
		if err == nil {
			return result, nil
		}
		lastErr = err
	}
	if lastErr != nil {
		return domain.ToolResult{}, lastErr
	}
	return domain.ToolResult{}, fmt.Errorf("skill %q not found", p)
}

func skillStringArg(input map[string]any, key string) string {
	v, _ := input[key].(string)
	return v
}

func (h *ReadSkill) lookupByID(p, projectDir string) (skillDir, resource string) {
	slash := strings.Trim(filepath.ToSlash(p), "/")
	if slash == "" || strings.Contains(slash, "{") || filepath.IsAbs(p) {
		return "", ""
	}
	roots := paths.SkillLookupRoots(
		paths.HomeSkillDirs(h.dataDir),
		paths.PluginSkillDirs(h.dataDir),
		paths.ProjectSkillDirs(projectDir),
	)
	parts := strings.Split(slash, "/")
	for i := len(parts); i >= 1; i-- {
		id := strings.Join(parts[:i], "/")
		rest := strings.Join(parts[i:], "/")
		for _, root := range roots {
			dir := paths.JoinSkillID(root, id)
			if paths.HasSKILLMD(dir) {
				return dir, rest
			}
		}
	}
	return "", ""
}

func (h *ReadSkill) fallbackCandidates(p string) []string {
	expanded := h.resolvePath(p)
	var out []string
	add := func(s string) {
		s = strings.TrimSpace(s)
		if s != "" {
			out = append(out, s)
		}
	}
	add(expanded)
	add(remapPluginPath(expanded, h.dataDir))
	add(remapPluginPath(p, h.dataDir))
	return out
}

func remapPluginPath(p, dataDir string) string {
	rel := pluginSkillsRel(p)
	if rel == "" {
		return ""
	}
	return filepath.Join(paths.WorkHomeFromDataDir(dataDir), filepath.FromSlash(rel))
}

func pluginSkillsRel(p string) string {
	slash := filepath.ToSlash(p)
	rel := ""
	if i := strings.Index(slash, "/plugins/"); i >= 0 {
		rel = slash[i+1:]
	} else if strings.HasPrefix(slash, "plugins/") {
		rel = slash
	} else {
		return ""
	}
	parts := strings.Split(rel, "/")
	if len(parts) < 4 || parts[0] != "plugins" || parts[2] != "skills" || parts[1] == "" || parts[3] == "" {
		return ""
	}
	return rel
}

func (h *ReadSkill) readAt(skillDir, resource, orig string) (domain.ToolResult, error) {
	absDir, err := filepath.Abs(skillDir)
	if err != nil {
		return domain.ToolResult{}, fmt.Errorf("path resolution: %w", err)
	}
	if resource == "" {
		return h.readAbs(absDir, orig)
	}
	full := filepath.Join(absDir, filepath.FromSlash(resource))
	absFull, err := filepath.Abs(full)
	if err != nil {
		return domain.ToolResult{}, fmt.Errorf("path resolution: %w", err)
	}
	if !paths.PathUnderRoot(absFull, absDir) {
		return domain.ToolResult{}, fmt.Errorf("invalid path")
	}
	return h.readAbs(absFull, orig)
}

func (h *ReadSkill) readAbs(absPath, orig string) (domain.ToolResult, error) {
	info, err := os.Stat(absPath)
	if err != nil {
		return domain.ToolResult{}, fmt.Errorf("path not found: %s", orig)
	}
	if info.IsDir() {
		data, err := os.ReadFile(filepath.Join(absPath, "SKILL.md"))
		if err != nil {
			return domain.ToolResult{}, fmt.Errorf("SKILL.md not found in %s", orig)
		}
		_, body, ok := splitSkillFrontmatter(string(data))
		if !ok {
			return domain.ToolResult{}, fmt.Errorf("SKILL.md has no frontmatter in %s", orig)
		}
		return domain.ToolResult{Content: body}, nil
	}

	skillRoot := h.findSkillRoot(absPath)
	if skillRoot == "" {
		return domain.ToolResult{}, fmt.Errorf("not a skill resource path: %s", orig)
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
