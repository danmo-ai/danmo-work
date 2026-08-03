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

/** Timeline event types treated as mid-turn process (folded when turn collapses). */
const PROCESS_EVENT_TYPES = new Set([
  '__tool_group__',
  '__tool_card__',
  'agent.thinking',
  'capability.activated',
  'context.compacted',
])

/**
 * When a turn is process-collapsed, hide tools/thinking/etc. and intermediate
 * agent.message chunks — keep the last LLM output plus interactive/error rows.
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
  let messageCount = 0
  for (const ev of events) {
    if (PROCESS_EVENT_TYPES.has(ev.type)) return true
    if (ev.type === 'agent.message') {
      messageCount++
      if (messageCount > 1) return true
    }
  }
  return false
}

/** True when the turn has finished (or failed) — process should fold by default. */
export function isTurnSettled(turn: Pick<StreamTurn, 'status'>): boolean {
  const s = turn.status
  return s === 'completed' || s === 'failed' || s === 'cancelled' || s === 'timeout'
}

/** User toggles override default collapse; unset = fold process after turn settles. */
export function useTurnCollapse(getTurns: () => StreamTurn[]) {
  const collapseOverrides = ref(new Map<string, boolean>())

  function clearCollapseOverrides() {
    collapseOverrides.value = new Map()
  }

  function defaultCollapsed(turn: StreamTurn, _turnIndex: number, _turns: StreamTurn[]): boolean {
    // While running (or status unknown), keep the full process open.
    // After the turn ends, fold intermediate process — not the whole turn
    // (final LLM output stays visible via timeline filtering).
    return isTurnSettled(turn)
  }

  function isTurnCollapsed(turnId: string): boolean {
    const override = collapseOverrides.value.get(turnId)
    if (override !== undefined) return override

    const turns = getTurns()
    const idx = turns.findIndex((t) => t.id === turnId)
    if (idx === -1) return false
    return defaultCollapsed(turns[idx], idx, turns)
  }

  function toggleTurnCollapse(turnId: string) {
    const next = !isTurnCollapsed(turnId)
    collapseOverrides.value.set(turnId, next)
    collapseOverrides.value = new Map(collapseOverrides.value)
  }

  /** Force process expanded so pending ask/approval cards keep surrounding context. */
  function ensureTurnExpanded(turnId: string) {
    if (!turnId || !isTurnCollapsed(turnId)) return
    collapseOverrides.value.set(turnId, false)
    collapseOverrides.value = new Map(collapseOverrides.value)
  }

  return {
    collapseOverrides,
    clearCollapseOverrides,
    isTurnCollapsed,
    toggleTurnCollapse,
    ensureTurnExpanded,
    defaultCollapsed,
  }
}

export type TurnCollapseApi = ReturnType<typeof useTurnCollapse>
