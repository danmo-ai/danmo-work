package service

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"sync"

	"danmo-work/core/domain"
	"danmo-work/core/port"
)

// ErrBuiltinSkill is returned when a caller tries to delete a template-backed
// or otherwise builtin skill from the library.
var ErrBuiltinSkill = errors.New("cannot delete builtin skill")

// ValidSkillResourcePrefixes are the allowed top-level dirs for skill resource files.
var ValidSkillResourcePrefixes = []string{"scripts/", "references/", "assets/"}

// NormalizeSkillResourcePath cleans and validates a skill resource relative path.
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
	store             port.SkillRepo
	filesRepo         port.SkillFileRepo
	mu                sync.RWMutex
	cache             map[string]*domain.Skill
	cachedList        bool
	listCache         []domain.Skill
	templateLoader    func(id string) (*domain.Skill, error)
	fileTemplateLoader func(id string) ([]domain.SkillFile, error)
}

func NewSkillManager(store port.SkillRepo, filesRepo port.SkillFileRepo) *SkillManager {
	return &SkillManager{
		store:     store,
		filesRepo: filesRepo,
		cache:     make(map[string]*domain.Skill),
	}
}

func (m *SkillManager) List(ctx context.Context) ([]domain.Skill, error) {
	m.mu.RLock()
	if m.cachedList {
		result := m.listCache
		m.mu.RUnlock()
		out := make([]domain.Skill, len(result))
		copy(out, result)
		m.enrichBatch(ctx, out)
		return out, nil
	}
	m.mu.RUnlock()
	list, err := m.store.List(ctx)
	if err != nil {
		return nil, err
	}
	m.mu.Lock()
	m.listCache = list
	m.cachedList = true
	m.mu.Unlock()
	out := make([]domain.Skill, len(list))
	copy(out, list)
	m.enrichBatch(ctx, out)
	return out, nil
}

func (m *SkillManager) Get(ctx context.Context, id string) (*domain.Skill, error) {
	m.mu.RLock()
	if s, ok := m.cache[id]; ok {
		m.mu.RUnlock()
		cp := *s
		m.enrich(ctx, &cp)
		return &cp, nil
	}
	m.mu.RUnlock()
	sk, err := m.store.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	m.mu.Lock()
	m.cache[id] = &sk
	m.mu.Unlock()
	cp := sk
	m.enrich(ctx, &cp)
	return &cp, nil
}

func (m *SkillManager) Upsert(ctx context.Context, s domain.Skill) error {
	// Never clear builtin via a partial client update, and treat any skill that
	// still has an embedded template as builtin (same rule as AgentManager).
	if existing, err := m.store.Get(ctx, s.ID); err == nil && existing.Builtin {
		s.Builtin = true
	}
	if m.hasTemplate(s.ID) {
		s.Builtin = true
	}
	s.TemplateDiverged = false
	if err := m.store.Upsert(ctx, s); err != nil {
		return err
	}
	m.mu.Lock()
	m.cache[s.ID] = &s
	m.cachedList = false
	m.mu.Unlock()
	return nil
}

func (m *SkillManager) Delete(ctx context.Context, id string) error {
	if m.hasTemplate(id) {
		return fmt.Errorf("%w %q", ErrBuiltinSkill, id)
	}
	if existing, err := m.store.Get(ctx, id); err == nil && existing.Builtin {
		return fmt.Errorf("%w %q", ErrBuiltinSkill, id)
	}
	if err := m.store.Delete(ctx, id); err != nil {
		return err
	}
	m.mu.Lock()
	delete(m.cache, id)
	m.cachedList = false
	m.mu.Unlock()
	return nil
}

// HasTemplate reports whether an embedded builtin template exists for id.
func (m *SkillManager) HasTemplate(id string) bool {
	return m.hasTemplate(id)
}

func (m *SkillManager) Files(ctx context.Context, skillID string) ([]domain.SkillFile, error) {
	return m.filesRepo.ListBySkill(ctx, skillID)
}

func (m *SkillManager) File(ctx context.Context, skillID, path string) (domain.SkillFile, error) {
	return m.filesRepo.Get(ctx, skillID, path)
}

func (m *SkillManager) UpsertFile(ctx context.Context, f domain.SkillFile) error {
	path, err := NormalizeSkillResourcePath(f.Path)
	if err != nil {
		return err
	}
	f.Path = path
	if f.SkillID == "" {
		return fmt.Errorf("skillId required")
	}
	if f.ID == "" {
		f.ID = f.SkillID + ":" + f.Path
	}
	if f.Size == 0 {
		f.Size = int64(len(f.Content))
	}
	return m.filesRepo.Upsert(ctx, f)
}

// EnsureFile inserts a skill resource file only when the path does not already
// exist. Existing content (including user edits) is left untouched.
func (m *SkillManager) EnsureFile(ctx context.Context, f domain.SkillFile) error {
	path, err := NormalizeSkillResourcePath(f.Path)
	if err != nil {
		return err
	}
	f.Path = path
	if _, err := m.filesRepo.Get(ctx, f.SkillID, f.Path); err == nil {
		return nil
	}
	return m.UpsertFile(ctx, f)
}

