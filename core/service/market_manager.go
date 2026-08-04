package service

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"sort"
	"strings"
	"sync"
	"time"

	"danmo-work/core/domain"
	"danmo-work/core/port"
)

// MarketManager merges catalogs from configured sources and installs packages.
type MarketManager struct {
	config   *ConfigManager
	registry port.MarketRegistry
	skills   *SkillManager
	agents   *AgentManager
	mcp      *MCPManager
	skillImp *SkillImporter
	agentImp *AgentImporter
	connImp  *ConnectorImporter

	mu            sync.Mutex
	cache         []domain.MarketListing
	cacheWarnings []string
	cacheAt       time.Time
	cacheTTL      time.Duration
	sourcePri     map[string]int
	sourceName    map[string]string
}

func NewMarketManager(
	config *ConfigManager,
	registry port.MarketRegistry,
	skills *SkillManager,
	agents *AgentManager,
	mcp *MCPManager,
) *MarketManager {
	return &MarketManager{
		config:   config,
		registry: registry,
		skills:   skills,
		agents:   agents,
		mcp:      mcp,
		skillImp: NewSkillImporter(),
		agentImp: NewAgentImporter(),
		connImp:  NewConnectorImporter(),
		cacheTTL: 6 * time.Hour,
	}
}

func (m *MarketManager) reloadFromConfig(ctx context.Context) error {
	cfg, err := m.config.Get(ctx)
	if err != nil {
		return err
	}
	if cfg.Market.CacheTTLHours > 0 {
		m.cacheTTL = time.Duration(cfg.Market.CacheTTLHours) * time.Hour
	}
	m.sourcePri = make(map[string]int)
	m.sourceName = make(map[string]string)
	for _, s := range cfg.Market.Sources {
		m.sourcePri[s.ID] = s.Priority
		m.sourceName[s.ID] = s.Name
	}
	return m.registry.Reload(cfg.Market.Sources)
}

func (m *MarketManager) ListSources(ctx context.Context) ([]domain.MarketSource, error) {
	cfg, err := m.config.Get(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]domain.MarketSource, len(cfg.Market.Sources))
	copy(out, cfg.Market.Sources)
	sort.SliceStable(out, func(i, j int) bool {
		return out[i].Priority < out[j].Priority
	})
	return out, nil
}

func (m *MarketManager) ListCatalog(ctx context.Context, refresh bool) (items []domain.MarketListing, warnings []string, err error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if !refresh && m.cache != nil && time.Since(m.cacheAt) < m.cacheTTL {
		out := make([]domain.MarketListing, len(m.cache))
		copy(out, m.cache)
		m.enrichInstalled(ctx, out)
		return out, append([]string(nil), m.cacheWarnings...), nil
	}

	if err := m.reloadFromConfig(ctx); err != nil {
		return nil, nil, err
	}

	var listings []domain.MarketListing
	var warns []string
	for _, src := range m.registry.List() {
		name := m.sourceName[src.SourceID()]
		if name == "" {
			name = src.SourceID()
		}
		cat, ferr := src.FetchCatalog(ctx)
		if ferr != nil {
			warns = append(warns, fmt.Sprintf("%s 访问失败: %v", name, ferr))
			continue
		}
		for _, item := range cat.Items {
			// Product-seeded connectors (e.g. github) are not sold via market.
			if item.Kind == domain.MarketKindConnector && IsProductBuiltinConnector(item.ID) {
				continue
			}
			listings = append(listings, domain.MarketListing{
				MarketItem: item,
				SourceID:   src.SourceID(),
				SourceName: name,
			})
		}
	}

	sort.SliceStable(listings, func(i, j int) bool {
		pi, pj := m.sourcePri[listings[i].SourceID], m.sourcePri[listings[j].SourceID]
		if pi != pj {
			return pi < pj
		}
		if listings[i].Kind != listings[j].Kind {
			return listings[i].Kind < listings[j].Kind
		}
		return listings[i].ID < listings[j].ID
	})

	m.cache = listings
	m.cacheWarnings = warns
	m.cacheAt = time.Now()
	out := make([]domain.MarketListing, len(listings))
	copy(out, listings)
	m.enrichInstalled(ctx, out)
	return out, append([]string(nil), warns...), nil
}

