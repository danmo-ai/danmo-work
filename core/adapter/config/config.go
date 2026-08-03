package config

import (
	"context"
	_ "embed"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"danmo-work/core/domain"
	"danmo-work/core/paths"
	"danmo-work/core/port"

	"github.com/spf13/viper"
	"gopkg.in/yaml.v3"
)

//go:embed default_models.yaml
var defaultModelsYAML []byte

var _ port.SearchConfigStore = (*Loader)(nil)
var _ port.ConfigStore = (*Loader)(nil)

// Loader reads and writes the user-editable ~/.danmo-work/config.yaml configuration.
// It is the source of truth for settings that should be readable and editable
// by all entry points (server, cli, tui, desktop). Viper is used for loading,
// defaults, and environment-variable binding; yaml.v3 is used for writing so
// that only the touched sections are persisted and other fields are preserved.
type Loader struct {
	path string
	v    *viper.Viper
	mu   sync.RWMutex
}

// NewLoader returns a config loader for the given path.
// If path is empty, it defaults to ~/.danmo-work/config.yaml.
// Relative data paths are resolved against ~/.danmo-work (not cwd).
func NewLoader(path string) *Loader {
	paths.MigrateLegacyOnce()
	if path == "" {
		path = paths.ConfigFile()
	}
	if abs, err := filepath.Abs(path); err == nil {
		path = abs
	}
	v := viper.New()
	v.SetConfigFile(path)
	v.SetConfigType("yaml")
	setDefaults(v)
	bindEnv(v)
	return &Loader{path: path, v: v}
}

func setDefaults(v *viper.Viper) {
	v.SetDefault("data.dir", paths.DataDir())
	v.SetDefault("data.database", paths.DatabaseFile())
	v.SetDefault("data.store", "sqlite")
	v.SetDefault("server.listen_addr", "0.0.0.0:7801")
	v.SetDefault("instance.id", "")
	v.SetDefault("runtime.auto_approve", false)
	v.SetDefault("runtime.sandbox.enabled", true)
	v.SetDefault("runtime.sandbox.mode", "workspace-write")
	v.SetDefault("runtime.sandbox.network", "deny")
	v.SetDefault("runtime.sandbox.backend", "")
	v.SetDefault("runtime.sandbox.shell", "auto")
	v.SetDefault("runtime.browser.enabled", true)
	v.SetDefault("runtime.browser.executable_path", "")
	v.SetDefault("runtime.browser.cdp_url", "")
	v.SetDefault("runtime.turn.doom_loop_threshold", 10)
	v.SetDefault("runtime.turn.max_steps_default", 200)
	v.SetDefault("runtime.turn.max_llm_failures", 3)
	v.SetDefault("runtime.turn.llm_http_timeout_sec", 600)
	v.SetDefault("runtime.tools.max_output_chars", 50000)
	v.SetDefault("runtime.team.max_delegation_depth", 3)
	v.SetDefault("runtime.memory.read_top_k", 10)
	v.SetDefault("runtime.table.max_rows_per_upsert", 50)
	v.SetDefault("runtime.table.max_rows_per_turn", 200)
	v.SetDefault("runtime.table.max_rows_per_table", 50000)
	v.SetDefault("runtime.table.max_row_bytes", 65536)
	v.SetDefault("runtime.table.max_tables_per_scope", 100)
	v.SetDefault("runtime.table.query_default_limit", 50)
	v.SetDefault("runtime.table.query_max_limit", 200)
	v.SetDefault("runtime.table.max_row_chars", 8000)
	v.SetDefault("runtime.knowledge.search_top_k", 3)
	v.SetDefault("runtime.knowledge.chapter_max_tokens", 512)
	v.SetDefault("runtime.knowledge.vector_hybrid", false)
	v.SetDefault("data.store_database", paths.StoreDatabaseFile())
	v.SetDefault("runtime.compaction.enabled", true)
	v.SetDefault("runtime.compaction.model", "")
	v.SetDefault("runtime.compaction.max_tokens", 128000)
	v.SetDefault("runtime.compaction.trigger_ratio", 0.85)
	v.SetDefault("runtime.compaction.cut_tokens", 16000)
	v.SetDefault("runtime.compaction.turn_interval", 6)
	v.SetDefault("runtime.compaction.sub_interval", 4)
	v.SetDefault("runtime.compaction.tool_truncate", 2000)
	v.SetDefault("runtime.compaction.keep_recent_tool_steps", 3)
	v.SetDefault("search.provider", string(domain.SearchProviderDuckDuckGo))
	v.SetDefault("search.base_url", "")
	v.SetDefault("search.api_key", "")
	v.SetDefault("search.timeout_ms", 15000)
	v.SetDefault("search.max_results", 5)
	v.SetDefault("search.proxy", "")
	v.SetDefault("search.user_agent", "")
	v.SetDefault("search.html_fallback", true)
	v.SetDefault("market.cache_ttl_hours", 6)
	v.SetDefault("channels.weixin.enabled", false)
	v.SetDefault("channels.weixin.auto_approve", true)
}

