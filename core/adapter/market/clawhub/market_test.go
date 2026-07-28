package clawhub

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"danmo-work/core/domain"
)

func TestMapSkillListingIDAndCompatibility(t *testing.T) {
	it, ok := mapSkillListing(rawSkill{
		Slug:          "gifgrep",
		DisplayName:   "Gifgrep",
		Summary:       "Search GIFs",
		OwnerHandle:   "steipete",
		LatestVersion: &rawVersion{Version: "1.0.1"},
		Metadata:      json.RawMessage(`{"os":["macos"],"openclaw":{"install":[{"kind":"brew"}]}}`),
	})
	if !ok {
		t.Fatal("expected ok")
	}
	if it.ID != "steipete__gifgrep" {
		t.Fatalf("id = %q", it.ID)
	}
	if it.Path != "gifgrep" {
		t.Fatalf("path = %q", it.Path)
	}
	if !strings.Contains(it.Compatibility, "os:macos") {
		t.Fatalf("compatibility = %q", it.Compatibility)
	}
	if !strings.Contains(it.Compatibility, "needs:brew") {
		t.Fatalf("compatibility = %q", it.Compatibility)
	}

	it2, ok := mapSkillListing(rawSkill{Slug: "solo"})
	if !ok {
		t.Fatal("expected ok")
	}
	if it2.ID != "clawhub__solo" {
		t.Fatalf("id without owner = %q", it2.ID)
	}
}

func TestFetchCatalogUsesNonSuspiciousAndMapsItems(t *testing.T) {
	var sawNonSuspicious bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/skills" {
			http.NotFound(w, r)
			return
		}
		if r.URL.Query().Get("nonSuspiciousOnly") == "true" {
			sawNonSuspicious = true
		}
		_ = json.NewEncoder(w).Encode(skillsListResponse{
			Items: []rawSkill{{
				Slug:          "demo-skill",
				DisplayName:   "Demo",
				Summary:       "A demo skill",
				OwnerHandle:   "acme",
				LatestVersion: &rawVersion{Version: "2.0.0"},
			}},
		})
	}))
	defer srv.Close()

	m := New(domain.MarketSource{
		ID:          "clawhub",
		Kind:        "clawhub",
		Repo:        srv.URL,
		Enabled:     true,
		CatalogPath: "10",
	})
	cat, err := m.FetchCatalog(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if !sawNonSuspicious {
		t.Fatal("expected nonSuspiciousOnly=true")
	}
	if len(cat.Items) != 1 {
		t.Fatalf("items = %d", len(cat.Items))
	}
	if cat.Items[0].ID != "acme__demo-skill" {
		t.Fatalf("id = %q", cat.Items[0].ID)
	}
	if cat.Items[0].Kind != domain.MarketKindSkill {
		t.Fatalf("kind = %q", cat.Items[0].Kind)
	}
}

func TestFetchPackageZipAndMissingSkillMD(t *testing.T) {
	goodZip := mustZip(t, map[string]string{
		"SKILL.md": "---\nname: demo\ndescription: d\n---\n\nBody\n",
	})
	badZip := mustZip(t, map[string]string{
		"README.md": "no skill",
	})

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/api/v1/download" && r.URL.Query().Get("slug") == "demo":
			w.Header().Set("Content-Type", "application/zip")
			_, _ = w.Write(goodZip)
		case r.URL.Path == "/api/v1/download" && r.URL.Query().Get("slug") == "noskill":
			w.Header().Set("Content-Type", "application/zip")
			_, _ = w.Write(badZip)
		case r.URL.Path == "/api/v1/download" && r.URL.Query().Get("slug") == "gone":
			http.Error(w, "gone", http.StatusGone)
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	m := New(domain.MarketSource{ID: "clawhub", Kind: "clawhub", Repo: srv.URL, Enabled: true})
	ctx := context.Background()

	dir, cleanup, err := m.FetchPackage(ctx, domain.MarketItem{
		ID: "clawhub__demo", Path: "demo", Version: "1.0.0",
	}, "")
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()
	if _, err := os.Stat(filepath.Join(dir, "SKILL.md")); err != nil {
		t.Fatal(err)
	}

	if _, _, err := m.FetchPackage(ctx, domain.MarketItem{ID: "clawhub__noskill", Path: "noskill"}, ""); err == nil {
		t.Fatal("expected missing SKILL.md error")
	}
	if _, _, err := m.FetchPackage(ctx, domain.MarketItem{ID: "clawhub__gone", Path: "gone"}, ""); err == nil {
		t.Fatal("expected 410 error")
	}
}

func TestFetchPackageGitHubHandoff(t *testing.T) {
	archive := mustZip(t, map[string]string{
		"repo-abc/skills/demo/SKILL.md": "---\nname: demo\ndescription: d\n---\n\nHi\n",
	})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v1/download":
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(githubHandoff{
				SourceRef:  "public-github",
				ArchiveURL: "http://" + r.Host + "/archive.zip",
				Path:       "skills/demo",
			})
		case "/archive.zip":
			w.Header().Set("Content-Type", "application/zip")
			_, _ = w.Write(archive)
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	m := New(domain.MarketSource{ID: "clawhub", Kind: "clawhub", Repo: srv.URL, Enabled: true})
	dir, cleanup, err := m.FetchPackage(context.Background(), domain.MarketItem{
		ID: "clawhub__demo", Path: "demo",
	}, "")
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()
	if _, err := os.Stat(filepath.Join(dir, "SKILL.md")); err != nil {
		t.Fatal(err)
	}
}

func TestSlugFromCatalogID(t *testing.T) {
	if got := slugFromCatalogID("clawhub__gifgrep"); got != "gifgrep" {
		t.Fatalf("got %q", got)
	}
	if got := slugFromCatalogID("steipete__gifgrep"); got != "gifgrep" {
		t.Fatalf("got %q", got)
	}
}

func mustZip(t *testing.T, files map[string]string) []byte {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for name, content := range files {
		w, err := zw.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := io.WriteString(w, content); err != nil {
			t.Fatal(err)
		}
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}