func (m *MarketManager) enrichInstalled(ctx context.Context, list []domain.MarketListing) {
	for i := range list {
		switch list[i].Kind {
		case domain.MarketKindSkill:
			if sk, err := m.skills.Get(ctx, list[i].ID); err == nil && sk != nil {
				list[i].Installed = true
			}
		case domain.MarketKindExpert:
			if _, err := m.agents.Get(ctx, list[i].ID); err == nil {
				list[i].Installed = true
			}
		case domain.MarketKindConnector:
			if m.mcp == nil {
				continue
			}
			if _, ok, err := m.mcp.FindByCatalogID(ctx, list[i].ID); err == nil && ok {
				list[i].Installed = true
			}
		}
	}
}

func (m *MarketManager) Install(ctx context.Context, req domain.InstallMarketRequest) (*domain.InstallMarketResult, error) {
	if req.SourceID == "" || req.ID == "" || req.Kind == "" {
		return nil, fmt.Errorf("sourceId, kind, and id are required")
	}
	if err := m.reloadFromConfig(ctx); err != nil {
		return nil, err
	}
	market, ok := m.registry.Get(req.SourceID)
	if !ok {
		return nil, fmt.Errorf("market source %q not found or disabled", req.SourceID)
	}

	cat, err := market.FetchCatalog(ctx)
	if err != nil {
		return nil, err
	}
	var item *domain.MarketItem
	for i := range cat.Items {
		if cat.Items[i].ID == req.ID && string(cat.Items[i].Kind) == req.Kind {
			item = &cat.Items[i]
			break
		}
	}
	if item == nil {
		return nil, fmt.Errorf("item %s/%s not found in source %s", req.Kind, req.ID, req.SourceID)
	}

	ref := req.Ref
	result := &domain.InstallMarketResult{
		Kind:     req.Kind,
		ID:       req.ID,
		SourceID: req.SourceID,
		Ref:      ref,
		Version:  item.Version,
	}

	switch domain.MarketItemKind(req.Kind) {
	case domain.MarketKindSkill:
		if err := m.installSkill(ctx, market, *item, ref, req.Overwrite, result); err != nil {
			return nil, err
		}
	case domain.MarketKindExpert:
		// Install skill deps first.
		for _, depID := range item.SkillDeps {
			depItem := findSkillItem(cat.Items, depID)
			if depItem == nil {
				depItem = &domain.MarketItem{
					Kind: domain.MarketKindSkill,
					ID:   depID,
					Path: "skills/" + depID,
				}
			}
			if err := m.installSkill(ctx, market, *depItem, ref, req.Overwrite, result); err != nil {
				return nil, fmt.Errorf("install skill dep %s: %w", depID, err)
			}
		}
		// Then connector deps (each may download platform binaries).
		for _, depID := range item.ConnectorDeps {
			depItem := findConnectorItem(cat.Items, depID)
			if depItem == nil {
				depItem = &domain.MarketItem{
					Kind: domain.MarketKindConnector,
					ID:   depID,
					Path: "connectors/" + depID,
				}
			}
			if IsProductBuiltinConnector(depItem.ID) {
				return nil, fmt.Errorf("connector dep %q is a product builtin; not installable from the market", depItem.ID)
			}
			if err := m.installConnector(ctx, market, *depItem, ref, req.Overwrite, result); err != nil {
				return nil, fmt.Errorf("install connector dep %s: %w", depID, err)
			}
		}
		if err := m.installExpert(ctx, market, *item, ref, req.Overwrite, result); err != nil {
			return nil, err
		}
	case domain.MarketKindBundle:
		// Same dependency order as expert for now.
		for _, depID := range item.SkillDeps {
			depItem := findSkillItem(cat.Items, depID)
			if depItem == nil {
				depItem = &domain.MarketItem{Kind: domain.MarketKindSkill, ID: depID, Path: "skills/" + depID}
			}
			if err := m.installSkill(ctx, market, *depItem, ref, req.Overwrite, result); err != nil {
				return nil, fmt.Errorf("install skill dep %s: %w", depID, err)
			}
		}
		for _, depID := range item.ConnectorDeps {
			depItem := findConnectorItem(cat.Items, depID)
			if depItem == nil {
				depItem = &domain.MarketItem{Kind: domain.MarketKindConnector, ID: depID, Path: "connectors/" + depID}
			}
			if IsProductBuiltinConnector(depItem.ID) {
				return nil, fmt.Errorf("connector dep %q is a product builtin", depItem.ID)
			}
			if err := m.installConnector(ctx, market, *depItem, ref, req.Overwrite, result); err != nil {
				return nil, fmt.Errorf("install connector dep %s: %w", depID, err)
			}
		}
		if item.Path != "" {
			if err := m.installExpert(ctx, market, *item, ref, req.Overwrite, result); err != nil {
				return nil, err
			}
		}
	case domain.MarketKindConnector:
		if IsProductBuiltinConnector(req.ID) {
			return nil, fmt.Errorf("connector %q is a product builtin (auto-seeded); configure it under Connectors — not installable from the market", req.ID)
		}
		if err := m.installConnector(ctx, market, *item, ref, req.Overwrite, result); err != nil {
			return nil, err
		}
	default:
		return nil, fmt.Errorf("unsupported kind %q", req.Kind)
	}

	// Invalidate catalog cache so Installed flags refresh.
	m.mu.Lock()
	m.cache = nil
	m.cacheWarnings = nil
	m.mu.Unlock()
	return result, nil
}