func bindEnv(v *viper.Viper) {
	_ = v.BindEnv("data.dir", "WORK_DATA_DIR")
	_ = v.BindEnv("data.database", "WORK_DB_PATH")
	_ = v.BindEnv("data.store_database", "WORK_STORE_DB_PATH")
	_ = v.BindEnv("data.store", "WORK_STORE")
	_ = v.BindEnv("server.listen_addr", "WORK_ADDR")
	_ = v.BindEnv("runtime.auto_approve", "WORK_AUTO_APPROVE")
	_ = v.BindEnv("runtime.sandbox.enabled", "WORK_SANDBOX_ENABLED")
	_ = v.BindEnv("runtime.sandbox.mode", "WORK_SANDBOX_MODE")
	_ = v.BindEnv("runtime.sandbox.network", "WORK_SANDBOX_NETWORK")
	_ = v.BindEnv("runtime.sandbox.backend", "WORK_SANDBOX_BACKEND")
	_ = v.BindEnv("runtime.sandbox.shell", "WORK_SANDBOX_SHELL")
	_ = v.BindEnv("runtime.browser.enabled", "WORK_BROWSER_ENABLED")
	_ = v.BindEnv("runtime.browser.executable_path", "WORK_BROWSER_EXECUTABLE")
	_ = v.BindEnv("runtime.browser.cdp_url", "WORK_BROWSER_CDP_URL")
	_ = v.BindEnv("instance.id", "WORK_INSTANCE_ID")
}

// Path returns the resolved config file path.
func (l *Loader) Path() string { return l.path }

// Load reads the configuration file (if it exists), applies defaults and
// environment-variable overrides, and returns the resolved config.
func (l *Loader) Load(_ context.Context) (*domain.ConfigFile, error) {
	l.mu.RLock()
	defer l.mu.RUnlock()

	if err := l.v.ReadInConfig(); err != nil {
		// Ignore "file not found" so that defaults + env vars still work.
		if !isConfigNotFound(err) {
			return nil, err
		}
	}

	var cfg domain.ConfigFile
	if err := l.v.Unmarshal(&cfg); err != nil {
		return nil, err
	}

	if cfg.Data.Dir == "" {
		cfg.Data.Dir = paths.DataDir()
	}
	if !filepath.IsAbs(cfg.Data.Dir) {
		cfg.Data.Dir = paths.ResolveAgainstHome(cfg.Data.Dir)
	}
	if cfg.Data.Database == "" {
		cfg.Data.Database = paths.DatabaseFile()
	}
	if !filepath.IsAbs(cfg.Data.Database) {
		cfg.Data.Database = paths.ResolveAgainstHome(cfg.Data.Database)
	}
	if cfg.Data.StoreDatabase == "" {
		cfg.Data.StoreDatabase = paths.StoreDatabaseFile()
	}
	if !filepath.IsAbs(cfg.Data.StoreDatabase) {
		cfg.Data.StoreDatabase = paths.ResolveAgainstHome(cfg.Data.StoreDatabase)
	}
	if cfg.Runtime.Table == (domain.ConfigTableSection{}) {
		cfg.Runtime.Table = domain.DefaultTableSection()
	}

	if cfg.Search.Provider == "" {
		cfg.Search.Provider = domain.SearchProviderDuckDuckGo
	}

	// Merge built-in LLM presets (fill empty fields; append missing ids).
	cfg.LLM.Providers = mergeLLMPresets(cfg.LLM.Providers, defaultLLMPresets(), legacyPresetBaseURLs(l.v))
	// Migrate legacy llm.model_limits → llm.models (renamed in 0.9.x).
	// Without this, Load returns an empty catalog and the next Save rewrites
	// the llm section, permanently wiping the old key.
	if len(cfg.LLM.Models) == 0 {
		if migrated := migrateLegacyModelLimits(l.v); len(migrated) > 0 {
			cfg.LLM.Models = migrated
		}
	}
	// Append new built-in entries and fill empty reasoning_dialect / zero limits.
	cfg.LLM.Models = mergeModelConfigs(cfg.LLM.Models, defaultModelConfigs())

	if cfg.Market.CacheTTLHours <= 0 {
		cfg.Market.CacheTTLHours = 6
	}
	if len(cfg.Market.Sources) == 0 {
		cfg.Market.Sources = defaultMarketSources()
	}
	// Migrate legacy runtime.memory.recall_top_k → read_top_k.
	if cfg.Runtime.Memory.ReadTopK <= 0 {
		if legacy := l.v.GetInt("runtime.memory.recall_top_k"); legacy > 0 {
			cfg.Runtime.Memory.ReadTopK = legacy
		} else {
			cfg.Runtime.Memory.ReadTopK = 10
		}
	}
	return &cfg, nil
}

