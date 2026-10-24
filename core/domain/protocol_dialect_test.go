package domain

import "testing"

func TestInferProtocolDialect(t *testing.T) {
	cases := map[string]string{
		"deepseek/deepseek-chat":       ProtocolDialectDeepSeek,
		"deepseek/deepseek-flash":      ProtocolDialectDeepSeek,
		"qwen/qwen3-max":               ProtocolDialectQwen,
		"moonshot/kimi-k2.6":           ProtocolDialectKimi,
		"kimi-k2.7-code":               ProtocolDialectKimiCode,
		"kimi-for-coding-highspeed":    ProtocolDialectKimiCode,
		"kimi-for-coding":              ProtocolDialectKimiK3,
		"kimi-k2.8-preview":            ProtocolDialectKimiK3,
		"k3":                           ProtocolDialectKimiK3,
		"k3-256k":                      ProtocolDialectKimiK3,
		"moonshot/kimi-k3":             ProtocolDialectKimiK3,
		"zhipu/glm-4.7":                ProtocolDialectGLM,
		"glm-5.1":                      ProtocolDialectGLM,
		"minimax/MiniMax-M3":           ProtocolDialectMiniMax,
		"google/gemini-2.5-pro":        ProtocolDialectGemini,
		"xai/grok-4.5":                 ProtocolDialectGrok,
		"openai/gpt-4o":                "",
	}
	for in, want := range cases {
		if got := InferProtocolDialect(in); got != want {
			t.Errorf("%s: got %q want %q", in, got, want)
		}
	}
}

func TestProtocolDialectInfosCoverConstants(t *testing.T) {
	want := map[string]bool{
		ProtocolDialectOpenAI:   true,
		ProtocolDialectDeepSeek: true,
		ProtocolDialectQwen:     true,
		ProtocolDialectKimi:     true,
		ProtocolDialectKimiCode: true,
		ProtocolDialectKimiK3:   true,
		ProtocolDialectGLM:      true,
		ProtocolDialectMiniMax:  true,
		ProtocolDialectGemini:   true,
		ProtocolDialectGrok:     true,
	}
	for _, info := range ProtocolDialectInfos {
		if !want[info.ID] {
			t.Errorf("unexpected dialect info %q", info.ID)
		}
		delete(want, info.ID)
	}
	for id := range want {
		t.Errorf("missing dialect info for %q", id)
	}
}
