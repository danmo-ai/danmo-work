package llm

import "danmo-work/core/port"

// openaiCompatibleUsage captures token usage from OpenAI Chat Completions /
// Responses and vendor-compatible gateways. Cache-hit field names differ:
//
//   - OpenAI / Qwen / GLM: prompt_tokens_details.cached_tokens
//   - OpenAI Responses:    input_tokens_details.cached_tokens
//   - DeepSeek:            prompt_cache_hit_tokens
//   - Anthropic-compat:    cache_read_input_tokens / cache_creation_input_tokens
//   - Kimi / Moonshot:     top-level cached_tokens
type openaiCompatibleUsage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
	InputTokens      int `json:"input_tokens"`
	OutputTokens     int `json:"output_tokens"`

	CachedTokens             int `json:"cached_tokens"`
	PromptCacheHitTokens     int `json:"prompt_cache_hit_tokens"`
	CacheReadInputTokens     int `json:"cache_read_input_tokens"`
	CacheCreationInputTokens int `json:"cache_creation_input_tokens"`

	PromptTokensDetails struct {
		CachedTokens        int `json:"cached_tokens"`
		CacheCreationTokens int `json:"cache_creation_tokens"`
	} `json:"prompt_tokens_details"`
	InputTokensDetails struct {
		CachedTokens int `json:"cached_tokens"`
	} `json:"input_tokens_details"`
}

func (u openaiCompatibleUsage) toLLMUsage() *port.LLMUsage {
	prompt := u.PromptTokens
	if prompt == 0 {
		prompt = u.InputTokens
	}
	completion := u.CompletionTokens
	if completion == 0 {
		completion = u.OutputTokens
	}
	out := &port.LLMUsage{
		PromptTokens:        prompt,
		CompletionTokens:    completion,
		TotalTokens:         u.TotalTokens,
		CacheReadTokens:     u.cacheRead(),
		CacheCreationTokens: u.cacheCreation(),
	}
	if out.Empty() {
		return nil
	}
	return out
}

func (u openaiCompatibleUsage) cacheRead() int {
	// Prefer the nested OpenAI shape when present so a vendor that also
	// copies a different number onto a top-level alias cannot override it.
	return firstPositive(
		u.PromptTokensDetails.CachedTokens,
		u.InputTokensDetails.CachedTokens,
		u.PromptCacheHitTokens,
		u.CacheReadInputTokens,
		u.CachedTokens,
	)
}

func (u openaiCompatibleUsage) cacheCreation() int {
	return firstPositive(
		u.PromptTokensDetails.CacheCreationTokens,
		u.CacheCreationInputTokens,
	)
}

func firstPositive(vals ...int) int {
	for _, v := range vals {
		if v > 0 {
			return v
		}
	}
	return 0
}
