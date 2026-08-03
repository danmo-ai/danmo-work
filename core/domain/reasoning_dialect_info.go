package domain

// ReasoningDialectInfo documents one Chat Completions thinking dialect.
// Keep in sync with core/adapter/llm/REASONING_DIALECTS.md.
type ReasoningDialectInfo struct {
	ID          string   `json:"id"`
	Label       string   `json:"label"`
	Vendors     string   `json:"vendors"`
	RequestHint string   `json:"requestHint"`
	Echo        bool     `json:"echo"`
	EffortsHint string   `json:"effortsHint"`
	DocsURL     string   `json:"docsUrl,omitempty"`
	AlwaysOn    bool     `json:"alwaysOn,omitempty"`
	Aliases     []string `json:"aliases,omitempty"`
}

// ReasoningDialectInfos is the machine-readable dialect catalog for API / UI.
var ReasoningDialectInfos = []ReasoningDialectInfo{
	{
		ID:          ReasoningDialectOpenAI,
		Label:       "OpenAI",
		Vendors:     "OpenAI GPT-5 / o-series; OpenAI-compatible gateways",
		RequestHint: "reasoning_effort when on",
		Echo:        false,
		EffortsHint: "none | minimal | low | medium | high | xhigh | max",
		DocsURL:     "https://platform.openai.com/docs/guides/reasoning",
	},
	{
		ID:          ReasoningDialectDeepSeek,
		Label:       "DeepSeek",
		Vendors:     "deepseek-v4-*, deepseek-chat, deepseek-reasoner",
		RequestHint: "thinking.type + reasoning_effort (high|max)",
		Echo:        true,
		EffortsHint: "off → thinking.disabled; on → high|max",
		DocsURL:     "https://api-docs.deepseek.com/",
	},
	{
		ID:          ReasoningDialectQwen,
		Label:       "Qwen",
		Vendors:     "Qwen3.x via DashScope compatible-mode",
		RequestHint: "enable_thinking (+ optional thinking_budget)",
		Echo:        true,
		EffortsHint: "off/on; budget via effort_budget_tokens",
		DocsURL:     "https://docs.qwencloud.com/developer-guides/text-generation/thinking",
	},
	{
		ID:          ReasoningDialectKimi,
		Label:       "Kimi K2.6",
		Vendors:     "kimi-k2.6, kimi-k2.5",
		RequestHint: "thinking.type (+ keep:all when on)",
		Echo:        true,
		EffortsHint: "toggle only",
		DocsURL:     "https://platform.kimi.ai/docs/api/models-overview",
	},
	{
		ID:          ReasoningDialectKimiCode,
		Label:       "Kimi K2.7 Code",
		Vendors:     "kimi-k2.7-code*, kimi-for-coding*",
		RequestHint: "always thinking; enabled+keep:all only (disabled errors)",
		Echo:        true,
		AlwaysOn:    true,
		EffortsHint: "always on",
		DocsURL:     "https://platform.kimi.ai/docs/api/models-overview",
	},
	{
		ID:          ReasoningDialectKimiK3,
		Label:       "Kimi K3",
		Vendors:     "kimi-k3, k3, k3-256k",
		RequestHint: "reasoning_effort only (no thinking object)",
		Echo:        true,
		AlwaysOn:    true,
		EffortsHint: "low | high | max (off→low)",
		DocsURL:     "https://platform.kimi.ai/docs/guide/kimi-k3-quickstart",
	},
	{
		ID:          ReasoningDialectGLM,
		Label:       "GLM / Zhipu",
		Vendors:     "glm-*, Zhipu / Z.AI",
		RequestHint: "thinking.type + clear_thinking:false; optional reasoning_effort",
		Echo:        true,
		EffortsHint: "off/on; effort on GLM-5.2+",
		DocsURL:     "https://docs.z.ai/guides/capabilities/thinking",
	},
	{
		ID:          ReasoningDialectMiniMax,
		Label:       "MiniMax",
		Vendors:     "MiniMax-M*",
		RequestHint: "reasoning_split + thinking.type adaptive|disabled",
		Echo:        true,
		EffortsHint: "toggle; M2.x cannot fully disable",
		DocsURL:     "https://platform.minimax.io/docs/api-reference/text-openai-api",
	},
	{
		ID:          ReasoningDialectGemini,
		Label:       "Gemini",
		Vendors:     "gemini-* (OpenAI compatibility)",
		RequestHint: "reasoning_effort (none when off); no thinking_config mix",
		Echo:        false,
		EffortsHint: "none | minimal | low | medium | high",
		DocsURL:     "https://ai.google.dev/gemini-api/docs/openai",
	},
	{
		ID:          ReasoningDialectGrok,
		Label:       "Grok / xAI",
		Vendors:     "grok-*",
		RequestHint: "reasoning_effort",
		Echo:        false,
		EffortsHint: "low | medium | high (4.5 off→low); 4.3 may use none",
		DocsURL:     "https://docs.x.ai/developers/model-capabilities/text/reasoning",
	},
}
