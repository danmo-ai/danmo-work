<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { Check, ChevronDown, CopyDocument, Download, Search } from '@danqing/dq-shell'
import { apiBaseUrl, saveBlobAs } from '@/utils/desktop'
import { formatTokenCount } from '@/composables/useSessionContextUsage'
import { useSessionsStore } from '@/stores/sessions'
import type { StreamEvent, TurnLog } from '@/types/mission'

type RowState = 'running' | 'ok' | 'error' | 'pending' | 'muted'

type RowKind =
  | 'system'
  | 'user'
  | 'think'
  | 'assistant'
  | 'tool'
  | 'ask'
  | 'permission'
  | 'delegate'
  | 'compacted'
  | 'error'

interface TrajRow {
  ev: StreamEvent
  seq: number
  kind: RowKind
  tag: string
  text: string
  elapsed: string
  clock: string
  state: RowState
  detail: string
  hasDetail: boolean
  turnIndex: number
  timeMs: number
}

interface TrajGroup {
  key: string
  turnId: string | null
  index: number
  status: string | null
  agentName: string | null
  goal: string
  startTime: number | null
  endTime: number | null
  toolCount: number
  tokens: number
  rows: TrajRow[]
}

interface DetailTabItem {
  id: string
  label: string
}

interface TimelineSpan {
  index: number
  kind: RowKind
  isError: boolean
  start: number
  end: number
  lane: number
  label: string
}

interface TimelineModel {
  spans: TimelineSpan[]
  boundaries: { turn: number; time: number }[]
  start: number
  end: number
}

type GroupItem =
  | { type: 'step'; row: TrajRow; title: string }
  | { type: 'row'; row: TrajRow }
  | { type: 'batch'; key: number; rows: TrajRow[] }

const props = defineProps<{
  streamEvents: StreamEvent[]
  turns: TurnLog[]
}>()

const { t } = useI18n()
const sessions = useSessionsStore()

const FOLLOW_THRESHOLD_PX = 24
const DETAILS_MIN_WIDTH = 280
const TABLE_MIN_WIDTH = 220
const DEFAULT_DETAILS_WIDTH = 340
const MINIMUM_DRAG_PX = 3
const MINIMUM_ZOOM_SPANS = 4
const EDGE_PAN_ZONE_FRACTION = 0.08
const EDGE_PAN_STEP_FRACTION = 0.025
const MAXIMUM_EDGE_PAN_PX = 32

const query = ref('')
const foldTools = ref(false)
const collapsedTurns = ref(new Set<number>())
const expandedToolBatches = ref(new Set<number>())
const selectedSeq = ref<number | null>(null)
const activeTabId = ref('summary')
const copiedSeq = ref<number | null>(null)
const atBottom = ref(true)
const scrollRef = ref<HTMLElement | null>(null)
const ledgerRef = ref<HTMLElement | null>(null)
const detailsWidth = ref(DEFAULT_DETAILS_WIDTH)

const timelineMode = ref<'sequence' | 'duration'>('sequence')
const timelineRange = ref<{ start: number; end: number } | null>(null)
const draftRange = ref<{ start: number; end: number } | null>(null)
const viewport = ref<{ start: number; end: number } | null>(null)
const animateViewport = ref(false)
const hoverFraction = ref<number | null>(null)
const hoverSpan = ref<TimelineSpan | null>(null)
const timelineTrack = ref<HTMLElement | null>(null)

let dragRef: {
  pointerId: number
  anchorTime: number
  anchorClientX: number
  recordIndex: number | null
} | null = null
let panRef: {
  pointerId: number
  anchorClientX: number
  anchorStart: number
  moved: boolean
  pannable: boolean
} | null = null

const TAG_LABEL: Record<RowKind, string> = {
  system: 'SYSTEM',
  user: 'USER',
  think: 'THINK',
  assistant: 'ASSISTANT',
  tool: 'TOOL',
  ask: 'ASK',
  permission: 'PERMISSION',
  delegate: 'DELEGATE',
  compacted: 'COMPACTED',
  error: 'ERROR',
}

const KIND_ICON: Record<RowKind, string> = {
  system: '<svg width="13" height="13" viewBox="0 0 16 16" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round"><path d="M8 1.75a2.1 2.1 0 0 1 2.1 2.1c0 .6-.25 1.14-.65 1.52L13.6 9.5H2.4l4.15-4.13a2.06 2.06 0 0 1-.65-1.52c0-1.16.94-2.1 2.1-2.1Z"/><circle cx="8" cy="11" r="3.25"/></svg>',
  user: '<svg width="13" height="13" viewBox="0 0 16 16" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round"><circle cx="8" cy="5.25" r="3.25"/><path d="M2.5 14.5c.75-2.9 3-4.25 5.5-4.25s4.75 1.35 5.5 4.25"/></svg>',
  think: '<svg width="13" height="13" viewBox="0 0 16 16" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round"><path d="M8 2.5a4.5 4.5 0 0 0-4.32 5.6c-.36.66-.55 1.4-.55 2.15h3.37"/><path d="M8 2.5a4.5 4.5 0 0 1 4.32 5.6c.36.66.55 1.4.55 2.15H9.5"/><circle cx="8" cy="13.25" r="0.85" fill="currentColor" stroke="none"/></svg>',
  assistant: '<svg width="13" height="13" viewBox="0 0 16 16" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round"><path d="M8 1.75 9.65 5.9l4.35.9-3.3 2.85.9 4.35L8 11.65 4.4 14l.9-4.35L2 6.8l4.35-.9L8 1.75Z"/></svg>',
  tool: '<svg width="13" height="13" viewBox="0 0 16 16" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round"><path d="M14 3.3a3.8 3.8 0 0 1-4.8 4.8l-5.1 5.1a1.6 1.6 0 1 1-2.3-2.3l5.1-5.1A3.8 3.8 0 0 1 11.7 1l-2.3 2.3 2.3 2.3L14 3.3Z"/></svg>',
  ask: '<svg width="13" height="13" viewBox="0 0 16 16" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round"><circle cx="8" cy="8" r="6.25"/><path d="M6.2 6.1a1.9 1.9 0 0 1 3.7.6c0 1.3-1.9 1.7-1.9 3"/><circle cx="8" cy="11.6" r="0.85" fill="currentColor" stroke="none"/></svg>',
  permission: '<svg width="13" height="13" viewBox="0 0 16 16" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round"><path d="M8 1.5 13 3.5v4.2c0 3.1-2 5.4-5 6.8-3-1.4-5-3.7-5-6.8V3.5L8 1.5Z"/><path d="m5.8 8 1.5 1.5 2.9-3"/></svg>',
  delegate: '<svg width="13" height="13" viewBox="0 0 16 16" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round"><circle cx="6" cy="5.5" r="2.5"/><path d="M1.75 14c.6-2.2 2.3-3.25 4.25-3.25 1.2 0 2.25.4 3.05 1.15"/><circle cx="11.75" cy="8.5" r="2.25"/><path d="M10.25 14c.5-1.75 1.85-2.6 3.35-2.6 1.55 0 2.65.9 2.65 2.6"/></svg>',
  compacted: '<svg width="13" height="13" viewBox="0 0 16 16" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round"><path d="m2.5 2.5 3.75 3.75M3 6.25h3.25V3"/><path d="m13.5 2.5-3.75 3.75M13 6.25H9.75V3"/><path d="m2.5 13.5 3.75-3.75M3 9.75h3.25V13"/><path d="m13.5 13.5-3.75-3.75M13 9.75H9.75V13"/></svg>',
  error: '<svg width="13" height="13" viewBox="0 0 16 16" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round"><path d="M8 1.75 14.5 13a1 1 0 0 1-.87 1.5H2.37a1 1 0 0 1-.87-1.5L8 1.75Z"/><line x1="8" y1="6.25" x2="8" y2="9.75"/><circle cx="8" cy="11.75" r="0.85" fill="currentColor" stroke="none"/></svg>',
}

function asRecord(payload: unknown): Record<string, unknown> | null {
  return payload && typeof payload === 'object' ? (payload as Record<string, unknown>) : null
}

function prettyJSON(value: unknown): string {
  if (value === undefined || value === null) return ''
  try {
    return JSON.stringify(value, null, 2)
  } catch {
    return String(value)
  }
}

function str(value: unknown): string {
  return typeof value === 'string' ? value : value == null ? '' : String(value)
}

function clockLabel(createdAt: string): string {
  const d = new Date(createdAt)
  if (Number.isNaN(d.getTime())) return ''
  const pad = (n: number) => String(n).padStart(2, '0')
  return `${pad(d.getHours())}:${pad(d.getMinutes())}:${pad(d.getSeconds())}`
}

function elapsedLabel(startTime: number | null, createdAt: string): string {
  if (startTime === null) return ''
  const d = new Date(createdAt)
  if (Number.isNaN(d.getTime())) return ''
  const ms = Math.max(0, d.getTime() - startTime)
  const s = ms / 1000
  if (s < 60) return `${s.toFixed(1)} s`
  const m = Math.floor(s / 60)
  const rest = Math.floor(s % 60)
  return `${m}:${String(rest).padStart(2, '0')}`
}

function formatMillis(milliseconds: number): string {
  if (milliseconds < 1000) return `${Math.round(milliseconds)} ms`
  if (milliseconds < 60_000) return `${(milliseconds / 1000).toFixed(1)} s`
  const m = Math.floor(milliseconds / 60_000)
  const s = Math.round((milliseconds % 60_000) / 1000)
  return `${m}:${String(s).padStart(2, '0')}`
}

function agentNameOf(agentId: string | undefined | null): string | null {
  if (!agentId) return null
  const agent = sessions.agents.find((a) => a.id === agentId)
  return agent?.name ?? agentId
}

function toolInputRaw(p: Record<string, unknown> | null): string {
  if (!p) return ''
  const input = p.input ?? p.arguments ?? p.args
  return prettyJSON(input)
}

function toolNameOf(p: Record<string, unknown> | null): string {
  return str(p?.name)
}

function toolDescriptionOf(p: Record<string, unknown> | null): string {
  return str(p?.description)
}

function laneFor(kind: RowKind): number {
  if (kind === 'tool' || kind === 'ask' || kind === 'permission' || kind === 'delegate') return 2
  if (kind === 'assistant' || kind === 'think' || kind === 'compacted') return 1
  return 0
}