func findSkillItem(items []domain.MarketItem, id string) *domain.MarketItem {
	for i := range items {
		if items[i].ID == id && items[i].Kind == domain.MarketKindSkill {
			return &items[i]
		}
	}
	return nil
}

func findConnectorItem(items []domain.MarketItem, id string) *domain.MarketItem {
	for i := range items {
		if items[i].ID == id && items[i].Kind == domain.MarketKindConnector {
			return &items[i]
		}
	}
	return nil
}

func (m *MarketManager) installSkill(
	ctx context.Context,
	market port.Market,
	item domain.MarketItem,
	ref string,
	overwrite bool,
	result *domain.InstallMarketResult,
) error {
	if existing, err := m.skills.Get(ctx, item.ID); err == nil && existing != nil && !overwrite {
		result.Skipped = append(result.Skipped, item.ID)
		return nil
	}
	dir, cleanup, err := market.FetchPackage(ctx, item, ref)
	if err != nil {
		return err
	}
	if cleanup != nil {
		defer cleanup()
	}
	skill, files, err := m.skillImp.Import(dir)
	if err != nil {
		return err
	}
	// Market catalog id is authoritative (ClawHub uses owner__slug / clawhub__slug).
	prevID := skill.ID
	if item.ID != "" && skill.ID != item.ID {
		skill.ID = item.ID
		for i := range files {
			files[i].SkillID = item.ID
			files[i].ID = item.ID + ":" + files[i].Path
		}
	}
	// Re-normalize against the final meta id (Import may have prefixed with frontmatter name).
	if prevID != skill.ID {
		skill.Body = NormalizeSkillBodyRefsAfterIDChange(skill.Body, skill.ID, prevID)
	} else {
		skill.Body = NormalizeSkillBodyRefs(skill.Body, skill.ID)
	}
	if item.Name != "" {
		skill.Name = item.Name
	}
	if item.Description != "" && skill.Description == "" {
		skill.Description = item.Description
	}
	if item.Compatibility != "" && skill.Compatibility == "" {
		skill.Compatibility = item.Compatibility
	}
	if skill.Metadata == nil {
		skill.Metadata = map[string]string{}
	}
	// Force market provenance meta on install (badge reads marketSource from this).
	skill.MarketSource = market.SourceID()
	skill.Metadata["market.source"] = market.SourceID()
	if ref != "" {
		skill.Metadata["market.ref"] = ref
	} else {
		delete(skill.Metadata, "market.ref")
	}
	if item.Version != "" {
		skill.Metadata["market.version"] = item.Version
		if skill.Metadata["version"] == "" {
			skill.Metadata["version"] = item.Version
		}
	}
	if market.Kind() == "clawhub" {
		slug := strings.Trim(strings.ReplaceAll(item.Path, "\\", "/"), "/")
		if slug == "" {
			slug = item.ID
		}
		owner := strings.TrimSpace(item.Author)
		if owner != "" {
			skill.Metadata["clawhub.owner"] = owner
		}
		skill.Metadata["clawhub.slug"] = slug
		if owner != "" {
			skill.Metadata["clawhub.url"] = "https://clawhub.ai/" + owner + "/skills/" + slug
		} else {
			skill.Metadata["clawhub.url"] = "https://clawhub.ai/skills/" + slug
		}
		// Soft compatibility from nested openclaw/clawdbot install specs in SKILL.md.
		if skill.Compatibility == "" {
			skill.Compatibility = compatibilityFromSkillMetadata(skill.Metadata)
		}
	}
	if market.Kind() == "techleads" || market.Kind() == "tlc" || market.Kind() == "tech-leads-club" {
		name := strings.TrimSpace(item.Name)
		if name == "" {
			name = strings.TrimPrefix(item.ID, "tlc__")
		}
		skill.Metadata["techleads.name"] = name
		if p := strings.Trim(strings.ReplaceAll(item.Path, "\\", "/"), "/"); p != "" {
			skill.Metadata["techleads.path"] = p
		}
		if item.Category != "" {
			skill.Metadata["techleads.category"] = item.Category
		}
		skill.Metadata["techleads.url"] = "https://tech-leads-club.github.io/agent-skills/"
		if item.Author != "" {
			skill.Metadata["techleads.author"] = item.Author
		}
	}
	if err := m.skills.Upsert(ctx, *skill); err != nil {
		return err
	}
	_ = m.skills.DeleteFiles(ctx, skill.ID)
	for _, f := range files {
		if err := m.skills.UpsertFile(ctx, f); err != nil {
			return err
		}
	}
	result.Installed = append(result.Installed, skill.ID)
	return nil
}

