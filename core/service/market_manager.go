package service

import (
	"context"
	"encoding/json"
	"fmt"
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
	skillImp *SkillImporter
	agentImp *AgentImporter

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
) *MarketManager {
	return &MarketManager{
		config:   config,
		registry: registry,
		skills:   skills,
		agents:   agents,
		skillImp: NewSkillImporter(),
		agentImp: NewAgentImporter(),
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
				// Try installing by id path convention if catalog omits dep.
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
		if err := m.installExpert(ctx, market, *item, ref, req.Overwrite, result); err != nil {
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

// Uninstall removes a market-installed skill or expert. Builtin items are refused.
func (m *MarketManager) Uninstall(ctx context.Context, req domain.UninstallMarketRequest) error {
	if req.Kind == "" || req.ID == "" {
		return fmt.Errorf("kind and id are required")
	}
	switch domain.MarketItemKind(req.Kind) {
	case domain.MarketKindSkill:
		sk, err := m.skills.Get(ctx, req.ID)
		if err != nil || sk == nil {
			return fmt.Errorf("skill %q not found", req.ID)
		}
		if sk.MarketSource == "" {
			return fmt.Errorf("skill %q was not installed from the market", req.ID)
		}
		// Template-backed skills must return to the builtin pack instead of
		// disappearing from the library until the next process restart.
		if m.skills.HasTemplate(req.ID) {
			if _, err := m.skills.ResetFromTemplate(ctx, req.ID); err != nil {
				return err
			}
			break
		}
		if err := m.skills.Delete(ctx, req.ID); err != nil {
			return err
		}
		_ = m.skills.DeleteFiles(ctx, req.ID)
	case domain.MarketKindExpert:
		ag, err := m.agents.Get(ctx, req.ID)
		if err != nil || ag == nil {
			return fmt.Errorf("expert %q not found", req.ID)
		}
		if ag.MarketSource == "" {
			return fmt.Errorf("expert %q was not installed from the market", req.ID)
		}
		if err := m.agents.Delete(ctx, req.ID); err != nil {
			return err
		}
	default:
		return fmt.Errorf("unsupported kind %q", req.Kind)
	}
	m.mu.Lock()
	m.cache = nil
	m.cacheWarnings = nil
	m.mu.Unlock()
	return nil
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