// migrateLegacyModelLimits reads pre-rename llm.model_limits entries.
func migrateLegacyModelLimits(v *viper.Viper) []domain.ModelConfig {
	if v == nil || !v.IsSet("llm.model_limits") {
		return nil
	}
	raw := v.Get("llm.model_limits")
	if raw == nil {
		return nil
	}
	b, err := yaml.Marshal(raw)
	if err != nil {
		return nil
	}
	var models []domain.ModelConfig
	if err := yaml.Unmarshal(b, &models); err != nil {
		return nil
	}
	out := make([]domain.ModelConfig, 0, len(models))
	for _, m := range models {
		if m.Model == "" {
			continue
		}
		out = append(out, m)
	}
	return out
}

// DefaultModelConfigs returns the built-in per-model parameter catalog.
func DefaultModelConfigs() []domain.ModelConfig {
	return defaultModelConfigs()
}

// MergeModelConfigs fills empty catalog fields and appends missing built-in models.
func MergeModelConfigs(existing, defaults []domain.ModelConfig) []domain.ModelConfig {
	return mergeModelConfigs(existing, defaults)
}

// RefreshModelConfigs overlays built-in dialect/efforts/limits onto matching models
// and appends any missing built-in models. Custom-only entries are preserved.
func RefreshModelConfigs(existing, defaults []domain.ModelConfig) []domain.ModelConfig {
	return refreshModelConfigs(existing, defaults)
}

// defaultModelConfigs returns the built-in per-model parameter catalog.
// Pattern-based fallbacks were removed; this YAML (kept in sync with
// config.example.yaml) is the source of truth when the user has no models.
func defaultModelConfigs() []domain.ModelConfig {
	var models []domain.ModelConfig
	if err := yaml.Unmarshal(defaultModelsYAML, &models); err != nil {
		return nil
	}
	return models
}

// defaultMarketSources returns built-in official Git market sources.
// Users can disable, reorder, or add entries in ~/.danmo-work/config.yaml.
func defaultMarketSources() []domain.MarketSource {
	return []domain.MarketSource{
		{
			ID:          "official-github",
			Name:        "Official (GitHub)",
			Kind:        "git",
			Platform:    "github",
			Repo:        "https://github.com/danmo-ai/dq-market",
			Ref:         "main",
			CatalogPath: "catalog/index.json",
			Enabled:     true,
			Priority:    10,
		},
		{
			ID:          "official-gitee",
			Name:        "Official (Gitee)",
			Kind:        "git",
			Platform:    "gitee",
			Repo:        "https://gitee.com/danmo-ai/dq-market",
			Ref:         "main",
			CatalogPath: "catalog/index.json",
			Enabled:     false,
			Priority:    20,
		},
		{
			ID:       "clawhub",
			Name:     "ClawHub",
			Kind:     "clawhub",
			Repo:     "https://clawhub.ai",
			Enabled:  false,
			Priority: 30,
		},
		{
			ID:       "techleads",
			Name:     "Tech Leads Club",
			Kind:     "techleads",
			Repo:     "@tech-leads-club/skills-catalog",
			Ref:      "latest",
			Enabled:  true,
			Priority: 25,
		},
	}
}

