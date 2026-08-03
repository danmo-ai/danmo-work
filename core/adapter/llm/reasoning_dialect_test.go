package llm

import (
	"testing"

	"danmo-work/core/domain"
	"danmo-work/core/port"
)

func TestApplyReasoningDialectRequest_Qwen(t *testing.T) {
	body := map[string]any{}
	applyReasoningDialectRequest(body, domain.ReasoningDialectQwen, "high", &port.ModelGenParams{
		EffortBudgetTokens: map[string]int{"high": 8000},
	})
	if body["enable_thinking"] != true {
		t.Fatalf("enable_thinking: %v", body["enable_thinking"])
	}
	if body["thinking_budget"] != 8000 {
		t.Fatalf("thinking_budget: %v", body["thinking_budget"])
	}
	if _, ok := body["reasoning_effort"]; ok {
		t.Fatal("qwen must not set reasoning_effort")
	}

	bodyOff := map[string]any{}
	applyReasoningDialectRequest(bodyOff, domain.ReasoningDialectQwen, "off", nil)
	if bodyOff["enable_thinking"] != false {
		t.Fatalf("off enable_thinking: %v", bodyOff["enable_thinking"])
	}
}

func TestApplyReasoningDialectRequest_Kimi(t *testing.T) {
	body := map[string]any{}
	applyReasoningDialectRequest(body, domain.ReasoningDialectKimi, "high", nil)
	th, ok := body["thinking"].(map[string]any)
	if !ok || th["type"] != "enabled" || th["keep"] != "all" {
		t.Fatalf("thinking: %v", body["thinking"])
	}
	if _, ok := body["reasoning_effort"]; ok {
		t.Fatal("kimi must not set reasoning_effort")
	}

	bodyOff := map[string]any{}
	applyReasoningDialectRequest(bodyOff, domain.ReasoningDialectKimi, "off", nil)
	thOff := bodyOff["thinking"].(map[string]any)
	if thOff["type"] != "disabled" {
		t.Fatalf("disabled: %v", thOff)
	}
}

func TestApplyReasoningDialectRequest_KimiK3(t *testing.T) {
	body := map[string]any{}
	applyReasoningDialectRequest(body, domain.ReasoningDialectKimiK3, "max", nil)
	if body["reasoning_effort"] != "max" {
		t.Fatalf("reasoning_effort: %v", body["reasoning_effort"])
	}
	if _, ok := body["thinking"]; ok {
		t.Fatal("kimi_k3 must not set thinking object")
	}

	bodyOff := map[string]any{}
	applyReasoningDialectRequest(bodyOff, domain.ReasoningDialectKimiK3, "off", nil)
	if bodyOff["reasoning_effort"] != "low" {
		t.Fatalf("off maps to low: %v", bodyOff["reasoning_effort"])
	}
}

func TestApplyReasoningDialectRequest_DeepSeek(t *testing.T) {
	body := map[string]any{}
	applyReasoningDialectRequest(body, domain.ReasoningDialectDeepSeek, "max", nil)
	th, ok := body["thinking"].(map[string]any)
	if !ok || th["type"] != "enabled" {
		t.Fatalf("thinking: %v", body["thinking"])
	}
	if body["reasoning_effort"] != "max" {
		t.Fatalf("reasoning_effort: %v", body["reasoning_effort"])
	}

	bodyLow := map[string]any{}
	applyReasoningDialectRequest(bodyLow, domain.ReasoningDialectDeepSeek, "low", nil)
	if bodyLow["reasoning_effort"] != "high" {
		t.Fatalf("low maps to high: %v", bodyLow["reasoning_effort"])
	}

	bodyOff := map[string]any{}
	applyReasoningDialectRequest(bodyOff, domain.ReasoningDialectDeepSeek, "off", nil)
	if bodyOff["thinking"].(map[string]any)["type"] != "disabled" {
		t.Fatalf("off: %v", bodyOff["thinking"])
	}
	if _, ok := bodyOff["reasoning_effort"]; ok {
		t.Fatal("off must not set reasoning_effort")
	}
}

func TestApplyReasoningDialectRequest_KimiCode(t *testing.T) {
	body := map[string]any{}
	applyReasoningDialectRequest(body, domain.ReasoningDialectKimiCode, "off", nil)
	th := body["thinking"].(map[string]any)
	if th["type"] != "enabled" || th["keep"] != "all" {
		t.Fatalf("always on: %v", th)
	}
}

func TestApplyReasoningDialectRequest_Grok(t *testing.T) {
	body := map[string]any{}
	applyReasoningDialectRequest(body, domain.ReasoningDialectGrok, "medium", nil)
	if body["reasoning_effort"] != "medium" {
		t.Fatalf("got %v", body["reasoning_effort"])
	}
	bodyOff := map[string]any{}
	applyReasoningDialectRequest(bodyOff, domain.ReasoningDialectGrok, "off", nil)
	if bodyOff["reasoning_effort"] != "low" {
		t.Fatalf("off→low: %v", bodyOff["reasoning_effort"])
	}
}

