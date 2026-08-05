import type { Agent } from '@/types'

/** Subagents the lead may summon (excludes current lead id). */
export function listSummonableExperts(agents: Agent[], excludeId: string | null | undefined): Agent[] {
  const exclude = (excludeId ?? '').trim()
  return agents
    .filter((a) => a.mode === 'subagent' && a.id !== exclude)
    .slice()
    .sort((a, b) => a.name.localeCompare(b.name, undefined, { sensitivity: 'base' }))
}

export function filterSummonableExperts(
  agents: Agent[],
  query: string,
  excludeId: string | null | undefined,
): Agent[] {
  const list = listSummonableExperts(agents, excludeId)
  const q = query.trim().toLowerCase()
  if (!q) return list
  return list.filter((a) => {
    const hay = [a.id, a.name, a.description ?? '', a.persona ?? ''].join('\n').toLowerCase()
    return hay.includes(q)
  })
}

/** Prefix user input so the model delegates via delegate_agent. */
export function buildExpertSummonPrefix(
  experts: Agent[],
  useExpertLine: (name: string, id: string) => string,
  delegateHint: string,
): string {
  if (!experts.length) return ''
  const lines = experts.map((a) => useExpertLine(a.name || a.id, a.id))
  return `${lines.join('\n')}\n${delegateHint}\n\n`
}

export function prependExpertSummon(
  userInput: string,
  experts: Agent[],
  useExpertLine: (name: string, id: string) => string,
  delegateHint: string,
): string {
  const prefix = buildExpertSummonPrefix(experts, useExpertLine, delegateHint)
  if (!prefix) return userInput
  return userInput.trim() ? `${prefix}${userInput}` : prefix.trimEnd()
}
