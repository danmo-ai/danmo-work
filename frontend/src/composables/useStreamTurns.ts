import { ref } from 'vue'
import type { StreamEvent } from '@/types/mission'

export interface ToolCard {
  callId: string
  name: string
  description: string
  status: string
  inputStr: string
  output: string
  error: string
  seq: number
  stepNum: number
}

export interface UserImageAttachment {
  name?: string
  mimeType?: string
  dataUrl: string
}

export interface StreamTurn {
  id: string
  parentTurnId?: string
  goal: string
  userText?: string
  userImages?: UserImageAttachment[]
  agentId?: string
  agentName?: string
  status?: string
  events: StreamEvent[]
  childTurnIds: string[]
}

/** Aggregate consecutive synthetic tool-card events into a group row. */
export function groupConsecutiveToolCards(events: StreamEvent[]): StreamEvent[] {
  const out: StreamEvent[] = []
  let i = 0
  while (i < events.length) {
    const ev = events[i]
    if (ev.type !== '__tool_card__') {
      out.push(ev)
      i++
      continue
    }
    const start = i
    while (i < events.length && events[i].type === '__tool_card__') i++
    const run = events.slice(start, i)
    // Always wrap as a group, including a single tool.
    const cards = run.map((e) => e.payload as ToolCard)
    out.push({
      seq: run[0].seq,
      type: '__tool_group__',
      sessionId: run[0].sessionId || '',
      turnId: run[0].turnId,
      createdAt: run[0].createdAt || '',
      payload: { cards, seq: run[0].seq },
    } as unknown as StreamEvent)
  }
  return out
}

/** Timeline event types treated as mid-turn process (folded when process collapses). */
const PROCESS_EVENT_TYPES = new Set([
  '__tool_group__',
  '__tool_card__',
  'agent.thinking',
  'capability.activated',
  'context.compacted',
  // Resolved/settled interactive rows — expand to review; keep timeline clean when folded.
  'ask_user.pending',
  'permission.ask',
])

/**
 * When process is collapsed, hide tools/thinking/asks/etc. and intermediate
 * agent.message chunks — keep the last LLM output plus errors/report meta.
 */
export function filterCollapsedTimelineEvents(events: StreamEvent[]): StreamEvent[] {
  let lastMessageIdx = -1
  for (let i = events.length - 1; i >= 0; i--) {
    if (events[i].type === 'agent.message') {
      lastMessageIdx = i
      break
    }
  }

  return events.filter((ev, idx) => {
    if (PROCESS_EVENT_TYPES.has(ev.type)) return false
    if (ev.type === 'agent.message') return idx === lastMessageIdx
    return true
  })
}

/** True when collapsing would hide mid-turn process (tools, thinking, or earlier messages). */
export function hasFoldableProcess(events: StreamEvent[]): boolean {
  return countFoldableProcessEvents(events) > 0
}

/** How many timeline events would be hidden when process is folded. */
export function countFoldableProcessEvents(events: StreamEvent[]): number {
  if (events.length === 0) return 0
  return events.length - filterCollapsedTimelineEvents(events).length
}

/** True when the turn has finished — process/body should fold by default. */
export function isTurnSettled(turn: Pick<StreamTurn, 'status'>): boolean {
  const s = turn.status
  // turn.ended uses report status ("done"); turn.failed / DB use completed|failed|cancelled|timeout.
  return s === 'completed' || s === 'done' || s === 'failed' || s === 'cancelled' || s === 'timeout'
}

/**
 * Two independent folds:
 * - process: mid-turn tools/thinking/etc. (default on after turn settles)
 * - body: entire turn (user + timeline); header toggle; default collapsed after settle
 */
export function useTurnCollapse(getTurns: () => StreamTurn[]) {
  const processCollapseOverrides = ref(new Map<string, boolean>())
  const bodyCollapseOverrides = ref(new Map<string, boolean>())

  function clearCollapseOverrides() {
    processCollapseOverrides.value = new Map()
    bodyCollapseOverrides.value = new Map()
  }

  function defaultProcessCollapsed(turn: StreamTurn): boolean {
    // While running (or status unknown), keep the full process open.
    // After the turn ends, fold intermediate process — final answer stays visible.
    return isTurnSettled(turn)
  }

  function defaultBodyCollapsed(turn: StreamTurn): boolean {
    // Running turns stay open; finished turns fold the whole body by default.
    return isTurnSettled(turn)
  }

  function isProcessCollapsed(turnId: string): boolean {
    const override = processCollapseOverrides.value.get(turnId)
    if (override !== undefined) return override

    const turns = getTurns()
    const idx = turns.findIndex((t) => t.id === turnId)
    if (idx === -1) return false
    return defaultProcessCollapsed(turns[idx])
  }

  function toggleProcessCollapse(turnId: string) {
    const next = !isProcessCollapsed(turnId)
    processCollapseOverrides.value.set(turnId, next)
    processCollapseOverrides.value = new Map(processCollapseOverrides.value)
  }

  /** Force process expanded so pending ask/approval cards keep surrounding context. */
  function ensureProcessExpanded(turnId: string) {
    if (!turnId || !isProcessCollapsed(turnId)) return
    processCollapseOverrides.value.set(turnId, false)
    processCollapseOverrides.value = new Map(processCollapseOverrides.value)
  }

  function isTurnBodyCollapsed(turnId: string): boolean {
    const override = bodyCollapseOverrides.value.get(turnId)
    if (override !== undefined) return override

    const turns = getTurns()
    const idx = turns.findIndex((t) => t.id === turnId)
    if (idx === -1) return false
    return defaultBodyCollapsed(turns[idx])
  }

  function toggleTurnBodyCollapse(turnId: string) {
    const next = !isTurnBodyCollapsed(turnId)
    bodyCollapseOverrides.value.set(turnId, next)
    bodyCollapseOverrides.value = new Map(bodyCollapseOverrides.value)
  }

  return {
    clearCollapseOverrides,
    isProcessCollapsed,
    toggleProcessCollapse,
    ensureProcessExpanded,
    isTurnBodyCollapsed,
    toggleTurnBodyCollapse,
  }
}

export type TurnCollapseApi = ReturnType<typeof useTurnCollapse>
