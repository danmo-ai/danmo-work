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
	"danmo-work/core/paths"

	"gopkg.in/yaml.v3"
)

var ValidSkillResourcePrefixes = []string{"scripts/", "references/", "assets/"}

var ErrBuiltinSkill = fmt.Errorf("cannot delete builtin skill")

func NormalizeSkillResourcePath(path string) (string, error) {
	p := strings.TrimSpace(path)
	p = strings.TrimPrefix(p, "/")
	p = strings.ReplaceAll(p, "\\", "/")
	if p == "" {
		return "", fmt.Errorf("path required")
	}
	if strings.Contains(p, "..") {
		return "", fmt.Errorf("invalid path: must not contain \"..\"")
	}
	ok := false
	for _, prefix := range ValidSkillResourcePrefixes {
		if strings.HasPrefix(p, prefix) {
			ok = true
			break
		}
	}
	if !ok {
		return "", fmt.Errorf("invalid resource path %q: must be under scripts/, references/, or assets/", p)
	}
	if strings.HasSuffix(p, "/") {
		return "", fmt.Errorf("invalid path: must be a file, not a directory")
	}
	return p, nil
}

type SkillManager struct {
	dataDir         string
	globalDir       string
	pluginSkillDirs []string
	mu              sync.RWMutex
}

func NewSkillManager(dataDir string) *SkillManager {
	return &SkillManager{
		dataDir:   dataDir,
		globalDir: filepath.Join(dataDir, "skills"),
	}
}

// SetPluginSkillDirs replaces the plugin skill directories list (batch set, called by PluginManager.Init).
func (m *SkillManager) SetPluginSkillDirs(dirs []string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.pluginSkillDirs = append([]string{}, dirs...) // defensive copy
}

// RegisterPluginSkillDir appends a single plugin skills/ directory.
func (m *SkillManager) RegisterPluginSkillDir(dir string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, existing := range m.pluginSkillDirs {
		if existing == dir {
			return
		}
	}
	m.pluginSkillDirs = append(m.pluginSkillDirs, dir)
}

// UnregisterPluginSkillDir removes a plugin skills/ directory.
func (m *SkillManager) UnregisterPluginSkillDir(dir string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	filtered := m.pluginSkillDirs[:0]
	for _, d := range m.pluginSkillDirs {
		if d != dir {
			filtered = append(filtered, d)
		}
	}
	m.pluginSkillDirs = filtered
}

func (m *SkillManager) DataDir() string { return m.dataDir }

// PluginSkillDirs returns a copy of registered plugin skill directories.
func (m *SkillManager) PluginSkillDirs() []string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return append([]string{}, m.pluginSkillDirs...)
}

// globalDirs returns all global skill roots (read), low → high priority.
func (m *SkillManager) globalDirs() []string {
	return []string{
		paths.AgentsSkillDir(),
		m.globalDir,
	}
}

func (m *SkillManager) List(ctx context.Context) ([]domain.Skill, error) {
	m.mu.RLock()
	dirs := append([]string{}, m.pluginSkillDirs...)
	m.mu.RUnlock()
	dirs = append(dirs, m.globalDir)
	skills, _ := ScanSkillDirs(dirs)
	return skills, nil
}

func (m *SkillManager) Get(ctx context.Context, id string) (*domain.Skill, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	for _, dir := range m.globalDirs() {
		skillDir := filepath.Join(dir, id)
		mdPath := filepath.Join(skillDir, "SKILL.md")
		data, err := os.ReadFile(mdPath)
		if err != nil {
			continue
		}
		return parseSkillMarkdown(string(data), skillDir)
	}
	for _, dir := range m.pluginSkillDirs {
		skillDir := filepath.Join(dir, id)
		mdPath := filepath.Join(skillDir, "SKILL.md")
		data, err := os.ReadFile(mdPath)
		if err != nil {
			continue
		}
		return parseSkillMarkdown(string(data), skillDir)
	}
	return nil, fmt.Errorf("skill %q not found", id)
}

func (m *SkillManager) Upsert(ctx context.Context, s domain.Skill) error {
	if s.ID == "" {
		s.ID = s.Name
	}
	if s.ID == "" {
		return fmt.Errorf("skill id or name is required")
	}
	dir := filepath.Join(m.globalDir, s.ID)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	if s.Source == "" {
		s.Source = "user"
	}
	if existing, err := m.Get(ctx, s.ID); err == nil && existing.Source == "builtin" {
		return fmt.Errorf("cannot modify builtin skill %q", s.ID)
	}
	return writeSkillFile(dir, s)
}

