package domain

import "testing"

func TestInferReasoningDialect(t *testing.T) {
	cases := map[string]string{
		"deepseek/deepseek-chat":       ReasoningDialectDeepSeek,
		"qwen/qwen3-max":               ReasoningDialectQwen,
		"moonshot/kimi-k2.6":           ReasoningDialectKimi,
		"kimi-k2.7-code":               ReasoningDialectKimiCode,
		"kimi-for-coding-highspeed":    ReasoningDialectKimiCode,
		"k3":                           ReasoningDialectKimiK3,
		"k3-256k":                      ReasoningDialectKimiK3,
		"moonshot/kimi-k3":             ReasoningDialectKimiK3,
		"zhipu/glm-4.7":                ReasoningDialectGLM,
		"glm-5.1":                      ReasoningDialectGLM,
		"minimax/MiniMax-M3":           ReasoningDialectMiniMax,
		"google/gemini-2.5-pro":        ReasoningDialectGemini,
		"xai/grok-4.5":                 ReasoningDialectGrok,
		"openai/gpt-4o":                "",
	}
	for in, want := range cases {
		if got := InferReasoningDialect(in); got != want {
			t.Errorf("%s: got %q want %q", in, got, want)
		}
	}
}

func TestReasoningDialectInfosCoverConstants(t *testing.T) {
	want := map[string]bool{
		ReasoningDialectOpenAI:   true,
		ReasoningDialectDeepSeek: true,
		ReasoningDialectQwen:     true,
		ReasoningDialectKimi:     true,
		ReasoningDialectKimiCode: true,
		ReasoningDialectKimiK3:   true,
		ReasoningDialectGLM:      true,
		ReasoningDialectMiniMax:  true,
		ReasoningDialectGemini:   true,
		ReasoningDialectGrok:     true,
	}
	for _, info := range ReasoningDialectInfos {
		if !want[info.ID] {
			t.Errorf("unexpected dialect info %q", info.ID)
		}
		delete(want, info.ID)
	}
	for id := range want {
		t.Errorf("missing dialect info for %q", id)
	}
}
