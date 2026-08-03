package domain

import "testing"

func TestNormalizeProtocol(t *testing.T) {
	if got := NormalizeProtocol(LLMProviderLocal); got != LLMProviderOpenAI {
		t.Errorf("local -> openai, got %q", got)
	}
	if got := NormalizeProtocol(LLMProviderOpenAI); got != LLMProviderOpenAI {
		t.Errorf("openai unchanged, got %q", got)
	}
	if got := NormalizeProtocol(LLMProviderOpenAIResponses); got != LLMProviderOpenAIResponses {
		t.Errorf("openai_responses unchanged, got %q", got)
	}
	if got := NormalizeProtocol(LLMProviderAnthropic); got != LLMProviderAnthropic {
		t.Errorf("anthropic unchanged, got %q", got)
	}
}
