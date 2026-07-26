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
    if (run.length === 1) {
      out.push(run[0])
      continue
    }
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

/** User toggles override default collapse; unset = derive from turn status + position. */
export function useTurnCollapse(getTurns: () => StreamTurn[]) {
  const collapseOverrides = ref(new Map<string, boolean>())

  function clearCollapseOverrides() {
    collapseOverrides.value = new Map()
  }

  function defaultCollapsed(_turn: StreamTurn, turnIndex: number, turns: StreamTurn[]): boolean {
    // Always keep the latest turn open so the active conversation stays visible
    // (including after completed). Older turns stay collapsed by default.
    return turnIndex !== turns.length - 1
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

  /** Force a turn body visible so pending ask/approval cards can be scrolled into view. */
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
