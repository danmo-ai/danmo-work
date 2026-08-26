package domain

// ConfigFile is the full user-editable configuration that is persisted to
// ~/.danmo-work/config.yaml. It mirrors the file layout so the API can expose and
// update sections independently.
type ConfigFile struct {
	Data     ConfigDataSection     `json:"data" mapstructure:"data"`
	Server   ConfigServerSection   `json:"server" mapstructure:"server"`
	Instance ConfigInstanceSection `json:"instance" mapstructure:"instance"`
	Runtime  ConfigRuntimeSection  `json:"runtime" mapstructure:"runtime"`
	Search   SearchConfig          `json:"search" mapstructure:"search"`
	LLM      ConfigLLMSection      `json:"llm" mapstructure:"llm"`
	Market   ConfigMarketSection   `json:"market" mapstructure:"market"`
	Channels ConfigChannelsSection `json:"channels" mapstructure:"channels"`
	Remote   ConfigRemoteSection   `json:"remote" mapstructure:"remote"`
}

// ConfigRemoteSection enables the PC → danmo-hub outbound connector.
type ConfigRemoteSection struct {
	Enabled     bool   `json:"enabled" mapstructure:"enabled"`
	HubURL      string `json:"hubUrl" mapstructure:"hub_url"`
	LocalBase   string `json:"localBase" mapstructure:"local_base"`
	TLSInsecure bool   `json:"tlsInsecure" mapstructure:"tls_insecure"`
}

type ConfigLLMSection struct {
	Providers []LLMProviderPreset `json:"providers" mapstructure:"providers" yaml:"providers"`
	Models    []ModelConfig       `json:"models,omitempty" mapstructure:"models" yaml:"models,omitempty"`
}

type ConfigDataSection struct {
	Dir           string `json:"dir" mapstructure:"dir"`
	Database      string `json:"database" mapstructure:"database"`
	StoreDatabase string `json:"storeDatabase" mapstructure:"store_database"`
	// HistoryDatabase holds the history plane (turn_log_entries +
	// stream_events). Empty = derived: <dir of database>/history.db.
	HistoryDatabase string `json:"historyDatabase" mapstructure:"history_database"`
	Store           string `json:"store" mapstructure:"store"`
}

type ConfigServerSection struct {
	ListenAddr string `json:"listenAddr" mapstructure:"listen_addr"`
}

type ConfigInstanceSection struct {
	ID string `json:"id" mapstructure:"id"`
}

type ConfigRuntimeSection struct {
	AutoApprove     bool                      `json:"autoApprove" mapstructure:"auto_approve"`
	PermissionMode  PermissionMode            `json:"permissionMode" mapstructure:"permission_mode"`
	PermissionRules []PermissionRule          `json:"permissionRules,omitempty" mapstructure:"permission_rules" yaml:"permission_rules,omitempty"`
	Sandbox         ConfigSandboxSection      `json:"sandbox" mapstructure:"sandbox"`
	Environment     *ConfigEnvironmentSection `json:"environment,omitempty" mapstructure:"environment" yaml:"environment,omitempty"`
	Browser         ConfigBrowserSection      `json:"browser" mapstructure:"browser"`
	Computer        ConfigComputerSection     `json:"computer" mapstructure:"computer"`
	Turn            ConfigTurnSection         `json:"turn" mapstructure:"turn"`
	Team            ConfigTeamSection         `json:"team" mapstructure:"team"`
	Tools           ConfigToolsSection        `json:"tools" mapstructure:"tools"`
	Memory          ConfigMemorySection       `json:"memory" mapstructure:"memory"`
	Table           ConfigTableSection        `json:"table" mapstructure:"table"`
	Knowledge       ConfigKnowledgeSection    `json:"knowledge" mapstructure:"knowledge"`
	Compaction      ConfigCompactionSection   `json:"compaction" mapstructure:"compaction"`
	Retention       ConfigRetentionSection    `json:"retention" mapstructure:"retention"`
}

// ConfigRetentionSection bounds history.db growth. Orphaned history (sessions
// that no longer exist) is always cleaned at startup; age-based pruning is
// opt-in.
type ConfigRetentionSection struct {
	// HistoryMaxAgeDays prunes turn entries + stream events of sessions whose
	// last activity is older than this many days. Session/turn metadata and
	// memories are kept; the pruned sessions lose timeline and LLM replay
	// context. 0 disables age-based pruning (default).
	HistoryMaxAgeDays int `json:"historyMaxAgeDays" mapstructure:"history_max_age_days"`
}

