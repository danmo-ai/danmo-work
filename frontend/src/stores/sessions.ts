import { defineStore } from 'pinia'
import { ref, reactive, computed, watch } from 'vue'
import { fetchJSON, asArray, isNotFoundError } from '@/api/client'
import { apiBaseUrl } from '@/utils/desktop'
import { i18n } from '@/i18n'
import type { Session, TurnLog, StreamEvent, Agent, Skill, WorkerCard, AgentRun, UpdateSessionPayload, LLMModel } from '@/types/mission'
import { useSkillsStore } from '@/stores/skills'

const base = apiBaseUrl()
const MODEL_KEY = 'teams-composer-model'
const EFFORT_KEY = 'teams-composer-effort'

function encodeModelId(modelBaseId: string, effort: string): string {
  if (!effort || effort === 'off') return modelBaseId
  return `${modelBaseId}/${effort}`
}

function decodeModelId(modelId: string): { baseModelId: string; effort: string } {
  const parts = modelId.split('/')
  if (parts.length >= 3) {
    return { baseModelId: `${parts[0]}/${parts[1]}`, effort: parts[2] }
  }
  return { baseModelId: modelId, effort: '' }
}

export const useSessionsStore = defineStore('sessions', () => {
  const sessions = ref<Session[]>([])
  const currentSessionId = ref<string | null>(null)
  const turns = ref<TurnLog[]>([])
  const streamEvents = ref<StreamEvent[]>([])
  const agents = ref<Agent[]>([])
  const skills = ref<Skill[]>([])
  const workers = ref<WorkerCard[]>([])
  const agentRuns = ref<AgentRun[]>([])

  const selectedModelId = ref(localStorage.getItem(MODEL_KEY) ?? '')
  const selectedEffort = ref(localStorage.getItem(EFFORT_KEY) ?? '')
  const selectedProjectId = ref<string | null>(null)
  const selectedAgentId = ref<string | null>(null)
  const composingNew = ref(true)
  const loading = ref(false)

  /** Keep Composer selection valid when the current model is removed; never auto-pick newly added models. */
  function syncModelSelection(models: LLMModel[], _previousIds?: Set<string>) {
    if (!models.length) {
      selectedModelId.value = ''
      selectedEffort.value = ''
      return
    }
    const ids = models.map((m) => m.id)
    const current = decodeModelId(selectedModelId.value)
    if (selectedModelId.value && ids.includes(current.baseModelId)) {
      // Selection still valid — leave Composer / session choice alone.
      return
    }
    // No selection or model was deleted — fall back to first available.
    const m = models[0]
    const efforts = m.availableEfforts && m.availableEfforts.length > 0 ? m.availableEfforts : ['off']
    selectedEffort.value = efforts[0]
    selectedModelId.value = encodeModelId(m.id, efforts[0])
  }

  watch(selectedModelId, (v) => {
    localStorage.setItem(MODEL_KEY, v)
    // Persist model change to current session (skip compose / missing session)
    if (composingNew.value || !currentSessionId.value) return
    const idx = sessions.value.findIndex((x) => x.id === currentSessionId.value)
    if (idx < 0) return
    sessions.value[idx] = { ...sessions.value[idx], modelId: v }
    void updateSession(currentSessionId.value, { modelId: v })
  })

  watch(selectedEffort, (v) => {
    localStorage.setItem(EFFORT_KEY, v)
  })

  watch(selectedAgentId, (v) => {
    // Persist agent change to current session (skip compose / missing session)
    if (composingNew.value || !currentSessionId.value || !v) return
    const idx = sessions.value.findIndex((x) => x.id === currentSessionId.value)
    if (idx < 0) return
    sessions.value[idx] = { ...sessions.value[idx], agentId: v }
    void updateSession(currentSessionId.value, { agentId: v })
  })

  let eventSource: EventSource | null = null
  let lastSeq = 0

  const currentSession = computed(() =>
    sessions.value.find((t) => t.id === currentSessionId.value) ?? null,
  )

  const sessionsByProject = computed(() => {
    const map = new Map<string, Session[]>()
    const sorted = [...sessions.value].sort((a, b) => new Date(b.createdAt).getTime() - new Date(a.createdAt).getTime())
    for (const t of sorted) {
      const key = t.projectId ?? '__none__'
      if (!map.has(key)) map.set(key, [])
      map.get(key)!.push(t)
    }
    return map
  })

  /** Child turn IDs from delegate.started — not registered in engine cancel map. */
  const childTurnIds = computed(() => {
    const ids = new Set<string>()
    for (const ev of streamEvents.value) {
      if (ev.type !== 'delegate.started') continue
      const payload = ev.payload as { childTurnId?: string } | null
      const id = payload?.childTurnId
      if (id) ids.add(id)
    }
    return ids
  })

  /** Prefer root/parent running turns for cancel; fall back to any running turn. */
  const runningTurnId = computed(() => {
    const children = childTurnIds.value
    const root = turns.value.find((t) => t.status === 'running' && !children.has(t.id))
    if (root) return root.id
    const any = turns.value.find((t) => t.status === 'running')
    return any?.id ?? null
  })

  async function loadCatalog() {
    try {
      agents.value = asArray(await fetchJSON<Agent[]>('/agents'))
    } catch {
      agents.value = []
    }
    try {
      skills.value = asArray(await fetchJSON<Skill[]>('/skills'))
    } catch {
      skills.value = []
    }
    // New compose (or empty selection) defaults to Team.
    if (!selectedAgentId.value || !agents.value.some((a) => a.id === selectedAgentId.value)) {
      selectedAgentId.value = defaultAgentId() || null
    }
  }

  function defaultAgentId(): string {
    const priority = ['team', 'default', 'planner']
    for (const id of priority) {
      if (agents.value.find((a) => a.id === id && a.mode !== 'subagent')) return id
    }
    const primary = agents.value.find((a) => a.mode !== 'subagent')
    return primary?.id ?? agents.value[0]?.id ?? ''
  }

  async function loadSessions() {
    try {
      sessions.value = asArray(await fetchJSON<Session[]>('/sessions'))
      try {
        const { useWeixinStore } = await import('@/stores/weixin')
        await useWeixinStore().refreshBindings()
      } catch {
        /* weixin optional */
      }
    } catch (e) {
      sessions.value = []
    }
  }

  async function createSession(
    content: string,
    projectId?: string | null,
    attachments?: Array<{
      type: string
      name?: string
      mimeType?: string
      data: string
    }>,
  ) {
    loading.value = true
    try {
      const agentId = selectedAgentId.value || defaultAgentId()
      if (!agentId) {
        throw new Error(i18n.global.t('sessions.noAgent'))
      }
      const body: Record<string, unknown> = {
        agentId,
        modelId: selectedModelId.value,
        content,
        projectId: projectId ?? undefined,
      }
      if (attachments?.length) body.attachments = attachments
      const t = await fetchJSON<Session>('/sessions', {
        method: 'POST',
        body: JSON.stringify(body),
      })
      sessions.value = [t, ...sessions.value.filter((x) => x.id !== t.id)]
      currentSessionId.value = t.id
      selectedProjectId.value = t.projectId ?? null
      composingNew.value = false
      streamEvents.value = []
      lastSeq = 0
      turns.value = []
      subscribeEvents(t.id)
      void pollSession(t.id)
      void loadTurns(t.id)
      // Load any events that may have been emitted before SSE connected
      void (async () => {
        try {
          const existing = await fetchJSON<StreamEvent[]>(`/sessions/${t.id}/events/poll?since=0`)
          if (Array.isArray(existing)) {
            for (const ev of existing) {
              if (ev.seq > lastSeq) {
                lastSeq = ev.seq
                streamEvents.value.push(ev)
              }
            }
          }
        } catch (e) {
          console.error('[createSession] failed to load initial events', e)
        }
      })()
      return t
    } finally {
      loading.value = false
    }
  }

  async function updateSession(id: string, payload: UpdateSessionPayload) {
    try {
      const t = await fetchJSON<Session>(`/sessions/${id}`, {
        method: 'PATCH',
        body: JSON.stringify(payload),
      })
      const idx = sessions.value.findIndex((x) => x.id === id)
      if (idx >= 0) sessions.value[idx] = t
    } catch (e) {
      // Stale session id / DB switch must not crash the app (e.g. after saving a provider
      // syncModelSelection PATCHes the current session and may 404).
      console.warn('[updateSession]', id, e)
      if (isNotFoundError(e)) {
        if (currentSessionId.value === id) {
          sessions.value = sessions.value.filter((s) => s.id !== id)
          startCompose()
        }
        return
      }
      throw e
    }
  }

  async function deleteSession(id: string) {
    await fetchJSON(`/sessions/${id}`, { method: 'DELETE' })
    sessions.value = sessions.value.filter((t) => t.id !== id)
    if (currentSessionId.value === id) {
      startCompose()
    }
  }

  function removeSessionsForProject(projectId: string) {
    const removed = new Set(
      sessions.value.filter((s) => s.projectId === projectId).map((s) => s.id),
    )
    sessions.value = sessions.value.filter((s) => s.projectId !== projectId)
    if (selectedProjectId.value === projectId) {
      selectedProjectId.value = null
    }
    if (currentSessionId.value && removed.has(currentSessionId.value)) {
      startCompose()
    }
  }

  async function loadTurns(sessionId: string) {
    try {
      turns.value = asArray(await fetchJSON<TurnLog[]>(`/sessions/${sessionId}/turns`))
      console.log('[loadTurns] turns for', sessionId, ':', turns.value)
    } catch (e) {
      console.error('[loadTurns] failed for', sessionId, e)
      turns.value = []
    }
  }

  async function sendTurn(
    userInput: string,
    attachments?: Array<{
      type: string
      name?: string
      mimeType?: string
      data: string
    }>,
  ) {
    const hasAtts = Boolean(attachments?.length)
    if (!currentSessionId.value || (!userInput.trim() && !hasAtts) || loading.value) return
    loading.value = true
    try {
      const body: Record<string, unknown> = { userInput: userInput.trim() }
      if (selectedAgentId.value) body.agentId = selectedAgentId.value
      body.modelId = selectedModelId.value
      if (hasAtts) body.attachments = attachments
      await fetchJSON(`/sessions/${currentSessionId.value}/turns`, {
        method: 'POST',
        body: JSON.stringify(body),
      })
      // Refresh turns so runningTurnId is set while waiting (e.g. for approvals).
      await loadTurns(currentSessionId.value)
      const idx = sessions.value.findIndex((x) => x.id === currentSessionId.value)
      if (idx >= 0 && sessions.value[idx].status !== 'active') {
        sessions.value[idx] = { ...sessions.value[idx], status: 'active' }
      }
      pollSession(currentSessionId.value)
    } finally {
      loading.value = false
    }
  }

  async function cancelTurn(turnId: string) {
    const sid = currentSessionId.value
    if (!sid) return
    await fetchJSON(`/sessions/${sid}/turns/${turnId}`, { method: 'DELETE' })
    await loadTurns(sid)
    // Heal zombie child turns left "running" after parent cancel (pre-fix race).
    const leftovers = turns.value.filter((t) => t.status === 'running')
    for (const t of leftovers) {
      try {
        await fetchJSON(`/sessions/${sid}/turns/${t.id}`, { method: 'DELETE' })
      } catch {
        /* ignore */
      }
    }
    if (leftovers.length) await loadTurns(sid)
  }

  async function resumeTurn(turnId: string) {
    await fetchJSON(`/sessions/${currentSessionId.value}/turns/${turnId}/resume`, { method: 'POST' })
    await loadTurns(currentSessionId.value!)
  }

  async function pollSession(id: string) {
    const tick = async () => {
      if (currentSessionId.value !== id) return
      try {
        const t = await fetchJSON<Session>(`/sessions/${id}`)
        const idx = sessions.value.findIndex((x) => x.id === id)
        if (idx >= 0) {
          sessions.value[idx] = t
        } else {
          sessions.value = [t, ...sessions.value]
        }
        if (t.status === 'completed' || t.status === 'failed') return
      } catch {
        /* ignore */
      }
      setTimeout(tick, 800)
    }
    void tick()
  }

  function subscribeEvents(sessionId: string) {
    eventSource?.close()
    const url = `${base}/api/v1/sessions/${sessionId}/events?stream=1&since_seq=${lastSeq}`
    eventSource = new EventSource(url)
    eventSource.onmessage = (ev) => {
      try {
        const parsed = JSON.parse(ev.data) as StreamEvent
        pushEvent(parsed)
      } catch {
        /* ignore */
      }
    }
  }

  function pushEvent(parsed: StreamEvent) {
    if (parsed.seq <= lastSeq) return
    lastSeq = parsed.seq
    streamEvents.value.push(parsed)
    if (
      parsed.type === 'permission.decided' ||
      parsed.type === 'tool.running' ||
      parsed.type === 'tool.completed' ||
      parsed.type === 'tool.error'
    ) {
      hydrateDecidedApprovals(streamEvents.value)
    }
    // Keep turn list in sync so runningTurnId stays accurate during approvals.
    if (parsed.type === 'turn.started' || parsed.type === 'turn.ended' || parsed.type === 'turn.failed') {
      const sid = currentSessionId.value
      if (sid) void loadTurns(sid)
    }
  }

  const decidedApprovalIds = reactive(new Set<string>())

  /** Resolve approval IDs that are no longer actionable from the event stream.
   *  Covers: permission.decided events, and historical asks whose tool call already
   *  reached running/completed/error (before decided events existed). */
  function collectDecidedApprovalIds(events: StreamEvent[]): Set<string> {
    const decided = new Set<string>()
    const askByCallId = new Map<string, string>()
    let lastPending: { callId: string; name: string; description: string } | null = null

    for (const e of events) {
      const p = e.payload as Record<string, unknown> | null
      if (e.type === 'tool.pending') {
        const callId = String(p?.callId ?? '')
        if (callId) {
          lastPending = {
            callId,
            name: String(p?.name ?? ''),
            description: String(p?.description ?? ''),
          }
        }
        continue
      }
      if (e.type === 'permission.ask') {
        const approvalId = String(p?.approvalId ?? p?.id ?? '')
        if (!approvalId) continue
        const callId = String(p?.callId ?? '')
        if (callId) {
          askByCallId.set(callId, approvalId)
        } else if (
          lastPending &&
          lastPending.name === String(p?.tool ?? p?.name ?? '') &&
          lastPending.description === String(p?.description ?? '')
        ) {
          askByCallId.set(lastPending.callId, approvalId)
        }
        continue
      }
      if (e.type === 'permission.decided') {
        const approvalId = String(p?.approvalId ?? p?.id ?? '')
        if (approvalId) decided.add(approvalId)
        continue
      }
      if (e.type === 'tool.running' || e.type === 'tool.completed' || e.type === 'tool.error') {
        const callId = String(p?.callId ?? '')
        const approvalId = callId ? askByCallId.get(callId) : undefined
        if (approvalId) decided.add(approvalId)
      }
    }
    return decided
  }

  function hydrateDecidedApprovals(events: StreamEvent[]) {
    for (const id of collectDecidedApprovalIds(events)) {
      decidedApprovalIds.add(id)
    }
  }

  async function decideApproval(approvalId: string, approved: boolean, scope: 'once' | 'session' = 'once') {
    await fetchJSON(`/approvals/${approvalId}/decide`, {
      method: 'POST',
      body: JSON.stringify({ approved, scope }),
    })
    decidedApprovalIds.add(approvalId)
  }

  async function resolveAskUser(askId: string, answer: string) {
    await fetchJSON(`/asks/${askId}/resolve`, {
      method: 'POST',
      body: JSON.stringify({ answer }),
    })
  }

  const pendingApprovals = computed(() =>
    streamEvents.value.filter((e) => {
      if (e.type !== 'permission.ask') return false
      const p = e.payload as Record<string, unknown> | null
      const id = String(p?.approvalId ?? p?.id ?? '')
      return id && !decidedApprovalIds.has(id)
    }),
  )

  const pendingAsks = computed(() =>
    streamEvents.value.filter((e) => e.type === 'ask_user.pending'),
  )

  const resolvedAskCallIds = computed(() => {
    const ids = new Set<string>()
    for (const e of streamEvents.value) {
      if (e.type !== 'tool.completed') continue
      const p = e.payload as Record<string, unknown> | null
      if (p?.name === 'ask_user' && typeof p?.callId === 'string') {
        ids.add(p.callId)
      }
    }
    return ids
  })

  async function selectSession(id: string) {
    currentSessionId.value = id
    composingNew.value = false
    const t = sessions.value.find((x) => x.id === id)
    selectedProjectId.value = t?.projectId ?? null
    selectedAgentId.value = t?.agentId ?? null
    if (t?.modelId) {
      const decoded = decodeModelId(t.modelId)
      selectedModelId.value = t.modelId
      if (decoded.effort) {
        selectedEffort.value = decoded.effort
      }
    }
    streamEvents.value = []
    lastSeq = 0
    turns.value = []
    decidedApprovalIds.clear()
    try {
      const existing = await fetchJSON<StreamEvent[]>(`/sessions/${id}/events/poll?since=0`)
      console.log('[selectSession] events for', id, ':', existing)
      if (Array.isArray(existing)) {
        for (const ev of existing) {
          if (ev.seq > lastSeq) lastSeq = ev.seq
          streamEvents.value.push(ev)
        }
        hydrateDecidedApprovals(streamEvents.value)
      }
    } catch (e) {
      console.error('[selectSession] failed to load events for', id, e)
    }
    subscribeEvents(id)
    void pollSession(id)
    void loadTurns(id)
  }

  function startCompose(projectId?: string | null) {
    composingNew.value = true
    currentSessionId.value = null
    selectedProjectId.value = projectId ?? null
    selectedAgentId.value = defaultAgentId() || 'team'
    streamEvents.value = []
    turns.value = []
    eventSource?.close()
  }

  return {
    sessions,
    currentSessionId,
    currentSession,
    turns,
    streamEvents,
    agents,
    skills,
    workers,
    agentRuns,
    selectedModelId,
    selectedEffort,
    selectedProjectId,
    selectedAgentId,
    composingNew,
    loading,
    runningTurnId,
    sessionsByProject,
    loadCatalog,
    loadSessions,
    createSession,
    updateSession,
    deleteSession,
    removeSessionsForProject,
    loadTurns,
    sendTurn,
    cancelTurn,
    resumeTurn,
    selectSession,
    startCompose,
    decideApproval,
    resolveAskUser,
    pendingApprovals,
    pendingAsks,
    resolvedAskCallIds,
    decidedApprovalIds,
    syncModelSelection,
  }
})
