package domain

import "strings"

// InferReasoningDialect guesses Chat Completions thinking dialect from a model id
// (may be "provider/model" or bare model name). Empty if unknown.
// Order matches core/adapter/llm/REASONING_DIALECTS.md.
func InferReasoningDialect(modelID string) string {
	s := strings.ToLower(modelID)
	base := s
	if i := strings.LastIndex(s, "/"); i >= 0 {
		base = s[i+1:]
	}
	switch {
	case strings.Contains(s, "deepseek"):
		return ReasoningDialectDeepSeek
	case strings.Contains(s, "qwen"):
		return ReasoningDialectQwen
	// K3 before other kimi* ids.
	case base == "k3",
		base == "k3-256k",
		strings.Contains(s, "kimi-k3"),
		strings.HasPrefix(base, "k3-"):
		return ReasoningDialectKimiK3
	// K2.7 Code / Kimi Code coding SKUs — disabled thinking is rejected.
	case strings.Contains(s, "kimi-k2.7"),
		strings.Contains(s, "kimi-for-coding"):
		return ReasoningDialectKimiCode
	case strings.Contains(s, "kimi"), strings.Contains(s, "moonshot"):
		return ReasoningDialectKimi
	case strings.Contains(s, "glm"), strings.Contains(s, "zhipu"), strings.Contains(s, "chatglm"):
		return ReasoningDialectGLM
	case strings.Contains(s, "minimax"):
		return ReasoningDialectMiniMax
	case strings.Contains(s, "gemini"):
		return ReasoningDialectGemini
	case strings.Contains(s, "grok"):
		return ReasoningDialectGrok
	default:
		return ""
	}
}
