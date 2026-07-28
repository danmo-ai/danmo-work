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
}

func NewSkillImporter() *SkillImporter {
	return &SkillImporter{}
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

	var files []domain.SkillFile
	for _, sub := range []string{"scripts", "references", "assets"} {
		subDir := filepath.Join(dirPath, sub)
		_ = filepath.WalkDir(subDir, func(fullPath string, d os.DirEntry, walkErr error) error {
			if walkErr != nil || d.IsDir() {
				return nil
			}
			rel, err := filepath.Rel(dirPath, fullPath)
			if err != nil {
				return nil
			}
			relPath := filepath.ToSlash(rel)
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
	}

	return skill, files, nil
}

func (i *SkillImporter) ImportAll(skillsDir string) ([]domain.Skill, []domain.SkillFile, error) {
	entries, err := os.ReadDir(skillsDir)
	if err != nil {
		return nil, nil, err
	}

	var skills []domain.Skill
	var allFiles []domain.SkillFile

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		skillPath := filepath.Join(skillsDir, entry.Name())
		if _, err := os.Stat(filepath.Join(skillPath, "SKILL.md")); err != nil {
			continue
		}
		skill, files, err := i.Import(skillPath)
		if err != nil {
			continue
		}
		skills = append(skills, *skill)
		allFiles = append(allFiles, files...)
	}

	return skills, allFiles, nil
}

func (i *SkillImporter) ParseSkillMD(content string) (*domain.Skill, error) {
	var fm skillFrontmatter
	parts := strings.SplitN(content, "---", 3)
	if len(parts) < 3 {
		return nil, fmt.Errorf("invalid SKILL.md: missing YAML frontmatter")
	}
	if err := yaml.Unmarshal([]byte(strings.TrimSpace(parts[1])), &fm); err != nil {
		return nil, err
	}
	if fm.Name == "" {
		return nil, fmt.Errorf("invalid SKILL.md: name is required in frontmatter")
	}
	return &domain.Skill{
		ID:            fm.Name,
		Name:          fm.Name,
		Description:   fm.Description,
		License:       fm.License,
		Compatibility: fm.Compatibility,
		Metadata:      coerceMetadata(fm.Metadata),
		AllowedTools:  fm.AllowedTools,
		Body:          strings.TrimSpace(parts[2]),
	}, nil
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
