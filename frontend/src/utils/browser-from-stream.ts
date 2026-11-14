import type { StreamEvent } from '@/types/mission'

const OPEN_TOOLS = new Set(['browser_navigate', 'browser_act', 'browser_snapshot', 'browser_screenshot'])

export function isBrowserActivityEvent(event: StreamEvent): boolean {
  if (event.type !== 'tool.running') return false
  const payload = event.payload as { name?: unknown } | null
  return typeof payload?.name === 'string' && OPEN_TOOLS.has(payload.name)
}

export function isBrowserCloseEvent(event: StreamEvent): boolean {
  if (event.type !== 'tool.completed' && event.type !== 'tool.error') return false
  const payload = event.payload as { name?: unknown } | null
  return payload?.name === 'browser_close'
}
