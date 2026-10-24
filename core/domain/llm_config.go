package domain

import "encoding/json"

// LLMProviderType is the wire protocol used to talk to an LLM endpoint.
// It is NOT a vendor name — vendor presets (deepseek, qwen, …) only supply
// defaults (protocol + base URL + default protocol dialect); one protocol maps
// to one client implementation. Chat Completions protocol dialects are bound
// on the provider (DefaultProtocolDialect) and optionally overridden per model.
type LLMProviderType string

const (
	// LLMProviderOpenAI is the OpenAI Chat Completions protocol (/chat/completions).
	// Most third-party "OpenAI-compatible" endpoints use this. Existing configs keep this value.
	LLMProviderOpenAI LLMProviderType = "openai"
	// LLMProviderOpenAIResponses is the OpenAI Responses protocol (/responses).
	LLMProviderOpenAIResponses LLMProviderType = "openai_responses"
	// LLMProviderAnthropic is the Anthropic Messages protocol (/messages).
	LLMProviderAnthropic LLMProviderType = "anthropic"
	// LLMProviderMock is the in-process mock client (tests / demos).
	LLMProviderMock LLMProviderType = "mock"
	// LLMProviderLocal is a deprecated alias of LLMProviderOpenAI.
	LLMProviderLocal LLMProviderType = "local"
)

// NormalizeProtocol maps legacy / alias protocol values onto canonical ones.
// Unknown values are returned unchanged so the dispatcher can reject them.
func NormalizeProtocol(p LLMProviderType) LLMProviderType {
	switch p {
	case LLMProviderLocal:
		return LLMProviderOpenAI
	default:
		return p
	}
}

type LLMModelRef struct {
	Name    string `json:"name"`
	Enabled bool   `json:"enabled"`
	// ProtocolDialect overrides the provider DefaultProtocolDialect for this model.
	// Empty = inherit provider default (then Infer → openai).
	ProtocolDialect string `json:"protocolDialect,omitempty"`
}

