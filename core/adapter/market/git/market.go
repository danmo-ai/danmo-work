package git

import (
	"archive/zip"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"danmo-work/core/domain"
	"danmo-work/core/port"
)

var _ port.Market = (*Market)(nil)

// Market fetches catalogs and packages from a Git-hosted market repo.
type Market struct {
	source domain.MarketSource
	client *http.Client
}

func New(source domain.MarketSource) *Market {
	timeout := 60 * time.Second
	return &Market{
		source: source,
		client: &http.Client{Timeout: timeout},
	}
}

func (m *Market) SourceID() string { return m.source.ID }
func (m *Market) Kind() string {
	if m.source.Kind != "" {
		return m.source.Kind
	}
	return "git"
}

func (m *Market) catalogPath() string {
	if m.source.CatalogPath != "" {
		return strings.TrimPrefix(m.source.CatalogPath, "/")
	}
	return "catalog/index.json"
}

func (m *Market) refOr(ref string) string {
	if ref != "" {
		return ref
	}
	if m.source.Ref != "" {
		return m.source.Ref
	}
	return "main"
}

func (m *Market) FetchCatalog(ctx context.Context) (domain.MarketCatalog, error) {
	var empty domain.MarketCatalog
	data, err := m.readRepoFile(ctx, m.catalogPath(), m.refOr(""))
	if err != nil {
		return empty, err
	}
	var cat domain.MarketCatalog
	if err := json.Unmarshal(data, &cat); err != nil {
		return empty, fmt.Errorf("parse catalog: %w", err)
	}
	cat.SourceID = m.source.ID
	return cat, nil
}

func (m *Market) FetchPackage(ctx context.Context, item domain.MarketItem, ref string) (string, func(), error) {
	ref = m.refOr(ref)
	pkgPath := strings.Trim(strings.ReplaceAll(item.Path, "\\", "/"), "/")
	if pkgPath == "" || strings.Contains(pkgPath, "..") {
		return "", nil, fmt.Errorf("invalid package path %q", item.Path)
	}

	if m.isLocal() {
		src := filepath.Join(m.localRoot(), filepath.FromSlash(pkgPath))
		info, err := os.Stat(src)
		if err != nil {
			return "", nil, fmt.Errorf("local package: %w", err)
		}
		if !info.IsDir() {
			return "", nil, fmt.Errorf("local package is not a directory: %s", src)
		}
		return src, func() {}, nil
	}

	tmpRoot, err := os.MkdirTemp("", "dq-market-*")
	if err != nil {
		return "", nil, err
	}
	cleanup := func() { _ = os.RemoveAll(tmpRoot) }

	if err := m.downloadZipExtractPath(ctx, ref, pkgPath, tmpRoot); err != nil {
		cleanup()
		return "", nil, err
	}
	dest := filepath.Join(tmpRoot, filepath.FromSlash(pkgPath))
	if st, err := os.Stat(dest); err != nil || !st.IsDir() {
		cleanup()
		return "", nil, fmt.Errorf("package path %q not found in archive", pkgPath)
	}
	return dest, cleanup, nil
}

func (m *Market) isLocal() bool {
	p := strings.ToLower(m.source.Platform)
	if p == "local" || p == "file" {
		return true
	}
	repo := m.source.Repo
	return strings.HasPrefix(repo, "/") || strings.HasPrefix(repo, "file://")
}

func (m *Market) localRoot() string {
	repo := m.source.Repo
	repo = strings.TrimPrefix(repo, "file://")
	return filepath.Clean(repo)
}

func (m *Market) readRepoFile(ctx context.Context, relPath, ref string) ([]byte, error) {
	relPath = strings.TrimPrefix(relPath, "/")
	if m.isLocal() {
		p := filepath.Join(m.localRoot(), filepath.FromSlash(relPath))
		return os.ReadFile(p)
	}
	url, err := m.rawURL(relPath, ref)
	if err != nil {
		return nil, err
	}
	return m.httpGet(ctx, url)
}

func (m *Market) rawURL(relPath, ref string) (string, error) {
	owner, name, err := splitOwnerRepo(m.source.Repo)
	if err != nil {
		return "", err
	}
	relPath = strings.TrimPrefix(relPath, "/")
	switch strings.ToLower(m.source.Platform) {
	case "gitee":
		return fmt.Sprintf("https://gitee.com/%s/%s/raw/%s/%s", owner, name, ref, relPath), nil
	case "github", "":
		return fmt.Sprintf("https://raw.githubusercontent.com/%s/%s/%s/%s", owner, name, ref, relPath), nil
	case "generic":
		base := strings.TrimSuffix(m.source.Repo, ".git")
		base = strings.TrimSuffix(base, "/")
		// Best-effort: many hosts support /-/raw/<ref>/<path> (GitLab-style).
		return fmt.Sprintf("%s/-/raw/%s/%s", base, ref, relPath), nil
	default:
		return fmt.Sprintf("https://raw.githubusercontent.com/%s/%s/%s/%s", owner, name, ref, relPath), nil
	}
}