const decidedApprovalIds = computed(() => {
  const ids = new Set<string>()
  for (const ev of props.streamEvents) {
    if (ev.type !== 'permission.decided') continue
    const id = str(asRecord(ev.payload)?.approvalId)
    if (id) ids.add(id)
  }
  return ids
})

const settledAskCallIds = computed(() => {
  const ids = new Set<string>()
  for (const ev of props.streamEvents) {
    if (ev.type !== 'tool.completed') continue
    const p = asRecord(ev.payload)
    if (str(p?.name) === 'ask_user' && str(p?.callId)) {
      ids.add(str(p?.callId))
    }
  }
  return ids
})

const toolDurations = computed(() => {
  const starts = new Map<string, number>()
  const ends = new Map<string, number>()
  for (const ev of props.streamEvents) {
    const p = asRecord(ev.payload)
    const callId = str(p?.callId)
    if (!callId) continue
    const t = new Date(ev.createdAt).getTime()
    if (ev.type === 'tool.pending' || ev.type === 'tool.running') {
      if (!starts.has(callId)) starts.set(callId, t)
    } else if (ev.type === 'tool.completed' || ev.type === 'tool.error') {
      ends.set(callId, t)
    }
  }
  const map = new Map<string, { start: number; end: number }>()
  for (const [callId, start] of starts) {
    const end = ends.get(callId)
    if (end !== undefined && end >= start) map.set(callId, { start, end })
  }
  return map
})

function stateLabel(state: RowState): string {
  const map: Record<RowState, string> = {
    running: t('sessions.trajectory.running'),
    ok: t('sessions.trajectory.completed'),
    error: t('sessions.trajectory.failed'),
    pending: t('sessions.trajectory.pending'),
    muted: t('sessions.trajectory.settled'),
  }
  return map[state]
}

function llmUsageLabel(p: Record<string, unknown>): string {
  const parts = [formatTokenCount(Number(p?.totalTokens ?? 0))]
  const cache = Number(p?.cacheReadTokens ?? 0)
  if (cache > 0) {
    parts.push(`${formatTokenCount(cache)} ${t('sessions.trajectory.fieldCacheRead')}`)
  }
  if (p?.model) parts.push(String(p.model))
  return parts.join(' · ')
}

function buildRow(ev: StreamEvent, startTime: number | null, turnIndex: number): TrajRow {
  const p = asRecord(ev.payload)
  const base = {
    ev,
    seq: ev.seq,
    elapsed: elapsedLabel(startTime, ev.createdAt),
    clock: clockLabel(ev.createdAt),
    turnIndex,
    timeMs: new Date(ev.createdAt).getTime(),
  }
  const mk = (kind: RowKind, state: RowState, text: string, detail: string, hasDetail: boolean): TrajRow => ({
    ...base,
    kind,
    tag: TAG_LABEL[kind],
    state,
    text,
    detail,
    hasDetail,
  })

  switch (ev.type) {
    case 'user.message':
      return mk('user', 'ok', str(p?.content), str(p?.content), Boolean(str(p?.content)))
    case 'agent.message':
      return mk('assistant', 'ok', str(p?.text), str(p?.text), Boolean(str(p?.text)))
    case 'agent.thinking':
      return mk('think', 'ok', str(p?.text), str(p?.text), Boolean(str(p?.text)))
    case 'tool.pending': {
      const name = toolNameOf(p)
      const desc = toolDescriptionOf(p)
      return mk('tool', 'pending', desc ? `${name} · ${desc}` : name, toolInputRaw(p), Boolean(toolInputRaw(p)))
    }
    case 'tool.running': {
      const name = toolNameOf(p)
      return mk('tool', 'running', name, toolInputRaw(p), Boolean(toolInputRaw(p)))
    }
    case 'tool.completed': {
      const name = toolNameOf(p)
      const output = str(p?.output)
      return mk('tool', 'ok', name, output, Boolean(output))
    }
    case 'tool.error': {
      const name = toolNameOf(p)
      const errMsg = str(p?.error)
      const cancelled = errMsg === 'cancelled' || /context canceled/i.test(errMsg)
      return mk('tool', cancelled ? 'muted' : 'error', name, errMsg, Boolean(errMsg))
    }
    case 'permission.ask': {
      const tool = str(p?.tool)
      const approvalId = str(p?.approvalId)
      const decided = decidedApprovalIds.value.has(approvalId)
      const extra = str(p?.reason) || str(p?.description)
      const text = decided || !extra ? tool : `${tool} · ${extra}`
      return mk('permission', decided ? 'muted' : 'pending', text, prettyJSON(p), true)
    }
    case 'permission.decided': {
      const approved = p?.approved === true
      return mk('permission', 'muted', approved ? '允许' : '拒绝', prettyJSON(p), true)
    }
    case 'ask_user.pending': {
      const callId = str(p?.callId)
      const settled = settledAskCallIds.value.has(callId)
      return mk('ask', settled ? 'muted' : 'pending', str(p?.question), prettyJSON(p), true)
    }
    case 'delegate.started': {
      const agent = agentNameOf(str(p?.agentId)) ?? 'Agent'
      return mk('delegate', 'running', `${agent} · ${str(p?.goal)}`, prettyJSON(p), true)
    }
    case 'delegate.completed': {
      const agent = agentNameOf(str(p?.agentId)) ?? 'Agent'
      const status = str(p?.status)
      const summary = str(p?.summary)
      return mk('delegate', 'ok', `${agent}${status ? ` · ${status}` : ''}`, summary || prettyJSON(p), Boolean(summary))
    }
    case 'capability.activated':
      return mk('system', 'muted', str(p?.name), prettyJSON(p), true)
    case 'context.compacted':
      return mk(
        'compacted',
        'muted',
        t('sessions.compactionRow', {
          turns: Number(p?.turnsCompacted ?? 0),
          before: formatTokenCount(Number(p?.tokensBefore ?? 0)),
          after: formatTokenCount(Number(p?.tokensAfter ?? 0)),
        }),
        prettyJSON(p),
        true,
      )
    case 'llm.usage':
      return mk(
        'system',
        'muted',
        llmUsageLabel(p),
        prettyJSON(p),
        true,
      )
    case 'step.started':
      return mk('system', 'muted', p?.title ? str(p.title) : `Step ${str(p?.step)}`, '', false)
    case 'step.ended':
      return mk('system', 'muted', '', '', false)
    case 'turn.started':
      return mk('system', 'muted', '', '', false)
    case 'turn.ended':
      return mk('system', 'muted', `Turn 结束${p?.status ? ` · ${str(p.status)}` : ''}`, str(p?.summary), Boolean(str(p?.summary)))
    case 'turn.failed':
      return mk('error', 'error', `Turn 失败${p?.message ? ` · ${str(p.message)}` : ''}`, prettyJSON(p), true)
    case 'session.completed':
      return mk('system', 'ok', str(p?.summary), str(p?.summary), Boolean(str(p?.summary)))
    case 'error':
      return mk('error', 'error', str(p?.message), prettyJSON(p), true)
    default:
      return mk('system', 'muted', ev.type, prettyJSON(p), Boolean(prettyJSON(p)))
  }
}

const groups = computed<TrajGroup[]>(() => {
  const turnInfo = new Map<string, TurnLog>()
  for (const tlog of props.turns) turnInfo.set(tlog.id, tlog)

  const byKey = new Map<string, TrajGroup>()
  const order: string[] = []
  const sessionKey = '__session__'
  let turnIndex = 0

  const ensureGroup = (key: string, turnId: string | null): TrajGroup => {
    let g = byKey.get(key)
    if (!g) {
      g = {
        key,
        turnId,
        index: key === sessionKey ? -1 : turnIndex++,
        status: turnId ? (turnInfo.get(turnId)?.status ?? null) : null,
        agentName: turnId ? agentNameOf(turnInfo.get(turnId)?.agentId) : null,
        goal: turnId ? (turnInfo.get(turnId)?.goal ?? '') : '',
        startTime: null,
        endTime: null,
        toolCount: 0,
        tokens: 0,
        rows: [],
      }
      byKey.set(key, g)
      order.push(key)
    }
    return g
  }

  for (const ev of props.streamEvents) {
    const p = asRecord(ev.payload)
    if (ev.type === 'turn.started') {
      const turnId = str(p?.turnId) || ev.turnId || sessionKey
      const g = ensureGroup(turnId, turnId === sessionKey ? null : turnId)
      const agentId = str(p?.agentId)
      if (agentId) {
        g.agentName = agentNameOf(agentId)
        turnInfo.set(turnId, {
          ...(turnInfo.get(turnId) ?? ({} as TurnLog)),
          id: turnId,
          sessionId: ev.sessionId,
          agentId,
          goal: str(p?.goal),
          status: 'running',
        })
      }
      const goalText = str(p?.goal)
      if (goalText) g.goal = goalText
      g.startTime = new Date(ev.createdAt).getTime()
      continue
    }
    if (ev.type === 'turn.ended' || ev.type === 'turn.failed') {
      const turnId = str(p?.turnId) || ev.turnId
      const g = ensureGroup(turnId || sessionKey, turnId || null)
      const status = ev.type === 'turn.failed' ? 'failed' : (str(p?.status) || 'completed')
      g.status = status === 'done' ? 'completed' : status
      g.endTime = new Date(ev.createdAt).getTime()
      g.rows.push(buildRow(ev, g.startTime, g.index))
      continue
    }
    const turnId = ev.turnId ?? sessionKey
    const g = ensureGroup(turnId, turnId === sessionKey ? null : turnId)
    if (g.startTime === null) g.startTime = new Date(ev.createdAt).getTime()
    const row = buildRow(ev, g.startTime, g.index)
    if (row.kind === 'tool' || row.kind === 'delegate') g.toolCount += 1
    if (ev.type === 'llm.usage') g.tokens += Number(p?.totalTokens ?? 0)
    if (ev.type === 'step.ended') continue
    g.rows.push(row)
  }

  const seenTurnIds = new Set<string>()
  for (const g of byKey.values()) if (g.turnId) seenTurnIds.add(g.turnId)
  for (const tlog of props.turns) {
    if (seenTurnIds.has(tlog.id)) continue
    const g = ensureGroup(tlog.id, tlog.id)
    g.status = tlog.status
    g.agentName = agentNameOf(tlog.agentId)
    g.goal = tlog.goal
  }

  const list: TrajGroup[] = []
  for (const key of order) {
    const g = byKey.get(key)
    if (g) list.push(g)
  }
  list.sort((a, b) => {
    const ta = a.rows[0]?.ev.createdAt ?? ''
    const tb = b.rows[0]?.ev.createdAt ?? ''
    if (ta && tb && ta !== tb) return ta < tb ? -1 : 1
    return a.index - b.index
  })
  return list
})

