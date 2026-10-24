package llm

import "danmo-work/core/domain"

func ApplyReasoningEffort(provider domain.LLMProviderType, effort string, genParams *EffortConfig, body map[string]any) {
	if effort == "" || effort == "off" {
		return
	}
	switch provider {
	case domain.LLMProviderAnthropic:
		mode := ""
		if genParams != nil {
			mode = genParams.ThinkingMode
		}
		// Default (empty or adaptive): Anthropic adaptive thinking + effort.
		if mode == "" || mode == "adaptive" {
			body["thinking"] = map[string]any{
				"type": "adaptive",
			}
			body["effort"] = effort
			body["display"] = "summarized"
			return
		}
		var budget int
		if genParams != nil {
			budget = genParams.EffortBudgetTokens[effort]
		}
		if budget == 0 {
			budget = defaultAnthropicEffortBudgets()[effort]
		}
		if budget == 0 {
			return
		}
		body["thinking"] = map[string]any{
			"type":          "enabled",
			"budget_tokens": budget,
		}
	default:
		body["reasoning_effort"] = effort
	}
}

type EffortConfig struct {
	ThinkingMode       string
	EffortBudgetTokens map[string]int
}