func (m *Market) archiveURL(ref string) (string, error) {
	owner, name, err := splitOwnerRepo(m.source.Repo)
	if err != nil {
		return "", err
	}
	switch strings.ToLower(m.source.Platform) {
	case "gitee":
		return fmt.Sprintf("https://gitee.com/%s/%s/repository/archive/%s.zip", owner, name, ref), nil
	case "github", "":
		return fmt.Sprintf("https://codeload.github.com/%s/%s/zip/refs/heads/%s", owner, name, ref), nil
	case "generic":
		base := strings.TrimSuffix(m.source.Repo, ".git")
		base = strings.TrimSuffix(base, "/")
		return fmt.Sprintf("%s/-/archive/%s/%s-%s.zip", base, ref, name, ref), nil
	default:
		return fmt.Sprintf("https://codeload.github.com/%s/%s/zip/refs/heads/%s", owner, name, ref), nil
	}
}

func (m *Market) downloadZipExtractPath(ctx context.Context, ref, pkgPath, destRoot string) error {
	url, err := m.archiveURL(ref)
	if err != nil {
		return err
	}
	// Tags fallback for GitHub if branch zip fails.
	data, err := m.httpGet(ctx, url)
	if err != nil && strings.ToLower(m.source.Platform) != "gitee" {
		owner, name, splitErr := splitOwnerRepo(m.source.Repo)
		if splitErr == nil {
			alt := fmt.Sprintf("https://codeload.github.com/%s/%s/zip/refs/tags/%s", owner, name, ref)
			data, err = m.httpGet(ctx, alt)
		}
	}
	if err != nil {
		return fmt.Errorf("download archive: %w", err)
	}

	zipPath := filepath.Join(destRoot, "repo.zip")
	if err := os.WriteFile(zipPath, data, 0600); err != nil {
		return err
	}
	zr, err := zip.OpenReader(zipPath)
	if err != nil {
		return err
	}
	defer zr.Close()

	prefix := pkgPath + "/"
	extracted := 0
	for _, f := range zr.File {
		name := f.Name
		// Strip top-level folder (owner-repo-sha/...)
		parts := strings.SplitN(name, "/", 2)
		if len(parts) < 2 {
			continue
		}
		rel := parts[1]
		if rel != pkgPath && !strings.HasPrefix(rel, prefix) {
			continue
		}
		target := filepath.Join(destRoot, filepath.FromSlash(rel))
		if !strings.HasPrefix(filepath.Clean(target), filepath.Clean(destRoot)+string(os.PathSeparator)) {
			continue
		}
		if f.FileInfo().IsDir() {
			if err := os.MkdirAll(target, 0755); err != nil {
				return err
			}
			continue
		}
		if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
			return err
		}
		if err := copyZipFile(f, target); err != nil {
			return err
		}
		extracted++
	}
	if extracted == 0 {
		return fmt.Errorf("no files extracted for path %q", pkgPath)
	}
	return nil
}

func copyZipFile(f *zip.File, target string) error {
	rc, err := f.Open()
	if err != nil {
		return err
	}
	defer rc.Close()
	out, err := os.OpenFile(target, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
	if err != nil {
		return err
	}
	defer out.Close()
	_, err = io.Copy(out, rc)
	return err
}

func (m *Market) httpGet(ctx context.Context, url string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	if m.source.Token != "" {
		req.Header.Set("Authorization", "Bearer "+m.source.Token)
		// GitHub also accepts token prefix.
		if strings.Contains(url, "github.com") || strings.Contains(url, "githubusercontent.com") || strings.Contains(url, "codeload.github.com") {
			req.Header.Set("Authorization", "token "+m.source.Token)
		}
	}
	req.Header.Set("User-Agent", "danmo-work-market")
	resp, err := m.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 64<<20))
	if err != nil {
		return nil, err
	}
	if resp.StatusCode >= 300 {
		return nil, fmt.Errorf("GET %s: HTTP %d", url, resp.StatusCode)
	}
	return body, nil
}

func splitOwnerRepo(repo string) (owner, name string, err error) {
	repo = strings.TrimSpace(repo)
	repo = strings.TrimSuffix(repo, ".git")
	if strings.HasPrefix(repo, "https://") || strings.HasPrefix(repo, "http://") {
		// https://github.com/owner/name
		u := strings.TrimPrefix(repo, "https://")
		u = strings.TrimPrefix(u, "http://")
		parts := strings.Split(u, "/")
		if len(parts) < 3 {
			return "", "", fmt.Errorf("invalid repo URL %q", repo)
		}
		return parts[1], parts[2], nil
	}
	parts := strings.Split(repo, "/")
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return "", "", fmt.Errorf("invalid repo %q (want owner/name)", repo)
	}
	return parts[0], parts[1], nil
}