const timelineModel = computed<TimelineModel | null>(() => {
  if (timelineMode.value === 'sequence') {
    const spans: TimelineSpan[] = []
    const boundaries: { turn: number; time: number }[] = []
    let cursor = 0
    for (const g of groups.value) {
      if (g.turnId !== null) boundaries.push({ turn: g.index, time: cursor })
      for (const row of g.rows) {
        if (row.kind === 'system') continue
        spans.push({
          index: row.seq,
          kind: row.kind,
          isError: row.state === 'error',
          start: cursor,
          end: cursor + 1,
          lane: laneFor(row.kind),
          label: row.text,
        })
        cursor += 1
      }
    }
    if (spans.length === 0) return null
    return { spans, boundaries, start: 0, end: cursor }
  }

  const spans: TimelineSpan[] = []
  const boundaries: { turn: number; time: number }[] = []
  for (const g of groups.value) {
    if (g.turnId !== null && g.startTime !== null) {
      boundaries.push({ turn: g.index, time: g.startTime })
    }
    for (const row of g.rows) {
      if (row.kind === 'system') continue
      let start = row.timeMs
      let end = start
      if (row.kind === 'tool') {
        const callId = str(asRecord(row.ev.payload)?.callId)
        const range = callId ? toolDurations.value.get(callId) : undefined
        if (range) {
          start = range.start
          end = range.end
        }
      }
      if (Number.isNaN(start) || Number.isNaN(end)) continue
      spans.push({
        index: row.seq,
        kind: row.kind,
        isError: row.state === 'error',
        start,
        end: Math.max(end, start + 120),
        lane: laneFor(row.kind),
        label: row.text,
      })
    }
  }
  if (spans.length === 0) return null
  const start = Math.min(...spans.map((s) => s.start))
  const end = Math.max(...spans.map((s) => s.end))
  return { spans, boundaries, start, end: Math.max(end, start + 1) }
})

const timelineDomain = computed(() => {
  const m = timelineModel.value
  if (!m) return null
  const full = Math.max(1, m.end - m.start)
  const vp = viewport.value
  const duration = vp ? Math.min(full, Math.max(1, vp.end - vp.start)) : full
  const start = vp
    ? Math.min(Math.max(vp.start, m.start), m.end - duration)
    : m.start
  return { start, duration, full, modelStart: m.start, modelEnd: m.end }
})

const visibleRange = computed(() => draftRange.value ?? timelineRange.value)

const focusIndexes = computed<Set<number> | null>(() => {
  const m = timelineModel.value
  const range = timelineRange.value
  if (!m || !range) return null
  const set = new Set<number>()
  for (const span of m.spans) {
    if (span.end >= range.start && span.start <= range.end) set.add(span.index)
  }
  return set
})

const rangeFractionStyle = computed(() => {
  const dom = timelineDomain.value
  const range = visibleRange.value
  if (!dom || !range) return null
  const clamp = (v: number) => Math.min(1, Math.max(0, v))
  const boundedStart = Math.min(dom.modelEnd, Math.max(dom.modelStart, range.start))
  const boundedEnd = Math.min(dom.modelEnd, Math.max(dom.modelStart, range.end))
  const left = clamp((boundedStart - dom.start) / dom.duration)
  const width = clamp((boundedEnd - dom.start) / dom.duration) - left
  return {
    '--trajectory-selection-left': `${left * 100}%`,
    '--trajectory-selection-width': `${Math.max(0, width) * 100}%`,
  }
})

const domainStyle = computed(() => {
  const m = timelineModel.value
  const dom = timelineDomain.value
  if (!m || !dom) return undefined
  return {
    '--trajectory-domain-left': `${(-(dom.start - m.start) / dom.duration) * 100}%`,
    '--trajectory-domain-width': `${(m.end - m.start) / dom.duration * 100}%`,
  }
})

function spanStyle(span: TimelineSpan): Record<string, string> {
  const dom = timelineDomain.value
  if (!dom) return {}
  const left = ((span.start - dom.start) / dom.duration) * 100
  const width = Math.max(0, ((span.end - dom.start) / dom.duration) * 100 - left)
  return {
    '--trajectory-span-left': `${left}%`,
    '--trajectory-span-width': `${width}%`,
    top: `${span.lane * 14}px`,
  }
}

function boundaryStyle(time: number): Record<string, string> {
  const m = timelineModel.value
  if (!m) return {}
  return { '--trajectory-turn-left': `${((time - m.start) / Math.max(1, m.end - m.start)) * 100}%` }
}

function boundaryVisible(time: number): boolean {
  const m = timelineModel.value
  const dom = timelineDomain.value
  if (!m || !dom) return false
  return time > m.start && time >= dom.start && time <= dom.start + dom.duration
}

function spanInsideRange(span: TimelineSpan): boolean {
  const idx = focusIndexes.value
  if (idx === null) return true
  return idx.has(span.index)
}

function spanMatchesSearch(span: TimelineSpan): boolean {
  if (!q.value) return true
  const row = findRow(span.index)
  if (!row) return true
  return (
    row.text.toLowerCase().includes(q.value) ||
    row.tag.toLowerCase().includes(q.value) ||
    row.detail.toLowerCase().includes(q.value) ||
    row.ev.type.toLowerCase().includes(q.value)
  )
}

function findRow(seq: number): TrajRow | null {
  for (const g of groups.value) {
    const row = g.rows.find((r) => r.seq === seq)
    if (row) return row
  }
  return null
}

function fractionAt(event: PointerEvent): number {
  const rect = (event.currentTarget as HTMLElement).getBoundingClientRect()
  return Math.min(1, Math.max(0, (event.clientX - rect.left) / Math.max(1, rect.width)))
}

function timeAt(fraction: number): number {
  const dom = timelineDomain.value
  if (!dom) return 0
  return dom.start + fraction * dom.duration
}

function recordIndexAt(event: PointerEvent): number | null {
  const target = event.target as HTMLElement | null
  const el = target?.closest<HTMLElement>('[data-timeline-record-index]')
  const value = el?.dataset.timelineRecordIndex
  if (value === undefined) return null
  const index = Number(value)
  return Number.isFinite(index) ? index : null
}

function orderedRange(left: number, right: number): { start: number; end: number } {
  return left <= right ? { start: left, end: right } : { start: right, end: left }
}

function onTimelinePointerDown(event: PointerEvent) {
  const m = timelineModel.value
  const dom = timelineDomain.value
  if (!m || !dom) return
  if (event.button === 2) {
    panRef = {
      pointerId: event.pointerId,
      anchorClientX: event.clientX,
      anchorStart: dom.start,
      moved: false,
      pannable: viewport.value !== null,
    }
    if (viewport.value !== null) animateViewport.value = false
    return
  }
  if (event.button !== 0) return
  const fraction = fractionAt(event)
  const anchorTime = timeAt(fraction)
  hoverFraction.value = fraction
  dragRef = {
    pointerId: event.pointerId,
    anchorTime,
    anchorClientX: event.clientX,
    recordIndex: recordIndexAt(event),
  }
  if (typeof (event.currentTarget as HTMLElement).setPointerCapture === 'function') {
    ;(event.currentTarget as HTMLElement).setPointerCapture(event.pointerId)
  }
  draftRange.value = { start: anchorTime, end: anchorTime }
}

function onTimelinePointerMove(event: PointerEvent) {
  const m = timelineModel.value
  const dom = timelineDomain.value
  if (!m || !dom) return
  const rect = (event.currentTarget as HTMLElement).getBoundingClientRect()
  const fraction = fractionAt(event)
  hoverFraction.value = fraction
  const index = recordIndexAt(event)
  hoverSpan.value = index === null ? null : (m.spans.find((s) => s.index === index) ?? null)
  const pan = panRef
  if (pan !== null && pan.pointerId === event.pointerId) {
    if (Math.abs(event.clientX - pan.anchorClientX) >= MINIMUM_DRAG_PX) pan.moved = true
    if (!pan.pannable) return
    const delta = (event.clientX - pan.anchorClientX) / Math.max(1, rect.width)
    const nextStart = Math.min(
      Math.max(pan.anchorStart - delta * dom.duration, m.start),
      m.end - dom.duration,
    )
    viewport.value = { start: nextStart, end: nextStart + dom.duration }
    return
  }
  const drag = dragRef
  if (drag === null || drag.pointerId !== event.pointerId) return
  let nextDomainStart = dom.start
  if (viewport.value !== null) {
    const localX = event.clientX - rect.left
    const edgeWidth = Math.min(MAXIMUM_EDGE_PAN_PX, Math.max(1, rect.width * EDGE_PAN_ZONE_FRACTION))
    const direction = localX < edgeWidth ? -1 : localX > rect.width - edgeWidth ? 1 : 0
    if (direction !== 0) {
      const edgeDistance = direction < 0 ? edgeWidth - localX : localX - (rect.width - edgeWidth)
      const strength = Math.min(1, Math.max(0, edgeDistance / edgeWidth))
      const desiredStart =
        dom.start + direction * dom.duration * EDGE_PAN_STEP_FRACTION * Math.max(0.2, strength)
      nextDomainStart = Math.min(Math.max(desiredStart, m.start), m.end - dom.duration)
      if (nextDomainStart !== dom.start) {
        animateViewport.value = false
        viewport.value = { start: nextDomainStart, end: nextDomainStart + dom.duration }
      }
    }
  }
  const pointTime = nextDomainStart + fraction * dom.duration
  draftRange.value = orderedRange(drag.anchorTime, pointTime)
}

