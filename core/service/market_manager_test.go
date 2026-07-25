package service

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
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
	mgr := NewMarketManager(configMgr, reg, skills, agents)

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
	mgr := NewMarketManager(configMgr, &fakeRegistry{}, skills, agents)

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
