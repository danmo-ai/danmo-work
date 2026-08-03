# Reasoning dialects (Chat Completions)

Source of truth for OpenAI-compatible **thinking / reasoning** request shapes.
Anthropic uses `thinking_mode` + Messages API — not listed here.

When adding a vendor model, pick a dialect below (or add a new row + client branch).
Do **not** rely on silent `openai` fallback for vendors with custom fields.

| Dialect ID | Vendors / model IDs | Enable thinking | Effort / intensity | Echo `reasoning_content` | Official notes |
|---|---|---|---|---|---|
| `openai` | OpenAI GPT-5 / o-series; many gateways | `reasoning_effort` when on | `none` `minimal` `low` `medium` `high` `xhigh` `max` (model-dependent) | No | [Reasoning](https://platform.openai.com/docs/guides/reasoning) |
| `deepseek` | `deepseek-v4-*`, `deepseek-chat`, `deepseek-reasoner` | `thinking.type` = `enabled` / `disabled` | When on: `reasoning_effort` `high` \| `max` (map low/medium→high) | Yes (tool / multi-turn) | [api-docs.deepseek.com](https://api-docs.deepseek.com/) |
| `qwen` | Qwen3.x via DashScope compatible-mode | `enable_thinking` bool | Optional `thinking_budget` from `effort_budget_tokens` | Yes | [Thinking](https://docs.qwencloud.com/developer-guides/text-generation/thinking) |
| `kimi` | `kimi-k2.6`, `kimi-k2.5` | `thinking.type` + `keep: all` when on | Toggle only (no effort field) | Yes when preserved | [models-overview](https://platform.kimi.ai/docs/api/models-overview) |
| `kimi_code` | `kimi-k2.7-code*`, `kimi-for-coding*` | Always on; only `enabled`+`keep:all` (or omit). **`disabled` errors** | N/A | Yes (required) | Same as above — K2.7 Code |
| `kimi_k3` | `kimi-k3`, Kimi Code `k3`, `k3-256k` | Always on; **no** `thinking` object | `reasoning_effort` `low` \| `high` \| `max` (default max). `off`→`low` | Yes (required) | [K3 quickstart](https://platform.kimi.ai/docs/guide/kimi-k3-quickstart) |
| `glm` | Zhipu / Z.AI `glm-*` | `thinking.type`; `clear_thinking: false` for agents | Optional `reasoning_effort` (GLM-5.2+) | Yes | [Deep Thinking](https://docs.z.ai/guides/capabilities/thinking) |
| `minimax` | `MiniMax-M*` | `thinking.type` `adaptive` / `disabled`; always `reasoning_split: true` | Toggle (+ format split) | Yes | [OpenAI API](https://platform.minimax.io/docs/api-reference/text-openai-api) |
| `gemini` | Google Gemini OpenAI compat | `reasoning_effort` (`none` when off) | `none` `minimal` `low` `medium` `high` | No | Do not mix with `thinking_config` | [OpenAI compatibility](https://ai.google.dev/gemini-api/docs/openai) |
| `grok` | xAI `grok-*` | `reasoning_effort` | `low` `medium` `high` (4.5 cannot disable → `off`→`low`); 4.3 also `none` | No | [xAI reasoning](https://docs.x.ai/developers/model-capabilities/text/reasoning) |

## Inference rules (`InferReasoningDialect`)

Order matters (first match wins):

1. `deepseek` → `deepseek`
2. `qwen` → `qwen`
3. `k3` / `k3-256k` / `kimi-k3` / `k3-*` → `kimi_k3`
4. `kimi-k2.7` / `kimi-for-coding` → `kimi_code`
5. `kimi` / `moonshot` → `kimi`
6. `glm` / `zhipu` / `chatglm` → `glm`
7. `minimax` → `minimax`
8. `gemini` → `gemini`
9. `grok` → `grok`
10. else empty → runtime defaults to `openai`

## Checklist for a new vendor dialect

1. Read official Chat Completions (or compatible) docs for thinking fields.
2. Add constant + row in this file + `domain.ReasoningDialectInfos`.
3. Branch in `applyReasoningDialectRequest` / `dialectEchoesReasoning`.
4. Tag models in `default_models.yaml` + `config.example.yaml`.
5. Extend `InferReasoningDialect` and unit tests.
6. Add Settings dropdown + i18n labels.
