package domain

// ProtocolDialectInfo documents one Chat Completions protocol dialect
// (vendor wire shape for thinking fields). Keep in sync with PROTOCOL_DIALECTS.md.
type ProtocolDialectInfo struct {
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

// ProtocolDialectInfos is the machine-readable dialect catalog for API / UI.
var ProtocolDialectInfos = []ProtocolDialectInfo{
	{
		ID:          ProtocolDialectOpenAI,
		Label:       "OpenAI",
		Vendors:     "OpenAI GPT-5 / o-series; OpenAI-compatible gateways",
		RequestHint: "reasoning_effort when on",
		Echo:        false,
		EffortsHint: "none | minimal | low | medium | high | xhigh | max",
		DocsURL:     "https://platform.openai.com/docs/guides/reasoning",
	},
	{
		ID:          ProtocolDialectDeepSeek,
		Label:       "DeepSeek",
		Vendors:     "deepseek-flash, deepseek-v4-pro; legacy deepseek-v4-flash*, deepseek-chat, deepseek-reasoner",
		RequestHint: "thinking.type + reasoning_effort (low|high|max)",
		Echo:        true,
		EffortsHint: "off → thinking.disabled; on → low|high|max",
		DocsURL:     "https://api-docs.deepseek.com/guides/thinking_mode",
	},
	{
		ID:          ProtocolDialectQwen,
		Label:       "Qwen",
		Vendors:     "Qwen3.x via DashScope compatible-mode",
		RequestHint: "enable_thinking (+ optional thinking_budget)",
		Echo:        true,
		EffortsHint: "off/on; budget via built-in effort→thinking_budget map",
		DocsURL:     "https://docs.qwencloud.com/developer-guides/text-generation/thinking",
	},
	{
		ID:          ProtocolDialectKimi,
		Label:       "Kimi K2.6",
		Vendors:     "kimi-k2.6, kimi-k2.5",
		RequestHint: "thinking.type (+ keep:all when on)",
		Echo:        true,
		EffortsHint: "toggle only",
		DocsURL:     "https://platform.kimi.ai/docs/api/models-overview",
	},
	{
		ID:          ProtocolDialectKimiCode,
		Label:       "Kimi K2.7 Code",
		Vendors:     "kimi-k2.7-code*, kimi-for-coding-highspeed",
		RequestHint: "always thinking; enabled+keep:all only (disabled errors)",
		Echo:        true,
		AlwaysOn:    true,
		EffortsHint: "always on",
		DocsURL:     "https://www.kimi.com/code/docs/en/kimi-code/models.html",
	},
	{
		ID:          ProtocolDialectKimiK3,
		Label:       "Kimi K3 / K2.8",
		Vendors:     "kimi-k3, k3, k3-256k, kimi-for-coding (K2.8 Preview), kimi-k2.8*",
		RequestHint: "reasoning_effort only (no thinking object)",
		Echo:        true,
		AlwaysOn:    true,
		EffortsHint: "low | high | max (off→low)",
		DocsURL:     "https://www.kimi.com/code/docs/en/kimi-code/models.html",
	},
	{
		ID:          ProtocolDialectGLM,
		Label:       "GLM / Zhipu",
		Vendors:     "glm-*, Zhipu / Z.AI",
		RequestHint: "thinking.type + clear_thinking:false; optional reasoning_effort",
		Echo:        true,
		EffortsHint: "off/on; effort on GLM-5.2+",
		DocsURL:     "https://docs.z.ai/guides/capabilities/thinking",
	},
	{
		ID:          ProtocolDialectMiniMax,
		Label:       "MiniMax",
		Vendors:     "MiniMax-M*",
		RequestHint: "reasoning_split + thinking.type adaptive|disabled",
		Echo:        true,
		EffortsHint: "toggle; M2.x cannot fully disable",
		DocsURL:     "https://platform.minimax.io/docs/api-reference/text-openai-api",
	},
	{
		ID:          ProtocolDialectGemini,
		Label:       "Gemini",
		Vendors:     "gemini-* (OpenAI compatibility)",
		RequestHint: "reasoning_effort (none when off); no thinking_config mix",
		Echo:        false,
		EffortsHint: "none | minimal | low | medium | high",
		DocsURL:     "https://ai.google.dev/gemini-api/docs/openai",
	},
	{
		ID:          ProtocolDialectGrok,
		Label:       "Grok / xAI",
		Vendors:     "grok-*",
		RequestHint: "reasoning_effort",
		Echo:        false,
		EffortsHint: "low | medium | high (4.5 off→low); 4.3 may use none",
		DocsURL:     "https://docs.x.ai/developers/model-capabilities/text/reasoning",
	},
}
