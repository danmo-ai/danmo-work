package service

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
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
	dataDir := t.TempDir()
	skills := NewSkillManager(dataDir)
	agents := NewAgentManager(dataDir)
	mcp := NewMCPManager(dataDir)
	mgr := NewMarketManager(configMgr, reg, skills, agents, mcp, nil)

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
		ID:       "notion-mcp",
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(res3.Installed) != 1 || res3.Installed[0] != "notion-mcp" {
		t.Fatalf("unexpected connector install: %+v", res3)
	}
	srv, err := mcp.Get(ctx, "notion-mcp")
	if err != nil {
		t.Fatal("connector not installed")
	}
	if srv.MarketSource != "local" || srv.CatalogID != "notion-mcp" {
		t.Fatalf("connector provenance: %+v", srv)
	}
	list2, _, err := mgr.ListCatalog(ctx, true)
	if err != nil {
		t.Fatal(err)
	}
	foundInstalled := false
	for _, item := range list2 {
		if item.Kind == domain.MarketKindConnector && item.ID == "notion-mcp" && item.Installed {
			foundInstalled = true
			break
		}
	}
	if !foundInstalled {
		t.Fatal("expected notion-mcp marked installed in catalog")
	}
	// Legacy/builtin GitHub connector must not install from market (filtered or rejected).
	if _, err := mgr.Install(ctx, domain.InstallMarketRequest{
		SourceID: "local",
		Kind:     "connector",
		ID:       "github-mcp",
	}); err == nil {
		t.Fatal("expected github-mcp market install to fail")
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
  "schemaVersion": 1,
  "items": [{"id":"tlc__pr-review","name":"PR Review","path":"skills/pr-review","kind":"skill"}]
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
	dataDir := t.TempDir()
	skills := NewSkillManager(dataDir)
	agents := NewAgentManager(dataDir)
	mgr := NewMarketManager(NewConfigManager(cfgStore), &fakeRegistry{m: &fakeMarket{id: "local", root: root}}, skills, agents, nil, nil)

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
	dataDir := t.TempDir()
	skills := NewSkillManager(dataDir)
	_ = skills.Upsert(ctx, domain.Skill{ID: "debugging", Name: "debugging", Body: "builtin-body", Source: "builtin"})
	agents := NewAgentManager(dataDir)
	mgr := NewMarketManager(configMgr, &fakeRegistry{}, skills, agents, nil, nil)

	_ = skills.Upsert(ctx, domain.Skill{
		ID: "debugging", Name: "debugging", Body: "market-body",
		MarketSource: "local",
		Metadata:     map[string]string{"market.source": "local"},
	})

	if _, err := mgr.Uninstall(ctx, domain.UninstallMarketRequest{
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
	if got.Source != "builtin" {
		t.Fatal("restored skill should be builtin")
	}
}

func TestMarketInstallExpertPullsConnectorDepsScript(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("bash deps fixture")
	}
	home := t.TempDir()
	prevHome := marketDepsHome
	marketDepsHome = func() string { return home }
	t.Cleanup(func() { marketDepsHome = prevHome })

	root := t.TempDir()
	skillPkg := filepath.Join(root, "skills", "cg-skill")
	connPkg := filepath.Join(root, "connectors", "cg-conn")
	expertPkg := filepath.Join(root, "experts", "cg-expert")
	for _, d := range []string{skillPkg, connPkg, expertPkg, filepath.Join(connPkg, "deps")} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	payload := []byte("CGfake-expert-dep")
	markerScript := "#!/bin/sh\nmkdir -p \"$DANMO_HOME/bin\"\nprintf '%s' '" + string(payload) + "' > \"$DANMO_HOME/bin/codegraph\"\nchmod +x \"$DANMO_HOME/bin/codegraph\"\necho deps-ok\n"
	platform := runtime.GOOS
	if platform != "darwin" && platform != "linux" {
		t.Skip("unix deps only")
	}
	scriptPath := filepath.Join(connPkg, "deps", platform+".sh")
	if err := os.WriteFile(scriptPath, []byte(markerScript), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(skillPkg, "SKILL.md"), []byte("---\nname: cg-skill\ndescription: t\n---\n\nbody\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	connJSON := `{
  "id":"cg-conn","name":"CG Conn","transport":"stdio","command":"codegraph","args":"serve --mcp","auth":"none","ambientMount":false
}`
	if err := os.WriteFile(filepath.Join(connPkg, "connector.json"), []byte(connJSON), 0o644); err != nil {
		t.Fatal(err)
	}
	agentMD := `---
id: cg-expert
name: CG Expert
mode: subagent
skills:
  - cg-skill
mcp_servers:
  - cg-conn
---

prompt
`
	if err := os.WriteFile(filepath.Join(expertPkg, "AGENT.md"), []byte(agentMD), 0o644); err != nil {
		t.Fatal(err)
	}
	cat := fmt.Sprintf(`{
  "schemaVersion": 2,
  "items": [
    {"id":"cg-skill","name":"CG Skill","kind":"skill","path":"skills/cg-skill"},
    {"id":"cg-conn","name":"CG Conn","kind":"connector","path":"connectors/cg-conn","deps":{"%s":"deps/%s.sh"}},
    {"id":"cg-expert","name":"CG Expert","kind":"expert","path":"experts/cg-expert","skillDeps":["cg-skill"],"connectorDeps":["cg-conn"]}
  ]
}`, platform, platform)
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
	dataDir := t.TempDir()
	skills := NewSkillManager(dataDir)
	agents := NewAgentManager(dataDir)
	mcp := NewMCPManager(dataDir)
	mgr := NewMarketManager(NewConfigManager(cfgStore), &fakeRegistry{m: &fakeMarket{id: "local", root: root}}, skills, agents, mcp, nil)

	res, err := mgr.Install(context.Background(), domain.InstallMarketRequest{
		SourceID: "local", Kind: "expert", ID: "cg-expert", Overwrite: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	wantInstalled := map[string]bool{"cg-skill": true, "cg-conn": true, "cg-expert": true}
	for _, id := range res.Installed {
		delete(wantInstalled, id)
	}
	if len(wantInstalled) != 0 {
		t.Fatalf("missing installs %v; got %+v", wantInstalled, res)
	}
	if res.DepsScript == "" {
		t.Fatalf("expected deps script run, got %+v", res)
	}
	if len(res.DepsRuns) == 0 {
		t.Fatalf("expected depsRuns, got %+v", res)
	}
	if !strings.Contains(res.DepsLog, "deps-ok") {
		t.Fatalf("deps log missing: %q", res.DepsLog)
	}
	if _, err := skills.Get(context.Background(), "cg-skill"); err != nil {
		t.Fatal("skill dep missing")
	}
	if _, err := agents.Get(context.Background(), "cg-expert"); err != nil {
		t.Fatal("expert missing")
	}
	if _, err := mcp.Get(context.Background(), "cg-conn"); err != nil {
		t.Fatal("connector dep missing")
	}
	bin := filepath.Join(home, "bin", "codegraph")
	b, err := os.ReadFile(bin)
	if err != nil {
		t.Fatal(err)
	}
	if string(b) != string(payload) {
		t.Fatalf("payload mismatch")
	}
}

func TestFirstLaunchScriptsHaveNoCodegraphUnpack(t *testing.T) {
	root := filepath.Clean(filepath.Join("..", "..", "scripts", "first_launch"))
	paths := []string{
		filepath.Join(root, "darwin", "post-install.sh"),
		filepath.Join(root, "linux", "post-install.sh"),
		filepath.Join(root, "windows", "post-install.ps1"),
	}
	for _, p := range paths {
		data, err := os.ReadFile(p)
		if err != nil {
			t.Fatal(err)
		}
		s := string(data)
		if strings.Contains(s, "install_codegraph") || strings.Contains(s, "Expand-Archive") {
			t.Fatalf("%s still unpacks codegraph", p)
		}
	}
}
