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
}

type ConfigLLMSection struct {
	Providers []LLMProviderPreset `json:"providers" mapstructure:"providers" yaml:"providers"`
	Models    []ModelConfig       `json:"models,omitempty" mapstructure:"models" yaml:"models,omitempty"`
}

type ConfigDataSection struct {
	Dir           string `json:"dir" mapstructure:"dir"`
	Database      string `json:"database" mapstructure:"database"`
	StoreDatabase string `json:"storeDatabase" mapstructure:"store_database"`
	Store         string `json:"store" mapstructure:"store"`
}

type ConfigServerSection struct {
	ListenAddr string `json:"listenAddr" mapstructure:"listen_addr"`
}

type ConfigInstanceSection struct {
	ID string `json:"id" mapstructure:"id"`
}

type ConfigRuntimeSection struct {
	AutoApprove    bool                    `json:"autoApprove" mapstructure:"auto_approve"`
	PermissionMode PermissionMode          `json:"permissionMode" mapstructure:"permission_mode"`
	Sandbox        ConfigSandboxSection    `json:"sandbox" mapstructure:"sandbox"`
	Browser        ConfigBrowserSection    `json:"browser" mapstructure:"browser"`
	Turn           ConfigTurnSection       `json:"turn" mapstructure:"turn"`
	Team           ConfigTeamSection       `json:"team" mapstructure:"team"`
	Tools          ConfigToolsSection      `json:"tools" mapstructure:"tools"`
	Memory         ConfigMemorySection     `json:"memory" mapstructure:"memory"`
	Table          ConfigTableSection      `json:"table" mapstructure:"table"`
	Knowledge      ConfigKnowledgeSection  `json:"knowledge" mapstructure:"knowledge"`
	Compaction     ConfigCompactionSection `json:"compaction" mapstructure:"compaction"`
}

type ConfigCompactionSection struct {
	Enabled      bool    `json:"enabled" mapstructure:"enabled"`
	Model        string  `json:"model" mapstructure:"model"`
	MaxTokens    int     `json:"maxTokens" mapstructure:"max_tokens"`
	TriggerRatio float64 `json:"triggerRatio" mapstructure:"trigger_ratio"`
	CutTokens    int     `json:"cutTokens" mapstructure:"cut_tokens"`
	TurnInterval int     `json:"turnInterval" mapstructure:"turn_interval"`
	SubInterval  int     `json:"subInterval" mapstructure:"sub_interval"`
	ToolTruncate int     `json:"toolTruncate" mapstructure:"tool_truncate"`
}

type ConfigTurnSection struct {
	DoomLoopThreshold int `json:"doomLoopThreshold" mapstructure:"doom_loop_threshold"`
	MaxStepsDefault   int `json:"maxStepsDefault" mapstructure:"max_steps_default"`
	// MaxLLMFailures is consecutive LLM Chat errors before the turn fails
	// (independent of max_steps). Resets after any successful Chat response.
	MaxLLMFailures int `json:"maxLLMFailures" mapstructure:"max_llm_failures"`
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
}
