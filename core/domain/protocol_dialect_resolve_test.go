package domain

import "testing"

func TestResolveProtocolDialect_ModelOverride(t *testing.T) {
	cfg := LLMProviderConfig{
		DefaultProtocolDialect: ProtocolDialectQwen,
		Models: []LLMModelRef{
			{Name: "custom-m", Enabled: true, ProtocolDialect: ProtocolDialectDeepSeek},
			{Name: "inherit-m", Enabled: true},
		},
	}
	if got := ResolveProtocolDialect(cfg, "custom-m"); got != ProtocolDialectDeepSeek {
		t.Fatalf("override: got %q", got)
	}
	if got := ResolveProtocolDialect(cfg, "inherit-m"); got != ProtocolDialectQwen {
		t.Fatalf("inherit provider default: got %q", got)
	}
}

func TestResolveProtocolDialect_InferFallback(t *testing.T) {
	cfg := LLMProviderConfig{
		Models: []LLMModelRef{{Name: "deepseek-flash", Enabled: true}},
	}
	if got := ResolveProtocolDialect(cfg, "deepseek-flash"); got != ProtocolDialectDeepSeek {
		t.Fatalf("infer: got %q", got)
	}
}

func TestResolveProtocolDialect_OpenAIDefault(t *testing.T) {
	cfg := LLMProviderConfig{
		Models: []LLMModelRef{{Name: "unknown-model-xyz", Enabled: true}},
	}
	if got := ResolveProtocolDialect(cfg, "unknown-model-xyz"); got != ProtocolDialectOpenAI {
		t.Fatalf("default openai: got %q", got)
	}
}

func TestResolveProtocolDialect_MissingModelUsesProviderDefault(t *testing.T) {
	cfg := LLMProviderConfig{DefaultProtocolDialect: ProtocolDialectGLM}
	if got := ResolveProtocolDialect(cfg, "not-listed"); got != ProtocolDialectGLM {
		t.Fatalf("missing model: got %q", got)
	}
}
