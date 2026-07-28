package techleads

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"danmo-work/core/domain"
)

func TestFetchCatalogSkipsDeprecated(t *testing.T) {
	reg := registryDoc{
		Version: "1.0.0",
		Skills: []registrySkill{
			{Name: "good-skill", Description: "ok", Category: "quality", Path: "(quality)/good-skill", Files: []string{"SKILL.md"}, Version: "1.0"},
			{Name: "old-skill", Description: "gone", Category: "quality", Path: "(quality)/old-skill", Files: []string{"SKILL.md"}},
		},
		Deprecated: []registryDeprec{{Name: "old-skill", Message: "use good-skill"}},
	}
	m := New(domain.MarketSource{ID: "techleads", Kind: "techleads", Ref: "0.16.0", Enabled: true})
	m.regSnap = &reg
	m.pkgVer = "0.16.0"
	cat, err := m.FetchCatalog(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(cat.Items) != 1 {
		t.Fatalf("items = %d, want 1 (deprecated skipped)", len(cat.Items))
	}
	if cat.Items[0].ID != "tlc__good-skill" {
		t.Fatalf("id = %q", cat.Items[0].ID)
	}
	if cat.Items[0].Category != "quality" {
		t.Fatalf("category = %q", cat.Items[0].Category)
	}
}

func TestFetchPackageDownloadsListedFiles(t *testing.T) {
	reg := registryDoc{
		Version: "1.0.0",
		Skills: []registrySkill{{
			Name: "demo",
			Path: "(quality)/demo",
			Files: []string{
				"SKILL.md",
				"references/a.md",
				"templates/b.md",
			},
		}},
	}
	files := map[string]string{
		"SKILL.md":       "---\nname: demo\ndescription: d\n---\n\nBody\n",
		"references/a.md": "# A\n",
		"templates/b.md":  "# B\n",
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		p := r.URL.Path
		switch {
		case strings.HasSuffix(p, "/latest"):
			_ = json.NewEncoder(w).Encode(map[string]string{"version": "9.9.9"})
		case strings.Contains(p, "skills-registry.json"):
			_ = json.NewEncoder(w).Encode(reg)
		case strings.Contains(p, "/skills/(quality)/demo/"):
			rel := p[strings.Index(p, "/demo/")+len("/demo/"):]
			content, ok := files[rel]
			if !ok {
				http.NotFound(w, r)
				return
			}
			_, _ = w.Write([]byte(content))
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	m := &Market{
		source: domain.MarketSource{ID: "techleads", Kind: "techleads", Ref: "9.9.9", Enabled: true},
		client: srv.Client(),
	}
	// Monkey-patch CDN by replacing httpGet to rewrite hosts — easier to set base via custom packageName
	// and override jsdelivr by testing through loadRegistry with regSnap + custom download.
	// Directly call download path with rewritten endpoints by setting regSnap and using a test helper.
	m.regSnap = &reg
	m.pkgVer = "9.9.9"

	// Use FetchPackage but need CDN to hit srv. Temporarily replace via wrapping client Transport.
	m.client = &http.Client{Transport: rewriteCDN(srv.URL)}
	dir, cleanup, err := m.FetchPackage(context.Background(), domain.MarketItem{
		ID: "tlc__demo", Name: "demo", Path: "(quality)/demo",
	}, "")
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()
	for _, f := range []string{"SKILL.md", "references/a.md", "templates/b.md"} {
		if _, err := os.Stat(filepath.Join(dir, f)); err != nil {
			t.Fatalf("missing %s: %v", f, err)
		}
	}
}

func TestFetchPackageRejectsDeprecated(t *testing.T) {
	reg := registryDoc{
		Skills: []registrySkill{{
			Name: "old", Path: "(x)/old", Files: []string{"SKILL.md"},
		}},
		Deprecated: []registryDeprec{{Name: "old", Message: "gone"}},
	}
	m := New(domain.MarketSource{ID: "techleads", Kind: "techleads", Enabled: true})
	m.regSnap = &reg
	m.pkgVer = "1.0.0"
	_, _, err := m.FetchPackage(context.Background(), domain.MarketItem{ID: "tlc__old", Name: "old", Path: "(x)/old"}, "")
	if err == nil || !strings.Contains(err.Error(), "deprecated") {
		t.Fatalf("err = %v", err)
	}
}

type rewriteRoundTripper struct {
	base string
}

func rewriteCDN(base string) http.RoundTripper {
	return rewriteRoundTripper{base: strings.TrimRight(base, "/")}
}

func (r rewriteRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	u := *req.URL
	// Map jsdelivr/unpkg/npm paths onto the test server.
	path := u.Path
	if strings.Contains(u.Host, "registry.npmjs.org") {
		path = "/@tech-leads-club/skills-catalog/latest"
	}
	nu, err := http.NewRequest(req.Method, r.base+path+"?"+u.RawQuery, req.Body)
	if err != nil {
		return nil, err
	}
	nu.Header = req.Header
	return http.DefaultTransport.RoundTrip(nu)
}

func TestSanitizeAndSkillName(t *testing.T) {
	if got := sanitizeID("PR Review"); got != "pr-review" {
		t.Fatalf("sanitize = %q", got)
	}
	if got := skillNameFromItem(domain.MarketItem{ID: "tlc__accessibility"}); got != "accessibility" {
		t.Fatalf("name = %q", got)
	}
}