func (m *SkillManager) Delete(ctx context.Context, id string) error {
	s, err := m.Get(ctx, id)
	if err != nil {
		return err
	}
	if s.Source == "builtin" {
		return fmt.Errorf("cannot delete builtin skill %q", id)
	}
	return os.RemoveAll(filepath.Join(m.globalDir, id))
}

func (m *SkillManager) skillDir(ctx context.Context, skillID string) (string, error) {
	sk, err := m.Get(ctx, skillID)
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(sk.Dir) != "" {
		return sk.Dir, nil
	}
	return filepath.Join(m.globalDir, skillID), nil
}

func (m *SkillManager) Files(ctx context.Context, skillID string) ([]domain.SkillFile, error) {
	dir, err := m.skillDir(ctx, skillID)
	if err != nil {
		return nil, err
	}
	return readSkillFilesFromDir(dir, skillID)
}

func (m *SkillManager) File(ctx context.Context, skillID, path string) (domain.SkillFile, error) {
	p, err := NormalizeSkillResourcePath(path)
	if err != nil {
		return domain.SkillFile{}, err
	}
	dir, err := m.skillDir(ctx, skillID)
	if err != nil {
		return domain.SkillFile{}, err
	}
	fullPath := filepath.Join(dir, p)
	data, err := os.ReadFile(fullPath)
	if err != nil {
		return domain.SkillFile{}, err
	}
	info, _ := os.Stat(fullPath)
	var size int64
	if info != nil {
		size = info.Size()
	}
	return domain.SkillFile{
		ID:      skillID + ":" + p,
		SkillID: skillID,
		Path:    p,
		Content: data,
		Size:    size,
	}, nil
}

func (m *SkillManager) UpsertFile(ctx context.Context, f domain.SkillFile) error {
	p, err := NormalizeSkillResourcePath(f.Path)
	if err != nil {
		return err
	}
	f.Path = p
	if f.SkillID == "" {
		return fmt.Errorf("skillId required")
	}
	sk, err := m.Get(ctx, f.SkillID)
	if err == nil && (sk.Source == "builtin" || sk.Builtin) {
		return fmt.Errorf("cannot modify builtin skill %q", f.SkillID)
	}
	skillDir := filepath.Join(m.globalDir, f.SkillID)
	if err == nil && strings.TrimSpace(sk.Dir) != "" {
		skillDir = sk.Dir
	}
	if err := os.MkdirAll(skillDir, 0o755); err != nil {
		return err
	}
	fullPath := filepath.Join(skillDir, p)
	if err := os.MkdirAll(filepath.Dir(fullPath), 0o755); err != nil {
		return err
	}
	return os.WriteFile(fullPath, f.Content, 0o644)
}

func (m *SkillManager) DeleteFile(ctx context.Context, skillID, path string) error {
	p, err := NormalizeSkillResourcePath(path)
	if err != nil {
		return err
	}
	sk, err := m.Get(ctx, skillID)
	if err == nil && (sk.Source == "builtin" || sk.Builtin) {
		return fmt.Errorf("cannot modify builtin skill %q", skillID)
	}
	dir, err := m.skillDir(ctx, skillID)
	if err != nil {
		return err
	}
	return os.Remove(filepath.Join(dir, p))
}

func (m *SkillManager) DeleteFiles(ctx context.Context, skillID string) error {
	sk, err := m.Get(ctx, skillID)
	if err == nil && (sk.Source == "builtin" || sk.Builtin) {
		return fmt.Errorf("cannot modify builtin skill %q", skillID)
	}
	dir := filepath.Join(m.globalDir, skillID)
	if err == nil && strings.TrimSpace(sk.Dir) != "" {
		dir = sk.Dir
	}
	for _, prefix := range []string{"scripts", "references", "assets"} {
		_ = os.RemoveAll(filepath.Join(dir, prefix))
	}
	return nil
}

// HasTemplate always returns false — templates no longer exist as a separate concept.
func (m *SkillManager) HasTemplate(id string) bool {
	return false
}

// ResetFromTemplate is no longer supported.
func (m *SkillManager) ResetFromTemplate(ctx context.Context, id string) (*domain.Skill, error) {
	return nil, fmt.Errorf("ResetFromTemplate is no longer supported: builtin skills are read-only")
}

// SetTemplateLoader and SetFileTemplateLoader are no-ops retained for interface compatibility.
func (m *SkillManager) SetTemplateLoader(fn func(id string) (*domain.Skill, error))          {}
func (m *SkillManager) SetFileTemplateLoader(fn func(id string) ([]domain.SkillFile, error)) {}

