package service

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"danmo-work/core/domain"

	"gopkg.in/yaml.v3"
)

type SkillImporter struct{}

type skillFrontmatter struct {
	Name          string            `yaml:"name"`
	Description   string            `yaml:"description"`
	License       string            `yaml:"license"`
	Compatibility string            `yaml:"compatibility"`
	Metadata      map[string]string `yaml:"metadata"`
	AllowedTools  string            `yaml:"allowed-tools"`
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
		Metadata:      fm.Metadata,
		AllowedTools:  fm.AllowedTools,
		Body:          strings.TrimSpace(parts[2]),
	}, nil
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