function onTimelinePointerUp(event: PointerEvent) {
  const m = timelineModel.value
  const dom = timelineDomain.value
  if (!m || !dom) return
  const pan = panRef
  if (pan !== null && pan.pointerId === event.pointerId) {
    const moved = pan.moved || Math.abs(event.clientX - pan.anchorClientX) >= MINIMUM_DRAG_PX
    panRef = null
    if (!moved) timelineRange.value = null
    return
  }
  const drag = dragRef
  if (drag === null || drag.pointerId !== event.pointerId) return
  const pointFraction = fractionAt(event)
  const pointTime = timeAt(pointFraction)
  const selected = orderedRange(drag.anchorTime, pointTime)
  hoverFraction.value = pointFraction
  hoverSpan.value = recordIndexAt(event) === null
    ? null
    : (m.spans.find((s) => s.index === recordIndexAt(event)) ?? null)
  dragRef = null
  draftRange.value = null
  const click = Math.abs(event.clientX - drag.anchorClientX) < MINIMUM_DRAG_PX
  const clickedSpan = click && drag.recordIndex !== null
    ? m.spans.find((s) => s.index === drag.recordIndex)
    : undefined
  if (clickedSpan !== undefined) {
    timelineRange.value = null
    const row = findRow(clickedSpan.index)
    if (row) selectRow(row)
    return
  }
  const minimumSelectionDuration = Math.min(dom.duration, Math.max(1, (m.end - m.start) / Math.max(1, m.spans.length)))
  const committed =
    selected.end - selected.start < minimumSelectionDuration
      ? (() => {
          const center = click ? selected.start : (selected.start + selected.end) / 2
          const width = minimumSelectionDuration
          const start = Math.min(Math.max(center - width / 2, m.start), m.end - width)
          return { start, end: start + width }
        })()
      : selected
  timelineRange.value = committed
  if (click) {
    const point = selected.start
    const nearest = m.spans.reduce((candidate, span) => {
      const candidateDistance =
        point < candidate.start ? candidate.start - point : point > candidate.end ? point - candidate.end : 0
      const spanDistance = point < span.start ? span.start - point : point > span.end ? point - span.end : 0
      return spanDistance < candidateDistance ? span : candidate
    })
    scrollToRow(nearest.index)
  }
}

function onTimelineWheel(event: WheelEvent) {
  const m = timelineModel.value
  const dom = timelineDomain.value
  const track = timelineTrack.value
  if (!m || !dom || !track) return
  event.preventDefault()
  animateViewport.value = false
  const rect = track.getBoundingClientRect()
  const anchorFraction = Math.min(1, Math.max(0, (event.clientX - rect.left) / Math.max(1, rect.width)))
  const nextDuration = Math.min(
    m.end - m.start,
    Math.max(
      Math.min(timelineMode.value === 'sequence' ? MINIMUM_ZOOM_SPANS : 20, m.end - m.start),
      dom.duration * Math.exp(event.deltaY * 0.0015),
    ),
  )
  if (nextDuration >= (m.end - m.start) * 0.999) {
    viewport.value = null
    return
  }
  const anchorTime = dom.start + anchorFraction * dom.duration
  const nextStart = Math.min(Math.max(anchorTime - anchorFraction * nextDuration, m.start), m.end - nextDuration)
  viewport.value = { start: nextStart, end: nextStart + nextDuration }
}

function onTimelineDoubleClick(event: MouseEvent) {
  event.preventDefault()
  timelineRange.value = null
}

function onTimelineKeydown(event: KeyboardEvent) {
  if (event.key === 'Escape' && timelineRange.value !== null) {
    event.preventDefault()
    timelineRange.value = null
  }
}

function clearTimelineState() {
  dragRef = null
  panRef = null
  draftRange.value = null
  hoverFraction.value = null
  hoverSpan.value = null
}

function scrollToRow(seq: number) {
  const el = scrollRef.value?.querySelector<HTMLElement>(`[data-row-seq="${seq}"]`)
  if (el) el.scrollIntoView({ block: 'center', behavior: 'smooth' })
}

watch(timelineMode, () => {
  timelineRange.value = null
  viewport.value = null
})

const q = computed(() => query.value.trim().toLowerCase())

const visibleGroups = computed(() => {
  const isCollapsed = (g: TrajGroup) => collapsedTurns.value.has(g.index) && g.turnId !== null
  return groups.value
    .map((g) => {
      const collapsed = isCollapsed(g)
      let hiddenCount = 0
      const items: GroupItem[] = []
      if (!collapsed) {
        let pendingBatch: TrajRow[] = []
        const flushBatch = () => {
          if (pendingBatch.length === 0) return
          const key = pendingBatch[0].seq
          const expanded = expandedToolBatches.value.has(key)
          if (expanded) {
            for (const row of pendingBatch) items.push({ type: 'row', row })
          } else {
            items.push({ type: 'batch', key, rows: pendingBatch })
          }
          pendingBatch = []
        }
        for (const row of g.rows) {
          if (row.ev.type === 'step.started') {
            flushBatch()
            items.push({ type: 'step', row, title: row.text || `Step ${str(asRecord(row.ev.payload)?.step)}` })
            continue
          }
          if (row.text === '' && row.ev.type === 'turn.started') continue
          if (q.value) {
            const matches =
              row.text.toLowerCase().includes(q.value) ||
              row.tag.toLowerCase().includes(q.value) ||
              row.detail.toLowerCase().includes(q.value) ||
              row.ev.type.toLowerCase().includes(q.value)
            if (!matches) {
              hiddenCount += 1
              continue
            }
          }
          if (foldTools.value && row.kind === 'tool') {
            pendingBatch.push(row)
            continue
          }
          flushBatch()
          items.push({ type: 'row', row })
        }
        flushBatch()
      }
      return { group: g, collapsed, items, hiddenCount }
    })
    .filter(({ collapsed, group }) => !collapsed || group.rows.length > 0)
})

const allTurnsCollapsed = computed(() => {
  const turnGroups = groups.value.filter((g) => g.turnId !== null)
  return turnGroups.length > 0 && turnGroups.every((g) => collapsedTurns.value.has(g.index))
})

const allToolsFolded = computed(() => foldTools.value)

const totalTokens = computed(() => groups.value.reduce((sum, g) => sum + g.tokens, 0))
const totalEvents = computed(() => groups.value.reduce((sum, g) => sum + g.rows.length, 0))

const selectedRow = computed<TrajRow | null>(() => {
  if (selectedSeq.value === null) return null
  return findRow(selectedSeq.value)
})

const selectedGroup = computed<TrajGroup | null>(() => {
  const row = selectedRow.value
  if (!row) return null
  return groups.value.find((g) => g.rows.some((r) => r.seq === row.seq)) ?? null
})

const selectedTabs = computed<DetailTabItem[]>(() => {
  const row = selectedRow.value
  if (!row) return []
  const tabs: DetailTabItem[] = [{ id: 'summary', label: t('sessions.trajectory.tabSummary') }]
  if (row.kind === 'assistant' || row.kind === 'user' || row.kind === 'think') {
    tabs.push({ id: 'preview', label: t('sessions.trajectory.tabPreview') })
  }
  if (row.kind === 'tool') {
    const p = asRecord(row.ev.payload)
    if (toolInputRaw(p)) tabs.push({ id: 'payload', label: t('sessions.trajectory.tabPayload') })
    if (row.ev.type === 'tool.completed' && str(p?.output)) {
      tabs.push({ id: 'result', label: t('sessions.trajectory.tabResult') })
    }
  }
  tabs.push({ id: 'raw', label: t('sessions.trajectory.tabRaw') })
  return tabs
})

const selectedSummaryFields = computed<Array<[string, string]>>(() => {
  const row = selectedRow.value
  if (!row) return []
  const p = asRecord(row.ev.payload)
  const fields: Array<[string, string]> = [
    [t('sessions.trajectory.fieldEvent'), row.ev.type],
    [t('sessions.trajectory.fieldStatus'), stateLabel(row.state)],
    [t('sessions.trajectory.fieldSeq'), `#${row.seq}`],
    [t('sessions.trajectory.fieldTime'), row.clock ? `${row.clock} · ${row.elapsed}` : row.elapsed],
  ]
  const loc = selectedGroup.value
  if (loc && loc.turnId !== null) {
    fields.push([t('sessions.trajectory.fieldLocation'), `${t('sessions.trajectory.turn')} ${loc.index + 1}`])
  }
  if (row.kind === 'tool') {
    const name = toolNameOf(p)
    if (name) fields.push([t('sessions.trajectory.fieldTool'), name])
    const callId = str(p?.callId)
    if (callId) fields.push([t('sessions.trajectory.fieldCallId'), callId])
    const desc = toolDescriptionOf(p)
    if (desc) fields.push([t('sessions.trajectory.fieldDescription'), desc])
    if (row.ev.type === 'tool.error' && str(p?.error)) {
      fields.push([t('sessions.toolError'), str(p?.error)])
    }
  }
  if (row.kind === 'ask') {
    const question = str(p?.question)
    if (question) fields.push([t('sessions.trajectory.fieldQuestion'), question])
  }
  if (row.kind === 'permission') {
    const tool = str(p?.tool)
    if (tool) fields.push([t('sessions.trajectory.fieldTool'), tool])
    const reason = str(p?.reason)
    if (reason) fields.push([t('sessions.trajectory.fieldReason'), reason])
  }
  if (row.kind === 'delegate') {
    const agent = agentNameOf(str(p?.agentId))
    if (agent) fields.push([t('sessions.trajectory.fieldAgent'), agent])
    const goal = str(p?.goal)
    if (goal) fields.push([t('sessions.trajectory.fieldGoal'), goal])
  }
  if (row.kind === 'compacted') {
    fields.push([t('sessions.trajectory.fieldTurns'), str(p?.turnsCompacted)])
    fields.push([t('sessions.trajectory.fieldBefore'), formatTokenCount(Number(p?.tokensBefore ?? 0))])
    fields.push([t('sessions.trajectory.fieldAfter'), formatTokenCount(Number(p?.tokensAfter ?? 0))])
  }
  if (row.ev.type === 'llm.usage') {
    const model = str(p?.model)
    if (model) fields.push([t('sessions.trajectory.fieldModel'), model])
    fields.push([t('sessions.trajectory.fieldInputTokens'), formatTokenCount(Number(p?.promptTokens ?? 0))])
    fields.push([t('sessions.trajectory.fieldOutputTokens'), formatTokenCount(Number(p?.completionTokens ?? 0))])
    fields.push([t('sessions.trajectory.fieldTotalTokens'), formatTokenCount(Number(p?.totalTokens ?? 0))])
    if (Number(p?.cacheReadTokens ?? 0) > 0) {
      fields.push([t('sessions.trajectory.fieldCacheRead'), formatTokenCount(Number(p.cacheReadTokens))])
    }
    if (Number(p?.cacheCreationTokens ?? 0) > 0) {
      fields.push([t('sessions.trajectory.fieldCacheWrite'), formatTokenCount(Number(p.cacheCreationTokens))])
    }
  }
  if (row.kind === 'error') {
    const message = str(p?.message)
    if (message) fields.push([t('sessions.trajectory.fieldMessage'), message])
  }
  return fields
})

