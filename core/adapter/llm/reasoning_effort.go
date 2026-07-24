package llm

import "danmo-work/core/domain"

func ApplyReasoningEffort(provider domain.LLMProviderType, effort string, genParams *EffortConfig, body map[string]any) {
	if effort == "" || effort == "off" {
		return
	}
	switch provider {
	case domain.LLMProviderAnthropic:
		if genParams != nil && genParams.ThinkingMode == "adaptive" {
			body["thinking"] = map[string]any{
				"type": "adaptive",
			}
			body["effort"] = effort
			body["display"] = "summarized"
		} else {
			var budget int
			if genParams != nil {
				budget = genParams.EffortBudgetTokens[effort]
			}
			if budget == 0 {
				return
			}
			body["thinking"] = map[string]any{
				"type":          "enabled",
				"budget_tokens": budget,
			}
		}
	default:
		body["reasoning_effort"] = effort
	}
}

type EffortConfig struct {
	ThinkingMode       string
	EffortBudgetTokens map[string]int
}
