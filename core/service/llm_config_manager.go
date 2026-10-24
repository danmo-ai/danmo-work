package service

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"danmo-work/core/domain"
	"danmo-work/core/port"
)

type LLMConfigManager struct {
	store    port.LLMConfigRepo
	modelCfg *ModelConfigRegistry
	mu       sync.RWMutex
	cache    map[string]domain.LLMProviderConfig
	loaded   bool
}

func NewLLMConfigManager(store port.LLMConfigRepo, modelCfg *ModelConfigRegistry) *LLMConfigManager {
	return &LLMConfigManager{store: store, modelCfg: modelCfg}
}

func (m *LLMConfigManager) ensureCache(ctx context.Context) {
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
	cfgs, err := m.store.GetAll(ctx)
	if err != nil {
		return
	}
	m.cache = make(map[string]domain.LLMProviderConfig, len(cfgs))
	for _, cfg := range cfgs {
		m.cache[cfg.ID] = cfg
	}
	m.loaded = true
}

func (m *LLMConfigManager) GetAll(ctx context.Context) ([]domain.LLMProviderConfig, error) {
	m.ensureCache(ctx)
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := make([]domain.LLMProviderConfig, 0, len(m.cache))
	for _, cfg := range m.cache {
		out = append(out, cfg)
	}
	return out, nil
}

func (m *LLMConfigManager) GetByID(ctx context.Context, id string) (domain.LLMProviderConfig, error) {
	m.ensureCache(ctx)
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.cache[id], nil
}

func (m *LLMConfigManager) Upsert(ctx context.Context, req domain.UpsertLLMProviderConfigRequest) (domain.LLMProviderConfig, error) {
	if req.Provider == "" {
		return domain.LLMProviderConfig{}, fmt.Errorf("provider required")
	}
	if req.Name == "" {
		return domain.LLMProviderConfig{}, fmt.Errorf("name required")
	}

	// Load existing providers from DB first. Otherwise the first Upsert after
	// startup can mark an incomplete in-memory cache as fully loaded and hide
	// every other provider (e.g. deepseek disappears after saving a new one).
	m.ensureCache(ctx)

	now := time.Now().UTC().Format(time.RFC3339)

	m.mu.RLock()
	existing := m.cache[fmt.Sprintf("llm-%s", req.Name)]
	m.mu.RUnlock()

	defaultDialect := strings.TrimSpace(req.DefaultProtocolDialect)
	if defaultDialect == "" {
		defaultDialect = strings.TrimSpace(req.DefaultDialect)
	}
	cfg := domain.LLMProviderConfig{
		ID:                     existing.ID,
		Provider:               domain.NormalizeProtocol(req.Provider),
		Name:                   req.Name,
		APIKey:                 req.APIKey,
		BaseURL:                req.BaseURL,
		DefaultProtocolDialect: defaultDialect,
		Models:                 normalizeModelRefDialects(req.Models),
		UpdatedAt:              now,
	}
	if cfg.ID == "" {
		cfg.ID = fmt.Sprintf("llm-%s", req.Name)
		cfg.CreatedAt = now
	}

	if err := m.store.Upsert(ctx, cfg); err != nil {
		return domain.LLMProviderConfig{}, err
	}

	m.mu.Lock()
	if m.cache == nil {
		m.cache = make(map[string]domain.LLMProviderConfig)
	}
	m.cache[cfg.ID] = cfg
	m.mu.Unlock()

	return cfg, nil
}

func (m *LLMConfigManager) Delete(ctx context.Context, id string) error {
	if err := m.store.Delete(ctx, id); err != nil {
		return err
	}
	m.mu.Lock()
	if m.cache != nil {
		delete(m.cache, id)
	}
	m.mu.Unlock()
	return nil
}

func (m *LLMConfigManager) ListModels(ctx context.Context) []domain.LLMModel {
	m.ensureCache(ctx)
	m.mu.RLock()
	defer m.mu.RUnlock()
	var out []domain.LLMModel
	for _, cfg := range m.cache {
		for _, ref := range cfg.Models {
			if !ref.Enabled {
				continue
			}
			modelID := cfg.Name + "/" + ref.Name
			var efforts []string
			vision := false
			if m.modelCfg != nil {
				efforts = m.modelCfg.AvailableEfforts(modelID)
				vision = m.modelCfg.SupportsVision(modelID)
			}
			if len(efforts) == 0 {
				efforts = DefaultEffortsForProvider(cfg.Provider)
			}
			out = append(out, domain.LLMModel{
				ID:               modelID,
				Name:             ref.Name,
				ProviderID:       cfg.ID,
				Provider:         string(cfg.Provider),
				Enabled:          ref.Enabled,
				AvailableEfforts: efforts,
				Vision:           vision,
				ProtocolDialect: domain.ResolveProtocolDialect(cfg, ref.Name),
			})
		}
	}
	return out
}

