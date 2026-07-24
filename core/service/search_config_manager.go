package service

import (
	"context"
	"fmt"
	"sync"

	"danmo-work/core/domain"
	"danmo-work/core/port"
)

type SearchConfigManager struct {
	store  port.SearchConfigStore
	mu     sync.RWMutex
	cache  domain.SearchConfig
	loaded bool
}

func NewSearchConfigManager(store port.SearchConfigStore) *SearchConfigManager {
	return &SearchConfigManager{store: store}
}

func (m *SearchConfigManager) ensureCache(ctx context.Context) {
	m.mu.RLock()
	if m.loaded {
		m.mu.RUnlock()
		return
	}
	m.mu.RUnlock()

	m.mu.Lock()
	defer m.mu.Unlock()
	if m.loaded {
		return
	}
	cfg, err := m.store.Get(ctx)
	if err != nil {
		m.cache = domain.SearchConfig{Provider: domain.SearchProviderDuckDuckGo}
	} else {
		m.cache = cfg
	}
	if m.cache.Provider == "" {
		m.cache.Provider = domain.SearchProviderDuckDuckGo
	}
	m.loaded = true
}

func (m *SearchConfigManager) Get(ctx context.Context) (domain.SearchConfig, error) {
	m.ensureCache(ctx)
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.cache, nil
}

func (m *SearchConfigManager) Upsert(ctx context.Context, req domain.UpsertSearchConfigRequest) (domain.SearchConfig, error) {
	if req.Provider == "" {
		return domain.SearchConfig{}, fmt.Errorf("provider required")
	}
	if !isValidSearchProvider(req.Provider) {
		return domain.SearchConfig{}, fmt.Errorf("invalid search provider: %s", req.Provider)
	}

	cfg := domain.SearchConfig{
		Provider:     req.Provider,
		BaseURL:      req.BaseURL,
		APIKey:       req.APIKey,
		TimeoutMs:    req.TimeoutMs,
		MaxResults:   req.MaxResults,
		Proxy:        req.Proxy,
		UserAgent:    req.UserAgent,
		HTMLFallback: req.HTMLFallback,
	}
	if err := m.store.Upsert(ctx, cfg); err != nil {
		return domain.SearchConfig{}, err
	}

	m.mu.Lock()
	m.cache = cfg
	m.loaded = true
	m.mu.Unlock()

	return cfg, nil
}

func isValidSearchProvider(p domain.SearchProvider) bool {
	switch p {
	case domain.SearchProviderDuckDuckGo, domain.SearchProviderBing, domain.SearchProviderTavily,
		domain.SearchProviderBocha, domain.SearchProviderMetaso, domain.SearchProviderSearxng,
		domain.SearchProviderBaidu, domain.SearchProviderVolcengine, domain.SearchProviderSofya,
		domain.SearchProviderBrave:
		return true
	}
	return false
}
