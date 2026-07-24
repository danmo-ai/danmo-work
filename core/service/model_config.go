package service

import (
	"context"
	"strings"
	"sync"

	"danmo-work/core/domain"
	"danmo-work/core/port"
)

// ModelConfigRegistry provides per-model lookups for context window, max output
// tokens, and generation parameters — all sourced from the YAML config file.
// When a model is not found in config, hardcoded defaults are used as fallback.
type ModelConfigRegistry struct {
	mu      sync.RWMutex
	config  []domain.ModelConfig
	byModel map[string]domain.ModelConfig // lazy index
}

// NewModelConfigRegistry creates an empty registry.
func NewModelConfigRegistry() *ModelConfigRegistry {
	return &ModelConfigRegistry{}
}

// SetModels replaces the config-based model entries.
func (r *ModelConfigRegistry) SetModels(models []domain.ModelConfig) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.config = models
	r.byModel = nil // invalidate index
}

func (r *ModelConfigRegistry) ensureIndex() {
	if r.byModel != nil {
		return
	}
	r.byModel = make(map[string]domain.ModelConfig, len(r.config))
	for _, l := range r.config {
		r.byModel[strings.ToLower(l.Model)] = l
	}
}

// lookup returns the config entry for a model ID, or nil if not found.
// Tries full modelID first (e.g. "anthropic/claude-sonnet-5"), then model name only.
func (r *ModelConfigRegistry) lookup(modelID string) *domain.ModelConfig {
	_, model := splitModelID(modelID)
	if model == "" {
		return nil
	}
	r.mu.RLock()
	r.ensureIndex()
	defer r.mu.RUnlock()
	key := strings.ToLower(modelID)
	if c, ok := r.byModel[key]; ok {
		return &c
	}
	if c, ok := r.byModel[strings.ToLower(model)]; ok {
		return &c
	}
	return nil
}

// AvailableEfforts returns the reasoning effort levels for a model.
// Returns nil when no efforts are configured.
func (r *ModelConfigRegistry) AvailableEfforts(modelID string) []string {
	if c := r.lookup(modelID); c != nil && len(c.AvailableEfforts) > 0 {
		return c.AvailableEfforts
	}
	return nil
}

// SupportsVision reports whether the model accepts image content parts.
// Default is false when the model is missing from config or vision is unset.
func (r *ModelConfigRegistry) SupportsVision(modelID string) bool {
	if c := r.lookup(modelID); c != nil {
		return c.Vision
	}
	return false
}

// ContextWindow returns the context window size for a given model ID.
// Lookup order: config exact match → default constant (128K).
func (r *ModelConfigRegistry) ContextWindow(modelID string) int {
	if c := r.lookup(modelID); c != nil && c.ContextWindow > 0 {
		return c.ContextWindow
	}
	return defaultModelContextWindow
}

// MaxOutputTokens returns the maximum output tokens for a given model.
// Lookup order: config exact match → default constant (8K).
func (r *ModelConfigRegistry) MaxOutputTokens(modelID string) int {
	if c := r.lookup(modelID); c != nil && c.MaxOutput > 0 {
		return c.MaxOutput
	}
	return defaultMaxOutputTokens
}

// GenParams returns generation parameter overrides for a given model.
// Lookup order: config exact match → nil (provider default).
func (r *ModelConfigRegistry) GenParams(modelID string) *port.ModelGenParams {
	c := r.lookup(modelID)
	if c == nil {
		return nil
	}
	return modelConfigToGenParams(*c)
}

func modelConfigToGenParams(c domain.ModelConfig) *port.ModelGenParams {
	p := &port.ModelGenParams{
		MaxTokens:          c.MaxOutput,
		Temperature:        c.Temperature,
		TopP:               c.TopP,
		FrequencyPenalty:   c.FrequencyPenalty,
		PresencePenalty:    c.PresencePenalty,
		Stop:               c.Stop,
		ThinkingMode:       c.ThinkingMode,
		EffortBudgetTokens: c.EffortBudgetTokens,
	}
	if p.MaxTokens == 0 && p.Temperature == 0 && p.TopP == 0 &&
		p.FrequencyPenalty == 0 && p.PresencePenalty == 0 &&
		len(p.Stop) == 0 && p.ThinkingMode == "" && len(p.EffortBudgetTokens) == 0 {
		return nil
	}
	return p
}

// AllModels returns the current config-based model configs.
func (r *ModelConfigRegistry) AllModels() []domain.ModelConfig {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]domain.ModelConfig, len(r.config))
	copy(out, r.config)
	return out
}

// LoadFromConfig reads model configs from the config store.
func (r *ModelConfigRegistry) LoadFromConfig(ctx context.Context, store port.ConfigStore) {
	if store == nil {
		return
	}
	cfg, err := store.Load(ctx)
	if err != nil {
		return
	}
	r.SetModels(cfg.LLM.Models)
}

func splitModelID(modelID string) (provider, model string) {
	parts := strings.SplitN(modelID, "/", 3)
	if len(parts) >= 2 {
		return parts[0], parts[1]
	}
	return "", modelID
}

const (
	defaultModelContextWindow = 128_000
	defaultMaxOutputTokens    = 8_192
)
