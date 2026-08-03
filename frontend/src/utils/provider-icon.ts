/**
 * Unified LLM provider badge (abbr + color).
 * One generation path for presets and saved configs — no emoji icons.
 */

/** Fixed design-token palette; known vendors pick a slot, others hash into it. */
export const PROVIDER_COLOR_PALETTE = [
  'var(--dq-accent)',
  'var(--dq-success)',
  'var(--dq-info)',
  'var(--dq-warning)',
  'var(--dq-system-orange)',
  'var(--dq-system-blue)',
  'var(--dq-danger)',
] as const

type ProviderVisual = {
  /** 1–3 letter badge text */
  abbr: string
  /** Index into PROVIDER_COLOR_PALETTE */
  colorIndex: number
}

/** Curated visuals for built-in vendor ids (preset.id). */
const KNOWN_PROVIDERS: Record<string, ProviderVisual> = {
  openai: { abbr: 'OA', colorIndex: 1 },
  anthropic: { abbr: 'AN', colorIndex: 3 },
  deepseek: { abbr: 'DS', colorIndex: 0 },
  google: { abbr: 'G', colorIndex: 2 },
  zhipu: { abbr: 'GLM', colorIndex: 6 },
  qwen: { abbr: 'QW', colorIndex: 4 },
  moonshot: { abbr: 'K', colorIndex: 5 },
  minimax: { abbr: 'MM', colorIndex: 4 },
  ollama: { abbr: 'OL', colorIndex: 5 },
  siliconflow: { abbr: 'SF', colorIndex: 0 },
  openrouter: { abbr: 'OR', colorIndex: 2 },
  together: { abbr: 'TG', colorIndex: 1 },
  fireworks: { abbr: 'FW', colorIndex: 4 },
  groq: { abbr: 'GQ', colorIndex: 3 },
  deepinfra: { abbr: 'DI', colorIndex: 6 },
  xai: { abbr: 'X', colorIndex: 0 },
}

function normalizeKey(raw: string): string {
  return raw.trim().toLowerCase().replace(/^llm-/, '').replace(/[\s_]+/g, '')
}

function hashHue(s: string): number {
  let h = 0
  for (let i = 0; i < s.length; i++) {
    h = (h * 31 + s.charCodeAt(i)) >>> 0
  }
  return h
}

function resolveKnown(idOrName: string): ProviderVisual | null {
  const key = normalizeKey(idOrName)
  if (KNOWN_PROVIDERS[key]) return KNOWN_PROVIDERS[key]
  for (const [id, meta] of Object.entries(KNOWN_PROVIDERS)) {
    if (key.includes(id) || id.includes(key)) return meta
  }
  return null
}

function initialsFrom(raw: string): string {
  const s = raw.trim()
  if (!s) return '?'
  const cleaned = s.replace(/^llm-/i, '')
  const parts = cleaned.split(/[\s\-_/]+/).filter(Boolean)
  if (parts.length >= 2) {
    return (parts[0][0] + parts[1][0]).toUpperCase()
  }
  const alnum = cleaned.replace(/[^a-zA-Z0-9\u4e00-\u9fff]/g, '')
  if (!alnum) return '?'
  if (/[\u4e00-\u9fff]/.test(alnum)) {
    return alnum.slice(0, 1)
  }
  return alnum.slice(0, Math.min(3, alnum.length)).toUpperCase()
}

export type ProviderBadge = {
  abbr: string
  color: string
}

/**
 * Badge for a provider preset or saved config.
 * Prefer stable `id` (preset id or config id); `name` is fallback for initials.
 */
export function providerBadge(id: string, name?: string): ProviderBadge {
  const known = resolveKnown(id) ?? (name ? resolveKnown(name) : null)
  if (known) {
    return {
      abbr: known.abbr,
      color: PROVIDER_COLOR_PALETTE[known.colorIndex % PROVIDER_COLOR_PALETTE.length],
    }
  }
  const seed = normalizeKey(id || name || '?')
  const idx = hashHue(seed) % PROVIDER_COLOR_PALETTE.length
  return {
    abbr: initialsFrom(name || id),
    color: PROVIDER_COLOR_PALETTE[idx],
  }
}

/** Custom / manual provider entry in the picker. */
export function customProviderBadge(): ProviderBadge {
  return {
    abbr: '+',
    color: 'var(--dq-label-tertiary)',
  }
}
