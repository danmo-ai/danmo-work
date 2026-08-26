package service

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"danmo-work/core/domain"

	"gopkg.in/yaml.v3"
)

type SkillImporter struct{}

type skillFrontmatter struct {
	Name          string `yaml:"name"`
	Description   string `yaml:"description"`
	License       string `yaml:"license"`
	Compatibility string `yaml:"compatibility"`
	Metadata      any    `yaml:"metadata"`
	AllowedTools  string `yaml:"allowed-tools"`
	Source        string `yaml:"source"`
}

func NewSkillImporter() *SkillImporter {
	return &SkillImporter{}
}

const SkillMetaRealPath = "real_path"

func attachSkillRealPath(sk *domain.Skill, dir string) {
	if sk == nil {
		return
	}
	sk.Dir = dir
	if sk.Metadata == nil {
		sk.Metadata = make(map[string]string)
	}
	sk.Metadata[SkillMetaRealPath] = dir
}

func skipSkillWalkDir(root, path, name string) bool {
	if path == root {
		return false
	}
	switch name {
	case "references", "scripts", "assets", "node_modules", ".git":
		return true
	}
	return strings.HasPrefix(name, ".")
}

func (i *SkillImporter) Import(dirPath string) (*domain.Skill, []domain.SkillFile, error) {
	skillMD, err := os.ReadFile(filepath.Join(dirPath, "SKILL.md"))
	if err != nil {
		return nil, nil, err
	}

	skill, err := i.ParseSkillMD(string(skillMD))
	if err != nil {
		return nil, nil, err
	}
	if skill == nil {
		return nil, nil, fmt.Errorf("invalid SKILL.md: missing or empty name in frontmatter")
	}
	skill.SourcePath = dirPath
	attachSkillRealPath(skill, dirPath)
	// Prefix bare resource refs with skill meta id so read_skill paths match.
	skill.Body = NormalizeSkillBodyRefs(skill.Body, skill.ID)

	var files []domain.SkillFile
	_ = filepath.WalkDir(dirPath, func(fullPath string, d os.DirEntry, walkErr error) error {
		if walkErr != nil || d.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(dirPath, fullPath)
		if err != nil {
			return nil
		}
		relPath := filepath.ToSlash(rel)
		base := filepath.Base(relPath)
		// Skip the skill body itself and common meta/junk files.
		if relPath == "SKILL.md" || relPath == "skill.md" || relPath == "skills.md" {
			return nil
		}
		if strings.HasPrefix(base, ".") || base == "_meta.json" || base == ".DS_Store" {
			return nil
		}
		// Skip nested VCS / lock junk if present.
		if strings.Contains(relPath, "/.git/") || strings.HasPrefix(relPath, ".git/") {
			return nil
		}
		data, err := os.ReadFile(fullPath)
		if err != nil {
			return nil
		}
		info, _ := d.Info()
		size := int64(0)
		if info != nil {
			size = info.Size()
		}
		files = append(files, domain.SkillFile{
			ID:      skill.ID + ":" + relPath,
			SkillID: skill.ID,
			Path:    relPath,
			Content: data,
			Size:    size,
		})
		return nil
	})

	return skill, files, nil
}

func (i *SkillImporter) ImportAll(skillsDir string) ([]domain.Skill, []domain.SkillFile, error) {
	if _, err := os.Stat(skillsDir); err != nil {
		return nil, nil, err
	}

	var skills []domain.Skill
	var allFiles []domain.SkillFile

	err := filepath.WalkDir(skillsDir, func(path string, d os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return nil
		}
		if d.IsDir() {
			if skipSkillWalkDir(skillsDir, path, d.Name()) {
				return filepath.SkipDir
			}
			return nil
		}
		if d.Name() != "SKILL.md" {
			return nil
		}
		skillDir := filepath.Dir(path)
		rel, err := filepath.Rel(skillsDir, skillDir)
		if err != nil || rel == "." {
			return nil
		}
		id := filepath.ToSlash(rel)
		if id == "" || strings.Contains(id, "..") {
			return nil
		}
		skill, files, err := i.Import(skillDir)
		if err != nil || skill == nil {
			return nil
		}
		skill.ID = id
		attachSkillRealPath(skill, skillDir)
		for j := range files {
			files[j].SkillID = id
			files[j].ID = id + ":" + files[j].Path
		}
		skills = append(skills, *skill)
		allFiles = append(allFiles, files...)
		return nil
	})
	if err != nil {
		return skills, allFiles, err
	}
	return skills, allFiles, nil
}