// LoadSkillsFromFS reads all skill directories from the given path.
func LoadSkillsFromFS(dir string) ([]domain.Skill, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var result []domain.Skill
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		skillDir := filepath.Join(dir, entry.Name())
		mdPath := filepath.Join(skillDir, "SKILL.md")
		data, err := os.ReadFile(mdPath)
		if err != nil {
			continue
		}
		skill, err := parseSkillMarkdown(string(data), skillDir)
		if err != nil {
			log.Printf("[skills] parse %s: %v", mdPath, err)
			continue
		}
		result = append(result, *skill)
	}
	return result, nil
}

func parseSkillMarkdown(content, skillDir string) (*domain.Skill, error) {
	id := filepath.Base(skillDir)
	lines := strings.Split(strings.TrimPrefix(content, "\ufeff"), "\n")
	if len(lines) == 0 || strings.TrimSpace(lines[0]) != "---" {
		return nil, fmt.Errorf("skill %q: missing YAML frontmatter", id)
	}
	var fmTextLines []string
	for i := 1; i < len(lines); i++ {
		if strings.TrimSpace(lines[i]) == "---" {
			body := strings.TrimSpace(strings.Join(lines[i+1:], "\n"))
			fmText := strings.Join(fmTextLines, "\n")
			var fm struct {
				Name          string            `yaml:"name"`
				Description   string            `yaml:"description"`
				Source        string            `yaml:"source"`
				License       string            `yaml:"license"`
				Compatibility string            `yaml:"compatibility"`
				Metadata      map[string]string `yaml:"metadata"`
				AllowedTools  string            `yaml:"allowed-tools"`
			}
			if err := yaml.Unmarshal([]byte(fmText), &fm); err != nil {
				return nil, fmt.Errorf("skill %q: parse frontmatter: %w", id, err)
			}
			name := fm.Name
			if name == "" {
				name = id
			}
			source := fm.Source
			if source == "" {
				source = "builtin"
			}
			return &domain.Skill{
				ID: id, Name: name, Description: fm.Description, Source: source, Builtin: source == "builtin",
				License: fm.License, Compatibility: fm.Compatibility,
				Metadata: fm.Metadata, AllowedTools: fm.AllowedTools,
				Body: body, Dir: skillDir,
			}, nil
		}
		fmTextLines = append(fmTextLines, lines[i])
	}
	return nil, fmt.Errorf("skill %q: unclosed YAML frontmatter", id)
}

func writeSkillFile(dir string, s domain.Skill) error {
	mdPath := filepath.Join(dir, "SKILL.md")
	var b strings.Builder
	b.WriteString("---\n")
	if s.Name != "" {
		b.WriteString(fmt.Sprintf("name: %s\n", s.Name))
	} else {
		b.WriteString(fmt.Sprintf("name: %s\n", s.ID))
	}
	if s.Description != "" {
		b.WriteString(fmt.Sprintf("description: %s\n", s.Description))
	}
	if s.Source != "" {
		b.WriteString(fmt.Sprintf("source: %s\n", s.Source))
	}
	if s.License != "" {
		b.WriteString(fmt.Sprintf("license: %s\n", s.License))
	}
	if s.Compatibility != "" {
		b.WriteString(fmt.Sprintf("compatibility: %s\n", s.Compatibility))
	}
	if s.AllowedTools != "" {
		b.WriteString(fmt.Sprintf("allowed-tools: %s\n", s.AllowedTools))
	}
	if s.MarketSource != "" {
		b.WriteString(fmt.Sprintf("marketSource: %s\n", s.MarketSource))
	}
	b.WriteString("---\n")
	if s.Body != "" {
		b.WriteString("\n")
		b.WriteString(s.Body)
	}
	return os.WriteFile(mdPath, []byte(b.String()), 0o644)
}

func readSkillFiles(baseDir, skillID string) ([]domain.SkillFile, error) {
	return readSkillFilesFromDir(filepath.Join(baseDir, skillID), skillID)
}

func readSkillFilesFromDir(skillDir, skillID string) ([]domain.SkillFile, error) {
	var files []domain.SkillFile
	for _, sub := range []string{"scripts", "references", "assets"} {
		subDir := filepath.Join(skillDir, sub)
		_ = filepath.WalkDir(subDir, func(fullPath string, d os.DirEntry, err error) error {
			if err != nil || d.IsDir() {
				return nil
			}
			relPath, err := filepath.Rel(skillDir, fullPath)
			if err != nil {
				return nil
			}
			relPath = filepath.ToSlash(relPath)
			data, err := os.ReadFile(fullPath)
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