const tabBody = computed<string>(() => {
  const row = selectedRow.value
  if (!row) return ''
  const p = asRecord(row.ev.payload)
  if (activeTabId.value === 'summary') return ''
  if (activeTabId.value === 'preview') return row.detail
  if (activeTabId.value === 'payload') return toolInputRaw(p)
  if (activeTabId.value === 'result') return str(p?.output)
  return prettyJSON(p)
})

function toggleTurn(index: number) {
  const next = new Set(collapsedTurns.value)
  if (next.has(index)) next.delete(index)
  else next.add(index)
  collapsedTurns.value = next
}

function toggleAllTurns() {
  const turnGroups = groups.value.filter((g) => g.turnId !== null)
  if (allTurnsCollapsed.value) {
    collapsedTurns.value = new Set()
    return
  }
  collapsedTurns.value = new Set(turnGroups.map((g) => g.index))
}

function toggleAllTools() {
  foldTools.value = !foldTools.value
  if (!foldTools.value) expandedToolBatches.value = new Set()
}

function toggleBatch(key: number) {
  const next = new Set(expandedToolBatches.value)
  if (next.has(key)) next.delete(key)
  else next.add(key)
  expandedToolBatches.value = next
}

function selectRow(row: TrajRow) {
  selectedSeq.value = row.seq
}

function closeDetails() {
  selectedSeq.value = null
}

function activateTab(id: string) {
  activeTabId.value = id
}

watch(selectedSeq, () => {
  activeTabId.value = 'summary'
})

async function copyRow(row: TrajRow) {
  const text = row.detail || row.text
  try {
    await navigator.clipboard.writeText(text)
    copiedSeq.value = row.seq
    window.setTimeout(() => {
      if (copiedSeq.value === row.seq) copiedSeq.value = null
    }, 1500)
  } catch {
    /* ignore */
  }
}

async function downloadTurnLog(turnId: string) {
  const sid = sessions.currentSessionId
  if (!sid) return
  try {
    const base = apiBaseUrl()
    const url = `${base}/api/v1/sessions/${sid}/turns/${turnId}/log`
    const res = await fetch(url)
    if (!res.ok) return
    const blob = await res.blob()
    await saveBlobAs(blob, `${turnId}.zip`, {
      filters: [{ name: 'Zip', extensions: ['zip'] }],
    })
  } catch {
    /* ignore */
  }
}

function statusLabel(status: string | null | undefined): string {
  const map: Record<string, string> = {
    running: t('sessions.trajectory.running'),
    completed: t('sessions.trajectory.completed'),
    done: t('sessions.trajectory.completed'),
    failed: t('sessions.trajectory.failed'),
    cancelled: t('sessions.trajectory.cancelled'),
    timeout: t('sessions.trajectory.failed'),
  }
  return status ? (map[status] ?? status) : ''
}

function statusClass(status: string | null | undefined): string {
  if (status === 'running') return 'is-running'
  if (status === 'failed' || status === 'timeout') return 'is-failed'
  if (status === 'cancelled') return 'is-cancelled'
  return 'is-done'
}

function onScroll() {
  const el = scrollRef.value
  if (!el) return
  atBottom.value = el.scrollHeight - el.scrollTop - el.clientHeight <= FOLLOW_THRESHOLD_PX
}

function scrollToBottom() {
  const el = scrollRef.value
  if (!el) return
  el.scrollTop = el.scrollHeight
  atBottom.value = true
}

function startResize(e: PointerEvent) {
  e.preventDefault()
  const ledger = ledgerRef.value
  if (!ledger) return
  const startX = e.clientX
  const startWidth = detailsWidth.value
  const move = (ev: PointerEvent) => {
    const total = ledger.clientWidth
    const next = Math.min(Math.max(startWidth + (startX - ev.clientX), DETAILS_MIN_WIDTH), Math.max(DETAILS_MIN_WIDTH, total - TABLE_MIN_WIDTH))
    detailsWidth.value = Math.round(next)
  }
  const up = () => {
    window.removeEventListener('pointermove', move)
    window.removeEventListener('pointerup', up)
  }
  window.addEventListener('pointermove', move)
  window.addEventListener('pointerup', up)
}

watch(
  () => props.streamEvents.length,
  async () => {
    if (!atBottom.value) return
    await nextTick()
    scrollToBottom()
  },
)

onBeforeUnmount(() => {
  clearTimelineState()
})
</script>