type ConfigCompactionSection struct {
	Enabled bool   `json:"enabled" mapstructure:"enabled"`
	Model   string `json:"model" mapstructure:"model"`
	// TriggerRatio is the high-water fraction of the model's usable context
	// (context_window − max_output) that starts in-turn / session compaction.
	// Default 0.85. Budget comes from the model catalog / llm.models entry —
	// there is no separate compaction max_tokens override.
	TriggerRatio float64 `json:"triggerRatio" mapstructure:"trigger_ratio"`
	// LowWaterRatio is the retained byte-size ratio of in-turn pressure
	// compaction: after the high-water trigger, dedup/prune runs and then the
	// oldest tool pairs are dropped until the remaining messages shrink to
	// this fraction of the post-dedup/prune byte size (drop the rest).
	// Default 0.50. Separate from CutTokens, which is the session-compaction
	// retain window.
	LowWaterRatio float64 `json:"lowWaterRatio" mapstructure:"low_water_ratio"`
	// CutTokens is how many recent estimated tokens session compaction keeps
	// after summarizing older history. Default 16000.
	CutTokens    int `json:"cutTokens" mapstructure:"cut_tokens"`
	TurnInterval int `json:"turnInterval" mapstructure:"turn_interval"`
	SubInterval  int `json:"subInterval" mapstructure:"sub_interval"`
	// ToolTruncate is the in-turn prune threshold (chars). Over-budget tool
	// results are rewritten to head+marker+tail when token pressure qualifies.
	// Default 8192.
	ToolTruncate int `json:"toolTruncate" mapstructure:"tool_truncate"`
	// KeepRecentToolSteps is how many latest LLM tool-call batches keep full
	// results during turn-internal prune/snip (pressure-gated). Default 3.
	KeepRecentToolSteps int `json:"keepRecentToolSteps" mapstructure:"keep_recent_tool_steps"`
}

type ConfigTurnSection struct {
	DoomLoopThreshold int `json:"doomLoopThreshold" mapstructure:"doom_loop_threshold"`
	MaxStepsDefault   int `json:"maxStepsDefault" mapstructure:"max_steps_default"`
	// MaxLLMFailures is consecutive LLM Chat errors before the turn fails
	// (independent of max_steps). Resets after any successful Chat response.
	MaxLLMFailures int `json:"maxLLMFailures" mapstructure:"max_llm_failures"`
	// LLMHTTPTimeoutSec is the per-request HTTP client timeout for non-streaming
	// LLM Chat (includes waiting for the full response body). High-effort
	// reasoning models often need well above 120s. Default 600.
	LLMHTTPTimeoutSec int `json:"llmHttpTimeoutSec" mapstructure:"llm_http_timeout_sec"`
}

// ConfigToolsSection controls local tool execution safeguards.
type ConfigToolsSection struct {
	// MaxOutputChars hard-caps a single tool result before it enters the
	// LLM context, UI stream, and turn log. Prevents oversized shell/MCP
	// outputs from blowing the context window.
	MaxOutputChars int `json:"maxOutputChars" mapstructure:"max_output_chars"`
}

type ConfigTeamSection struct {
	MaxDelegationDepth int `json:"maxDelegationDepth" mapstructure:"max_delegation_depth"`
}

// ConfigMemorySection controls durable memory_read result limits.
type ConfigMemorySection struct {
	ReadTopK int `json:"readTopK" mapstructure:"read_top_k"`
}

type ConfigKnowledgeSection struct {
	SearchTopK int `json:"searchTopK" mapstructure:"search_top_k"`
	// ChapterMaxTokens is the chapter merge budget for the Markdown splitter
	// (approximate subword units, tiersum-style). Default 512.
	ChapterMaxTokens int `json:"chapterMaxTokens" mapstructure:"chapter_max_tokens"`
	// VectorHybrid enables optional dense-vector branch fused with BM25 (P2).
	// Default false — BM25-only when off.
	VectorHybrid bool `json:"vectorHybrid" mapstructure:"vector_hybrid"`
}

// UpdateConfigFileRequest is sent by clients to update one or more sections
// of the configuration file. Only sections that are non-nil are modified;
// other sections are preserved as-is.
type UpdateConfigFileRequest struct {
	Data     *ConfigDataSection         `json:"data,omitempty"`
	Server   *ConfigServerSection       `json:"server,omitempty"`
	Instance *ConfigInstanceSection     `json:"instance,omitempty"`
	Runtime  *ConfigRuntimeSection      `json:"runtime,omitempty"`
	Search   *UpsertSearchConfigRequest `json:"search,omitempty"`
	LLM      *ConfigLLMSection          `json:"llm,omitempty"`
	Market   *ConfigMarketSection       `json:"market,omitempty"`
	Channels *ConfigChannelsSection     `json:"channels,omitempty"`
	Remote   *ConfigRemoteSection       `json:"remote,omitempty"`
}
