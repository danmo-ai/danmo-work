package techleads

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"danmo-work/core/domain"
	"danmo-work/core/port"
)

var _ port.Market = (*Market)(nil)

const (
	defaultPackage = "@tech-leads-club/skills-catalog"
	jsdelivrBase   = "https://cdn.jsdelivr.net/npm/"
	unpkgBase      = "https://unpkg.com/"
	npmLatestURL   = "https://registry.npmjs.org/%s/latest"
	maxFileBytes   = 5 << 20
	maxFiles       = 400
)

// Market fetches the curated Tech Leads Club skills catalog from npm CDN.
type Market struct {
	source domain.MarketSource
	client *http.Client

	mu      sync.Mutex
	pkgVer  string
	regSnap *registryDoc
}

func New(source domain.MarketSource) *Market {
	return &Market{
		source: source,
		client: &http.Client{Timeout: 60 * time.Second},
	}
}

func (m *Market) SourceID() string { return m.source.ID }
func (m *Market) Kind() string {
	if m.source.Kind != "" {
		return m.source.Kind
	}
	return "techleads"
}

func (m *Market) packageName() string {
	repo := strings.TrimSpace(m.source.Repo)
	switch {
	case repo == "":
		return defaultPackage
	case strings.HasPrefix(repo, "@"):
		return repo
	case strings.Contains(repo, "@tech-leads-club/skills-catalog"):
		return defaultPackage
	default:
		return defaultPackage
	}
}

func (m *Market) versionPin() string {
	ref := strings.TrimSpace(m.source.Ref)
	if ref == "" || ref == "main" || ref == "master" {
		return "latest"
	}
	return ref
}

func (m *Market) FetchCatalog(ctx context.Context) (domain.MarketCatalog, error) {
	var empty domain.MarketCatalog
	reg, err := m.loadRegistry(ctx)
	if err != nil {
		return empty, err
	}
	deprecated := map[string]bool{}
	for _, d := range reg.Deprecated {
		if d.Name != "" {
			deprecated[d.Name] = true
		}
	}
	items := make([]domain.MarketItem, 0, len(reg.Skills))
	for _, s := range reg.Skills {
		name := strings.TrimSpace(s.Name)
		if name == "" || deprecated[name] {
			continue
		}
		if strings.Contains(name, "..") || strings.Contains(s.Path, "..") {
			continue
		}
		path := strings.Trim(strings.ReplaceAll(s.Path, "\\", "/"), "/")
		if path == "" {
			continue
		}
		ver := strings.TrimSpace(s.Version)
		if ver == "" {
			ver = m.pkgVer
		}
		author := strings.TrimSpace(s.Author)
		id := "tlc__" + sanitizeID(name)
		items = append(items, domain.MarketItem{
			Kind:        domain.MarketKindSkill,
			ID:          id,
			Name:        name,
			Description: strings.TrimSpace(s.Description),
			Category:    strings.TrimSpace(s.Category),
			Version:     ver,
			Author:      author,
			Path:        path,
			Keywords:    categoryKeywords(s.Category),
		})
	}
	return domain.MarketCatalog{
		SchemaVersion: 1,
		Items:         items,
		SourceID:      m.source.ID,
	}, nil
}

func (m *Market) FetchPackage(ctx context.Context, item domain.MarketItem, ref string) (string, func(), error) {
	reg, err := m.loadRegistry(ctx)
	if err != nil {
		return "", nil, err
	}
	skillName := skillNameFromItem(item)
	var entry *registrySkill
	for i := range reg.Skills {
		if reg.Skills[i].Name == skillName {
			entry = &reg.Skills[i]
			break
		}
	}
	if entry == nil {
		// Fallback: match by path.
		path := strings.Trim(strings.ReplaceAll(item.Path, "\\", "/"), "/")
		for i := range reg.Skills {
			if strings.Trim(reg.Skills[i].Path, "/") == path {
				entry = &reg.Skills[i]
				break
			}
		}
	}
	if entry == nil {
		return "", nil, fmt.Errorf("techleads skill %q not found in registry", skillName)
	}
	for _, d := range reg.Deprecated {
		if d.Name == entry.Name {
			return "", nil, fmt.Errorf("techleads skill %q is deprecated: %s", entry.Name, d.Message)
		}
	}
	if len(entry.Files) == 0 {
		return "", nil, fmt.Errorf("techleads skill %q has no files", entry.Name)
	}
	if len(entry.Files) > maxFiles {
		return "", nil, fmt.Errorf("techleads skill %q has too many files (%d)", entry.Name, len(entry.Files))
	}

	ver := m.versionPin()
	if ver == "latest" && m.pkgVer != "" {
		ver = m.pkgVer
	}
	_ = ref // npm package version comes from source.Ref / resolved latest

	tmpRoot, err := os.MkdirTemp("", "dq-techleads-*")
	if err != nil {
		return "", nil, err
	}
	cleanup := func() { _ = os.RemoveAll(tmpRoot) }
	dest := filepath.Join(tmpRoot, "skill")
	if err := os.MkdirAll(dest, 0755); err != nil {
		cleanup()
		return "", nil, err
	}

	pkg := m.packageName()
	basePrimary := jsdelivrBase + pkg + "@" + ver + "/skills/" + strings.Trim(entry.Path, "/")
	baseFallback := unpkgBase + pkg + "@" + ver + "/skills/" + strings.Trim(entry.Path, "/")

	for _, rel := range entry.Files {
		rel = strings.Trim(strings.ReplaceAll(rel, "\\", "/"), "/")
		if rel == "" || strings.Contains(rel, "..") {
			cleanup()
			return "", nil, fmt.Errorf("invalid skill file path %q", rel)
		}
		data, err := m.httpGetFallback(ctx, basePrimary+"/"+rel, baseFallback+"/"+rel)
		if err != nil {
			cleanup()
			return "", nil, fmt.Errorf("download %s: %w", rel, err)
		}
		out := filepath.Join(dest, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(out), 0755); err != nil {
			cleanup()
			return "", nil, err
		}
		if err := os.WriteFile(out, data, 0644); err != nil {
			cleanup()
			return "", nil, err
		}
	}
	if _, err := os.Stat(filepath.Join(dest, "SKILL.md")); err != nil {
		cleanup()
		return "", nil, fmt.Errorf("techleads package missing SKILL.md")
	}
	return dest, cleanup, nil
}