// SetModelConfigs replaces in-memory model parameter entries (context, vision, etc.).
func (m *LLMConfigManager) SetModelConfigs(models []domain.ModelConfig) {
	if m.modelCfg != nil {
		m.modelCfg.SetModels(models)
	}
}

// MigrateDialectsFromModelConfigs copies legacy YAML reasoning_dialect onto
// matching provider model rows (when empty) and fills empty provider
// DefaultProtocolDialect from presetsByName (name → dialect) or Infer. Returns true
// when any model still carried legacy YAML fields that should be stripped.
func (m *LLMConfigManager) MigrateDialectsFromModelConfigs(ctx context.Context, models []domain.ModelConfig, presetsByName map[string]string) (hadLegacy bool, err error) {
	dialectByModel := map[string]string{}
	for _, mc := range models {
		if d := strings.TrimSpace(mc.LegacyProtocolDialect); d != "" {
			hadLegacy = true
			dialectByModel[strings.ToLower(mc.Model)] = d
			if _, bare, _ := splitModelIDParts(mc.Model); bare != "" && bare != mc.Model {
				dialectByModel[strings.ToLower(bare)] = d
			}
		}
		if mc.LegacyThinkingMode != "" || len(mc.LegacyEffortBudgetTokens) > 0 {
			hadLegacy = true
		}
	}

	m.ensureCache(ctx)
	m.mu.Lock()
	defer m.mu.Unlock()

	now := time.Now().UTC().Format(time.RFC3339)
	for id, cfg := range m.cache {
		changed := false
		if strings.TrimSpace(cfg.DefaultProtocolDialect) == "" {
			if d := strings.TrimSpace(presetsByName[strings.ToLower(cfg.Name)]); d != "" {
				cfg.DefaultProtocolDialect = d
				changed = true
			} else if d := strings.TrimSpace(presetsByName[strings.ToLower(strings.TrimPrefix(cfg.ID, "llm-"))]); d != "" {
				cfg.DefaultProtocolDialect = d
				changed = true
			}
		}
		for i, ref := range cfg.Models {
			if strings.TrimSpace(ref.ProtocolDialect) != "" {
				continue
			}
			key := strings.ToLower(ref.Name)
			full := strings.ToLower(cfg.Name + "/" + ref.Name)
			d := dialectByModel[full]
			if d == "" {
				d = dialectByModel[key]
			}
			if d == "" {
				continue
			}
			cfg.Models[i].ProtocolDialect = d
			changed = true
		}
		if !changed {
			continue
		}
		cfg.UpdatedAt = now
		if err := m.store.Upsert(ctx, cfg); err != nil {
			return hadLegacy, err
		}
		m.cache[id] = cfg
	}
	return hadLegacy, nil
}

func normalizeModelRefDialects(refs []domain.LLMModelRef) []domain.LLMModelRef {
	if len(refs) == 0 {
		return refs
	}
	out := make([]domain.LLMModelRef, len(refs))
	copy(out, refs)
	for i := range out {
		out[i].ProtocolDialect = strings.TrimSpace(out[i].ProtocolDialect)
	}
	return out
}

func splitModelIDParts(modelID string) (provider, model, effort string) {
	parts := strings.SplitN(modelID, "/", 3)
	if len(parts) >= 1 {
		provider = parts[0]
	}
	if len(parts) >= 2 {
		model = parts[1]
	}
	if len(parts) >= 3 {
		effort = parts[2]
	}
	return
}

func (m *LLMConfigManager) ResolveProvider(ctx context.Context, modelID string) (domain.LLMProviderConfig, string, error) {
	parts := strings.SplitN(modelID, "/", 3)
	if len(parts) < 2 {
		return domain.LLMProviderConfig{}, "", fmt.Errorf("invalid model identifier: %s, expected format: provider_name/model_name", modelID)
	}
	providerName := parts[0]
	modelName := parts[1]

	m.ensureCache(ctx)
	m.mu.RLock()
	cfg, ok := m.cache[fmt.Sprintf("llm-%s", providerName)]
	m.mu.RUnlock()
	if !ok || cfg.ID == "" {
		return domain.LLMProviderConfig{}, "", fmt.Errorf("provider not found: %s", providerName)
	}
	return cfg, modelName, nil
}

