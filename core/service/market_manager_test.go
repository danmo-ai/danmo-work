package service

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"danmo-work/core/domain"
	"danmo-work/core/port"
)

type memConfigStore struct {
	cfg *domain.ConfigFile
}

func (s *memConfigStore) Load(ctx context.Context) (*domain.ConfigFile, error) {
	cp := *s.cfg
	return &cp, nil
}

func (s *memConfigStore) Save(ctx context.Context, cfg *domain.ConfigFile) error {
	s.cfg = cfg
	return nil
}

// fakeMarket serves packages from a local directory tree shaped like dq-market.
type fakeMarket struct {
	id   string
	root string
}

func (m *fakeMarket) SourceID() string { return m.id }
func (m *fakeMarket) Kind() string     { return "git" }

func (m *fakeMarket) FetchCatalog(ctx context.Context) (domain.MarketCatalog, error) {
	data, err := os.ReadFile(filepath.Join(m.root, "catalog", "index.json"))
	if err != nil {
		return domain.MarketCatalog{}, err
	}
	var cat domain.MarketCatalog
	if err := json.Unmarshal(data, &cat); err != nil {
		return domain.MarketCatalog{}, err
	}
	cat.SourceID = m.id
	return cat, nil
}

func (m *fakeMarket) FetchPackage(ctx context.Context, item domain.MarketItem, ref string) (string, func(), error) {
	dir := filepath.Join(m.root, filepath.FromSlash(item.Path))
	return dir, func() {}, nil
}

type fakeRegistry struct {
	m port.Market
}

func (r *fakeRegistry) List() []port.Market {
	return []port.Market{r.m}
}

func (r *fakeRegistry) Get(sourceID string) (port.Market, bool) {
	if r.m != nil && r.m.SourceID() == sourceID {
		return r.m, true
	}
	return nil, false
}

func (r *fakeRegistry) Reload(sources []domain.MarketSource) error { return nil }