// UnmarshalJSON accepts protocolDialect and the short-lived reasoningDialect alias.
func (r *LLMModelRef) UnmarshalJSON(data []byte) error {
	var raw struct {
		Name             string `json:"name"`
		Enabled          bool   `json:"enabled"`
		ProtocolDialect  string `json:"protocolDialect"`
		ReasoningDialect string `json:"reasoningDialect"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	r.Name = raw.Name
	r.Enabled = raw.Enabled
	r.ProtocolDialect = raw.ProtocolDialect
	if r.ProtocolDialect == "" {
		r.ProtocolDialect = raw.ReasoningDialect
	}
	return nil
}

type LLMProviderConfig struct {
	ID        string          `json:"id"`
	Provider  LLMProviderType `json:"provider"`
	Name      string          `json:"name"`
	APIKey    string          `json:"apiKey,omitempty"`
	BaseURL   string          `json:"baseUrl,omitempty"`
	// DefaultProtocolDialect is the Chat Completions protocol dialect for models
	// that do not set ProtocolDialect. Ignored for Anthropic / Responses.
	DefaultProtocolDialect string        `json:"defaultProtocolDialect,omitempty"`
	Models         []LLMModelRef `json:"models,omitempty"`
	CreatedAt      string        `json:"createdAt"`
	UpdatedAt      string        `json:"updatedAt"`
}

type UpsertLLMProviderConfigRequest struct {
	Provider               LLMProviderType `json:"provider"`
	Name                   string          `json:"name"`
	APIKey                 string          `json:"apiKey,omitempty"`
	BaseURL                string          `json:"baseUrl,omitempty"`
	DefaultProtocolDialect string          `json:"defaultProtocolDialect,omitempty"`
	// DefaultDialect is a legacy JSON alias for DefaultProtocolDialect.
	DefaultDialect string        `json:"defaultDialect,omitempty"`
	Models         []LLMModelRef `json:"models,omitempty"`
}

type LLMModel struct {
	ID               string   `json:"id"`
	Name             string   `json:"name"`
	ProviderID       string   `json:"providerId"`
	Provider         string   `json:"provider"`
	Enabled          bool     `json:"enabled"`
	AvailableEfforts []string `json:"availableEfforts,omitempty"`
	// Vision is true when the model accepts image content parts.
	Vision bool `json:"vision"`
	// ProtocolDialect is the resolved dialect for this provider×model
	// (model override → provider default → infer → openai).
	ProtocolDialect string `json:"protocolDialect,omitempty"`
}

// DefaultEffortsOpenAI is used when a model has no available_efforts in config.
var DefaultEffortsOpenAI = []string{"off", "low", "medium", "high", "xhigh"}

// DefaultEffortsAnthropic is used when an Anthropic model has no available_efforts in config.
var DefaultEffortsAnthropic = []string{"off", "high", "max"}

// Protocol dialect values for OpenAI-compatible Chat Completions vendors.
const (
	ProtocolDialectOpenAI   = "openai"    // reasoning_effort
	ProtocolDialectDeepSeek = "deepseek"  // thinking.type + reasoning_effort + echo
	ProtocolDialectQwen     = "qwen"      // enable_thinking (+ thinking_budget)
	ProtocolDialectKimi     = "kimi"      // K2.6/K2.5: thinking.type / thinking.keep
	ProtocolDialectKimiCode = "kimi_code" // K2.7 Code / kimi-for-coding-highspeed: always-on preserved thinking
	ProtocolDialectKimiK3   = "kimi_k3"   // K3 / K2.8 Preview (kimi-for-coding): reasoning_effort + echo
	ProtocolDialectGLM      = "glm"       // Zhipu: thinking.type (+ clear_thinking) + optional reasoning_effort
	ProtocolDialectMiniMax  = "minimax"   // reasoning_split + optional thinking.type
	ProtocolDialectGemini   = "gemini"    // reasoning_effort (incl. "none" when off)
	ProtocolDialectGrok     = "grok"      // xAI reasoning_effort
)

// ModelConfig defines per-model capacity, sampling, vision, and available
// reasoning-intensity levels. Protocol dialect lives on the provider / model
// ref — not here. All fields are optional; unset values fall back to defaults.
type ModelConfig struct {
	Model            string   `json:"model" mapstructure:"model" yaml:"model"`
	ContextWindow    int      `json:"context_window,omitempty" mapstructure:"context_window" yaml:"context_window,omitempty"`
	MaxOutput        int      `json:"max_output,omitempty" mapstructure:"max_output" yaml:"max_output,omitempty"`
	Temperature      float64  `json:"temperature,omitempty" mapstructure:"temperature" yaml:"temperature,omitempty"`
	TopP             float64  `json:"top_p,omitempty" mapstructure:"top_p" yaml:"top_p,omitempty"`
	FrequencyPenalty float64  `json:"frequency_penalty,omitempty" mapstructure:"frequency_penalty" yaml:"frequency_penalty,omitempty"`
	PresencePenalty  float64  `json:"presence_penalty,omitempty" mapstructure:"presence_penalty" yaml:"presence_penalty,omitempty"`
	Stop             []string `json:"stop,omitempty" mapstructure:"stop" yaml:"stop,omitempty"`
	AvailableEfforts []string `json:"available_efforts,omitempty" mapstructure:"available_efforts" yaml:"available_efforts,omitempty"`
	// Vision marks whether the model accepts multimodal image input.
	// When false/omitted, image parts are stripped before Chat.
	Vision bool `json:"vision,omitempty" mapstructure:"vision" yaml:"vision,omitempty"`

	// Legacy YAML-only fields — migrated to provider DefaultProtocolDialect / model
	// ProtocolDialect (and Anthropic client defaults). Stripped after migrate;
	// never exposed on the JSON API.
	LegacyProtocolDialect   string         `json:"-" mapstructure:"reasoning_dialect" yaml:"reasoning_dialect,omitempty"`
	LegacyThinkingMode       string         `json:"-" mapstructure:"thinking_mode" yaml:"thinking_mode,omitempty"`
	LegacyEffortBudgetTokens map[string]int `json:"-" mapstructure:"effort_budget_tokens" yaml:"effort_budget_tokens,omitempty"`
}

// StripLegacyModelConfigFields clears deprecated YAML-only fields so Save
// does not rewrite them.
func StripLegacyModelConfigFields(models []ModelConfig) []ModelConfig {
	for i := range models {
		models[i].LegacyProtocolDialect = ""
		models[i].LegacyThinkingMode = ""
		models[i].LegacyEffortBudgetTokens = nil
	}
	return models
}

// ModelConfigsResponse is returned by GET/PUT/POST model-configs endpoints.
// CatalogDiverged is true when an explicit reset would change the local catalog
// (newer built-in params, missing built-in models, or local edits on known models).
type ModelConfigsResponse struct {
	Models          []ModelConfig `json:"models"`
	CatalogDiverged bool          `json:"catalogDiverged"`
}

// LLMProviderPreset is a template for quickly creating a provider config.
// It ships via config.yaml or built-in defaults and is exposed to the frontend
// so users can pick a preset instead of filling every field manually.
type LLMProviderPreset struct {
	ID             string          `json:"id" mapstructure:"id" yaml:"id"`
	Name           string          `json:"name" mapstructure:"name" yaml:"name"`
	Provider       LLMProviderType `json:"provider" mapstructure:"provider" yaml:"provider"` // protocol type
	BaseURL        string          `json:"baseUrl" mapstructure:"base_url" yaml:"base_url"`
	DefaultProtocolDialect string          `json:"defaultProtocolDialect,omitempty" mapstructure:"default_protocol_dialect" yaml:"default_protocol_dialect,omitempty"`
	Icon           string          `json:"icon" mapstructure:"icon" yaml:"icon"`
	Description    string          `json:"description" mapstructure:"description" yaml:"description"`
}
