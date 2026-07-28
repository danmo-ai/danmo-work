package clawhub

import (
	"archive/zip"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"danmo-work/core/domain"
	"danmo-work/core/port"
)

var _ port.Market = (*Market)(nil)

const (
	defaultBaseURL      = "https://clawhub.ai"
	defaultCatalogLimit = 200
	pageSize            = 50
)

// Market fetches skill catalogs and packages from the ClawHub registry.
type Market struct {
	source domain.MarketSource
	client *http.Client
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
	return "clawhub"
}

func (m *Market) baseURL() string {
	repo := strings.TrimRight(strings.TrimSpace(m.source.Repo), "/")
	if repo == "" {
		return defaultBaseURL
	}
	return repo
}

func (m *Market) catalogLimit() int {
	// Optional: catalog_path as integer string overrides the fetch cap.
	if n := strings.TrimSpace(m.source.CatalogPath); n != "" {
		var lim int
		if _, err := fmt.Sscanf(n, "%d", &lim); err == nil && lim > 0 {
			if lim > 500 {
				return 500
			}
			return lim
		}
	}
	return defaultCatalogLimit
}

func (m *Market) FetchCatalog(ctx context.Context) (domain.MarketCatalog, error) {
	var empty domain.MarketCatalog
	limit := m.catalogLimit()
	var items []domain.MarketItem
	cursor := ""
	for len(items) < limit {
		batch, next, err := m.fetchSkillsPage(ctx, pageSize, cursor)
		if err != nil {
			return empty, err
		}
		for _, it := range batch {
			items = append(items, it)
			if len(items) >= limit {
				break
			}
		}
		if next == "" || len(batch) == 0 {
			break
		}
		cursor = next
	}
	return domain.MarketCatalog{
		SchemaVersion: 1,
		Items:         items,
		SourceID:      m.source.ID,
	}, nil
}

func (m *Market) fetchSkillsPage(ctx context.Context, limit int, cursor string) ([]domain.MarketItem, string, error) {
	q := url.Values{}
	q.Set("limit", fmt.Sprintf("%d", limit))
	q.Set("sort", "recommended")
	q.Set("nonSuspiciousOnly", "true")
	if cursor != "" {
		q.Set("cursor", cursor)
	}
	endpoint := m.baseURL() + "/api/v1/skills?" + q.Encode()
	body, err := m.httpGet(ctx, endpoint, "application/json")
	if err != nil {
		return nil, "", err
	}
	var resp skillsListResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, "", fmt.Errorf("parse clawhub skills list: %w", err)
	}
	out := make([]domain.MarketItem, 0, len(resp.Items))
	for _, raw := range resp.Items {
		if it, ok := mapSkillListing(raw); ok {
			out = append(out, it)
		}
	}
	return out, resp.NextCursor, nil
}

type skillsListResponse struct {
	Items      []rawSkill `json:"items"`
	NextCursor string     `json:"nextCursor"`
}

type rawSkill struct {
	Slug          string          `json:"slug"`
	DisplayName   string          `json:"displayName"`
	Summary       string          `json:"summary"`
	Description   string          `json:"description"`
	Topics        []string        `json:"topics"`
	OwnerHandle   string          `json:"ownerHandle"`
	Owner         *rawOwner       `json:"owner"`
	UpdatedAt     int64           `json:"updatedAt"`
	LatestVersion *rawVersion     `json:"latestVersion"`
	Metadata      json.RawMessage `json:"metadata"`
	Tags          map[string]any  `json:"tags"`
}

type rawOwner struct {
	Handle      string `json:"handle"`
	DisplayName string `json:"displayName"`
}

type rawVersion struct {
	Version string `json:"version"`
}