<template>
  <div class="trajectory">
    <div class="trajectory__toolbar" role="toolbar" :aria-label="t('sessions.trajectory.toolbar')">
      <div class="trajectory__actions">
        <button
          type="button"
          class="trajectory__toggle"
          :aria-label="timelineMode === 'duration' ? t('sessions.trajectory.equalWidth') : t('sessions.trajectory.duration')"
          :aria-pressed="timelineMode === 'duration'"
          :title="timelineMode === 'duration' ? t('sessions.trajectory.equalWidth') : t('sessions.trajectory.duration')"
          @click="timelineMode = timelineMode === 'duration' ? 'sequence' : 'duration'"
        >
          <svg
            class="trajectory__toggle-icon"
            viewBox="0 0 16 16"
            fill="none"
            aria-hidden="true"
          >
            <circle cx="8" cy="8" r="5.25" />
            <path d="M8 4.75V8l2.25 1.5" />
          </svg>
          {{ t('sessions.trajectory.duration') }}
        </button>
        <button
          type="button"
          class="trajectory__toggle"
          :aria-label="allTurnsCollapsed ? t('sessions.trajectory.expandTurns') : t('sessions.trajectory.collapseTurns')"
          :aria-pressed="allTurnsCollapsed"
          :title="allTurnsCollapsed ? t('sessions.trajectory.expandTurns') : t('sessions.trajectory.collapseTurns')"
          @click="toggleAllTurns"
        >
          <span class="trajectory__action-icon">{{ allTurnsCollapsed ? '⊞' : '⊟' }}</span>
          {{ t('sessions.trajectory.turns') }}
        </button>
        <button
          type="button"
          class="trajectory__toggle"
          :aria-label="allToolsFolded ? t('sessions.trajectory.expandCalls') : t('sessions.trajectory.collapseCalls')"
          :aria-pressed="allToolsFolded"
          :title="allToolsFolded ? t('sessions.trajectory.expandCalls') : t('sessions.trajectory.collapseCalls')"
          @click="toggleAllTools"
        >
          <span class="trajectory__action-icon">{{ allToolsFolded ? '⊞' : '⊟' }}</span>
          {{ t('sessions.trajectory.calls') }}
        </button>
      </div>
      <div class="trajectory__search">
        <Search class="trajectory__search-icon" :size="11" :stroke-width="1.75" />
        <input
          v-model="query"
          type="search"
          class="trajectory__search-input"
          :aria-label="t('sessions.trajectory.search')"
          :placeholder="t('sessions.trajectory.searchPlaceholder')"
        />
      </div>
      <div class="trajectory__stats">
        {{ t('sessions.trajectory.turnsCount', { n: groups.length }) }} ·
        {{ t('sessions.trajectory.eventsCount', { n: totalEvents }) }}
        <template v-if="totalTokens > 0"> · {{ formatTokenCount(totalTokens) }}</template>
      </div>
    </div>

    <section class="trajectory__timeline" :aria-label="t('sessions.trajectory.timelineAria')">
      <div class="trajectory__plot">
        <div class="trajectory__lane-labels" aria-hidden="true">
          <span>{{ t('sessions.trajectory.laneInput') }}</span>
          <span>{{ t('sessions.trajectory.laneModel') }}</span>
          <span>{{ t('sessions.trajectory.laneTools') }}</span>
        </div>
        <div
          ref="timelineTrack"
          class="trajectory__track"
          tabindex="0"
          :aria-label="t('sessions.trajectory.timelineHint')"
          @keydown="onTimelineKeydown"
          @pointerdown="onTimelinePointerDown"
          @pointermove="onTimelinePointerMove"
          @pointerup="onTimelinePointerUp"
          @pointercancel="clearTimelineState"
          @pointerleave="if (!dragRef && !panRef) { hoverFraction = null; hoverSpan = null }"
          @dblclick="onTimelineDoubleClick"
          @wheel.prevent="onTimelineWheel"
          @contextmenu.prevent
        >
          <div v-if="!timelineModel" class="trajectory__track-empty">
            {{ t('sessions.trajectory.timelineEmpty') }}
          </div>
          <template v-else>
            <div
              v-if="hoverFraction !== null && hoverSpan === null && draftRange === null"
              class="trajectory__hover-line"
              aria-hidden="true"
              :style="{ '--trajectory-hover-left': `${hoverFraction * 100}%` }"
            />
            <template v-if="rangeFractionStyle">
              <div
                class="trajectory__selection"
                :data-dragging="draftRange === null ? undefined : 'true'"
                aria-hidden="true"
                :style="rangeFractionStyle"
              />
              <div
                class="trajectory__selection-edges"
                :data-dragging="draftRange === null ? undefined : 'true'"
                aria-hidden="true"
                :style="rangeFractionStyle"
              />
            </template>
            <div
              class="trajectory__turn-boundaries"
              :data-animate-viewport="animateViewport || undefined"
              aria-hidden="true"
              :style="domainStyle"
            >
              <span
                v-for="boundary in timelineModel.boundaries.filter((b) => boundaryVisible(b.time))"
                :key="boundary.turn"
                class="trajectory__turn-boundary"
                :style="boundaryStyle(boundary.time)"
              />
            </div>
            <div class="trajectory__lanes" :data-animate-viewport="animateViewport || undefined" :style="domainStyle">
              <span
                v-for="span in timelineModel.spans"
                :key="span.index"
                class="trajectory__span"
                :class="`trajectory__span--${span.kind}`"
                :data-timeline-record-index="span.index"
                :data-timeline-span="span.kind"
                :data-error="span.isError || undefined"
                :data-equal-duration="timelineMode === 'sequence' ? 'true' : undefined"
                :data-selected="selectedSeq === span.index ? undefined : spanInsideRange(span) ? undefined : 'false'"
                :data-current="selectedSeq === span.index || undefined"
                :data-search-match="spanMatchesSearch(span) ? undefined : 'false'"
                :style="spanStyle(span)"
                :title="span.label"
              />
            </div>
          </template>
        </div>
      </div>
      <div
        v-if="hoverSpan"
        class="trajectory__timeline-tooltip"
        :style="{ '--trajectory-hover-frac': hoverFraction ?? 0.5 }"
      >
        <span class="trajectory__timeline-tooltip-title">{{ TAG_LABEL[hoverSpan.kind] }} · {{ hoverSpan.label }}</span>
        <span class="trajectory__timeline-tooltip-time">
          {{ timelineMode === 'duration' ? formatMillis(hoverSpan.end - hoverSpan.start) : (findRow(hoverSpan.index)?.clock ?? '') }}
        </span>
      </div>
    </section>

    <div ref="ledgerRef" class="trajectory__ledger">
      <div ref="scrollRef" class="trajectory__scroll" @scroll.passive="onScroll">
        <div v-if="groups.length === 0" class="trajectory__empty">
          <p class="trajectory__empty-title">{{ t('sessions.trajectory.empty') }}</p>
          <p class="trajectory__empty-hint">{{ t('sessions.trajectory.emptyHint') }}</p>
        </div>

        <template v-else>
          <div class="trajectory__col-head" aria-hidden="true">
            <span class="trajectory__col-event">{{ t('sessions.trajectory.fieldEvent') }}</span>
            <span class="trajectory__col-time">{{ t('sessions.trajectory.timeColumn') }}</span>
          </div>

          <template v-for="{ group, collapsed, items, hiddenCount } in visibleGroups" :key="group.key">
            <header
              v-if="group.turnId !== null"
              class="trajectory__turn-head"
              :class="statusClass(group.status)"
              @click="toggleTurn(group.index)"
            >
              <ChevronDown
                class="trajectory__turn-chevron"
                :class="{ 'is-collapsed': collapsed }"
                :size="12"
                :stroke-width="2"
              />
              <span class="trajectory__turn-title">{{ t('sessions.trajectory.turn') }} {{ group.index + 1 }}</span>
              <span v-if="statusLabel(group.status)" class="trajectory__turn-status">
                {{ statusLabel(group.status) }}
              </span>
              <span v-if="group.agentName" class="trajectory__turn-agent">{{ group.agentName }}</span>
              <span v-if="group.goal" class="trajectory__turn-goal" :title="group.goal">{{ group.goal }}</span>
              <span class="trajectory__turn-meta">
                <template v-if="group.toolCount > 0">{{ group.toolCount }} tools · </template>
                <template v-if="group.tokens > 0">{{ formatTokenCount(group.tokens) }} · </template>
                {{ t('sessions.trajectory.timeColumn') }}
              </span>
              <button
                type="button"
                class="trajectory__turn-download"
                :title="t('sessions.trajectory.downloadLog')"
                :aria-label="t('sessions.trajectory.downloadLog')"
                @click.stop="downloadTurnLog(group.turnId!)"
              >
                <Download :size="13" :stroke-width="1.75" />
              </button>
            </header>

            <div v-if="collapsed" class="trajectory__collapsed" @click="toggleTurn(group.index)">
              <span class="trajectory__collapsed-text">
                {{ t('sessions.trajectory.collapsedSummary', { n: group.rows.length }) }}
              </span>
              <span class="trajectory__collapsed-hint">{{ t('sessions.trajectory.clickToExpand') }}</span>
            </div>

            <template v-else>
              <div v-if="hiddenCount > 0" class="trajectory__filtered-note">
                {{ t('sessions.trajectory.filteredCount', { n: hiddenCount }) }}
              </div>
              <template v-for="item in items" :key="item.type === 'batch' ? `b${item.key}` : item.row.seq">
                <div v-if="item.type === 'step'" class="trajectory__group-head">
                  <span class="trajectory__group-title">{{ item.title }}</span>
                  <span class="trajectory__group-desc">{{ item.row.elapsed }}</span>
                </div>

                <div
                  v-else-if="item.type === 'batch'"
                  class="trajectory__collapsed trajectory__batch"
                  @click="toggleBatch(item.key)"
                >
                  <span class="trajectory__collapsed-text">
                    {{ t('sessions.trajectory.toolsCollapsed', { n: item.rows.length }) }}
                  </span>
                  <span class="trajectory__collapsed-hint">{{ t('sessions.trajectory.clickToExpand') }}</span>
                </div>

                <div
                  v-else
                  class="trajectory__row"
                  :class="{
                    'is-selected': selectedSeq === item.row.seq,
                    'is-error': item.row.state === 'error',
                    'is-running': item.row.state === 'running' || item.row.state === 'pending',
                  }"
                  :data-row-seq="item.row.seq"
                  :data-kind="item.row.kind"
                  :data-timeline-focus="focusIndexes !== null && !focusIndexes.has(item.row.seq) ? 'outside' : undefined"
                  tabindex="0"
                  @click="selectRow(item.row)"
                  @dblclick="group.turnId !== null && toggleTurn(group.index)"
                >
                  <span class="trajectory__rail" aria-hidden="true" />
                  <span class="trajectory__tag-slot">
                    <span class="trajectory__tag" :class="`trajectory__tag--${item.row.kind}`">{{ item.row.tag }}</span>
                  </span>
                  <span class="trajectory__icon" v-html="KIND_ICON[item.row.kind]" />
                  <span class="trajectory__text" :title="item.row.text">{{ item.row.text }}</span>
                  <span v-if="item.row.state === 'running' || item.row.state === 'pending'" class="trajectory__live-dot" />
                  <span class="trajectory__time" :title="item.row.clock">{{ item.row.elapsed }}</span>
                  <button
                    v-if="item.row.hasDetail"
                    type="button"
                    class="trajectory__copy"
                    :title="t('sessions.trajectory.copy')"
                    :aria-label="t('sessions.trajectory.copy')"
                    @click.stop="copyRow(item.row)"
                  >
                    <Check v-if="copiedSeq === item.row.seq" :size="12" :stroke-width="2" class="trajectory__copy-check" />
                    <CopyDocument v-else :size="12" :stroke-width="1.75" />
                  </button>
                </div>
              </template>
            </template>
          </template>
        </template>
      </div>

      <aside
        v-if="selectedRow"
        class="trajectory__details"
        :style="{ width: `${detailsWidth}px` }"
        :aria-label="t('sessions.trajectory.detailsAria')"
      >
        <button
          type="button"
          class="trajectory__details-resize"
          aria-label="调整宽度"
          @pointerdown="startResize"
        />
        <div class="trajectory__details-header">
          <div class="trajectory__details-title">
            <span class="trajectory__tag" :class="`trajectory__tag--${selectedRow.kind}`">{{ selectedRow.tag }}</span>
            <span class="trajectory__details-location">
              <template v-if="selectedGroup && selectedGroup.turnId !== null">
                {{ t('sessions.trajectory.turn') }} {{ selectedGroup.index + 1 }} · {{ selectedRow.ev.type }}
              </template>
              <template v-else>{{ selectedRow.ev.type }}</template>
            </span>
          </div>
          <button
            type="button"
            class="trajectory__details-close"
            :aria-label="t('sessions.trajectory.closeDetails')"
            :title="t('sessions.trajectory.closeDetails')"
            @click="closeDetails"
          >
            <span aria-hidden="true">×</span>
          </button>
        </div>
        <div class="trajectory__detail-tabs" role="tablist" aria-label="事件详情">
          <button
            v-for="tab in selectedTabs"
            :key="tab.id"
            type="button"
            role="tab"
            :aria-selected="activeTabId === tab.id"
            class="trajectory__detail-tab"
            :class="{ 'is-active': activeTabId === tab.id }"
            @click="activateTab(tab.id)"
          >
            {{ tab.label }}
          </button>
        </div>
        <div class="trajectory__detail-body">
          <dl v-if="activeTabId === 'summary'" class="trajectory__overview">
            <div v-for="[label, value] in selectedSummaryFields" :key="label" class="trajectory__overview-row">
              <dt>{{ label }}</dt>
              <dd>{{ value }}</dd>
            </div>
          </dl>
          <pre v-else class="trajectory__detail-pre">{{ tabBody }}</pre>
        </div>
      </aside>
    </div>

    <button
      v-if="!atBottom"
      type="button"
      class="trajectory__to-bottom"
      :aria-label="t('sessions.trajectory.toBottom')"
      :title="t('sessions.trajectory.toBottom')"
      @click="scrollToBottom"
    >
      <ChevronDown :size="14" :stroke-width="2" />
    </button>
  </div>
</template>

<style scoped>
.trajectory {
  display: flex;
  flex-direction: column;
  overflow: hidden;
  height: 100%;
  min-height: 0;
  width: 100%;
  box-sizing: border-box;
  position: relative;
  color: var(--dq-label-primary);
}

/* ── Toolbar ─────────────────────────────────────────────── */

.trajectory__toolbar {
  flex: none;
  box-sizing: border-box;
  width: 100%;
  height: 32px;
  border-bottom: 1px solid var(--dq-border-subtle);
  display: flex;
  align-items: center;
  padding: 0 6px;
  gap: 8px;
  background: var(--dq-bg-base);
}

.trajectory__actions {
  display: flex;
  flex: none;
  align-items: center;
  gap: 2px;
}

.trajectory__toggle {
  display: inline-flex;
  flex: none;
  align-items: center;
  height: 20px;
  padding: 0 5px;
  gap: 4px;
  border: 0;
  border-radius: 3px;
  color: var(--dq-label-tertiary);
  background: transparent;
  cursor: pointer;
  font-size: 12px;
}

.trajectory__toggle:hover {
  color: var(--dq-label-primary);
  background: var(--dq-fill-control-hover);
}

.trajectory__toggle[aria-pressed='true'] {
  color: var(--dq-label-primary);
  background: var(--dq-fill-control-hover);
}

.trajectory__toggle:focus-visible {
  outline: 1px solid var(--dq-accent);
  outline-offset: 1px;
}

.trajectory__toggle-icon {
  flex: none;
  width: 12px;
  height: 12px;
  stroke: currentColor;
  stroke-width: 1.25;
  stroke-linecap: round;
  stroke-linejoin: round;
}

.trajectory__action-icon {
  color: var(--dq-label-tertiary);
  font-family: var(--dq-font-mono);
  font-size: 14px;
  line-height: 14px;
}

.trajectory__search {
  display: flex;
  flex: 0 1 164px;
  align-items: center;
  min-width: 84px;
  height: 22px;
  margin-left: auto;
  padding: 0 6px;
  gap: 4px;
  border: 1px solid var(--dq-border-subtle);
  border-radius: 4px;
  color: var(--dq-label-quaternary);
  background: var(--dq-bg-elevated);
}

.trajectory__search:hover {
  border-color: var(--dq-label-quaternary);
}

.trajectory__search:focus-within {
  border-color: var(--dq-accent);
  background: var(--dq-bg-base);
}