func (m *SkillManager) DeleteFile(ctx context.Context, skillID, path string) error {
	path, err := NormalizeSkillResourcePath(path)
	if err != nil {
		return err
	}
	return m.filesRepo.Delete(ctx, skillID, path)
}

func (m *SkillManager) DeleteFiles(ctx context.Context, skillID string) error {
	return m.filesRepo.DeleteBySkill(ctx, skillID)
}

func (m *SkillManager) SetTemplateLoader(fn func(id string) (*domain.Skill, error)) {
	m.templateLoader = fn
}

func (m *SkillManager) SetFileTemplateLoader(fn func(id string) ([]domain.SkillFile, error)) {
	m.fileTemplateLoader = fn
}

func (m *SkillManager) ResetFromTemplate(ctx context.Context, id string) (*domain.Skill, error) {
	if m.templateLoader == nil {
		return nil, fmt.Errorf("template loader not configured")
	}
	_, err := m.store.Get(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("skill %q not found: %w", id, err)
	}
	tmpl, err := m.templateLoader(id)
	if err != nil {
		return nil, fmt.Errorf("no template found for skill %q: %w", id, err)
	}
	tmpl.Builtin = true
	tmpl.TemplateDiverged = false
	tmpl.Body = NormalizeSkillBodyRefs(tmpl.Body, tmpl.ID)
	if err := m.store.Upsert(ctx, *tmpl); err != nil {
		return nil, err
	}
	// Reset resource files from template if available
	if m.fileTemplateLoader != nil {
		files, err := m.fileTemplateLoader(id)
		if err == nil {
			_ = m.filesRepo.DeleteBySkill(ctx, id)
			for _, f := range files {
				_ = m.filesRepo.Upsert(ctx, f)
			}
		}
	}
	m.mu.Lock()
	m.cache[id] = tmpl
	m.cachedList = false
	m.mu.Unlock()
	return tmpl, nil
}

func (m *SkillManager) enrichBatch(ctx context.Context, skills []domain.Skill) {
	for i := range skills {
		m.enrich(ctx, &skills[i])
	}
}

func (m *SkillManager) hasTemplate(id string) bool {
	if m.templateLoader == nil || id == "" {
		return false
	}
	_, err := m.templateLoader(id)
	return err == nil
}

// enrich marks template-backed skills as builtin (read-time, like AgentManager)
// and computes template drift for the library UI.
func (m *SkillManager) enrich(ctx context.Context, sk *domain.Skill) {
	if sk == nil {
		return
	}
	if sk.MarketSource == "" && sk.Metadata != nil {
		sk.MarketSource = sk.Metadata["market.source"]
	}
	if m.hasTemplate(sk.ID) {
		sk.Builtin = true
	}
	if !sk.Builtin {
		return
	}
	diverged, err := m.DivergedFromTemplate(ctx, *sk)
	if err != nil {
		return
	}
	sk.TemplateDiverged = diverged
}

// DivergedFromTemplate reports whether a builtin skill differs from its
// embedded template (metadata/body and/or resource files). Non-builtin skills
// and skills without a template loader are never considered diverged.
func (m *SkillManager) DivergedFromTemplate(ctx context.Context, sk domain.Skill) (bool, error) {
	if !sk.Builtin || m.templateLoader == nil {
		return false, nil
	}
	tmpl, err := m.templateLoader(sk.ID)
	if err != nil {
		return false, nil
	}
	if !skillContentEqual(sk, *tmpl) {
		return true, nil
	}
	if m.fileTemplateLoader == nil {
		return false, nil
	}
	tmplFiles, err := m.fileTemplateLoader(sk.ID)
	if err != nil {
		return false, nil
	}
	storedFiles, err := m.filesRepo.ListBySkill(ctx, sk.ID)
	if err != nil {
		return false, err
	}
	return !skillFilesEqual(storedFiles, tmplFiles), nil
}

func skillContentEqual(a, b domain.Skill) bool {
	if a.Name != b.Name ||
		a.Description != b.Description ||
		a.License != b.License ||
		a.Compatibility != b.Compatibility ||
		a.AllowedTools != b.AllowedTools ||
		a.SystemHint != b.SystemHint ||
		a.Body != b.Body {
		return false
	}
	if !mapsEqual(a.Metadata, b.Metadata) {
		return false
	}
	if !stringSlicesEqual(a.Keywords, b.Keywords) {
		return false
	}
	if !stringSlicesEqual(a.ToolIDs, b.ToolIDs) {
		return false
	}
	return true
}

func skillFilesEqual(stored, tmpl []domain.SkillFile) bool {
	if len(stored) != len(tmpl) {
		return false
	}
	byPath := make(map[string][]byte, len(stored))
	for _, f := range stored {
		byPath[f.Path] = f.Content
	}
	for _, f := range tmpl {
		content, ok := byPath[f.Path]
		if !ok || !bytes.Equal(content, f.Content) {
			return false
		}
	}
	return true
}

func mapsEqual(a, b map[string]string) bool {
	if len(a) != len(b) {
		return false
	}
	if len(a) == 0 {
		return true
	}
	for k, v := range a {
		if b[k] != v {
			return false
		}
	}
	return true
}

func stringSlicesEqual(a, b []string) bool {
	if len(a) == 0 && len(b) == 0 {
		return true
	}
	return slices.Equal(a, b)
}
