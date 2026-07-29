import type { Agent } from '@/types'

/** Parse agent id from delegate_agent tool input JSON. */
export function parseDelegateAgentId(inputStr?: string): string {
  if (!inputStr?.trim()) return ''
  try {
    const obj = JSON.parse(inputStr) as Record<string, unknown>
    const id = obj.agent_id ?? obj.agentId ?? obj.agent
    return typeof id === 'string' ? id.trim() : ''
  } catch {
    return ''
  }
}

/** Resolve a display label for a tool card name (friendly delegate title). */
export function friendlyToolDisplayName(
  name: string,
  inputStr: string | undefined,
  agents: Agent[],
  t: (key: string, values?: Record<string, unknown>) => string,
): string {
  if (name !== 'delegate_agent') return name
  const id = parseDelegateAgentId(inputStr)
  const agent = id ? agents.find((a) => a.id === id) : undefined
  const label = agent?.name?.trim() || id
  if (label) return t('sessions.summonExpert', { name: label })
  return t('sessions.summonExpertFallback')
}
