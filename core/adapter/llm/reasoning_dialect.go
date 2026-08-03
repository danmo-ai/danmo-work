package llm

import (
	"regexp"
	"strings"

	"danmo-work/core/domain"
	"danmo-work/core/port"
)

var thinkTagRe = regexp.MustCompile(`(?is)<think>(.*?)</think>`)

// resolveReasoningDialect returns a concrete dialect for Chat Completions.
func resolveReasoningDialect(gp *port.ModelGenParams, model string) string {
	if gp != nil && gp.ReasoningDialect != "" {
		return gp.ReasoningDialect
	}
	if d := domain.InferReasoningDialect(model); d != "" {
		return d
	}
	return domain.ReasoningDialectOpenAI
}

// applyReasoningDialectRequest mutates the Chat Completions body with
// vendor-specific thinking fields. See REASONING_DIALECTS.md for the matrix.
func applyReasoningDialectRequest(body map[string]any, dialect, effort string, gp *port.ModelGenParams) {
	on := effort != "" && effort != "off"
	switch dialect {
	case domain.ReasoningDialectQwen:
		// Alibaba Model Studio / DashScope compatible-mode.
		body["enable_thinking"] = on
		if on && gp != nil && len(gp.EffortBudgetTokens) > 0 {
			if budget := gp.EffortBudgetTokens[effort]; budget > 0 {
				body["thinking_budget"] = budget
			}
		}
	case domain.ReasoningDialectKimi:
		// platform.kimi.ai K2.6 / K2.5 — thinking.type + keep for preserved CoT.
		if on {
			body["thinking"] = map[string]any{
				"type": "enabled",
				"keep": "all",
			}
		} else {
			body["thinking"] = map[string]any{"type": "disabled"}
		}
	case domain.ReasoningDialectKimiCode:
		// K2.7 Code / kimi-for-coding — always on; disabled is rejected by API.
		body["thinking"] = map[string]any{
			"type": "enabled",
			"keep": "all",
		}
	case domain.ReasoningDialectKimiK3:
		// Kimi K3 (k3 / k3-256k / kimi-k3): thinking always on; control via reasoning_effort.
		e := effort
		if !on {
			e = "low"
		}
		body["reasoning_effort"] = e
	case domain.ReasoningDialectGLM:
		// docs.z.ai — thinking.type; clear_thinking:false preserves CoT for tools.
		if on {
			body["thinking"] = map[string]any{
				"type":           "enabled",
				"clear_thinking": false,
			}
			body["reasoning_effort"] = mapGLMEffort(effort)
		} else {
			body["thinking"] = map[string]any{"type": "disabled"}
		}
	case domain.ReasoningDialectMiniMax:
		// platform.minimax.io — reasoning_split exposes reasoning_content.
		body["reasoning_split"] = true
		if on {
			body["thinking"] = map[string]any{"type": "adaptive"}
		} else {
			body["thinking"] = map[string]any{"type": "disabled"}
		}
	case domain.ReasoningDialectGemini:
		if on {
			body["reasoning_effort"] = effort
		} else {
			body["reasoning_effort"] = "none"
		}
	case domain.ReasoningDialectDeepSeek:
		// api-docs.deepseek.com — V4: thinking.type + reasoning_effort high|max.
		if on {
			body["thinking"] = map[string]any{"type": "enabled"}
			body["reasoning_effort"] = mapDeepSeekEffort(effort)
		} else {
			body["thinking"] = map[string]any{"type": "disabled"}
		}
	case domain.ReasoningDialectGrok:
		// docs.x.ai — reasoning_effort; grok-4.5 cannot disable (off→low).
		// grok-4.3 accepts none|low|medium|high — pass through when not off.
		if effort == "" || effort == "off" {
			body["reasoning_effort"] = "low"
		} else {
			body["reasoning_effort"] = effort
		}
	default: // openai
		if on {
			body["reasoning_effort"] = effort
		}
	}
}

func mapDeepSeekEffort(effort string) string {
	switch strings.ToLower(effort) {
	case "max", "xhigh":
		return "max"
	default:
		// Official V4 documents high | max; map low/medium → high.
		return "high"
	}
}

func mapGLMEffort(effort string) string {
	switch strings.ToLower(effort) {
	case "max", "xhigh":
		return "max"
	case "minimal", "none", "low", "medium":
		// Z.AI maps low/medium → high; none/minimal skip thinking (handled by type).
		return "high"
	default:
		return effort
	}
}

// dialectEchoesReasoning reports whether assistant messages must carry
// reasoning_content when talking to this dialect (tool multi-turn).
func dialectEchoesReasoning(dialect string) bool {
	switch dialect {
	case domain.ReasoningDialectDeepSeek,
		domain.ReasoningDialectQwen,
		domain.ReasoningDialectKimi,
		domain.ReasoningDialectKimiCode,
		domain.ReasoningDialectKimiK3,
		domain.ReasoningDialectGLM,
		domain.ReasoningDialectMiniMax:
		return true
	default:
		return false
	}
}

// normalizeChatCompletionsReasoning picks reasoning text from message fields and,
// for MiniMax native format, strips <think>…</think> out of content when needed.
func normalizeChatCompletionsReasoning(content, reasoningContent, reasoning string, dialect string) (answer, reasoningOut string) {
	reasoningOut = strings.TrimSpace(reasoningContent)
	if reasoningOut == "" {
		reasoningOut = strings.TrimSpace(reasoning)
	}
	answer = content
	if reasoningOut == "" && (dialect == domain.ReasoningDialectMiniMax || dialect == domain.ReasoningDialectDeepSeek) {
		if m := thinkTagRe.FindStringSubmatch(content); len(m) == 2 {
			reasoningOut = strings.TrimSpace(m[1])
			answer = strings.TrimSpace(thinkTagRe.ReplaceAllString(content, ""))
		}
	}
	return answer, reasoningOut
}