type registryDoc struct {
	Version    string          `json:"version"`
	Skills     []registrySkill `json:"skills"`
	Deprecated []registryDeprec `json:"deprecated"`
	Categories json.RawMessage `json:"categories"`
}

type registrySkill struct {
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Category    string   `json:"category"`
	Path        string   `json:"path"`
	Files       []string `json:"files"`
	Author      string   `json:"author"`
	Version     string   `json:"version"`
	ContentHash string   `json:"contentHash"`
}

type registryDeprec struct {
	Name         string   `json:"name"`
	Message      string   `json:"message"`
	Alternatives []string `json:"alternatives"`
}

func (m *Market) loadRegistry(ctx context.Context) (*registryDoc, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.regSnap != nil {
		return m.regSnap, nil
	}
	ver := m.versionPin()
	if ver == "latest" {
		resolved, err := m.resolveLatest(ctx)
		if err == nil && resolved != "" {
			ver = resolved
		}
	}
	pkg := m.packageName()
	primary := jsdelivrBase + pkg + "@" + ver + "/skills-registry.json"
	fallback := unpkgBase + pkg + "@" + ver + "/skills-registry.json"
	body, err := m.httpGetFallback(ctx, primary, fallback)
	if err != nil {
		return nil, err
	}
	var reg registryDoc
	if err := json.Unmarshal(body, &reg); err != nil {
		return nil, fmt.Errorf("parse techleads registry: %w", err)
	}
	m.pkgVer = ver
	m.regSnap = &reg
	return &reg, nil
}

func (m *Market) resolveLatest(ctx context.Context) (string, error) {
	pkg := m.packageName()
	url := fmt.Sprintf(npmLatestURL, pkg)
	body, err := m.httpGet(ctx, url)
	if err != nil {
		return "", err
	}
	var meta struct {
		Version string `json:"version"`
	}
	if err := json.Unmarshal(body, &meta); err != nil {
		return "", err
	}
	return strings.TrimSpace(meta.Version), nil
}

func (m *Market) httpGetFallback(ctx context.Context, primary, fallback string) ([]byte, error) {
	data, err := m.httpGet(ctx, primary)
	if err == nil {
		return data, nil
	}
	if fallback == "" || fallback == primary {
		return nil, err
	}
	return m.httpGet(ctx, fallback)
}

func (m *Market) httpGet(ctx context.Context, endpoint string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "*/*")
	req.Header.Set("User-Agent", "danmo-work-techleads-market/1.0")
	resp, err := m.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxFileBytes+1024))
	if err != nil {
		return nil, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		msg := strings.TrimSpace(string(body))
		if len(msg) > 200 {
			msg = msg[:200]
		}
		if msg == "" {
			msg = resp.Status
		}
		return nil, fmt.Errorf("GET %s: %s", endpoint, msg)
	}
	if len(body) > maxFileBytes {
		return nil, fmt.Errorf("response too large")
	}
	return body, nil
}

func sanitizeID(s string) string {
	s = strings.TrimSpace(strings.ToLower(s))
	var b strings.Builder
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9', r == '-', r == '_', r == '.':
			b.WriteRune(r)
		case r == ' ':
			b.WriteByte('-')
		}
	}
	out := b.String()
	if out == "" {
		return "skill"
	}
	return out
}

func skillNameFromItem(item domain.MarketItem) string {
	if strings.HasPrefix(item.ID, "tlc__") {
		return strings.TrimPrefix(item.ID, "tlc__")
	}
	if item.Name != "" {
		return item.Name
	}
	return item.ID
}

func categoryKeywords(cat string) []string {
	cat = strings.TrimSpace(cat)
	if cat == "" {
		return nil
	}
	return []string{cat}
}
