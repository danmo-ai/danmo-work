package llm

import (
	"encoding/json"
	"testing"
)

func TestOpenAICompatibleUsageCacheReadShapes(t *testing.T) {
	tests := []struct {
		name         string
		raw          string
		wantRead     int
		wantCreate   int
		wantPrompt   int
		wantComplete int
	}{
		{
			name: "openai nested cached_tokens",
			raw: `{
				"prompt_tokens": 10, "completion_tokens": 5, "total_tokens": 15,
				"prompt_tokens_details": {"cached_tokens": 7}
			}`,
			wantRead: 7, wantPrompt: 10, wantComplete: 5,
		},
		{
			name: "deepseek prompt_cache_hit_tokens",
			raw: `{
				"prompt_tokens": 100, "completion_tokens": 20, "total_tokens": 120,
				"prompt_cache_hit_tokens": 80, "prompt_cache_miss_tokens": 20
			}`,
			wantRead: 80, wantPrompt: 100, wantComplete: 20,
		},
		{
			name: "kimi top-level cached_tokens",
			raw: `{
				"prompt_tokens": 50, "completion_tokens": 8, "total_tokens": 58,
				"cached_tokens": 40
			}`,
			wantRead: 40, wantPrompt: 50, wantComplete: 8,
		},
		{
			name: "anthropic-compat cache_read and cache_creation",
			raw: `{
				"prompt_tokens": 10, "completion_tokens": 2, "total_tokens": 12,
				"cache_read_input_tokens": 100, "cache_creation_input_tokens": 8
			}`,
			wantRead: 100, wantCreate: 8, wantPrompt: 10, wantComplete: 2,
		},
		{
			name: "responses input_tokens_details",
			raw: `{
				"input_tokens": 11, "output_tokens": 2, "total_tokens": 13,
				"input_tokens_details": {"cached_tokens": 4}
			}`,
			wantRead: 4, wantPrompt: 11, wantComplete: 2,
		},
		{
			name: "nested openai wins over top-level alias",
			raw: `{
				"prompt_tokens": 10, "completion_tokens": 1, "total_tokens": 11,
				"cached_tokens": 999,
				"prompt_tokens_details": {"cached_tokens": 400}
			}`,
			wantRead: 400, wantPrompt: 10, wantComplete: 1,
		},
		{
			name: "no cache fields",
			raw:  `{"prompt_tokens": 3, "completion_tokens": 1, "total_tokens": 4}`,
			wantRead: 0, wantPrompt: 3, wantComplete: 1,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var u openaiCompatibleUsage
			if err := json.Unmarshal([]byte(tt.raw), &u); err != nil {
				t.Fatalf("unmarshal: %v", err)
			}
			got := u.toLLMUsage()
			if got == nil {
				t.Fatal("expected usage")
			}
			if got.CacheReadTokens != tt.wantRead {
				t.Errorf("cache read: got %d want %d", got.CacheReadTokens, tt.wantRead)
			}
			if got.CacheCreationTokens != tt.wantCreate {
				t.Errorf("cache create: got %d want %d", got.CacheCreationTokens, tt.wantCreate)
			}
			if got.PromptTokens != tt.wantPrompt {
				t.Errorf("prompt: got %d want %d", got.PromptTokens, tt.wantPrompt)
			}
			if got.CompletionTokens != tt.wantComplete {
				t.Errorf("completion: got %d want %d", got.CompletionTokens, tt.wantComplete)
			}
		})
	}
}
