import type { Agent } from '@/types'

/** Stable category order for expert pickers / Teams rail. */
export const EXPERT_CATEGORY_ORDER = ['coding', 'research', 'office', 'creative', 'other'] as const

export type ExpertCategoryId = (typeof EXPERT_CATEGORY_ORDER)[number]

export function normalizeExpertCategory(category: string | undefined | null): ExpertCategoryId {
  const c = (category ?? '').trim().toLowerCase()
  if (c === 'coding' || c === 'research' || c === 'office' || c === 'creative') return c
  return 'other'
}

export interface ExpertCategoryGroup {
  id: ExpertCategoryId
  agents: Agent[]
}

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
    const hay = [a.id, a.name, a.description ?? '', a.persona ?? '', a.category ?? '']
      .join('\n')
      .toLowerCase()
    return hay.includes(q)
  })
}

/** Group summonable experts by category (stable order; empty groups omitted). */
export function groupSummonableExperts(
  agents: Agent[],
  query: string,
  excludeId: string | null | undefined,
): ExpertCategoryGroup[] {
  const filtered = filterSummonableExperts(agents, query, excludeId)
  const buckets = new Map<ExpertCategoryId, Agent[]>()
  for (const id of EXPERT_CATEGORY_ORDER) buckets.set(id, [])
  for (const a of filtered) {
    buckets.get(normalizeExpertCategory(a.category))!.push(a)
  }
  return EXPERT_CATEGORY_ORDER
    .map((id) => ({ id, agents: buckets.get(id)! }))
    .filter((g) => g.agents.length > 0)
}

/** How the lead should treat delegate_agent.goal when experts are summoned from the UI. */
export type ExpertSummonMode = 'coordinate' | 'relay'

/** Workbench constraint block — also signals pass-through delegation in runtime policy. */
export const WORKBENCH_CONSTRAINT_MARKER = '【工作台约束 — 必须遵守】'

/** Prefix user input so the model delegates via delegate_agent. */
export function buildExpertSummonPrefix(
  experts: Agent[],
  useExpertLine: (name: string, id: string) => string,
  delegateHint: string,
): string {
  if (!experts.length) return ''
  const lines = experts.map((a) => useExpertLine(a.name || a.id, a.id))
  const hint = delegateHint.trim()
  if (hint) return `${lines.join('\n')}\n${hint}\n\n`
  return `${lines.join('\n')}\n\n`
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

/** Relay when UI expert chips are selected or workbench constraint block is present. */
export function expertSummonModeForOutgoing(
  expertCount: number,
  userInput: string,
): ExpertSummonMode {
  if (expertCount > 0) return 'relay'
  if (userInput.includes(WORKBENCH_CONSTRAINT_MARKER)) return 'relay'
  return 'coordinate'
}