func (m *MarketManager) installExpert(
	ctx context.Context,
	market port.Market,
	item domain.MarketItem,
	ref string,
	overwrite bool,
	result *domain.InstallMarketResult,
) error {
	if _, err := m.agents.Get(ctx, item.ID); err == nil && !overwrite {
		result.Skipped = append(result.Skipped, item.ID)
		return nil
	}
	dir, cleanup, err := market.FetchPackage(ctx, item, ref)
	if err != nil {
		return err
	}
	if cleanup != nil {
		defer cleanup()
	}
	agent, err := m.agentImp.Import(dir)
	if err != nil {
		return err
	}
	// Force market provenance meta on install (stored as YAML frontmatter).
	agent.MarketSource = market.SourceID()
	meta := map[string]string{"market.source": market.SourceID()}
	if ref != "" {
		meta["market.ref"] = ref
	}
	if item.Version != "" {
		meta["market.version"] = item.Version
	}
	agent.SystemPrompt = EncodeAgentSystemPrompt(agent.SystemPrompt, meta)
	if err := m.agents.Upsert(ctx, *agent); err != nil {
		return err
	}
	result.Installed = append(result.Installed, agent.ID)
	return nil
}

func (m *MarketManager) installConnector(
	ctx context.Context,
	market port.Market,
	item domain.MarketItem,
	ref string,
	overwrite bool,
	result *domain.InstallMarketResult,
) error {
	if m.mcp == nil {
		return fmt.Errorf("connector market install is not configured")
	}
	existing, found, err := m.mcp.FindByCatalogID(ctx, item.ID)
	if err != nil {
		return err
	}
	if found && !overwrite {
		result.Skipped = append(result.Skipped, existing.ID)
		return nil
	}
	dir, cleanup, err := market.FetchPackage(ctx, item, ref)
	if err != nil {
		return err
	}
	if cleanup != nil {
		defer cleanup()
	}

	// Platform deps script (download binaries, apt/brew, init config, …).
	if scriptRel, logOut, derr := RunConnectorDepsForPackage(ctx, dir, item); derr != nil {
		return derr
	} else if scriptRel != "" {
		run := domain.ConnectorDepsRun{
			ConnectorID: item.ID,
			Script:      scriptRel,
			Log:         logOut,
			Phase:       "install",
		}
		result.DepsRuns = append(result.DepsRuns, run)
		result.DepsScript = scriptRel
		result.DepsLog = logOut
	}

	entry, err := m.connImp.Import(dir)
	if err != nil {
		return err
	}
	if item.ID != "" {
		entry.ID = item.ID
	}
	if item.Name != "" {
		entry.Name = item.Name
	}
	if item.Description != "" && entry.Description == "" {
		entry.Description = item.Description
	}
	if item.Category != "" && entry.Category == "" {
		entry.Category = item.Category
	}
	// Prefer resolved local binary when stdio command is a bare name.
	if entry.Transport == "stdio" || entry.Transport == "" {
		if bin := ResolveCodeGraphBin(); bin != "" && (entry.Command == "" || entry.Command == codeGraphBinName || entry.Command == codeGraphExecutableName() || entry.ID == CodeGraphServerID) {
			if entry.ID == CodeGraphServerID || entry.Command == codeGraphBinName || entry.Command == codeGraphExecutableName() {
				entry.Command = bin
			}
		}
		if entry.ID == CodeGraphServerID && strings.TrimSpace(entry.Env) == "" {
			entry.Env = CodeGraphMCPEnv()
		}
	}
	req := InstallCatalogEntry(*entry, entry.Name)
	req.CatalogID = entry.ID
	req.MarketSource = market.SourceID()
	if entry.ID == CodeGraphServerID {
		req.Network = "deny"
		if strings.TrimSpace(req.Env) == "" {
			req.Env = CodeGraphMCPEnv()
		}
	}
	if found {
		srv, uerr := m.mcp.Update(ctx, existing.ID, req)
		if uerr != nil {
			return uerr
		}
		result.Installed = append(result.Installed, srv.ID)
		m.refreshConnectorTools(ctx, srv.ID)
		return nil
	}
	req.ID = entry.ID
	srv, cerr := m.mcp.Create(ctx, req)
	if cerr != nil {
		return cerr
	}
	result.Installed = append(result.Installed, srv.ID)
	m.refreshConnectorTools(ctx, srv.ID)
	return nil
}