func TestApplyReasoningDialectRequest_GLM(t *testing.T) {
	body := map[string]any{}
	applyReasoningDialectRequest(body, domain.ReasoningDialectGLM, "high", nil)
	th, ok := body["thinking"].(map[string]any)
	if !ok || th["type"] != "enabled" || th["clear_thinking"] != false {
		t.Fatalf("thinking: %v", body["thinking"])
	}
	if body["reasoning_effort"] != "high" {
		t.Fatalf("reasoning_effort: %v", body["reasoning_effort"])
	}

	bodyOff := map[string]any{}
	applyReasoningDialectRequest(bodyOff, domain.ReasoningDialectGLM, "off", nil)
	thOff := bodyOff["thinking"].(map[string]any)
	if thOff["type"] != "disabled" {
		t.Fatalf("disabled: %v", thOff)
	}
	if _, ok := bodyOff["reasoning_effort"]; ok {
		t.Fatal("off must not set reasoning_effort")
	}
}

func TestApplyReasoningDialectRequest_MiniMax(t *testing.T) {
	body := map[string]any{}
	applyReasoningDialectRequest(body, domain.ReasoningDialectMiniMax, "high", nil)
	if body["reasoning_split"] != true {
		t.Fatalf("reasoning_split: %v", body["reasoning_split"])
	}
	th := body["thinking"].(map[string]any)
	if th["type"] != "adaptive" {
		t.Fatalf("thinking: %v", th)
	}

	bodyOff := map[string]any{}
	applyReasoningDialectRequest(bodyOff, domain.ReasoningDialectMiniMax, "off", nil)
	if bodyOff["reasoning_split"] != true {
		t.Fatal("reasoning_split should stay on for format")
	}
	if bodyOff["thinking"].(map[string]any)["type"] != "disabled" {
		t.Fatalf("disabled: %v", bodyOff["thinking"])
	}
}

func TestApplyReasoningDialectRequest_Gemini(t *testing.T) {
	body := map[string]any{}
	applyReasoningDialectRequest(body, domain.ReasoningDialectGemini, "medium", nil)
	if body["reasoning_effort"] != "medium" {
		t.Fatalf("reasoning_effort: %v", body["reasoning_effort"])
	}
	if _, ok := body["extra_body"]; ok {
		t.Fatal("must not nest SDK-only extra_body")
	}

	bodyOff := map[string]any{}
	applyReasoningDialectRequest(bodyOff, domain.ReasoningDialectGemini, "off", nil)
	if bodyOff["reasoning_effort"] != "none" {
		t.Fatalf("off: %v", bodyOff["reasoning_effort"])
	}
}

func TestNormalizeChatCompletionsReasoning_MiniMaxThinkTags(t *testing.T) {
	content := "<think>plan</think>\nanswer"
	ans, reason := normalizeChatCompletionsReasoning(content, "", "", domain.ReasoningDialectMiniMax)
	if reason != "plan" {
		t.Fatalf("reason: %q", reason)
	}
	if ans != "answer" {
		t.Fatalf("answer: %q", ans)
	}
}

func TestResolveReasoningDialect(t *testing.T) {
	if got := resolveReasoningDialect(&port.ModelGenParams{ReasoningDialect: "qwen"}, "x"); got != "qwen" {
		t.Fatalf("explicit: %s", got)
	}
	if got := resolveReasoningDialect(nil, "deepseek-chat"); got != domain.ReasoningDialectDeepSeek {
		t.Fatalf("infer deepseek: %s", got)
	}
	if got := resolveReasoningDialect(nil, "glm-4.7"); got != domain.ReasoningDialectGLM {
		t.Fatalf("infer glm: %s", got)
	}
	if got := resolveReasoningDialect(nil, "MiniMax-M3"); got != domain.ReasoningDialectMiniMax {
		t.Fatalf("infer minimax: %s", got)
	}
	if got := resolveReasoningDialect(nil, "k3-256k"); got != domain.ReasoningDialectKimiK3 {
		t.Fatalf("infer k3-256k: %s", got)
	}
	if got := resolveReasoningDialect(nil, "kimi-for-coding"); got != domain.ReasoningDialectKimiCode {
		t.Fatalf("infer kimi_code: %s", got)
	}
	if got := resolveReasoningDialect(nil, "grok-4.5"); got != domain.ReasoningDialectGrok {
		t.Fatalf("infer grok: %s", got)
	}
	if got := resolveReasoningDialect(nil, "gpt-4o"); got != domain.ReasoningDialectOpenAI {
		t.Fatalf("default openai: %s", got)
	}
}

func TestDialectEchoesReasoning(t *testing.T) {
	if !dialectEchoesReasoning(domain.ReasoningDialectDeepSeek) {
		t.Fatal("deepseek should echo")
	}
	if !dialectEchoesReasoning(domain.ReasoningDialectGLM) {
		t.Fatal("glm should echo")
	}
	if !dialectEchoesReasoning(domain.ReasoningDialectMiniMax) {
		t.Fatal("minimax should echo")
	}
	if !dialectEchoesReasoning(domain.ReasoningDialectKimiK3) {
		t.Fatal("kimi_k3 should echo")
	}
	if dialectEchoesReasoning(domain.ReasoningDialectOpenAI) {
		t.Fatal("openai should not echo")
	}
	if dialectEchoesReasoning(domain.ReasoningDialectGemini) {
		t.Fatal("gemini should not echo reasoning_content by default")
	}
}

func TestOpenAIChatCompletions_EchoesReasoningContent(t *testing.T) {
	echo := dialectEchoesReasoning(domain.ReasoningDialectKimi)
	if !echo {
		t.Fatal("expected echo")
	}
}