func mapSkillListing(raw rawSkill) (domain.MarketItem, bool) {
	slug := strings.TrimSpace(raw.Slug)
	if slug == "" || strings.Contains(slug, "..") {
		return domain.MarketItem{}, false
	}
	owner := strings.TrimSpace(raw.OwnerHandle)
	if owner == "" && raw.Owner != nil {
		owner = strings.TrimSpace(raw.Owner.Handle)
	}
	id := catalogID(owner, slug)
	name := strings.TrimSpace(raw.DisplayName)
	if name == "" {
		name = slug
	}
	desc := strings.TrimSpace(raw.Summary)
	if desc == "" {
		desc = strings.TrimSpace(raw.Description)
	}
	version := ""
	if raw.LatestVersion != nil {
		version = strings.TrimSpace(raw.LatestVersion.Version)
	}
	if version == "" && raw.Tags != nil {
		if latest, ok := raw.Tags["latest"].(string); ok {
			version = latest
		}
	}
	updatedAt := ""
	if raw.UpdatedAt > 0 {
		updatedAt = time.UnixMilli(raw.UpdatedAt).UTC().Format(time.RFC3339)
	}
	return domain.MarketItem{
		Kind:          domain.MarketKindSkill,
		ID:            id,
		Name:          name,
		Description:   desc,
		Keywords:      append([]string(nil), raw.Topics...),
		Version:       version,
		Author:        owner,
		Path:          slug,
		UpdatedAt:     updatedAt,
		Compatibility: compatibilityFromMetadata(raw.Metadata),
	}, true
}

func catalogID(owner, slug string) string {
	owner = sanitizeIDPart(owner)
	slug = sanitizeIDPart(slug)
	if owner != "" {
		return owner + "__" + slug
	}
	// List API often omits owner; prefix avoids colliding with official/builtin ids.
	return "clawhub__" + slug
}

func sanitizeIDPart(s string) string {
	s = strings.TrimSpace(strings.ToLower(s))
	var b strings.Builder
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9', r == '-', r == '_', r == '.':
			b.WriteRune(r)
		}
	}
	return b.String()
}

func compatibilityFromMetadata(raw json.RawMessage) string {
	if len(raw) == 0 || string(raw) == "null" {
		return ""
	}
	var meta map[string]any
	if err := json.Unmarshal(raw, &meta); err != nil {
		return ""
	}
	var parts []string
	if osVal, ok := meta["os"]; ok {
		switch v := osVal.(type) {
		case []any:
			var oses []string
			for _, x := range v {
				if s, ok := x.(string); ok && s != "" {
					oses = append(oses, s)
				}
			}
			if len(oses) > 0 {
				parts = append(parts, "os:"+strings.Join(oses, ","))
			}
		case []string:
			if len(v) > 0 {
				parts = append(parts, "os:"+strings.Join(v, ","))
			}
		}
	}
	// Soft tip when ClawHub declares install specs under openclaw/clawdbot namespaces.
	for _, key := range []string{"openclaw", "clawdbot", "clawdis"} {
		if nested, ok := meta[key].(map[string]any); ok {
			if install, ok := nested["install"]; ok {
				if arr, ok := install.([]any); ok && len(arr) > 0 {
					kinds := installKinds(arr)
					if kinds != "" {
						parts = append(parts, "needs:"+kinds)
					} else {
						parts = append(parts, "needs:deps")
					}
				}
			}
			if cfg, ok := nested["config"]; ok && cfg != nil {
				parts = append(parts, "openclaw-config")
			}
		}
	}
	return strings.Join(parts, "; ")
}

func installKinds(arr []any) string {
	seen := map[string]bool{}
	var kinds []string
	for _, x := range arr {
		obj, ok := x.(map[string]any)
		if !ok {
			continue
		}
		k, _ := obj["kind"].(string)
		k = strings.TrimSpace(k)
		if k == "" || seen[k] {
			continue
		}
		seen[k] = true
		kinds = append(kinds, k)
	}
	return strings.Join(kinds, ",")
}