// refreshConnectorTools dials the MCP server so status becomes connected and tools are listed.
// Failure is non-fatal: the connector row is already persisted (status may be "error").
func (m *MarketManager) refreshConnectorTools(ctx context.Context, id string) {
	if m.mcp == nil || strings.TrimSpace(id) == "" {
		return
	}
	if _, err := m.mcp.RefreshTools(ctx, id); err != nil {
		log.Printf("[market] connector %s: refresh tools after install: %v", id, err)
	}
}

// Uninstall removes a market-installed skill, expert, or connector. Builtin items are refused.
// For connectors with RunCleanup=true, an optional uninstall deps script is fetched and run first.
func (m *MarketManager) Uninstall(ctx context.Context, req domain.UninstallMarketRequest) (*domain.UninstallMarketResult, error) {
	if req.Kind == "" || req.ID == "" {
		return nil, fmt.Errorf("kind and id are required")
	}
	out := &domain.UninstallMarketResult{Kind: req.Kind, ID: req.ID}
	switch domain.MarketItemKind(req.Kind) {
	case domain.MarketKindSkill:
		sk, err := m.skills.Get(ctx, req.ID)
		if err != nil || sk == nil {
			return nil, fmt.Errorf("skill %q not found", req.ID)
		}
		if sk.MarketSource == "" {
			return nil, fmt.Errorf("skill %q was not installed from the market", req.ID)
		}
		// Template-backed skills must return to the builtin pack instead of
		// disappearing from the library until the next process restart.
		if m.skills.HasTemplate(req.ID) {
			if _, err := m.skills.ResetFromTemplate(ctx, req.ID); err != nil {
				return nil, err
			}
			break
		}
		if err := m.skills.Delete(ctx, req.ID); err != nil {
			return nil, err
		}
		_ = m.skills.DeleteFiles(ctx, req.ID)
	case domain.MarketKindExpert:
		ag, err := m.agents.Get(ctx, req.ID)
		if err != nil || ag == nil {
			return nil, fmt.Errorf("expert %q not found", req.ID)
		}
		if ag.MarketSource == "" {
			return nil, fmt.Errorf("expert %q was not installed from the market", req.ID)
		}
		if err := m.agents.Delete(ctx, req.ID); err != nil {
			return nil, err
		}
	case domain.MarketKindConnector:
		if m.mcp == nil {
			return nil, fmt.Errorf("connector market uninstall is not configured")
		}
		srv, ok, err := m.mcp.FindByCatalogID(ctx, req.ID)
		if err != nil {
			return nil, err
		}
		if !ok {
			return nil, fmt.Errorf("connector %q not found", req.ID)
		}
		if srv.MarketSource == "" {
			return nil, fmt.Errorf("connector %q was not installed from the market", req.ID)
		}
		if req.RunCleanup {
			if cerr := m.runConnectorCleanup(ctx, req, srv, out); cerr != nil {
				return out, cerr
			}
		}
		if err := m.mcp.Delete(ctx, srv.ID); err != nil {
			return out, err
		}
	default:
		return nil, fmt.Errorf("unsupported kind %q", req.Kind)
	}
	m.mu.Lock()
	m.cache = nil
	m.cacheWarnings = nil
	m.mu.Unlock()
	return out, nil
}