func TestMarketManagerInstallLocal(t *testing.T) {
	root := filepath.Clean(filepath.Join("..", "..", "..", "dq-market"))
	if _, err := os.Stat(filepath.Join(root, "catalog", "index.json")); err != nil {
		t.Skip("dq-market sibling repo not found")
	}
	abs, err := filepath.Abs(root)
	if err != nil {
		t.Fatal(err)
	}

	cfgStore := &memConfigStore{cfg: &domain.ConfigFile{
		Market: domain.ConfigMarketSection{
			CacheTTLHours: 1,
			Sources: []domain.MarketSource{{
				ID:       "local",
				Name:     "Local",
				Kind:     "git",
				Platform: "local",
				Repo:     abs,
				Enabled:  true,
				Priority: 1,
			}},
		},
	}}
	configMgr := NewConfigManager(cfgStore)
	reg := &fakeRegistry{m: &fakeMarket{id: "local", root: abs}}
	skills := NewSkillManager(newMemSkillRepo(), newMemSkillFileRepo())
	agents := NewAgentManager(newMemAgentRepo())
	mcp := NewMCPManager(newMemMCPServerRepo())
	mgr := NewMarketManager(configMgr, reg, skills, agents, mcp)

	ctx := context.Background()
	list, warnings, err := mgr.ListCatalog(ctx, true)
	if err != nil {
		t.Fatal(err)
	}
	if len(warnings) != 0 {
		t.Fatalf("unexpected warnings: %v", warnings)
	}
	if len(list) < 2 {
		t.Fatalf("expected >=2 catalog items, got %d", len(list))
	}

	res, err := mgr.Install(ctx, domain.InstallMarketRequest{
		SourceID: "local",
		Kind:     "skill",
		ID:       "meeting-notes",
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Installed) != 1 || res.Installed[0] != "meeting-notes" {
		t.Fatalf("unexpected install result: %+v", res)
	}
	sk, err := skills.Get(ctx, "meeting-notes")
	if err != nil || sk == nil {
		t.Fatal("skill not installed")
	}
	files, _ := skills.Files(ctx, "meeting-notes")
	if len(files) == 0 {
		t.Fatal("expected skill resource files")
	}

	res2, err := mgr.Install(ctx, domain.InstallMarketRequest{
		SourceID: "local",
		Kind:     "expert",
		ID:       "meeting-facilitator",
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(res2.Installed) < 1 {
		t.Fatalf("expected expert installed, got %+v", res2)
	}
	if _, err := agents.Get(ctx, "meeting-facilitator"); err != nil {
		t.Fatal("expert not installed")
	}

	res3, err := mgr.Install(ctx, domain.InstallMarketRequest{
		SourceID: "local",
		Kind:     "connector",
		ID:       "github-mcp",
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(res3.Installed) != 1 || res3.Installed[0] != "github-mcp" {
		t.Fatalf("unexpected connector install: %+v", res3)
	}
	srv, err := mcp.Get(ctx, "github-mcp")
	if err != nil {
		t.Fatal("connector not installed")
	}
	if srv.MarketSource != "local" || srv.CatalogID != "github-mcp" {
		t.Fatalf("connector provenance: %+v", srv)
	}
	list2, _, err := mgr.ListCatalog(ctx, true)
	if err != nil {
		t.Fatal(err)
	}
	foundInstalled := false
	for _, item := range list2 {
		if item.Kind == domain.MarketKindConnector && item.ID == "github-mcp" && item.Installed {
			foundInstalled = true
			break
		}
	}
	if !foundInstalled {
		t.Fatal("expected github-mcp marked installed in catalog")
	}
}

func TestMarketInstallNormalizesBodyWithCatalogID(t *testing.T) {
	root := t.TempDir()
	pkg := filepath.Join(root, "skills", "pr-review")
	if err := os.MkdirAll(filepath.Join(pkg, "references"), 0o755); err != nil {
		t.Fatal(err)
	}
	md := "---\nname: pr-review\ndescription: Review PRs\n---\n\nSee `references/guide.md`.\n"
	if err := os.WriteFile(filepath.Join(pkg, "SKILL.md"), []byte(md), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(pkg, "references", "guide.md"), []byte("g"), 0o644); err != nil {
		t.Fatal(err)
	}
	cat := `{
  "version": 1,
  "skills": [{"id":"tlc__pr-review","name":"PR Review","path":"skills/pr-review","kind":"skill"}]
}`
	if err := os.MkdirAll(filepath.Join(root, "catalog"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "catalog", "index.json"), []byte(cat), 0o644); err != nil {
		t.Fatal(err)
	}

	cfgStore := &memConfigStore{cfg: &domain.ConfigFile{
		Market: domain.ConfigMarketSection{
			Sources: []domain.MarketSource{{ID: "local", Kind: "git", Enabled: true}},
		},
	}}
	skills := NewSkillManager(newMemSkillRepo(), newMemSkillFileRepo())
	agents := NewAgentManager(newMemAgentRepo())
	mgr := NewMarketManager(NewConfigManager(cfgStore), &fakeRegistry{m: &fakeMarket{id: "local", root: root}}, skills, agents, nil)

	res, err := mgr.Install(context.Background(), domain.InstallMarketRequest{
		SourceID: "local", Kind: "skill", ID: "tlc__pr-review", Overwrite: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Installed) != 1 || res.Installed[0] != "tlc__pr-review" {
		t.Fatalf("install = %+v", res)
	}
	sk, err := skills.Get(context.Background(), "tlc__pr-review")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(sk.Body, "`tlc__pr-review/references/guide.md`") {
		t.Fatalf("body should use catalog id prefix: %q", sk.Body)
	}
	if strings.Contains(sk.Body, "`pr-review/references/") || strings.Contains(sk.Body, "`references/guide.md`") {
		t.Fatalf("body still has pre-override refs: %q", sk.Body)
	}
}

func TestMarketUninstallRestoresBuiltinSkill(t *testing.T) {
	ctx := context.Background()
	cfgStore := &memConfigStore{cfg: &domain.ConfigFile{}}
	configMgr := NewConfigManager(cfgStore)
	skills := NewSkillManager(newMemSkillRepo(), newMemSkillFileRepo())
	skills.SetTemplateLoader(func(id string) (*domain.Skill, error) {
		if id != "debugging" {
			return nil, os.ErrNotExist
		}
		return &domain.Skill{ID: "debugging", Name: "debugging", Body: "builtin-body", Builtin: true}, nil
	})
	skills.SetFileTemplateLoader(func(id string) ([]domain.SkillFile, error) {
		return nil, nil
	})
	agents := NewAgentManager(newMemAgentRepo())
	mgr := NewMarketManager(configMgr, &fakeRegistry{}, skills, agents, nil)

	_ = skills.Upsert(ctx, domain.Skill{
		ID: "debugging", Name: "debugging", Body: "market-body",
		MarketSource: "local",
		Metadata:     map[string]string{"market.source": "local"},
	})

	if err := mgr.Uninstall(ctx, domain.UninstallMarketRequest{
		Kind: "skill",
		ID:   "debugging",
	}); err != nil {
		t.Fatal(err)
	}
	got, err := skills.Get(ctx, "debugging")
	if err != nil {
		t.Fatal("builtin skill should remain after market uninstall")
	}
	if got.Body != "builtin-body" {
		t.Fatalf("body = %q, want builtin-body", got.Body)
	}
	if got.MarketSource != "" {
		t.Fatalf("marketSource = %q, want empty after restore", got.MarketSource)
	}
	if !got.Builtin {
		t.Fatal("restored skill should be builtin")
	}
}

type memAgentRepo struct {
	byID map[string]domain.Agent
}

func newMemAgentRepo() *memAgentRepo {
	return &memAgentRepo{byID: make(map[string]domain.Agent)}
}

func (r *memAgentRepo) List(ctx context.Context) ([]domain.Agent, error) {
	var out []domain.Agent
	for _, a := range r.byID {
		out = append(out, a)
	}
	return out, nil
}

func (r *memAgentRepo) Get(ctx context.Context, id string) (domain.Agent, error) {
	a, ok := r.byID[id]
	if !ok {
		return domain.Agent{}, os.ErrNotExist
	}
	return a, nil
}

func (r *memAgentRepo) Upsert(ctx context.Context, a domain.Agent) error {
	r.byID[a.ID] = a
	return nil
}

func (r *memAgentRepo) Delete(ctx context.Context, id string) error {
	delete(r.byID, id)
	return nil
}

type memMCPServerRepo struct {
	byID map[string]domain.MCPServer
}

func newMemMCPServerRepo() *memMCPServerRepo {
	return &memMCPServerRepo{byID: make(map[string]domain.MCPServer)}
}

func (r *memMCPServerRepo) List(ctx context.Context) ([]domain.MCPServer, error) {
	var out []domain.MCPServer
	for _, s := range r.byID {
		out = append(out, s)
	}
	return out, nil
}

func (r *memMCPServerRepo) Get(ctx context.Context, id string) (domain.MCPServer, error) {
	s, ok := r.byID[id]
	if !ok {
		return domain.MCPServer{}, os.ErrNotExist
	}
	return s, nil
}

func (r *memMCPServerRepo) Upsert(ctx context.Context, s domain.MCPServer) error {
	r.byID[s.ID] = s
	return nil
}

func (r *memMCPServerRepo) Delete(ctx context.Context, id string) error {
	delete(r.byID, id)
	return nil
}
