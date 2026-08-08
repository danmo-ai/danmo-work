package service

import (
	"context"
	"fmt"

	"danmo-work/core/domain"
	"danmo-work/core/port"
)

// ConfigManager exposes the full user-editable configuration file to API
// handlers and other callers. It delegates persistence to a port.ConfigStore.
type ConfigManager struct {
	store port.ConfigStore
}

func NewConfigManager(store port.ConfigStore) *ConfigManager {
	return &ConfigManager{store: store}
}

// Store returns the underlying config store (for optional migrator checks).
func (m *ConfigManager) Store() port.ConfigStore {
	return m.store
}

func (m *ConfigManager) Get(ctx context.Context) (*domain.ConfigFile, error) {
	return m.store.Load(ctx)
}

func (m *ConfigManager) Update(ctx context.Context, req domain.UpdateConfigFileRequest) (*domain.ConfigFile, error) {
	cfg, err := m.store.Load(ctx)
	if err != nil {
		return nil, fmt.Errorf("load config: %w", err)
	}

	if req.Data != nil {
		cfg.Data = *req.Data
	}
	if req.Server != nil {
		cfg.Server = *req.Server
	}
	if req.Instance != nil {
		cfg.Instance = *req.Instance
	}
	if req.Runtime != nil {
		cfg.Runtime = *req.Runtime
	}
	if req.Search != nil {
		if req.Search.Provider == "" {
			return nil, fmt.Errorf("search provider required")
		}
		if !isValidSearchProvider(req.Search.Provider) {
			return nil, fmt.Errorf("invalid search provider: %s", req.Search.Provider)
		}
		cfg.Search = domain.SearchConfig{
			Provider:     req.Search.Provider,
			BaseURL:      req.Search.BaseURL,
			APIKey:       req.Search.APIKey,
			TimeoutMs:    req.Search.TimeoutMs,
			MaxResults:   req.Search.MaxResults,
			Proxy:        req.Search.Proxy,
			UserAgent:    req.Search.UserAgent,
			HTMLFallback: req.Search.HTMLFallback,
		}
	}
	if req.LLM != nil {
		if req.LLM.Providers != nil {
			cfg.LLM.Providers = req.LLM.Providers
		}
		if req.LLM.Models != nil {
			cfg.LLM.Models = req.LLM.Models
		}
	}
	if req.Market != nil {
		cfg.Market = *req.Market
		if cfg.Market.CacheTTLHours <= 0 {
			cfg.Market.CacheTTLHours = 6
		}
	}
	if req.Channels != nil {
		cfg.Channels = *req.Channels
	}
	if req.Remote != nil {
		cfg.Remote = *req.Remote
	}

	if err := m.store.Save(ctx, cfg); err != nil {
		return nil, fmt.Errorf("save config: %w", err)
	}
	return cfg, nil
}
