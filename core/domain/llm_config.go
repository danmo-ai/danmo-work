package domain

type LLMProviderType string

const (
	LLMProviderOpenAI    LLMProviderType = "openai"
	LLMProviderAnthropic LLMProviderType = "anthropic"
	LLMProviderLocal     LLMProviderType = "local"
	LLMProviderMock      LLMProviderType = "mock"
)

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
	// Vision marks whether the model accepts multimodal image input.
	// When false/omitted, image parts are stripped before Chat.
	Vision bool `json:"vision,omitempty" mapstructure:"vision" yaml:"vision,omitempty"`
}

// LLMProviderPreset is a template for quickly creating a provider config.
// It ships via config.yaml or built-in defaults and is exposed to the frontend
// so users can pick a preset instead of filling every field manually.
type LLMProviderPreset struct {
	ID          string          `json:"id"`          // e.g. "deepseek"
	Name        string          `json:"name"`        // display name, e.g. "DeepSeek"
	Provider    LLMProviderType `json:"provider"`    // "openai" | "anthropic" | ...
	BaseURL     string          `json:"baseUrl"`     // default API endpoint
	Icon        string          `json:"icon"`        // emoji or icon key
	Description string          `json:"description"` // short human-readable description
}
