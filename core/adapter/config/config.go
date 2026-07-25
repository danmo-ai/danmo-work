package config

import (
	"context"
	"os"
	"path/filepath"
	"sync"

	"danmo-work/core/domain"
	"danmo-work/core/paths"
	"danmo-work/core/port"

	"github.com/spf13/viper"
	"gopkg.in/yaml.v3"
)

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
	v.SetDefault("runtime.tools.max_output_chars", 50000)
	v.SetDefault("runtime.team.max_delegation_depth", 3)
	v.SetDefault("runtime.memory.read_top_k", 10)
	v.SetDefault("runtime.knowledge.search_top_k", 3)
	v.SetDefault("runtime.compaction.enabled", true)
	v.SetDefault("runtime.compaction.model", "")
	v.SetDefault("runtime.compaction.max_tokens", 128000)
	v.SetDefault("runtime.compaction.trigger_ratio", 0.85)
	v.SetDefault("runtime.compaction.cut_tokens", 16000)
	v.SetDefault("runtime.compaction.turn_interval", 6)
	v.SetDefault("runtime.compaction.sub_interval", 4)
	v.SetDefault("runtime.compaction.tool_truncate", 2000)
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

	if cfg.Search.Provider == "" {
		cfg.Search.Provider = domain.SearchProviderDuckDuckGo
	}

	// Fill in default LLM presets if none are configured.
	if len(cfg.LLM.Providers) == 0 {
		cfg.LLM.Providers = defaultLLMPresets()
	}
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
			Repo:        "danqing/dq-market",
			Ref:         "main",
			CatalogPath: "catalog/index.json",
			Enabled:     false,
			Priority:    20,
		},
	}
}

// defaultLLMPresets returns the built-in provider presets for mainstream
// model vendors. Users can override these in ~/.danmo-work/config.yaml.
func defaultLLMPresets() []domain.LLMProviderPreset {
	return []domain.LLMProviderPreset{
		{ID: "openai", Name: "OpenAI", Provider: "openai", BaseURL: "https://api.openai.com/v1", Icon: "🟢", Description: "GPT 系列、o 系列推理模型"},
		{ID: "anthropic", Name: "Anthropic", Provider: "anthropic", BaseURL: "https://api.anthropic.com", Icon: "🟠", Description: "Claude Sonnet、Opus、Haiku"},
		{ID: "deepseek", Name: "DeepSeek", Provider: "openai", BaseURL: "https://api.deepseek.com", Icon: "🔵", Description: "DeepSeek 系列"},
		{ID: "google", Name: "Google Gemini", Provider: "openai", BaseURL: "https://generativelanguage.googleapis.com/v1beta/openai", Icon: "🔷", Description: "Gemini Pro、Flash"},
		{ID: "zhipu", Name: "智谱 (Zhipu)", Provider: "openai", BaseURL: "https://open.bigmodel.cn/api/paas/v4", Icon: "🟣", Description: "GLM-5.1、GLM-5、GLM-4.7"},
		{ID: "qwen", Name: "通义千问 (Qwen)", Provider: "openai", BaseURL: "https://dashscope.aliyuncs.com/compatible-mode/v1", Icon: "🟡", Description: "Qwen3.7 Max、Plus、Flash、Coder"},
		{ID: "moonshot", Name: "Moonshot (Kimi)", Provider: "openai", BaseURL: "https://api.kimi.com/coding/v1", Icon: "🌙", Description: "Kimi Code：k3、kimi-for-coding"},
		{ID: "minimax", Name: "MiniMax", Provider: "openai", BaseURL: "https://api.minimaxi.com/v1", Icon: "🎭", Description: "MiniMax M3、M2.7"},
		{ID: "ollama", Name: "Ollama (Local)", Provider: "openai", BaseURL: "http://localhost:11434/v1", Icon: "🦙", Description: "本地模型，通过 Ollama 运行"},
		{ID: "siliconflow", Name: "SiliconFlow", Provider: "openai", BaseURL: "https://api.siliconflow.cn/v1", Icon: "🌊", Description: "SiliconFlow 云模型平台"},
		{ID: "openrouter", Name: "OpenRouter", Provider: "openai", BaseURL: "https://openrouter.ai/api/v1", Icon: "🔀", Description: "多模型路由，统一接口"},
		{ID: "together", Name: "Together AI", Provider: "openai", BaseURL: "https://api.together.xyz/v1", Icon: "🤝", Description: "开源模型推理平台"},
		{ID: "fireworks", Name: "Fireworks AI", Provider: "openai", BaseURL: "https://api.fireworks.ai/inference/v1", Icon: "🎆", Description: "高性能推理服务"},
		{ID: "groq", Name: "Groq", Provider: "openai", BaseURL: "https://api.groq.com/openai/v1", Icon: "⚡", Description: "超快推理速度"},
		{ID: "deepinfra", Name: "DeepInfra", Provider: "openai", BaseURL: "https://api.deepinfra.com/v1/openai", Icon: "🏗️", Description: "开源模型部署平台"},
		{ID: "xai", Name: "xAI", Provider: "openai", BaseURL: "https://api.x.ai/v1", Icon: "✖", Description: "Grok 系列模型"},
	}
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