.trajectory__search-icon {
  flex: none;
}

.trajectory__search-input {
  min-width: 0;
  width: 100%;
  padding: 0;
  border: 0;
  outline: 0;
  color: var(--dq-label-primary);
  background: transparent;
  font-size: 12px;
}

.trajectory__search-input::placeholder {
  color: var(--dq-label-quaternary);
}

.trajectory__search-input::-webkit-search-cancel-button {
  width: 12px;
  height: 12px;
  cursor: pointer;
}

.trajectory__stats {
  flex: none;
  font-size: 12px;
  color: var(--dq-label-tertiary);
  font-variant-numeric: tabular-nums;
  white-space: nowrap;
}

/* ── Timeline overview ───────────────────────────────────── */

.trajectory__timeline {
  position: relative;
  z-index: 1;
  flex: none;
  border-bottom: 1px solid var(--dq-border-subtle);
  user-select: none;
}

.trajectory__plot {
  display: grid;
  grid-template-columns: 44px minmax(0, 1fr);
  height: 50px;
  overflow: hidden;
  background: var(--dq-bg-elevated);
}

.trajectory__lane-labels {
  position: relative;
  border-right: 1px solid var(--dq-border-hairline);
  color: var(--dq-label-quaternary);
  font-size: 10px;
  line-height: 1;
}

.trajectory__lane-labels span {
  position: absolute;
  right: 3px;
  display: flex;
  align-items: center;
  justify-content: flex-end;
  height: 8px;
  text-align: right;
}

.trajectory__lane-labels span:nth-child(1) {
  top: 7px;
}

.trajectory__lane-labels span:nth-child(2) {
  top: 21px;
}

.trajectory__lane-labels span:nth-child(3) {
  top: 35px;
}

.trajectory__track {
  position: relative;
  overflow: hidden;
  cursor: crosshair;
  touch-action: none;
}

.trajectory__track:focus-visible {
  outline: 1px solid var(--dq-accent);
  outline-offset: -1px;
}

.trajectory__track-empty {
  position: absolute;
  top: 50%;
  left: 50%;
  transform: translate(-50%, -50%);
  color: var(--dq-label-quaternary);
  font-size: 12px;
}

.trajectory__lanes {
  position: absolute;
  z-index: 2;
  top: 7px;
  bottom: 7px;
  left: var(--trajectory-domain-left);
  width: var(--trajectory-domain-width);
}

.trajectory__turn-boundaries {
  position: absolute;
  z-index: 3;
  top: 0;
  bottom: 0;
  left: var(--trajectory-domain-left);
  width: var(--trajectory-domain-width);
  pointer-events: none;
}

@media (prefers-reduced-motion: no-preference) {
  .trajectory__lanes[data-animate-viewport='true'],
  .trajectory__turn-boundaries[data-animate-viewport='true'] {
    transition: left 180ms ease-out;
  }
}

.trajectory__turn-boundary {
  position: absolute;
  top: 0;
  bottom: 0;
  left: var(--trajectory-turn-left);
  width: 1px;
  background: var(--dq-border);
}

.trajectory__span {
  position: absolute;
  left: calc(var(--trajectory-span-left) + 1px);
  width: max(2px, calc(var(--trajectory-span-width) - 2px));
  height: 8px;
  min-width: 2px;
  border-radius: 1px;
  background: var(--dq-label-tertiary);
  opacity: 0.78;
  pointer-events: auto;
}

.trajectory__span--user {
  background: var(--dq-accent);
}

.trajectory__span--assistant {
  background: color-mix(in srgb, var(--dq-accent) 60%, var(--dq-danger) 30%);
  opacity: 1;
}

.trajectory__span--think {
  background: color-mix(in srgb, var(--dq-accent) 40%, var(--dq-label-secondary) 40%);
  opacity: 1;
}

.trajectory__span--tool,
.trajectory__span--ask,
.trajectory__span--permission,
.trajectory__span--delegate {
  background: var(--dq-warning);
  opacity: 1;
}

.trajectory__span--compacted {
  background: var(--dq-label-secondary);
  opacity: 1;
}

.trajectory__span[data-error='true'] {
  background: var(--dq-danger);
}

.trajectory__span[data-equal-duration='true'] {
  width: 8px;
  min-width: 8px;
}

.trajectory__span[data-selected='false'] {
  opacity: 0.2;
}

.trajectory__span[data-current='true'] {
  z-index: 1;
  opacity: 1;
  box-shadow:
    0 0 0 1px var(--dq-bg-elevated),
    0 0 0 2px var(--dq-accent);
}

.trajectory__span[data-search-match='false'] {
  opacity: 0.14;
}

.trajectory__hover-line {
  position: absolute;
  z-index: 5;
  top: 0;
  bottom: 0;
  left: var(--trajectory-hover-left);
  width: 1px;
  background: var(--dq-label-quaternary);
  pointer-events: none;
}

.trajectory__selection {
  position: absolute;
  z-index: 1;
  top: 0;
  bottom: 0;
  left: var(--trajectory-selection-left, 0%);
  width: var(--trajectory-selection-width, 0%);
  min-width: 1px;
  background: color-mix(in srgb, var(--dq-accent) 12%, transparent);
  box-shadow:
    -100vw 0 0 100vw color-mix(in srgb, var(--dq-bg-page) 58%, transparent),
    100vw 0 0 100vw color-mix(in srgb, var(--dq-bg-page) 58%, transparent);
  pointer-events: none;
}

.trajectory__selection-edges {
  position: absolute;
  z-index: 4;
  top: 0;
  bottom: 0;
  left: var(--trajectory-selection-left, 0%);
  width: var(--trajectory-selection-width, 0%);
  min-width: 1px;
  pointer-events: none;
}

.trajectory__selection-edges::before,
.trajectory__selection-edges::after {
  content: '';
  position: absolute;
  top: 0;
  bottom: 0;
  width: 1px;
  background: var(--dq-accent);
}

.trajectory__selection-edges::before {
  left: 0;
}

.trajectory__selection-edges::after {
  right: -1px;
}

.trajectory__timeline-tooltip {
  position: absolute;
  z-index: 6;
  top: 56px;
  left: calc(44px + var(--trajectory-hover-frac, 0.5) * (100% - 44px));
  transform: translateX(-50%);
  display: flex;
  flex-direction: column;
  gap: 2px;
  max-width: 320px;
  padding: 4px 8px;
  border-radius: 4px;
  background: var(--dq-surface-elevated);
  border: 1px solid var(--dq-border);
  box-shadow: 0 2px 8px rgba(0, 0, 0, 0.4);
  pointer-events: none;
  white-space: nowrap;
}

.trajectory__timeline-tooltip-title {
  font-size: 11px;
  color: var(--dq-label-primary);
  overflow: hidden;
  text-overflow: ellipsis;
}

.trajectory__timeline-tooltip-time {
  font-family: var(--dq-font-mono);
  font-size: 11px;
  color: var(--dq-label-tertiary);
}

/* ── Ledger ──────────────────────────────────────────────── */

.trajectory__ledger {
  flex: 1;
  min-height: 0;
  display: flex;
  overflow: hidden;
}

.trajectory__scroll {
  flex: 1;
  min-width: 0;
  overflow-y: auto;
  padding-bottom: 12px;
  background: var(--dq-bg-base);
  scrollbar-width: thin;
  scrollbar-color: var(--dq-scrollbar-thumb) transparent;
}

.trajectory__scroll::-webkit-scrollbar {
  width: 8px;
}

.trajectory__scroll::-webkit-scrollbar-thumb {
  background: var(--dq-scrollbar-thumb);
  border-radius: 4px;
}

.trajectory__scroll::-webkit-scrollbar-thumb:hover {
  background: var(--dq-scrollbar-thumb-hover);
}

.trajectory__empty {
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  height: 100%;
  gap: 4px;
  text-align: center;
  padding: 0 24px;
}

.trajectory__empty-title {
  margin: 0;
  font-size: 13px;
  color: var(--dq-label-secondary);
}

.trajectory__empty-hint {
  margin: 0;
  font-size: 12px;
  color: var(--dq-label-tertiary);
  line-height: 1.6;
}

.trajectory__col-head {
  position: sticky;
  top: 0;
  z-index: 3;
  display: flex;
  align-items: center;
  justify-content: space-between;
  box-sizing: border-box;
  height: 30px;
  padding: 0 8px;
  border-bottom: 1px solid var(--dq-border-subtle);
  color: var(--dq-label-tertiary);
  background: var(--dq-bg-base);
  font-size: 12px;
  font-weight: 500;
  user-select: none;
}

.trajectory__col-event {
  padding-left: 12px;
}

.trajectory__col-time {
  flex: none;
  width: 52px;
  text-align: right;
}

.trajectory__turn-head {
  position: sticky;
  top: 30px;
  z-index: 2;
  display: flex;
  align-items: center;
  gap: 6px;
  box-sizing: border-box;
  width: 100%;
  height: 44px;
  padding: 0 8px;
  background: color-mix(in srgb, var(--dq-label-primary) 6%, transparent);
  cursor: pointer;
  user-select: none;
}

.trajectory__turn-chevron {
  flex: none;
  color: var(--dq-label-tertiary);
  transition: transform 0.15s ease;
}

.trajectory__turn-chevron.is-collapsed {
  transform: rotate(-90deg);
}

.trajectory__turn-title {
  flex: none;
  font-size: 13px;
  font-weight: 600;
  color: var(--dq-label-primary);
}

.trajectory__turn-status {
  flex: none;
  font-size: 12px;
  font-weight: 500;
  color: var(--dq-label-tertiary);
}

.trajectory__turn-head.is-running .trajectory__turn-status {
  color: var(--dq-accent);
}

.trajectory__turn-head.is-failed .trajectory__turn-status {
  color: var(--dq-danger);
}

.trajectory__turn-head.is-cancelled .trajectory__turn-status {
  color: var(--dq-label-quaternary);
  text-decoration: line-through;
}

.trajectory__turn-agent {
  flex: none;
  font-size: 12px;
  color: var(--dq-label-secondary);
}

.trajectory__turn-goal {
  flex: 1 1 auto;
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  font-size: 12px;
  color: var(--dq-label-tertiary);
}

.trajectory__turn-meta {
  flex: none;
  font-size: 12px;
  color: var(--dq-label-quaternary);
  font-variant-numeric: tabular-nums;
  white-space: nowrap;
}