func (i *SkillImporter) ParseSkillMD(content string) (*domain.Skill, error) {
	fmText, body, err := splitYAMLFrontmatter(content)
	if err != nil {
		return nil, err
	}
	var fm skillFrontmatter
	if err := yaml.Unmarshal([]byte(fmText), &fm); err != nil {
		return nil, err
	}
	if fm.Name == "" {
		return nil, fmt.Errorf("invalid SKILL.md: name is required in frontmatter")
	}
	source := strings.TrimSpace(fm.Source)
	return &domain.Skill{
		ID:            fm.Name,
		Name:          fm.Name,
		Description:   fm.Description,
		License:       fm.License,
		Compatibility: fm.Compatibility,
		Metadata:      coerceMetadata(fm.Metadata),
		AllowedTools:  fm.AllowedTools,
		Source:        source,
		Builtin:       source == "builtin",
		Body:          body,
	}, nil
}

// splitYAMLFrontmatter finds the opening/closing --- lines so values that
// contain "---" (e.g. quoted descriptions) do not break parsing.
func splitYAMLFrontmatter(content string) (fmText, body string, err error) {
	content = strings.TrimPrefix(content, "\ufeff")
	lines := strings.Split(content, "\n")
	if len(lines) == 0 || strings.TrimSpace(lines[0]) != "---" {
		return "", "", fmt.Errorf("invalid SKILL.md: missing YAML frontmatter")
	}
	var fmLines []string
	for i := 1; i < len(lines); i++ {
		if strings.TrimSpace(lines[i]) == "---" {
			return strings.Join(fmLines, "\n"), strings.TrimSpace(strings.Join(lines[i+1:], "\n")), nil
		}
		fmLines = append(fmLines, lines[i])
	}
	return "", "", fmt.Errorf("invalid SKILL.md: unclosed YAML frontmatter")
}

// coerceMetadata flattens Agentskills / ClawHub frontmatter metadata into string map.
// Nested objects (e.g. metadata.openclaw) are JSON-encoded so install tips survive.
func coerceMetadata(raw any) map[string]string {
	if raw == nil {
		return nil
	}
	switch v := raw.(type) {
	case map[string]string:
		if len(v) == 0 {
			return nil
		}
		out := make(map[string]string, len(v))
		for k, val := range v {
			out[k] = val
		}
		return out
	case map[string]any:
		return stringifyMetadataMap(v)
	case map[any]any:
		m := make(map[string]any, len(v))
		for k, val := range v {
			m[fmt.Sprint(k)] = val
		}
		return stringifyMetadataMap(m)
	case string:
		s := strings.TrimSpace(v)
		if s == "" {
			return nil
		}
		var nested map[string]any
		if err := json.Unmarshal([]byte(s), &nested); err == nil {
			return stringifyMetadataMap(nested)
		}
		return map[string]string{"value": s}
	default:
		b, err := json.Marshal(v)
		if err != nil {
			return nil
		}
		return map[string]string{"value": string(b)}
	}
}

func stringifyMetadataMap(m map[string]any) map[string]string {
	if len(m) == 0 {
		return nil
	}
	out := make(map[string]string, len(m))
	for k, v := range m {
		switch t := v.(type) {
		case string:
			out[k] = t
		case nil:
			continue
		default:
			b, err := json.Marshal(t)
			if err != nil {
				out[k] = fmt.Sprint(t)
			} else {
				out[k] = string(b)
			}
		}
	}
	return out
}

func (i *SkillImporter) ToSkillMD(s domain.Skill) string {
	var b strings.Builder
	b.WriteString("---\n")
	b.WriteString("name: " + s.Name + "\n")
	b.WriteString("description: " + s.Description + "\n")
	if s.License != "" {
		b.WriteString("license: " + s.License + "\n")
	}
	if s.Compatibility != "" {
		b.WriteString("compatibility: " + s.Compatibility + "\n")
	}
	if len(s.Metadata) > 0 {
		b.WriteString("metadata:\n")
		for k, v := range s.Metadata {
			if k == SkillMetaRealPath {
				continue
			}
			b.WriteString("  " + k + ": " + v + "\n")
		}
	}
	if s.AllowedTools != "" {
		b.WriteString("allowed-tools: " + s.AllowedTools + "\n")
	}
	b.WriteString("---\n")
	if s.Body != "" {
		b.WriteString("\n")
		b.WriteString(s.Body)
		b.WriteString("\n")
	}
	return b.String()
}
