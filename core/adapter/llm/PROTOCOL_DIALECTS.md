# Protocol dialects (Chat Completions)

Source of truth for OpenAI-compatible **protocol dialects**: vendor wire shapes for
thinking / reasoning fields on the Chat Completions protocol.
Anthropic Messages uses built-in adaptive thinking + Composer effort — not listed here.

A **protocol** (`openai` / `openai_responses` / `anthropic`) selects the HTTP client.
A **protocol dialect** further specializes how one protocol encodes vendor-specific
fields (here: thinking). Dialects only apply to Chat Completions (`openai`).

## Where dialect is configured

| Layer | Field | Role |
|-------|--------|------|
| Provider (SQLite `llm_configs`) | `defaultProtocolDialect` (`default_dialect` column) | Default for models without override |
| Model row (`models[]`) | `protocolDialect` | Optional per-model override (empty = inherit) |
| Runtime | `ResolveProtocolDialect` | model → provider default → `Infer(model)` → `openai` |

Model **params** catalog (`llm.models` YAML) keeps capacity, vision, sampling, and
`available_efforts` only — not dialect.

When adding a vendor model, pick a dialect below (or add a new row + client branch).
Do **not** rely on silent `openai` fallback for vendors with custom fields.

| Dialect ID | Vendors / model IDs | Enable thinking | Effort / intensity | Echo `reasoning_content` | Official notes |
|---|---|---|---|---|---|
| `openai` | OpenAI GPT-5 / o-series; many gateways | `reasoning_effort` when on | `none` `minimal` `low` `medium` `high` `xhigh` `max` (model-dependent) | No | [Reasoning](https://platform.openai.com/docs/guides/reasoning) |
| `deepseek` | `deepseek-flash`, `deepseek-v4-pro`, legacy `deepseek-v4-flash*` / `deepseek-chat` / `deepseek-reasoner` | `thinking.type` = `enabled` / `disabled` | When on: `reasoning_effort` `low` \| `high` \| `max` (minimal→low; medium/xhigh→high; ultra→max) | Yes (tool / multi-turn) | [Thinking Mode](https://api-docs.deepseek.com/guides/thinking_mode) |
| `qwen` | Qwen3.x via DashScope compatible-mode | `enable_thinking` bool | Optional `thinking_budget` from built-in effort→budget map | Yes | [Thinking](https://docs.qwencloud.com/developer-guides/text-generation/thinking) |
| `kimi` | `kimi-k2.6`, `kimi-k2.5` | `thinking.type` + `keep: all` when on | Toggle only (no effort field) | Yes when preserved | [models-overview](https://platform.kimi.ai/docs/api/models-overview) |
| `kimi_code` | `kimi-k2.7-code*`, `kimi-for-coding-highspeed` | Always on; only `enabled`+`keep:all` (or omit). **`disabled` errors** | N/A | Yes (required) | [Kimi Code models](https://www.kimi.com/code/docs/en/kimi-code/models.html) — K2.7 Code HighSpeed |
| `kimi_k3` | `kimi-k3`, Kimi Code `k3`, `k3-256k`, `kimi-for-coding` (K2.8 Preview), `kimi-k2.8*` | Always on; **no** `thinking` object | `reasoning_effort` `low` \| `high` \| `max` (K3 default high; K2.8 default max). `off`→`low` | Yes (required) | [Kimi Code models](https://www.kimi.com/code/docs/en/kimi-code/models.html) / [K3 quickstart](https://platform.kimi.ai/docs/guide/kimi-k3-quickstart) |
| `glm` | Zhipu / Z.AI `glm-*` | `thinking.type`; `clear_thinking: false` for agents | Optional `reasoning_effort` (GLM-5.2+) | Yes | [Deep Thinking](https://docs.z.ai/guides/capabilities/thinking) |
| `minimax` | `MiniMax-M*` | `thinking.type` `adaptive` / `disabled`; always `reasoning_split: true` | Toggle (+ format split) | Yes | [OpenAI API](https://platform.minimax.io/docs/api-reference/text-openai-api) |
| `gemini` | Google Gemini OpenAI compat | `reasoning_effort` (`none` when off) | `none` `minimal` `low` `medium` `high` | No | [OpenAI compatibility](https://ai.google.dev/gemini-api/docs/openai) — do not mix with `thinking_config` |
| `grok` | xAI `grok-*` | `reasoning_effort` | `low` `medium` `high` (4.5 cannot disable → `off`→`low`); 4.3 also `none` | No | [xAI reasoning](https://docs.x.ai/developers/model-capabilities/text/reasoning) |

## Resolution (`ResolveProtocolDialect`)

For a provider×model pair:

1. Model row `protocolDialect` if set
2. Provider `defaultProtocolDialect` if set
3. `InferProtocolDialect(modelName)`
4. else `openai`

## Inference rules (`InferProtocolDialect`)

Order matters (first match wins):

1. `deepseek` → `deepseek`
2. `qwen` → `qwen`
3. `k3` / `k3-256k` / `kimi-for-coding` / `kimi-k3` / `kimi-k2.8` / `k3-*` → `kimi_k3`
4. `kimi-k2.7` / `kimi-for-coding-highspeed` → `kimi_code`
5. `kimi` / `moonshot` → `kimi`
6. `glm` / `zhipu` / `chatglm` → `glm`
7. `minimax` → `minimax`
8. `gemini` → `gemini`
9. `grok` → `grok`
10. else empty → runtime defaults to `openai`

## Checklist for a new vendor dialect

1. Read official Chat Completions (or compatible) docs for thinking fields.
2. Add constant + row in this file + `domain.ProtocolDialectInfos`.
3. Branch in `applyProtocolDialectRequest` / `dialectEchoesReasoning`.
4. Set preset `default_protocol_dialect` (and optional per-model override) — not `default_models.yaml`.
5. Extend `InferProtocolDialect` and unit tests.
6. Add Settings dialect dropdown + i18n labels.
