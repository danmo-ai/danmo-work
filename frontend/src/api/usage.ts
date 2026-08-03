import { asArray, fetchJSON } from '@/api/client'

export interface UsageSummary {
  promptTokens: number
  completionTokens: number
  totalTokens: number
  callCount: number
  maxPromptTokens?: number
  turnCount: number
  avgTurnTokens?: number
}

export interface UsageRollup {
  grain: string
  refId: string
  projectId?: string
  sessionId?: string
  model?: string
  agentId?: string
  promptTokens: number
  completionTokens: number
  totalTokens: number
  callCount: number
  maxPromptTokens?: number
  updatedAt: string
}

export interface UsageBreakdown {
  summary: UsageSummary
  turns?: UsageRollup[]
  sessions?: UsageRollup[]
  models?: UsageRollup[]
  agents?: UsageRollup[]
}

export interface UsageSeriesPoint {
  periodStart: string
  promptTokens: number
  completionTokens: number
  totalTokens: number
  callCount: number
  model?: string
  agentId?: string
}

export type UsagePeriod = 'day' | 'week' | 'month'

export async function fetchSessionUsage(sessionId: string): Promise<UsageBreakdown> {
  return fetchJSON<UsageBreakdown>(`/sessions/${sessionId}/usage`)
}

export async function fetchProjectUsage(projectId: string): Promise<UsageBreakdown> {
  return fetchJSON<UsageBreakdown>(`/projects/${projectId}/usage`)
}

export async function fetchUsageSummary(params?: {
  projectId?: string
  model?: string
}): Promise<UsageSummary> {
  const q = new URLSearchParams()
  if (params?.projectId) q.set('project_id', params.projectId)
  if (params?.model) q.set('model', params.model)
  const qs = q.toString()
  return fetchJSON<UsageSummary>(`/usage/summary${qs ? `?${qs}` : ''}`)
}

export async function fetchUsageSeries(params: {
  period?: UsagePeriod
  projectId?: string
  model?: string
  agentId?: string
  grain?: string
  from?: string
  to?: string
}): Promise<UsageSeriesPoint[]> {
  const q = new URLSearchParams()
  if (params.period) q.set('period', params.period)
  if (params.projectId) q.set('project_id', params.projectId)
  if (params.model) q.set('model', params.model)
  if (params.agentId) q.set('agent_id', params.agentId)
  if (params.grain) q.set('grain', params.grain)
  if (params.from) q.set('from', params.from)
  if (params.to) q.set('to', params.to)
  const qs = q.toString()
  const res = await fetchJSON<{ points: UsageSeriesPoint[] | null }>(`/usage/series${qs ? `?${qs}` : ''}`)
  return asArray(res.points)
}

export async function fetchUsageModels(projectId?: string): Promise<UsageRollup[]> {
  const qs = projectId ? `?project_id=${encodeURIComponent(projectId)}` : ''
  const res = await fetchJSON<{ models: UsageRollup[] | null }>(`/usage/models${qs}`)
  return asArray(res.models)
}

export async function fetchUsageAgents(projectId?: string): Promise<UsageRollup[]> {
  const qs = projectId ? `?project_id=${encodeURIComponent(projectId)}` : ''
  const res = await fetchJSON<{ agents: UsageRollup[] | null }>(`/usage/agents${qs}`)
  return asArray(res.agents)
}