func (m *LLMConfigManager) FetchModels(ctx context.Context, configID string) ([]domain.LLMModelRef, error) {
	m.ensureCache(ctx)
	m.mu.RLock()
	cfg, ok := m.cache[configID]
	m.mu.RUnlock()
	if !ok || cfg.ID == "" {
		return nil, fmt.Errorf("config not found: %s", configID)
	}

	remoteModels, err := listRemoteModels(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("fetch models from remote: %w", err)
	}

	existingByName := make(map[string]domain.LLMModelRef, len(cfg.Models))
	for _, ref := range cfg.Models {
		existingByName[ref.Name] = ref
	}

	now := time.Now().UTC().Format(time.RFC3339)
	merged := make([]domain.LLMModelRef, 0, len(remoteModels))
	for _, name := range remoteModels {
		prev, ok := existingByName[name]
		enabled := ok && prev.Enabled
		dialect := ""
		if ok {
			dialect = prev.ProtocolDialect
		}
		merged = append(merged, domain.LLMModelRef{
			Name:             name,
			Enabled:          enabled,
			ProtocolDialect: dialect,
		})
	}

	cfg.Models = merged
	cfg.UpdatedAt = now

	if err := m.store.Upsert(ctx, cfg); err != nil {
		return nil, fmt.Errorf("save config: %w", err)
	}

	m.mu.Lock()
	if m.cache != nil {
		m.cache[cfg.ID] = cfg
	}
	m.mu.Unlock()

	return merged, nil
}

func (m *LLMConfigManager) FetchModelsFromRequest(ctx context.Context, req domain.UpsertLLMProviderConfigRequest) ([]domain.LLMModelRef, error) {
	cfg := domain.LLMProviderConfig{
		Provider: domain.NormalizeProtocol(req.Provider),
		APIKey:   req.APIKey,
		BaseURL:  req.BaseURL,
	}
	remoteModels, err := listRemoteModels(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("fetch models from remote: %w", err)
	}
	refs := make([]domain.LLMModelRef, 0, len(remoteModels))
	for _, name := range remoteModels {
		refs = append(refs, domain.LLMModelRef{Name: name, Enabled: true})
	}
	return refs, nil
}

func (m *LLMConfigManager) ToggleModel(ctx context.Context, configID, modelName string, enabled bool) (domain.LLMProviderConfig, error) {
	m.ensureCache(ctx)
	m.mu.RLock()
	cfg, ok := m.cache[configID]
	m.mu.RUnlock()
	if !ok || cfg.ID == "" {
		return domain.LLMProviderConfig{}, fmt.Errorf("config not found: %s", configID)
	}

	found := false
	for i, ref := range cfg.Models {
		if ref.Name == modelName {
			cfg.Models[i].Enabled = enabled
			found = true
			break
		}
	}
	if !found {
		return domain.LLMProviderConfig{}, fmt.Errorf("model not found: %s", modelName)
	}

	now := time.Now().UTC().Format(time.RFC3339)
	cfg.UpdatedAt = now

	if err := m.store.Upsert(ctx, cfg); err != nil {
		return domain.LLMProviderConfig{}, fmt.Errorf("save config: %w", err)
	}

	m.mu.Lock()
	if m.cache != nil {
		m.cache[cfg.ID] = cfg
	}
	m.mu.Unlock()

	return cfg, nil
}

func listRemoteModels(ctx context.Context, cfg domain.LLMProviderConfig) ([]string, error) {
	switch domain.NormalizeProtocol(cfg.Provider) {
	case domain.LLMProviderMock:
		return []string{"mock-gpt-4", "mock-claude"}, nil
	case domain.LLMProviderAnthropic:
		return nil, fmt.Errorf("Anthropic API does not expose a model list endpoint; configure models manually via config.yaml")
	case domain.LLMProviderOpenAI, domain.LLMProviderOpenAIResponses:
		return listOpenAICompatibleModels(ctx, cfg.BaseURL, cfg.APIKey)
	default:
		return nil, fmt.Errorf("unsupported LLM protocol %q", cfg.Provider)
	}
}

func listOpenAICompatibleModels(ctx context.Context, baseURL, apiKey string) ([]string, error) {
	if baseURL == "" {
		baseURL = "https://api.openai.com/v1"
	}
	url := strings.TrimRight(baseURL, "/") + "/models"

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	if apiKey != "" {
		httpReq.Header.Set("Authorization", "Bearer "+apiKey)
	}

	resp, err := http.DefaultClient.Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("http %d: %s", resp.StatusCode, string(body))
	}

	var result struct {
		Data []struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("parse models response: %w", err)
	}

	var names []string
	for _, m := range result.Data {
		if m.ID != "" {
			names = append(names, m.ID)
		}
	}
	return names, nil
}