func (m *MarketManager) runConnectorCleanup(
	ctx context.Context,
	req domain.UninstallMarketRequest,
	srv domain.MCPServer,
	out *domain.UninstallMarketResult,
) error {
	sourceID := strings.TrimSpace(req.SourceID)
	if sourceID == "" {
		sourceID = srv.MarketSource
	}
	market, ok := m.registry.Get(sourceID)
	if !ok {
		return fmt.Errorf("market source %q not found for cleanup", sourceID)
	}
	item, err := m.lookupConnectorCatalogItem(ctx, market, req.ID)
	if err != nil {
		return err
	}
	ref := strings.TrimSpace(req.Ref)
	dir, cleanup, err := market.FetchPackage(ctx, *item, ref)
	if err != nil {
		return fmt.Errorf("fetch connector package for cleanup: %w", err)
	}
	if cleanup != nil {
		defer cleanup()
	}
	scriptRel, logOut, err := RunConnectorUninstallForPackage(ctx, dir, *item)
	if err != nil {
		return err
	}
	if scriptRel == "" {
		return nil
	}
	run := domain.ConnectorDepsRun{
		ConnectorID: item.ID,
		Script:      scriptRel,
		Log:         logOut,
		Phase:       "uninstall",
	}
	out.CleanupRuns = append(out.CleanupRuns, run)
	out.CleanupScript = scriptRel
	out.CleanupLog = logOut
	return nil
}

func (m *MarketManager) lookupConnectorCatalogItem(ctx context.Context, market port.Market, id string) (*domain.MarketItem, error) {
	cat, err := market.FetchCatalog(ctx)
	if err != nil {
		return nil, err
	}
	if item := findConnectorItem(cat.Items, id); item != nil {
		cp := *item
		return &cp, nil
	}
	// Fallback: package path convention when catalog entry is missing.
	return &domain.MarketItem{
		Kind: domain.MarketKindConnector,
		ID:   id,
		Path: "connectors/" + id,
	}, nil
}

// compatibilityFromSkillMetadata derives a soft tip from nested ClawHub runtime metadata.
func compatibilityFromSkillMetadata(meta map[string]string) string {
	if len(meta) == 0 {
		return ""
	}
	var parts []string
	for _, key := range []string{"openclaw", "clawdbot", "clawdis"} {
		raw, ok := meta[key]
		if !ok || strings.TrimSpace(raw) == "" {
			continue
		}
		var nested map[string]any
		if err := json.Unmarshal([]byte(raw), &nested); err != nil {
			continue
		}
		if install, ok := nested["install"].([]any); ok && len(install) > 0 {
			seen := map[string]bool{}
			var kinds []string
			for _, x := range install {
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
			if len(kinds) > 0 {
				parts = append(parts, "needs:"+strings.Join(kinds, ","))
			} else {
				parts = append(parts, "needs:deps")
			}
		}
		if cfg, ok := nested["config"]; ok && cfg != nil {
			parts = append(parts, "openclaw-config")
		}
	}
	return strings.Join(parts, "; ")
}
