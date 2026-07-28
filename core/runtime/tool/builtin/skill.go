package builtin

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"danmo-work/core/domain"
	"danmo-work/core/service"
)

// ReadSkill reads a skill's instructions or its bundled resource files.
// Prefers per-turn filesystem skills (scanned on New Turn); falls back to DB.
//
// Path convention:
//   - "git-workflow"              → skill body (first segment = skill meta id)
//   - "debugging/references/patterns.md" → resource file (id + relative path)
//   - Skill id matches <available_skills><path>, not display name / directory name
//   - Valid resource subdirectories: scripts/, references/, assets/
//   - Bare paths like "references/foo.md" are rejected (no current-skill context)
type ReadSkill struct {
	Skills *service.SkillManager

	mu       sync.RWMutex
	fsSkills map[string]domain.Skill
	fsFiles  map[string][]domain.SkillFile
}

// SetTurnFS updates the filesystem skill overlay for the current turn.
func (h *ReadSkill) SetTurnFS(skills map[string]domain.Skill, files map[string][]domain.SkillFile) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.fsSkills = skills
	h.fsFiles = files
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
		Description: "Read a skill's instructions or bundled resource files.\n\n" +
			"**Path format:**\n" +
			"- First path segment is the skill **meta id** from <available_skills><path> (not display name, not folder name).\n" +
			"- Skill instructions: path=\"git-workflow\" (id only)\n" +
			"- Resource file: path=\"debugging/references/patterns.md\" (id + relative path)\n" +
			"- Market skills often use namespaced ids (e.g. path=\"tlc__pr-review/references/guide.md\").\n" +
			"- Resource subdirectories: scripts/, references/, assets/\n" +
			"- Do **not** use bare resource paths (references/…, scripts/…, assets/…) — there is no current-skill context.\n" +
			"- Do **not** use read_file / exec_shell to load skill pack files; use read_skill.\n\n" +
			"**Examples:**\n" +
			"- path=\"git-workflow\"                       → skill instructions\n" +
			"- path=\"debugging/references/patterns.md\"   → reference file\n" +
			"- path=\"tlc__pr-review/references/guide.md\" → market skill resource\n\n" +
			"**Anti-examples (invalid):**\n" +
			"- path=\"references/patterns.md\"             → missing skill id\n" +
			"- path=\"scripts/run.sh\"                     → missing skill id",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"path": map[string]any{
					"type":        "string",
					"description": "Skill meta id from <available_skills><path> (e.g. \"git-workflow\"), or id + resource path (e.g. \"debugging/references/patterns.md\", \"tlc__pr-review/references/guide.md\"). Never bare references/… / scripts/… / assets/….",
				},
			},
			"required": []string{"path"},
		},
	}
}

func (h *ReadSkill) Execute(ctx context.Context, input map[string]any) (domain.ToolResult, error) {
	path, _ := input["path"].(string)
	if path == "" {
		return domain.ToolResult{}, fmt.Errorf("path is required")
	}

	// Security: reject path traversal
	if strings.Contains(path, "..") {
		return domain.ToolResult{}, fmt.Errorf("invalid path: must not contain \"..\"")
	}

	if err := rejectBareSkillResourcePath(path); err != nil {
		return domain.ToolResult{}, err
	}

	// Split path: first segment is skill ID, rest is resource path
	parts := strings.SplitN(path, "/", 2)
	skillID := parts[0]
	resPath := ""
	if len(parts) > 1 {
		resPath = parts[1]
	}

	sk, files, fromFS := h.lookup(ctx, skillID)
	if sk == nil {
		return domain.ToolResult{}, fmt.Errorf("skill %q not found", skillID)
	}

	// No resource path → return skill body (instructions)
	if resPath == "" {
		if sk.Body == "" {
			return domain.ToolResult{Content: fmt.Sprintf("Skill %q has no instructions.", skillID)}, nil
		}
		return domain.ToolResult{Content: sk.Body}, nil
	}

	// Validate resource subdirectory
	if !isValidResourcePath(resPath) {
		return domain.ToolResult{}, fmt.Errorf("invalid resource path %q: must be under scripts/, references/, or assets/", resPath)
	}

	if !fromFS && h.Skills != nil {
		var err error
		files, err = h.Skills.Files(ctx, skillID)
		if err != nil {
			return domain.ToolResult{}, fmt.Errorf("failed to list files for skill %q: %w", skillID, err)
		}
	}

	for _, f := range files {
		if f.Path == resPath {
			return domain.ToolResult{Content: string(f.Content)}, nil
		}
	}

	var available []string
	for _, f := range files {
		available = append(available, skillID+"/"+f.Path)
	}
	if len(available) > 0 {
		return domain.ToolResult{}, fmt.Errorf("resource %q not found in skill %q. Available: %s",
			path, skillID, strings.Join(available, ", "))
	}
	return domain.ToolResult{}, fmt.Errorf("resource %q not found in skill %q (no resource files available)", path, skillID)
}

// lookup prefers the turn filesystem overlay, then the DB SkillManager.
func (h *ReadSkill) lookup(ctx context.Context, skillID string) (*domain.Skill, []domain.SkillFile, bool) {
	h.mu.RLock()
	if sk, ok := h.fsSkills[skillID]; ok {
		files := h.fsFiles[skillID]
		h.mu.RUnlock()
		cp := sk
		return &cp, files, true
	}
	h.mu.RUnlock()

	if h.Skills == nil {
		return nil, nil, false
	}
	sk, err := h.Skills.Get(ctx, skillID)
	if err != nil || sk == nil {
		return nil, nil, false
	}
	return sk, nil, false
}

func isValidResourcePath(p string) bool {
	for _, prefix := range []string{"scripts/", "references/", "assets/"} {
		if strings.HasPrefix(p, prefix) {
			return true
		}
	}
	return false
}

// rejectBareSkillResourcePath fails fast when the model omits the skill id.
func rejectBareSkillResourcePath(path string) error {
	p := strings.TrimSpace(path)
	p = strings.TrimPrefix(p, "./")
	for _, root := range []string{"scripts", "references", "assets"} {
		if p == root || strings.HasPrefix(p, root+"/") {
			return fmt.Errorf(
				"bare resource path %q: include the skill id from <available_skills><path> (e.g. \"<skill-id>/%s\")",
				path, p,
			)
		}
	}
	return nil
}