func (m *Market) FetchPackage(ctx context.Context, item domain.MarketItem, ref string) (string, func(), error) {
	slug := strings.Trim(strings.ReplaceAll(item.Path, "\\", "/"), "/")
	if slug == "" {
		// Fallback: id may be owner__slug or clawhub__slug.
		slug = slugFromCatalogID(item.ID)
	}
	if slug == "" || strings.Contains(slug, "..") || strings.Contains(slug, "/") {
		return "", nil, fmt.Errorf("invalid clawhub slug %q", item.Path)
	}

	version := strings.TrimSpace(ref)
	if version == "" {
		version = strings.TrimSpace(item.Version)
	}

	q := url.Values{}
	q.Set("slug", slug)
	if version != "" {
		q.Set("version", version)
	} else {
		q.Set("tag", "latest")
	}
	endpoint := m.baseURL() + "/api/v1/download?" + q.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return "", nil, err
	}
	m.applyAuth(req)
	resp, err := m.client.Do(req)
	if err != nil {
		return "", nil, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 55<<20))
	if err != nil {
		return "", nil, err
	}
	if resp.StatusCode == http.StatusGone {
		return "", nil, fmt.Errorf("clawhub skill version gone (410): %s", slug)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		msg := strings.TrimSpace(string(body))
		if msg == "" {
			msg = resp.Status
		}
		return "", nil, fmt.Errorf("clawhub download %s: %s", slug, msg)
	}

	ct := strings.ToLower(resp.Header.Get("Content-Type"))
	if strings.Contains(ct, "json") || (len(body) > 0 && body[0] == '{') {
		return m.fetchFromGitHubHandoff(ctx, body)
	}

	tmpRoot, err := os.MkdirTemp("", "dq-clawhub-*")
	if err != nil {
		return "", nil, err
	}
	cleanup := func() { _ = os.RemoveAll(tmpRoot) }
	dest := filepath.Join(tmpRoot, "skill")
	if err := os.MkdirAll(dest, 0755); err != nil {
		cleanup()
		return "", nil, err
	}
	if err := unzipTo(body, dest); err != nil {
		cleanup()
		return "", nil, err
	}
	if _, err := os.Stat(filepath.Join(dest, "SKILL.md")); err != nil {
		// Some zips nest a single top-level folder.
		if nested, nerr := findSkillRoot(dest); nerr == nil {
			return nested, cleanup, nil
		}
		cleanup()
		return "", nil, fmt.Errorf("clawhub package missing SKILL.md")
	}
	return dest, cleanup, nil
}

type githubHandoff struct {
	SourceRef  string `json:"sourceRef"`
	Repo       string `json:"repo"`
	Commit     string `json:"commit"`
	Path       string `json:"path"`
	ArchiveURL string `json:"archiveUrl"`
}

func (m *Market) fetchFromGitHubHandoff(ctx context.Context, body []byte) (string, func(), error) {
	var handoff githubHandoff
	if err := json.Unmarshal(body, &handoff); err != nil {
		return "", nil, fmt.Errorf("parse clawhub github handoff: %w", err)
	}
	if handoff.ArchiveURL == "" {
		return "", nil, fmt.Errorf("clawhub github handoff missing archiveUrl")
	}
	data, err := m.httpGet(ctx, handoff.ArchiveURL, "")
	if err != nil {
		return "", nil, fmt.Errorf("clawhub github archive: %w", err)
	}
	tmpRoot, err := os.MkdirTemp("", "dq-clawhub-gh-*")
	if err != nil {
		return "", nil, err
	}
	cleanup := func() { _ = os.RemoveAll(tmpRoot) }
	extractRoot := filepath.Join(tmpRoot, "extract")
	if err := os.MkdirAll(extractRoot, 0755); err != nil {
		cleanup()
		return "", nil, err
	}
	if err := unzipTo(data, extractRoot); err != nil {
		cleanup()
		return "", nil, err
	}
	// GitHub archives nest under repo-commit/; then optional skill path.
	entries, err := os.ReadDir(extractRoot)
	if err != nil {
		cleanup()
		return "", nil, err
	}
	base := extractRoot
	if len(entries) == 1 && entries[0].IsDir() {
		base = filepath.Join(extractRoot, entries[0].Name())
	}
	pkgPath := strings.Trim(strings.ReplaceAll(handoff.Path, "\\", "/"), "/")
	dest := base
	if pkgPath != "" {
		dest = filepath.Join(base, filepath.FromSlash(pkgPath))
	}
	if st, err := os.Stat(dest); err != nil || !st.IsDir() {
		cleanup()
		return "", nil, fmt.Errorf("clawhub github path %q not found in archive", handoff.Path)
	}
	if _, err := os.Stat(filepath.Join(dest, "SKILL.md")); err != nil {
		cleanup()
		return "", nil, fmt.Errorf("clawhub github package missing SKILL.md")
	}
	return dest, cleanup, nil
}

