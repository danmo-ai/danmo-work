/** Resolve a --dq-font-size-* CSS variable to a pixel number (for APIs that need px). */
export function dqFontSizePx(
  token: '--dq-font-size-caption' | '--dq-font-size-body' | '--dq-font-size-prose' | '--dq-font-size-title',
  fallback = 14,
): number {
  if (typeof document === 'undefined') return fallback
  const raw = getComputedStyle(document.documentElement).getPropertyValue(token).trim()
  if (!raw) return fallback
  if (raw.endsWith('rem')) {
    const root = parseFloat(getComputedStyle(document.documentElement).fontSize) || 16
    return Math.round(parseFloat(raw) * root)
  }
  if (raw.endsWith('px')) return Math.round(parseFloat(raw))
  const n = parseFloat(raw)
  return Number.isFinite(n) ? Math.round(n) : fallback
}