// defaultLLMPresets returns the built-in provider presets for mainstream
// model vendors. Users can override these in ~/.danmo-work/config.yaml.
func defaultLLMPresets() []domain.LLMProviderPreset {
	return []domain.LLMProviderPreset{
		{ID: "openai", Name: "OpenAI", Provider: domain.LLMProviderOpenAI, BaseURL: "https://api.openai.com/v1", Description: "GPT 系列、o 系列推理模型"},
		{ID: "anthropic", Name: "Anthropic", Provider: domain.LLMProviderAnthropic, BaseURL: "https://api.anthropic.com/v1", Description: "Claude Sonnet、Opus、Haiku"},
		{ID: "deepseek", Name: "DeepSeek", Provider: domain.LLMProviderOpenAI, BaseURL: "https://api.deepseek.com/v1", Description: "DeepSeek 系列"},
		{ID: "google", Name: "Google Gemini", Provider: domain.LLMProviderOpenAI, BaseURL: "https://generativelanguage.googleapis.com/v1beta/openai", Description: "Gemini Pro、Flash"},
		{ID: "zhipu", Name: "智谱 (Zhipu)", Provider: domain.LLMProviderOpenAI, BaseURL: "https://open.bigmodel.cn/api/paas/v4", Description: "GLM-5.1、GLM-5、GLM-4.7"},
		{ID: "qwen", Name: "通义千问 (Qwen)", Provider: domain.LLMProviderOpenAI, BaseURL: "https://dashscope.aliyuncs.com/compatible-mode/v1", Description: "Qwen3.7 Max、Plus、Flash、Coder"},
		{ID: "moonshot", Name: "Moonshot (Kimi)", Provider: domain.LLMProviderOpenAI, BaseURL: "https://api.kimi.com/coding/v1", Description: "Kimi Code：k3、k3-256k、kimi-for-coding"},
		{ID: "minimax", Name: "MiniMax", Provider: domain.LLMProviderOpenAI, BaseURL: "https://api.minimaxi.com/v1", Description: "MiniMax M3、M2.7"},
		{ID: "ollama", Name: "Ollama (Local)", Provider: domain.LLMProviderOpenAI, BaseURL: "http://localhost:11434/v1", Description: "本地模型，通过 Ollama 运行"},
		{ID: "siliconflow", Name: "SiliconFlow", Provider: domain.LLMProviderOpenAI, BaseURL: "https://api.siliconflow.cn/v1", Description: "SiliconFlow 云模型平台"},
		{ID: "openrouter", Name: "OpenRouter", Provider: domain.LLMProviderOpenAI, BaseURL: "https://openrouter.ai/api/v1", Description: "多模型路由，统一接口"},
		{ID: "together", Name: "Together AI", Provider: domain.LLMProviderOpenAI, BaseURL: "https://api.together.xyz/v1", Description: "开源模型推理平台"},
		{ID: "fireworks", Name: "Fireworks AI", Provider: domain.LLMProviderOpenAI, BaseURL: "https://api.fireworks.ai/inference/v1", Description: "高性能推理服务"},
		{ID: "groq", Name: "Groq", Provider: domain.LLMProviderOpenAI, BaseURL: "https://api.groq.com/openai/v1", Description: "超快推理速度"},
		{ID: "deepinfra", Name: "DeepInfra", Provider: domain.LLMProviderOpenAI, BaseURL: "https://api.deepinfra.com/v1/openai", Description: "开源模型部署平台"},
		{ID: "xai", Name: "xAI", Provider: domain.LLMProviderOpenAI, BaseURL: "https://api.x.ai/v1", Description: "Grok 系列模型"},
	}
}

// legacyPresetBaseURLs recovers BaseURL values stored under the broken yaml
// key "baseurl" (written before LLMProviderPreset had yaml:"base_url" tags).
func legacyPresetBaseURLs(v *viper.Viper) map[string]string {
	out := map[string]string{}
	if v == nil || !v.IsSet("llm.providers") {
		return out
	}
	raw, ok := v.Get("llm.providers").([]any)
	if !ok {
		return out
	}
	for _, item := range raw {
		m, ok := item.(map[string]any)
		if !ok {
			continue
		}
		id, _ := m["id"].(string)
		if id == "" {
			continue
		}
		if u, ok := m["base_url"].(string); ok && strings.TrimSpace(u) != "" {
			out[id] = strings.TrimSpace(u)
			continue
		}
		if u, ok := m["baseurl"].(string); ok && strings.TrimSpace(u) != "" {
			out[id] = strings.TrimSpace(u)
		}
	}
	return out
}