func slugFromCatalogID(id string) string {
	id = strings.TrimSpace(id)
	if id == "" {
		return ""
	}
	if strings.HasPrefix(id, "clawhub__") {
		return strings.TrimPrefix(id, "clawhub__")
	}
	if i := strings.LastIndex(id, "__"); i >= 0 && i+2 < len(id) {
		return id[i+2:]
	}
	return id
}

func findSkillRoot(dir string) (string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return "", err
	}
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		p := filepath.Join(dir, e.Name())
		if _, err := os.Stat(filepath.Join(p, "SKILL.md")); err == nil {
			return p, nil
		}
	}
	return "", fmt.Errorf("SKILL.md not found")
}

func unzipTo(data []byte, dest string) error {
	tmpZip, err := os.CreateTemp("", "dq-clawhub-*.zip")
	if err != nil {
		return err
	}
	zipPath := tmpZip.Name()
	defer os.Remove(zipPath)
	if _, err := tmpZip.Write(data); err != nil {
		tmpZip.Close()
		return err
	}
	if err := tmpZip.Close(); err != nil {
		return err
	}
	zr, err := zip.OpenReader(zipPath)
	if err != nil {
		return fmt.Errorf("open zip: %w", err)
	}
	defer zr.Close()
	for _, f := range zr.File {
		name := filepath.Clean(f.Name)
		if name == "." || strings.HasPrefix(name, "..") {
			continue
		}
		target := filepath.Join(dest, name)
		if !strings.HasPrefix(target, filepath.Clean(dest)+string(os.PathSeparator)) && filepath.Clean(target) != filepath.Clean(dest) {
			continue
		}
		if f.FileInfo().IsDir() {
			_ = os.MkdirAll(target, 0755)
			continue
		}
		if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
			return err
		}
		rc, err := f.Open()
		if err != nil {
			return err
		}
		out, err := os.OpenFile(target, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
		if err != nil {
			rc.Close()
			return err
		}
		_, copyErr := io.Copy(out, io.LimitReader(rc, 20<<20))
		rc.Close()
		out.Close()
		if copyErr != nil {
			return copyErr
		}
	}
	return nil
}

func (m *Market) applyAuth(req *http.Request) {
	if tok := strings.TrimSpace(m.source.Token); tok != "" {
		req.Header.Set("Authorization", "Bearer "+tok)
	}
	req.Header.Set("Accept", "*/*")
	req.Header.Set("User-Agent", "danmo-work-clawhub-market/1.0")
}

func (m *Market) httpGet(ctx context.Context, endpoint, accept string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, err
	}
	m.applyAuth(req)
	if accept != "" {
		req.Header.Set("Accept", accept)
	}
	resp, err := m.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 55<<20))
	if err != nil {
		return nil, err
	}
	if resp.StatusCode == http.StatusTooManyRequests {
		return nil, fmt.Errorf("clawhub rate limited (429)")
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		msg := strings.TrimSpace(string(body))
		if msg == "" {
			msg = resp.Status
		}
		return nil, fmt.Errorf("clawhub GET %s: %s", endpoint, msg)
	}
	return body, nil
}
