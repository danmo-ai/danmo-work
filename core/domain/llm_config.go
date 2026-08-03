package domain

// LLMProviderType is the wire protocol used to talk to an LLM endpoint.
// It is NOT a vendor name — vendor presets (deepseek, qwen, …) only supply
// defaults (protocol + base URL); one protocol maps to one client implementation.
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
}

type LLMProviderConfig struct {
	ID        string          `json:"id"`
	Provider  LLMProviderType `json:"provider"`
	Name      string          `json:"name"`
	APIKey    string          `json:"apiKey,omitempty"`
	BaseURL   string          `json:"baseUrl,omitempty"`
	Models    []LLMModelRef   `json:"models,omitempty"`
	CreatedAt string          `json:"createdAt"`
	UpdatedAt string          `json:"updatedAt"`
}

type UpsertLLMProviderConfigRequest struct {
	Provider LLMProviderType `json:"provider"`
	Name     string          `json:"name"`
	APIKey   string          `json:"apiKey,omitempty"`
	BaseURL  string          `json:"baseUrl,omitempty"`
	Models   []LLMModelRef   `json:"models,omitempty"`
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
}

// DefaultEffortsOpenAI is used when a model has no available_efforts in config.
var DefaultEffortsOpenAI = []string{"off", "low", "medium", "high", "xhigh"}

// DefaultEffortsAnthropic is used when an Anthropic model has no available_efforts in config.
var DefaultEffortsAnthropic = []string{"off", "high", "max"}

// Reasoning dialect values for OpenAI-compatible Chat Completions vendors.
const (
	ReasoningDialectOpenAI   = "openai"    // reasoning_effort
	ReasoningDialectDeepSeek = "deepseek"  // thinking.type + reasoning_effort + echo
	ReasoningDialectQwen     = "qwen"      // enable_thinking (+ thinking_budget)
	ReasoningDialectKimi     = "kimi"      // K2.6/K2.5: thinking.type / thinking.keep
	ReasoningDialectKimiCode = "kimi_code" // K2.7 Code / kimi-for-coding: always-on preserved thinking
	ReasoningDialectKimiK3   = "kimi_k3"   // K3: always-on via reasoning_effort + echo
	ReasoningDialectGLM      = "glm"       // Zhipu: thinking.type (+ clear_thinking) + optional reasoning_effort
	ReasoningDialectMiniMax  = "minimax"   // reasoning_split + optional thinking.type
	ReasoningDialectGemini   = "gemini"    // reasoning_effort (incl. "none" when off)
	ReasoningDialectGrok     = "grok"      // xAI reasoning_effort
)

// ModelConfig defines per-model configuration including context window, max
// output tokens, and generation parameter overrides. All fields are optional;
// unset values fall back to built-in pattern rules.
type ModelConfig struct {
	Model            string   `json:"model" mapstructure:"model" yaml:"model"`
	ContextWindow    int      `json:"context_window,omitempty" mapstructure:"context_window" yaml:"context_window,omitempty"`
	MaxOutput        int      `json:"max_output,omitempty" mapstructure:"max_output" yaml:"max_output,omitempty"`
	Temperature      float64  `json:"temperature,omitempty" mapstructure:"temperature" yaml:"temperature,omitempty"`
	TopP             float64  `json:"top_p,omitempty" mapstructure:"top_p" yaml:"top_p,omitempty"`
	FrequencyPenalty float64  `json:"frequency_penalty,omitempty" mapstructure:"frequency_penalty" yaml:"frequency_penalty,omitempty"`
	PresencePenalty  float64  `json:"presence_penalty,omitempty" mapstructure:"presence_penalty" yaml:"presence_penalty,omitempty"`
	Stop             []string `json:"stop,omitempty" mapstructure:"stop" yaml:"stop,omitempty"`
	AvailableEfforts   []string       `json:"available_efforts,omitempty" mapstructure:"available_efforts" yaml:"available_efforts,omitempty"`
	ThinkingMode       string         `json:"thinking_mode,omitempty" mapstructure:"thinking_mode" yaml:"thinking_mode,omitempty"`
	EffortBudgetTokens map[string]int `json:"effort_budget_tokens,omitempty" mapstructure:"effort_budget_tokens" yaml:"effort_budget_tokens,omitempty"`
	// ReasoningDialect selects Chat Completions thinking request/response shape
	// for OpenAI-compatible vendors (see ReasoningDialectInfos / REASONING_DIALECTS.md).
	// Empty → inferred from model id when possible, else openai.
	ReasoningDialect string `json:"reasoning_dialect,omitempty" mapstructure:"reasoning_dialect" yaml:"reasoning_dialect,omitempty"`
	// Vision marks whether the model accepts multimodal image input.
	// When false/omitted, image parts are stripped before Chat.
	Vision bool `json:"vision,omitempty" mapstructure:"vision" yaml:"vision,omitempty"`
}

// LLMProviderPreset is a template for quickly creating a provider config.
// It ships via config.yaml or built-in defaults and is exposed to the frontend
// so users can pick a preset instead of filling every field manually.
type LLMProviderPreset struct {
	ID          string          `json:"id" mapstructure:"id" yaml:"id"`
	Name        string          `json:"name" mapstructure:"name" yaml:"name"`
	Provider    LLMProviderType `json:"provider" mapstructure:"provider" yaml:"provider"` // protocol type
	BaseURL     string          `json:"baseUrl" mapstructure:"base_url" yaml:"base_url"`
	Icon        string          `json:"icon" mapstructure:"icon" yaml:"icon"`
	Description string          `json:"description" mapstructure:"description" yaml:"description"`
}