// mergeLLMPresets fills empty fields on existing presets from defaults, appends
// any built-in presets missing from the user list, and applies legacyBaseURLs
// when BaseURL is still empty after merge.
func mergeLLMPresets(existing, defaults []domain.LLMProviderPreset, legacyBaseURLs map[string]string) []domain.LLMProviderPreset {
	if len(existing) == 0 {
		out := make([]domain.LLMProviderPreset, len(defaults))
		copy(out, defaults)
		return out
	}
	byID := make(map[string]domain.LLMProviderPreset, len(defaults))
	for _, d := range defaults {
		byID[d.ID] = d
	}
	seen := make(map[string]bool, len(existing))
	out := make([]domain.LLMProviderPreset, 0, len(existing)+len(defaults))
	for _, p := range existing {
		if def, ok := byID[p.ID]; ok {
			if p.Name == "" {
				p.Name = def.Name
			}
			if p.Provider == "" {
				p.Provider = def.Provider
			}
			if strings.TrimSpace(p.BaseURL) == "" {
				p.BaseURL = def.BaseURL
			}
			if p.Icon == "" {
				p.Icon = def.Icon
			}
			if p.Description == "" {
				p.Description = def.Description
			}
		}
		if strings.TrimSpace(p.BaseURL) == "" {
			if u := legacyBaseURLs[p.ID]; u != "" {
				p.BaseURL = u
			}
		}
		if p.Provider != "" {
			p.Provider = domain.NormalizeProtocol(p.Provider)
		}
		out = append(out, p)
		if p.ID != "" {
			seen[p.ID] = true
		}
	}
	for _, d := range defaults {
		if seen[d.ID] {
			continue
		}
		out = append(out, d)
	}
	return out
}

// mergeModelConfigs fills empty reasoning_dialect (and related zero fields) from
// the built-in catalog, and appends any default models missing from the user list.
func mergeModelConfigs(existing, defaults []domain.ModelConfig) []domain.ModelConfig {
	if len(defaults) == 0 {
		return existing
	}
	byModel := make(map[string]domain.ModelConfig, len(defaults))
	for _, d := range defaults {
		if d.Model == "" {
			continue
		}
		byModel[d.Model] = d
	}
	seen := make(map[string]bool, len(existing))
	out := make([]domain.ModelConfig, 0, len(existing)+len(defaults))
	for _, m := range existing {
		if def, ok := byModel[m.Model]; ok {
			if m.ReasoningDialect == "" && def.ReasoningDialect != "" {
				m.ReasoningDialect = def.ReasoningDialect
			}
			if m.ContextWindow == 0 && def.ContextWindow > 0 {
				m.ContextWindow = def.ContextWindow
			}
			if m.MaxOutput == 0 && def.MaxOutput > 0 {
				m.MaxOutput = def.MaxOutput
			}
			if len(m.AvailableEfforts) == 0 && len(def.AvailableEfforts) > 0 {
				m.AvailableEfforts = append([]string(nil), def.AvailableEfforts...)
			}
			if m.ThinkingMode == "" && def.ThinkingMode != "" {
				m.ThinkingMode = def.ThinkingMode
			}
			if len(m.EffortBudgetTokens) == 0 && len(def.EffortBudgetTokens) > 0 {
				m.EffortBudgetTokens = def.EffortBudgetTokens
			}
		}
		out = append(out, m)
		if m.Model != "" {
			seen[m.Model] = true
		}
	}
	for _, d := range defaults {
		if d.Model == "" || seen[d.Model] {
			continue
		}
		out = append(out, d)
	}
	return out
}

