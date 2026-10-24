package llm

import (
	"regexp"
	"strings"

	"danmo-work/core/domain"
	"danmo-work/core/port"
)

var thinkTagRe = regexp.MustCompile(`(?is)<think>(.*?)</think>`)

// resolveProtocolDialect returns a concrete protocol dialect for Chat Completions.
func resolveProtocolDialect(gp *port.ModelGenParams, model string) string {
	if gp != nil && gp.ProtocolDialect != "" {
		return gp.ProtocolDialect
	}
	if d := domain.InferProtocolDialect(model); d != "" {
		return d
	}
	return domain.ProtocolDialectOpenAI
}

// applyProtocolDialectRequest mutates the Chat Completions body with
// vendor-specific thinking fields. See PROTOCOL_DIALECTS.md for the matrix.
func applyProtocolDialectRequest(body map[string]any, dialect, effort string, gp *port.ModelGenParams) {
	on := effort != "" && effort != "off"
	switch dialect {
	case domain.ProtocolDialectQwen:
		// Alibaba Model Studio / DashScope compatible-mode.
		body["enable_thinking"] = on
		if on && gp != nil && len(gp.EffortBudgetTokens) > 0 {
			if budget := gp.EffortBudgetTokens[effort]; budget > 0 {
				body["thinking_budget"] = budget
			}
		}
	case domain.ProtocolDialectKimi:
		// platform.kimi.ai K2.6 / K2.5 — thinking.type + keep for preserved CoT.
		if on {
			body["thinking"] = map[string]any{
				"type": "enabled",
				"keep": "all",
			}
		} else {
			body["thinking"] = map[string]any{"type": "disabled"}
		}
	case domain.ProtocolDialectKimiCode:
		// K2.7 Code / kimi-for-coding-highspeed — always on; disabled is rejected by API.
		body["thinking"] = map[string]any{
			"type": "enabled",
			"keep": "all",
		}
	case domain.ProtocolDialectKimiK3:
		// K3 / K2.8 Preview (kimi-for-coding): thinking always on; control via reasoning_effort.
		e := effort
		if !on {
			e = "low"
		}
		body["reasoning_effort"] = e
	case domain.ProtocolDialectGLM:
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
	case domain.ProtocolDialectMiniMax:
		// platform.minimax.io — reasoning_split exposes reasoning_content.
		body["reasoning_split"] = true
		if on {
			body["thinking"] = map[string]any{"type": "adaptive"}
		} else {
			body["thinking"] = map[string]any{"type": "disabled"}
		}
	case domain.ProtocolDialectGemini:
		if on {
			body["reasoning_effort"] = effort
		} else {
			body["reasoning_effort"] = "none"
		}
	case domain.ProtocolDialectDeepSeek:
		// api-docs.deepseek.com — thinking.type + reasoning_effort low|high|max.
		if on {
			body["thinking"] = map[string]any{"type": "enabled"}
			body["reasoning_effort"] = mapDeepSeekEffort(effort)
		} else {
			body["thinking"] = map[string]any{"type": "disabled"}
		}
	case domain.ProtocolDialectGrok:
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
	// Official mapping (Thinking Mode): minimal→low, low→low, medium/high/xhigh→high, max/ultra→max.
	switch strings.ToLower(effort) {
	case "minimal", "low":
		return "low"
	case "max", "ultra":
		return "max"
	default:
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
	case domain.ProtocolDialectDeepSeek,
		domain.ProtocolDialectQwen,
		domain.ProtocolDialectKimi,
		domain.ProtocolDialectKimiCode,
		domain.ProtocolDialectKimiK3,
		domain.ProtocolDialectGLM,
		domain.ProtocolDialectMiniMax:
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
	if reasoningOut == "" && (dialect == domain.ProtocolDialectMiniMax || dialect == domain.ProtocolDialectDeepSeek) {
		if m := thinkTagRe.FindStringSubmatch(content); len(m) == 2 {
			reasoningOut = strings.TrimSpace(m[1])
			answer = strings.TrimSpace(thinkTagRe.ReplaceAllString(content, ""))
		}
	}
	return answer, reasoningOut
}
