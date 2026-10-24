package llm

import (
	"testing"

	"danmo-work/core/domain"
	"danmo-work/core/port"
)

func TestApplyProtocolDialectRequest_Qwen(t *testing.T) {
	body := map[string]any{}
	applyProtocolDialectRequest(body, domain.ProtocolDialectQwen, "high", &port.ModelGenParams{
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
	applyProtocolDialectRequest(bodyOff, domain.ProtocolDialectQwen, "off", nil)
	if bodyOff["enable_thinking"] != false {
		t.Fatalf("off enable_thinking: %v", bodyOff["enable_thinking"])
	}
}

func TestApplyProtocolDialectRequest_Kimi(t *testing.T) {
	body := map[string]any{}
	applyProtocolDialectRequest(body, domain.ProtocolDialectKimi, "high", nil)
	th, ok := body["thinking"].(map[string]any)
	if !ok || th["type"] != "enabled" || th["keep"] != "all" {
		t.Fatalf("thinking: %v", body["thinking"])
	}
	if _, ok := body["reasoning_effort"]; ok {
		t.Fatal("kimi must not set reasoning_effort")
	}

	bodyOff := map[string]any{}
	applyProtocolDialectRequest(bodyOff, domain.ProtocolDialectKimi, "off", nil)
	thOff := bodyOff["thinking"].(map[string]any)
	if thOff["type"] != "disabled" {
		t.Fatalf("disabled: %v", thOff)
	}
}

func TestApplyProtocolDialectRequest_KimiK3(t *testing.T) {
	body := map[string]any{}
	applyProtocolDialectRequest(body, domain.ProtocolDialectKimiK3, "max", nil)
	if body["reasoning_effort"] != "max" {
		t.Fatalf("reasoning_effort: %v", body["reasoning_effort"])
	}
	if _, ok := body["thinking"]; ok {
		t.Fatal("kimi_k3 must not set thinking object")
	}

	bodyOff := map[string]any{}
	applyProtocolDialectRequest(bodyOff, domain.ProtocolDialectKimiK3, "off", nil)
	if bodyOff["reasoning_effort"] != "low" {
		t.Fatalf("off maps to low: %v", bodyOff["reasoning_effort"])
	}
}

func TestApplyProtocolDialectRequest_DeepSeek(t *testing.T) {
	body := map[string]any{}
	applyProtocolDialectRequest(body, domain.ProtocolDialectDeepSeek, "max", nil)
	th, ok := body["thinking"].(map[string]any)
	if !ok || th["type"] != "enabled" {
		t.Fatalf("thinking: %v", body["thinking"])
	}
	if body["reasoning_effort"] != "max" {
		t.Fatalf("reasoning_effort: %v", body["reasoning_effort"])
	}

	bodyLow := map[string]any{}
	applyProtocolDialectRequest(bodyLow, domain.ProtocolDialectDeepSeek, "low", nil)
	if bodyLow["reasoning_effort"] != "low" {
		t.Fatalf("low maps to low: %v", bodyLow["reasoning_effort"])
	}
	bodyMed := map[string]any{}
	applyProtocolDialectRequest(bodyMed, domain.ProtocolDialectDeepSeek, "medium", nil)
	if bodyMed["reasoning_effort"] != "high" {
		t.Fatalf("medium maps to high: %v", bodyMed["reasoning_effort"])
	}

	bodyOff := map[string]any{}
	applyProtocolDialectRequest(bodyOff, domain.ProtocolDialectDeepSeek, "off", nil)
	if bodyOff["thinking"].(map[string]any)["type"] != "disabled" {
		t.Fatalf("off: %v", bodyOff["thinking"])
	}
	if _, ok := bodyOff["reasoning_effort"]; ok {
		t.Fatal("off must not set reasoning_effort")
	}
}

func TestApplyProtocolDialectRequest_KimiCode(t *testing.T) {
	body := map[string]any{}
	applyProtocolDialectRequest(body, domain.ProtocolDialectKimiCode, "off", nil)
	th := body["thinking"].(map[string]any)
	if th["type"] != "enabled" || th["keep"] != "all" {
		t.Fatalf("always on: %v", th)
	}
}

func TestApplyProtocolDialectRequest_Grok(t *testing.T) {
	body := map[string]any{}
	applyProtocolDialectRequest(body, domain.ProtocolDialectGrok, "medium", nil)
	if body["reasoning_effort"] != "medium" {
		t.Fatalf("got %v", body["reasoning_effort"])
	}
	bodyOff := map[string]any{}
	applyProtocolDialectRequest(bodyOff, domain.ProtocolDialectGrok, "off", nil)
	if bodyOff["reasoning_effort"] != "low" {
		t.Fatalf("off→low: %v", bodyOff["reasoning_effort"])
	}
}

func TestApplyProtocolDialectRequest_GLM(t *testing.T) {
	body := map[string]any{}
	applyProtocolDialectRequest(body, domain.ProtocolDialectGLM, "high", nil)
	th, ok := body["thinking"].(map[string]any)
	if !ok || th["type"] != "enabled" || th["clear_thinking"] != false {
		t.Fatalf("thinking: %v", body["thinking"])
	}
	if body["reasoning_effort"] != "high" {
		t.Fatalf("reasoning_effort: %v", body["reasoning_effort"])
	}

	bodyOff := map[string]any{}
	applyProtocolDialectRequest(bodyOff, domain.ProtocolDialectGLM, "off", nil)
	thOff := bodyOff["thinking"].(map[string]any)
	if thOff["type"] != "disabled" {
		t.Fatalf("disabled: %v", thOff)
	}
	if _, ok := bodyOff["reasoning_effort"]; ok {
		t.Fatal("off must not set reasoning_effort")
	}
}

func TestApplyProtocolDialectRequest_MiniMax(t *testing.T) {
	body := map[string]any{}
	applyProtocolDialectRequest(body, domain.ProtocolDialectMiniMax, "high", nil)
	if body["reasoning_split"] != true {
		t.Fatalf("reasoning_split: %v", body["reasoning_split"])
	}
	th := body["thinking"].(map[string]any)
	if th["type"] != "adaptive" {
		t.Fatalf("thinking: %v", th)
	}

	bodyOff := map[string]any{}
	applyProtocolDialectRequest(bodyOff, domain.ProtocolDialectMiniMax, "off", nil)
	if bodyOff["reasoning_split"] != true {
		t.Fatal("reasoning_split should stay on for format")
	}
	if bodyOff["thinking"].(map[string]any)["type"] != "disabled" {
		t.Fatalf("disabled: %v", bodyOff["thinking"])
	}
}

func TestApplyProtocolDialectRequest_Gemini(t *testing.T) {
	body := map[string]any{}
	applyProtocolDialectRequest(body, domain.ProtocolDialectGemini, "medium", nil)
	if body["reasoning_effort"] != "medium" {
		t.Fatalf("reasoning_effort: %v", body["reasoning_effort"])
	}
	if _, ok := body["extra_body"]; ok {
		t.Fatal("must not nest SDK-only extra_body")
	}

	bodyOff := map[string]any{}
	applyProtocolDialectRequest(bodyOff, domain.ProtocolDialectGemini, "off", nil)
	if bodyOff["reasoning_effort"] != "none" {
		t.Fatalf("off: %v", bodyOff["reasoning_effort"])
	}
}

func TestNormalizeChatCompletionsReasoning_MiniMaxThinkTags(t *testing.T) {
	content := "<think>plan</think>\nanswer"
	ans, reason := normalizeChatCompletionsReasoning(content, "", "", domain.ProtocolDialectMiniMax)
	if reason != "plan" {
		t.Fatalf("reason: %q", reason)
	}
	if ans != "answer" {
		t.Fatalf("answer: %q", ans)
	}
}

func TestResolveProtocolDialect(t *testing.T) {
	if got := resolveProtocolDialect(&port.ModelGenParams{ProtocolDialect: "qwen"}, "x"); got != "qwen" {
		t.Fatalf("explicit: %s", got)
	}
	if got := resolveProtocolDialect(nil, "deepseek-chat"); got != domain.ProtocolDialectDeepSeek {
		t.Fatalf("infer deepseek: %s", got)
	}
	if got := resolveProtocolDialect(nil, "glm-4.7"); got != domain.ProtocolDialectGLM {
		t.Fatalf("infer glm: %s", got)
	}
	if got := resolveProtocolDialect(nil, "MiniMax-M3"); got != domain.ProtocolDialectMiniMax {
		t.Fatalf("infer minimax: %s", got)
	}
	if got := resolveProtocolDialect(nil, "k3-256k"); got != domain.ProtocolDialectKimiK3 {
		t.Fatalf("infer k3-256k: %s", got)
	}
	if got := resolveProtocolDialect(nil, "kimi-for-coding"); got != domain.ProtocolDialectKimiK3 {
		t.Fatalf("infer kimi_k3 for kimi-for-coding: %s", got)
	}
	if got := resolveProtocolDialect(nil, "kimi-for-coding-highspeed"); got != domain.ProtocolDialectKimiCode {
		t.Fatalf("infer kimi_code for highspeed: %s", got)
	}
	if got := resolveProtocolDialect(nil, "grok-4.5"); got != domain.ProtocolDialectGrok {
		t.Fatalf("infer grok: %s", got)
	}
	if got := resolveProtocolDialect(nil, "gpt-4o"); got != domain.ProtocolDialectOpenAI {
		t.Fatalf("default openai: %s", got)
	}
}

func TestDialectEchoesReasoning(t *testing.T) {
	if !dialectEchoesReasoning(domain.ProtocolDialectDeepSeek) {
		t.Fatal("deepseek should echo")
	}
	if !dialectEchoesReasoning(domain.ProtocolDialectGLM) {
		t.Fatal("glm should echo")
	}
	if !dialectEchoesReasoning(domain.ProtocolDialectMiniMax) {
		t.Fatal("minimax should echo")
	}
	if !dialectEchoesReasoning(domain.ProtocolDialectKimiK3) {
		t.Fatal("kimi_k3 should echo")
	}
	if dialectEchoesReasoning(domain.ProtocolDialectOpenAI) {
		t.Fatal("openai should not echo")
	}
	if dialectEchoesReasoning(domain.ProtocolDialectGemini) {
		t.Fatal("gemini should not echo reasoning_content by default")
	}
}

func TestOpenAIChatCompletions_EchoesReasoningContent(t *testing.T) {
	echo := dialectEchoesReasoning(domain.ProtocolDialectKimi)
	if !echo {
		t.Fatal("expected echo")
	}
}