.trajectory__turn-download {
  display: flex;
  align-items: center;
  justify-content: center;
  width: 22px;
  height: 22px;
  padding: 0;
  border: none;
  border-radius: 5px;
  background: transparent;
  color: var(--dq-label-tertiary);
  cursor: pointer;
  opacity: 0;
  transition: background 0.12s ease, color 0.12s ease, opacity 0.12s ease;
}

.trajectory__turn-head:hover .trajectory__turn-download,
.trajectory__turn-head:focus-within .trajectory__turn-download {
  opacity: 1;
}

.trajectory__turn-download:hover {
  background: var(--dq-fill-control-hover);
  color: var(--dq-label-primary);
}

.trajectory__group-head {
  display: flex;
  align-items: center;
  box-sizing: border-box;
  height: 36px;
  padding: 0 8px 0 20px;
  gap: 24px;
  min-width: 0;
  border-bottom: 1px solid var(--dq-border-hairline);
}

.trajectory__group-title {
  flex: none;
  font-size: 13px;
  color: var(--dq-label-primary);
}

.trajectory__group-desc {
  flex: 1 1 auto;
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  font-size: 13px;
  color: var(--dq-label-tertiary);
}

.trajectory__collapsed {
  display: flex;
  align-items: center;
  gap: 8px;
  box-sizing: border-box;
  height: 30px;
  padding: 0 8px 0 20px;
  border-bottom: 1px solid var(--dq-border-hairline);
  cursor: pointer;
}

.trajectory__collapsed:hover {
  background: var(--dq-surface-inset-hover);
}

.trajectory__collapsed-text {
  font-size: 13px;
  font-weight: 600;
  color: var(--dq-label-secondary);
}

.trajectory__collapsed-hint {
  font-size: 12px;
  color: var(--dq-label-quaternary);
}

.trajectory__filtered-note {
  padding: 4px 20px;
  font-size: 12px;
  color: var(--dq-label-quaternary);
}

.trajectory__row {
  position: relative;
  display: flex;
  align-items: center;
  box-sizing: border-box;
  height: 30px;
  padding: 0 8px 0 20px;
  gap: 10px;
  border-bottom: 1px solid var(--dq-border-hairline);
  min-width: 0;
  cursor: pointer;
  outline: none;
  transition:
    background-color 120ms ease,
    opacity 120ms ease;
}

.trajectory__row:hover {
  background: var(--dq-surface-inset-hover);
}

.trajectory__row:focus-visible {
  box-shadow: inset 0 0 0 1px var(--dq-accent);
}

.trajectory__row.is-selected {
  background: var(--dq-surface-list-selected);
}

.trajectory__row[data-timeline-focus='outside'] {
  opacity: 0.24;
}

.trajectory__rail {
  position: absolute;
  z-index: 4;
  left: 0;
  top: -1px;
  bottom: -1px;
  width: 2px;
  background: color-mix(in srgb, var(--dq-accent) 22%, var(--dq-bg-base));
  pointer-events: none;
}

.trajectory__tag-slot {
  flex: none;
  width: 76px;
  display: flex;
  align-items: center;
  min-width: 0;
}

.trajectory__tag {
  display: inline-flex;
  align-items: center;
  box-sizing: border-box;
  height: 22px;
  max-width: 100%;
  padding: 0 4px;
  border-radius: 6px;
  font-size: 12px;
  font-weight: 600;
  white-space: nowrap;
}

.trajectory__tag--system {
  color: var(--dq-label-secondary);
  background: var(--dq-fill-muted-surface);
}

.trajectory__tag--user {
  color: var(--dq-success);
  background: var(--dq-success-surface);
}

.trajectory__tag--think {
  color: color-mix(in srgb, var(--dq-success) 68%, var(--dq-label-secondary));
  background: var(--dq-success-surface);
}

.trajectory__tag--assistant {
  color: color-mix(in srgb, var(--dq-accent) 60%, var(--dq-label-secondary));
  background: var(--dq-accent-surface);
}

.trajectory__tag--tool {
  color: var(--dq-warning);
  background: var(--dq-warning-surface);
}

.trajectory__tag--delegate {
  color: color-mix(in srgb, var(--dq-warning) 62%, var(--dq-label-tertiary));
  background: color-mix(in srgb, var(--dq-warning-surface) 58%, transparent);
}

.trajectory__tag--ask {
  color: color-mix(in srgb, var(--dq-accent) 68%, var(--dq-label-secondary));
  background: var(--dq-accent-surface);
}

.trajectory__tag--permission {
  color: var(--dq-warning);
  background: var(--dq-warning-surface);
}

.trajectory__tag--compacted {
  color: var(--dq-label-secondary);
  background: var(--dq-fill-muted-surface);
}

.trajectory__tag--error {
  color: var(--dq-danger);
  background: var(--dq-danger-surface);
}

.trajectory__icon {
  flex: none;
  display: inline-flex;
  color: var(--dq-label-tertiary);
}

.trajectory__text {
  flex: 1 1 auto;
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  font-size: 13px;
  color: var(--dq-label-primary);
}

.trajectory__row.is-error .trajectory__text {
  color: var(--dq-label-secondary);
}

.trajectory__live-dot {
  flex: none;
  width: 5px;
  height: 5px;
  border-radius: 50%;
  background: var(--dq-accent);
  box-shadow: 0 0 0 3px color-mix(in srgb, var(--dq-accent) 18%, transparent);
  animation: trajectory-pulse 1.6s ease-in-out infinite;
}

@keyframes trajectory-pulse {
  0%,
  100% {
    opacity: 1;
  }
  50% {
    opacity: 0.35;
  }
}

.trajectory__time {
  flex: none;
  width: 52px;
  text-align: right;
  font-family: var(--dq-font-mono);
  font-size: 12px;
  color: var(--dq-label-tertiary);
  font-variant-numeric: tabular-nums;
  white-space: nowrap;
}

.trajectory__copy {
  display: flex;
  align-items: center;
  justify-content: center;
  width: 22px;
  height: 22px;
  padding: 0;
  border: none;
  border-radius: 5px;
  background: transparent;
  color: var(--dq-label-tertiary);
  cursor: pointer;
  opacity: 0;
  transition: background 0.12s ease, color 0.12s ease, opacity 0.12s ease;
}

.trajectory__row:hover .trajectory__copy,
.trajectory__row:focus-within .trajectory__copy {
  opacity: 1;
}

.trajectory__copy:hover {
  background: var(--dq-fill-control-hover);
  color: var(--dq-label-primary);
}

.trajectory__copy-check {
  color: var(--dq-success);
}

/* ── Details pane ────────────────────────────────────────── */

.trajectory__details {
  position: relative;
  display: flex;
  flex: none;
  flex-direction: column;
  min-width: 0;
  min-height: 0;
  border-left: 1px solid var(--dq-border-subtle);
  background: var(--dq-bg-base);
}

.trajectory__details-resize {
  position: absolute;
  z-index: 6;
  top: 0;
  bottom: 0;
  left: -4px;
  width: 8px;
  padding: 0;
  border: 0;
  background: transparent;
  cursor: col-resize;
  touch-action: none;
  user-select: none;
}

.trajectory__details-header {
  display: flex;
  flex: none;
  align-items: center;
  justify-content: space-between;
  box-sizing: border-box;
  height: 42px;
  padding: 0 8px 0 12px;
  border-bottom: 1px solid var(--dq-border-subtle);
}

.trajectory__details-title {
  display: flex;
  align-items: center;
  min-width: 0;
  gap: 8px;
}

.trajectory__details-location {
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  font-family: var(--dq-font-mono);
  font-size: 12px;
  color: var(--dq-label-tertiary);
}

.trajectory__details-close {
  display: flex;
  align-items: center;
  justify-content: center;
  flex: none;
  width: 22px;
  height: 22px;
  padding: 0;
  border: none;
  border-radius: 5px;
  background: transparent;
  color: var(--dq-label-tertiary);
  font-size: 16px;
  line-height: 1;
  cursor: pointer;
}

.trajectory__details-close:hover {
  background: var(--dq-fill-control-hover);
  color: var(--dq-label-primary);
}

.trajectory__detail-tabs {
  display: flex;
  flex: none;
  align-items: center;
  gap: 2px;
  box-sizing: border-box;
  height: 34px;
  padding: 0 6px;
  border-bottom: 1px solid var(--dq-border-subtle);
}

.trajectory__detail-tab {
  display: inline-flex;
  align-items: center;
  height: 24px;
  padding: 0 8px;
  border: 0;
  border-radius: 4px;
  background: transparent;
  color: var(--dq-label-tertiary);
  cursor: pointer;
  font-size: 12px;
}

.trajectory__detail-tab:hover {
  color: var(--dq-label-primary);
  background: var(--dq-fill-control-hover);
}

.trajectory__detail-tab.is-active {
  color: var(--dq-label-primary);
  background: var(--dq-fill-muted-surface);
}

.trajectory__detail-body {
  flex: 1;
  min-height: 0;
  overflow: auto;
}

.trajectory__overview {
  display: flex;
  flex-direction: column;
  gap: 1px;
  margin: 0;
  padding: 8px 12px;
}

.trajectory__overview-row {
  display: flex;
  align-items: baseline;
  gap: 10px;
  padding: 5px 0;
}

.trajectory__overview-row dt {
  flex: none;
  width: 76px;
  font-size: 12px;
  color: var(--dq-label-quaternary);
}

.trajectory__overview-row dd {
  flex: 1;
  min-width: 0;
  margin: 0;
  font-size: 12px;
  line-height: 1.6;
  color: var(--dq-label-primary);
  word-break: break-word;
}

.trajectory__detail-pre {
  margin: 0;
  padding: 10px 12px;
  font-family: var(--dq-font-mono);
  font-size: 12px;
  line-height: 1.6;
  color: var(--dq-label-secondary);
  white-space: pre-wrap;
  word-break: break-word;
}

.trajectory__to-bottom {
  position: absolute;
  right: 14px;
  bottom: 12px;
  z-index: 3;
  display: flex;
  align-items: center;
  justify-content: center;
  width: 28px;
  height: 28px;
  padding: 0;
  border: 1px solid var(--dq-border);
  border-radius: 50%;
  background: var(--dq-surface-elevated);
  color: var(--dq-label-secondary);
  cursor: pointer;
  box-shadow: 0 2px 8px rgba(0, 0, 0, 0.4);
}

.trajectory__to-bottom:hover {
  color: var(--dq-label-primary);
  border-color: var(--dq-border-strong);
}
</style>
