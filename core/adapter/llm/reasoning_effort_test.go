package llm

import (
	"testing"

	"danmo-work/core/domain"
)

func TestApplyReasoningEffort_AnthropicAdaptiveDefault(t *testing.T) {
	body := map[string]any{}
	ApplyReasoningEffort(domain.LLMProviderAnthropic, "high", nil, body)
	th, _ := body["thinking"].(map[string]any)
	if th["type"] != "adaptive" {
		t.Fatalf("thinking: %+v", th)
	}
	if body["effort"] != "high" {
		t.Fatalf("effort: %v", body["effort"])
	}
}

func TestApplyReasoningEffort_AnthropicEmptyModeIsAdaptive(t *testing.T) {
	body := map[string]any{}
	ApplyReasoningEffort(domain.LLMProviderAnthropic, "max", &EffortConfig{ThinkingMode: ""}, body)
	th, _ := body["thinking"].(map[string]any)
	if th["type"] != "adaptive" {
		t.Fatalf("thinking: %+v", th)
	}
}

func TestApplyReasoningEffort_AnthropicEnabledUsesBuiltinBudget(t *testing.T) {
	body := map[string]any{}
	ApplyReasoningEffort(domain.LLMProviderAnthropic, "high", &EffortConfig{ThinkingMode: "enabled"}, body)
	th, _ := body["thinking"].(map[string]any)
	if th["type"] != "enabled" {
		t.Fatalf("thinking: %+v", th)
	}
	if th["budget_tokens"] != 16000 {
		t.Fatalf("budget: %v", th["budget_tokens"])
	}
}

func TestApplyReasoningEffort_OffSkips(t *testing.T) {
	body := map[string]any{}
	ApplyReasoningEffort(domain.LLMProviderAnthropic, "off", nil, body)
	if _, ok := body["thinking"]; ok {
		t.Fatalf("unexpected thinking: %+v", body)
	}
}
