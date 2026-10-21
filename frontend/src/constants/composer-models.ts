// Composer model constants removed — models are now loaded dynamically from LLM store
export interface ComposerModelOption {
  id: string
  label: string
}

export const COMPOSER_MODELS: ComposerModelOption[] = [
  { id: 'openai/gpt-5.6', label: 'openai/gpt-5.6' },
  { id: 'anthropic/claude-sonnet-5', label: 'anthropic/claude-sonnet-5' },
  { id: 'deepseek/deepseek-flash', label: 'deepseek/deepseek-flash' },
]

export const DEFAULT_COMPOSER_MODEL_ID = COMPOSER_MODELS[0].id