// refreshModelConfigs overlays built-in dialect/efforts/limits onto matching models
// (user-only entries kept), then appends any missing built-in models.
func refreshModelConfigs(existing, defaults []domain.ModelConfig) []domain.ModelConfig {
	if len(defaults) == 0 {
		return existing
	}
	byModel := make(map[string]domain.ModelConfig, len(defaults))
	for _, d := range defaults {
		if d.Model != "" {
			byModel[d.Model] = d
		}
	}
	seen := make(map[string]bool, len(existing))
	out := make([]domain.ModelConfig, 0, len(existing)+len(defaults))
	for _, m := range existing {
		if def, ok := byModel[m.Model]; ok {
			if def.ReasoningDialect != "" {
				m.ReasoningDialect = def.ReasoningDialect
			}
			if def.ContextWindow > 0 {
				m.ContextWindow = def.ContextWindow
			}
			if def.MaxOutput > 0 {
				m.MaxOutput = def.MaxOutput
			}
			if len(def.AvailableEfforts) > 0 {
				m.AvailableEfforts = append([]string(nil), def.AvailableEfforts...)
			}
			if def.ThinkingMode != "" {
				m.ThinkingMode = def.ThinkingMode
			}
			if len(def.EffortBudgetTokens) > 0 {
				m.EffortBudgetTokens = def.EffortBudgetTokens
			}
			m.Vision = def.Vision
			if def.Temperature != 0 {
				m.Temperature = def.Temperature
			}
		}
		out = append(out, m)
		if m.Model != "" {
			seen[m.Model] = true
		}
	}
	for _, d := range defaults {
		if d.Model == "" || seen[d.Model] {
			continue
		}
		out = append(out, d)
	}
	return out
}

// Save writes the full configuration back to the YAML file.
func (l *Loader) Save(_ context.Context, cfg *domain.ConfigFile) error {
	l.mu.Lock()
	defer l.mu.Unlock()

	if cfg == nil {
		return nil
	}

	root := map[string]any{}
	if data, err := os.ReadFile(l.path); err == nil {
		_ = yaml.Unmarshal(data, &root)
	} else if !os.IsNotExist(err) {
		return err
	}

	root["data"] = cfg.Data
	root["server"] = cfg.Server
	root["instance"] = cfg.Instance
	root["runtime"] = cfg.Runtime
	root["search"] = cfg.Search
	root["llm"] = cfg.LLM
	root["market"] = cfg.Market
	root["channels"] = cfg.Channels

	if err := os.MkdirAll(filepath.Dir(l.path), 0755); err != nil {
		return err
	}
	out, err := yaml.Marshal(&root)
	if err != nil {
		return err
	}
	return os.WriteFile(l.path, out, 0600)
}

// Get returns the search configuration for the app manager.
func (l *Loader) Get(ctx context.Context) (domain.SearchConfig, error) {
	cfg, err := l.Load(ctx)
	if err != nil {
		return domain.SearchConfig{}, err
	}
	return cfg.Search, nil
}

// Upsert persists the search configuration to the YAML file, preserving all
// other top-level keys.
func (l *Loader) Upsert(ctx context.Context, cfg domain.SearchConfig) error {
	l.mu.Lock()
	defer l.mu.Unlock()

	root := map[string]any{}
	if data, err := os.ReadFile(l.path); err == nil {
		_ = yaml.Unmarshal(data, &root)
	} else if !os.IsNotExist(err) {
		return err
	}

	search, _ := root["search"].(map[string]any)
	if search == nil {
		search = map[string]any{}
		root["search"] = search
	}

	search["provider"] = string(cfg.Provider)
	if cfg.BaseURL != "" {
		search["base_url"] = cfg.BaseURL
	} else {
		delete(search, "base_url")
	}
	if cfg.APIKey != "" {
		search["api_key"] = cfg.APIKey
	} else {
		delete(search, "api_key")
	}
	search["timeout_ms"] = cfg.TimeoutMs
	search["max_results"] = cfg.MaxResults
	if cfg.Proxy != "" {
		search["proxy"] = cfg.Proxy
	} else {
		delete(search, "proxy")
	}
	if cfg.UserAgent != "" {
		search["user_agent"] = cfg.UserAgent
	} else {
		delete(search, "user_agent")
	}
	if cfg.HTMLFallback != nil {
		search["html_fallback"] = *cfg.HTMLFallback
	} else {
		search["html_fallback"] = true
	}

	if err := os.MkdirAll(filepath.Dir(l.path), 0755); err != nil {
		return err
	}
	out, err := yaml.Marshal(&root)
	if err != nil {
		return err
	}
	return os.WriteFile(l.path, out, 0600)
}

func isConfigNotFound(err error) bool {
	if err == nil {
		return true
	}
	if _, ok := err.(viper.ConfigFileNotFoundError); ok {
		return true
	}
	return os.IsNotExist(err)
}
