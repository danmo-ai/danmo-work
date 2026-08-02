<script setup lang="ts">
import { computed, ref, watch, onMounted, onUnmounted, nextTick } from 'vue'
import { storeToRefs } from 'pinia'
import { useRouter } from 'vue-router'
import { useI18n } from 'vue-i18n'
import { useSessionsStore } from '@/stores/sessions'
import { useWorkspaceUiStore } from '@/stores/workspaceUi'
import { useSessionActivityStore } from '@/stores/sessionActivity'
import FloatingComposer from '@/components/composer/FloatingComposer.vue'
import ComposerPendingDecisions from '@/components/composer/ComposerPendingDecisions.vue'
import ComposerPendingQueue from '@/components/composer/ComposerPendingQueue.vue'
import WelcomeEmpty from '@/components/center/WelcomeEmpty.vue'
import ApprovalRail from '@/components/center/ApprovalRail.vue'
import ActiveSessionsBar from '@/components/center/ActiveSessionsBar.vue'
import ToolCardBlock from '@/components/center/ToolCardBlock.vue'
import ToolCardGroup from '@/components/center/ToolCardGroup.vue'
import TurnSection from '@/components/center/TurnSection.vue'
import AgentMessageBlock from '@/components/center/AgentMessageBlock.vue'
import ThinkingBlock from '@/components/center/ThinkingBlock.vue'
import PermissionAskBlock from '@/components/center/PermissionAskBlock.vue'
import AskUserBlock, { type AskUserFormField } from '@/components/center/AskUserBlock.vue'
import { groupConsecutiveToolCards, useTurnCollapse, type StreamTurn, type ToolCard, type UserImageAttachment } from '@/composables/useStreamTurns'
import RightWorkspacePanel from '@/components/center/RightWorkspacePanel.vue'
import DocumentStage from '@/components/office/DocumentStage.vue'
import {
  DqDrawer,
  Document,
  FolderChecked,
  MagicStick,
  Terminal,
  Library,
  Grid,
} from '@danqing/dq-shell'
import type { RightWorkspaceTab } from '@/stores/workspaceUi'
import { renderMarkdown } from '@/utils/markdown-render'
import { toast } from '@/utils/feedback'
import { apiBaseUrl, saveBlobAs } from '@/utils/desktop'
import type { ElementAttachment } from '@/types/element-attachment'
import type { CodeSelectionAttachment } from '@/types/code-attachment'
import type { OfficeEditAttachment } from '@/types/office-edit-attachment'
import { fetchJSON } from '@/api/client'
import { formatTokenCount, useSessionContextUsage } from '@/composables/useSessionContextUsage'
import { routeOfficeFile } from '@/utils/office-route'

import type { StreamEvent, TurnLog } from '@/types/mission'

const router = useRouter()
const { t } = useI18n()
const sessions = useSessionsStore()
const workspaceUi = useWorkspaceUiStore()
const sessionActivity = useSessionActivityStore()
const { rightTab, rightDrawerOpen, stage, layoutMode, changesCount, memoryCount } = storeToRefs(workspaceUi)
const rightPanelRef = ref<InstanceType<typeof RightWorkspacePanel> | null>(null)
const { tokensForTurn } = useSessionContextUsage()
const isEditingTitle = ref(false)
const editingTitle = ref('')
const composerRef = ref<InstanceType<typeof FloatingComposer> | null>(null)

const bodyRef = ref<HTMLElement | null>(null)

const isStageLayout = computed(() => layoutMode.value === 'stage' && !!stage.value)

/** Grid columns for chat / stage; immersive leaves layout to CSS. */
const bodyGridStyle = computed(() => {
  if (layoutMode.value === 'immersive' && stage.value) return undefined
  if (isStageLayout.value) {
    return { gridTemplateColumns: 'minmax(200px, 32%) minmax(0, 1fr)' }
  }
  return { gridTemplateColumns: 'minmax(0, 1fr)' }
})

const rightDrawerTitle = computed(() => {
  const map: Record<RightWorkspaceTab, string> = {
    plan: t('sessions.tabPlan'),
    files: t('sessions.tabFiles'),
    memory: t('sessions.tabMemory'),
    tables: t('sessions.tabTables'),
    changes: t('sessions.tabChanges'),
    terminal: t('sessions.tabTerminal'),
  }
  return map[rightTab.value]
})

const rightIconItems = computed(() => {
  const plan = planProgress.value
  return [
    {
      value: 'plan' as const,
      label: t('sessions.tabPlan'),
      icon: MagicStick,
      badge: plan.total > 0 ? `${plan.completed}/${plan.total}` : undefined,
    },
    { value: 'files' as const, label: t('sessions.tabFiles'), icon: Document },
    {
      value: 'memory' as const,
      label: t('sessions.tabMemory'),
      icon: Library,
      badge: memoryCount.value > 0 ? memoryCount.value : undefined,
    },
    { value: 'tables' as const, label: t('sessions.tabTables'), icon: Grid },
    {
      value: 'changes' as const,
      label: t('sessions.tabChanges'),
      icon: FolderChecked,
      badge: changesCount.value > 0 ? changesCount.value : undefined,
    },
    { value: 'terminal' as const, label: t('sessions.tabTerminal'), icon: Terminal },
  ]
})

function onRightIconClick(tab: RightWorkspaceTab) {
  workspaceUi.toggleRightDrawer(tab)
}

async function openFileInOffice(filePath: string) {
  if (!sessions.selectedProjectId) return
  let contentHint: string | undefined
  try {
    if (/\.(md|markdown|html?)$/i.test(filePath)) {
      const fc = await fetchJSON<{ content: string }>(
        `/projects/${sessions.selectedProjectId}/files/content?path=${encodeURIComponent(filePath)}`,
      )
      contentHint = fc.content
    }
  } catch {
    /* routing can proceed without hint */
  }
  const routed = routeOfficeFile(filePath, contentHint)
  if (routed.kind === 'preview') {
    const url = `${apiBaseUrl()}/api/v1/projects/${sessions.selectedProjectId}/raw/${encodeURIComponent(filePath)}`
    workspaceUi.openStage({ ...routed, url })
    return
  }
  workspaceUi.openStage(routed)
}

function onStageAttachElement(att: ElementAttachment) {
  composerRef.value?.addElementAttachment(att)
}

function onStageAttachCodeSelection(att: CodeSelectionAttachment) {
  composerRef.value?.addCodeSelectionAttachment(att)
}

function onStageAttachOfficeEdit(att: OfficeEditAttachment) {
  composerRef.value?.addOfficeEditAttachment(att)
}

/** Manual expand/collapse overrides for individual tool cards. */
const toolCardExpandOverride = ref(new Map<number, boolean>())
function findToolCardBySeq(seq: number): ToolCard | undefined {
  for (const turn of visibleTurns.value) {
    for (const ev of turn.events) {
      if (ev.type === '__tool_card__' && ev.seq === seq) return ev.payload as ToolCard
      if (ev.type === '__tool_group__') {
        const hit = toolGroupCards(ev).find((c) => c.seq === seq)
        if (hit) return hit
      }
    }
  }
  return undefined
}
function isToolCardExpanded(seq: number) {
  const override = toolCardExpandOverride.value.get(seq)
  if (override !== undefined) return override
  const card = findToolCardBySeq(seq)
  if (!card) return false
  if (card.name === 'delegate_agent' && delegateCardAwaiting(seq)) return true
  return card.status === 'running' || card.status === 'pending'
}
function toggleToolCard(seq: number) {
  toolCardExpandOverride.value.set(seq, !isToolCardExpanded(seq))
  toolCardExpandOverride.value = new Map(toolCardExpandOverride.value)
}

/** Manual expand/collapse overrides for consecutive tool groups (keyed by first card seq). */
const toolGroupExpandOverride = ref(new Map<number, boolean>())
function toggleToolGroup(seq: number, cards: ToolCard[]) {
  const next = !isToolGroupExpanded(seq, cards)
  toolGroupExpandOverride.value.set(seq, next)
  toolGroupExpandOverride.value = new Map(toolGroupExpandOverride.value)
}
function isToolGroupExpanded(seq: number, cards: ToolCard[]): boolean {
  const override = toolGroupExpandOverride.value.get(seq)
  if (override !== undefined) return override
  // Default: open while in-flight / awaiting; collapse when settled (including errors).
  return cards.some((c) => c.status === 'running' || c.status === 'pending')
}

const expandedThinking = ref(new Set<number>())
function toggleThinking(seq: number) {
  if (expandedThinking.value.has(seq)) {
    expandedThinking.value.delete(seq)
  } else {
    expandedThinking.value.add(seq)
  }
  expandedThinking.value = new Set(expandedThinking.value)
}
function isThinkingExpanded(seq: number) {
  return expandedThinking.value.has(seq)
}

// ── Smart auto-scroll ──
const userScrolledUp = ref(false)
let scrollTimeout: ReturnType<typeof setTimeout> | null = null
function onScrollAreaScroll() {
  const el = scrollAreaRef.value
  if (!el) return
  const threshold = 120
  const nearBottom = el.scrollHeight - el.scrollTop - el.clientHeight < threshold
  if (!nearBottom) {
    userScrolledUp.value = true
    if (scrollTimeout) clearTimeout(scrollTimeout)
    scrollTimeout = setTimeout(() => { userScrolledUp.value = false }, 3000)
  } else {
    userScrolledUp.value = false
  }
}
function autoScrollToBottom(force = false) {
  if (!force && userScrolledUp.value) return
  // Prefer keeping ask_user / permission cards clear of the floating composer.
  if (!force && workspaceUi.pendingApprovals > 0) return
  void nextTick(() => {
    const el = scrollAreaRef.value
    if (el) {
      el.scrollTop = el.scrollHeight
    }
  })
}

watch(
  () => sessions.streamEvents.length,
  () => { autoScrollToBottom() },
)
watch(
  () => sessions.streamEvents.at(-1)?.type,
  () => { autoScrollToBottom() },
)

const scrollAreaRef = ref<HTMLElement | null>(null)
const composerWrapRef = ref<HTMLElement | null>(null)
const composerStyle = ref<Record<string, string>>({})
/** Floating composer overlay height — keeps stream content above it. */
const composerOverlayPx = ref(140)
let composerResizeObs: ResizeObserver | null = null

function chatColumnMaxPx(): number {
  const raw = getComputedStyle(document.documentElement).getPropertyValue('--dq-chat-column-max').trim()
  const n = Number.parseFloat(raw)
  return Number.isFinite(n) && n > 0 ? n : 920
}

function updateComposerPosition() {
  const el = scrollAreaRef.value
  if (!el) return
  const rect = el.getBoundingClientRect()
  const gutter = 48
  const w = Math.min(chatColumnMaxPx(), Math.max(280, rect.width - gutter))
  composerStyle.value = {
    left: `${rect.left + (rect.width - w) / 2}px`,
    width: `${w}px`,
  }
}

function updateComposerOverlayHeight() {
  const wrap = composerWrapRef.value
  if (!wrap) return
  const h = Math.ceil(wrap.getBoundingClientRect().height)
  if (h > 0) composerOverlayPx.value = h
}

function syncComposerLayout() {
  updateComposerPosition()
  updateComposerOverlayHeight()
}

watch(layoutMode, () => { nextTick(syncComposerLayout) })
watch(rightDrawerOpen, () => { nextTick(syncComposerLayout) })
watch(
  () => workspaceUi.pendingApprovals,
  () => { nextTick(syncComposerLayout) },
)
onMounted(() => {
  nextTick(() => {
    syncComposerLayout()
    if (typeof ResizeObserver !== 'undefined' && composerWrapRef.value) {
      composerResizeObs = new ResizeObserver(() => updateComposerOverlayHeight())
      composerResizeObs.observe(composerWrapRef.value)
    }
  })
  window.addEventListener('resize', syncComposerLayout)
})
onUnmounted(() => {
  composerResizeObs?.disconnect()
  composerResizeObs = null
  window.removeEventListener('resize', syncComposerLayout)
})

type Turn = StreamTurn

// ── Tool SVG icon mapping ──
function toolSvgIcon(name: string): string {
  const n = name.toLowerCase()
  const base = 'width="15" height="15" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round"'
  if (n.includes('read') || n.includes('open_file') || n.includes('view'))
    return `<svg ${base}><path d="M14 2H6a2 2 0 0 0-2 2v16a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V8z"/><polyline points="14 2 14 8 20 8"/><line x1="16" y1="13" x2="8" y2="13"/><line x1="16" y1="17" x2="8" y2="17"/></svg>`
  if (n.includes('write') || n.includes('create_file') || n.includes('edit') || n.includes('search_replace') || n.includes('replace'))
    return `<svg ${base}><path d="M11 4H4a2 2 0 0 0-2 2v14a2 2 0 0 0 2 2h14a2 2 0 0 0 2-2v-7"/><path d="M18.5 2.5a2.121 2.121 0 0 1 3 3L12 15l-4 1 1-4 9.5-9.5z"/></svg>`
  if (n.includes('search') || n.includes('grep') || n.includes('find') || n.includes('glob') || n.includes('codebase') || n.includes('lsp'))
    return `<svg ${base}><circle cx="11" cy="11" r="8"/><line x1="21" y1="21" x2="16.65" y2="16.65"/></svg>`
  if (n.includes('bash') || n.includes('terminal') || n.includes('execute') || n.includes('run') || n.includes('shell'))
    return `<svg ${base}><polyline points="4 17 10 11 4 5"/><line x1="12" y1="19" x2="20" y2="19"/></svg>`
  if (n.includes('browser') || n.includes('web') || n.includes('fetch') || n.includes('navigate'))
    return `<svg ${base}><circle cx="12" cy="12" r="10"/><line x1="2" y1="12" x2="22" y2="12"/><path d="M12 2a15.3 15.3 0 0 1 4 10 15.3 15.3 0 0 1-4 10 15.3 15.3 0 0 1-4-10 15.3 15.3 0 0 1 4-10z"/></svg>`
  if (n.includes('delegate') || n.includes('agent') || n.includes('task'))
    return `<svg ${base}><path d="M17 21v-2a4 4 0 0 0-4-4H5a4 4 0 0 0-4 4v2"/><circle cx="9" cy="7" r="4"/><path d="M23 21v-2a4 4 0 0 0-3-3.87"/><path d="M16 3.13a4 4 0 0 1 0 7.75"/></svg>`
  if (n.includes('ask_user') || n.includes('question') || n.includes('approval') || n.includes('permission'))
    return `<svg ${base}><path d="M21 15a2 2 0 0 1-2 2H7l-4 4V5a2 2 0 0 1 2-2h14a2 2 0 0 1 2 2z"/></svg>`
  if (n.includes('plan') || n.includes('todo') || n.includes('todowrite'))
    return `<svg ${base}><path d="M9 11l3 3L22 4"/><path d="M21 12v7a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2h11"/></svg>`
  if (n.includes('git') || n.includes('commit') || n.includes('branch'))
    return `<svg ${base}><line x1="6" y1="3" x2="6" y2="15"/><circle cx="18" cy="6" r="3"/><circle cx="6" cy="18" r="3"/><path d="M18 9a9 9 0 0 1-9 9"/></svg>`
  if (n.includes('memory') || n.includes('knowledge') || n.includes('remember'))
    return `<svg ${base}><path d="M12 2a7 7 0 0 1 7 7c0 2.38-1.19 4.47-3 5.74V17a1 1 0 0 1-1 1H9a1 1 0 0 1-1-1v-2.26C6.19 13.47 5 11.38 5 9a7 7 0 0 1 7-7z"/><line x1="9" y1="21" x2="15" y2="21"/></svg>`
  if (n.includes('skill') || n.includes('capability'))
    return `<svg ${base}><polygon points="12 2 15.09 8.26 22 9.27 17 14.14 18.18 21.02 12 17.77 5.82 21.02 7 14.14 2 9.27 8.91 8.26 12 2"/></svg>`
  return `<svg ${base}><circle cx="12" cy="12" r="3"/><path d="M19.4 15a1.65 1.65 0 0 0 .33 1.82l.06.06a2 2 0 0 1-2.83 2.83l-.06-.06a1.65 1.65 0 0 0-1.82-.33 1.65 1.65 0 0 0-1 1.51V21a2 2 0 0 1-4 0v-.09A1.65 1.65 0 0 0 9 19.4a1.65 1.65 0 0 0-1.82.33l-.06.06a2 2 0 0 1-2.83-2.83l.06-.06A1.65 1.65 0 0 0 4.68 15a1.65 1.65 0 0 0-1.51-1H3a2 2 0 0 1 0-4h.09A1.65 1.65 0 0 0 4.6 9a1.65 1.65 0 0 0-.33-1.82l-.06-.06a2 2 0 0 1 2.83-2.83l.06.06A1.65 1.65 0 0 0 9 4.68a1.65 1.65 0 0 0 1-1.51V3a2 2 0 0 1 4 0v.09a1.65 1.65 0 0 0 1 1.51 1.65 1.65 0 0 0 1.82-.33l.06-.06a2 2 0 0 1 2.83 2.83l-.06.06A1.65 1.65 0 0 0 19.4 9a1.65 1.65 0 0 0 1.51 1H21a2 2 0 0 1 0 4h-.09a1.65 1.65 0 0 0-1.51 1z"/></svg>`
}

// ── Tool duration calculation ──
function toolDuration(startSeq: number, endSeq: number, events: StreamEvent[]): number | null {
  const startEv = events.find(e => e.seq === startSeq)
  const endEv = events.find(e => e.seq === endSeq)
  if (startEv?.createdAt && endEv?.createdAt) {
    return new Date(endEv.createdAt).getTime() - new Date(startEv.createdAt).getTime()
  }
  return null
}

function formatDuration(ms: number): string {
  if (ms < 1000) return `${ms}ms`
  if (ms < 60000) return `${(ms / 1000).toFixed(1)}s`
  const mins = Math.floor(ms / 60000)
  const secs = Math.floor((ms % 60000) / 1000)
  return `${mins}m ${secs}s`
}

function mergeToolCard(toolCards: Record<string, ToolCard>, ev: StreamEvent) {
  const p = asRecord(ev.payload)
  const callId = String(p?.callId ?? '')
  if (!callId) return

  const inputStr = toolInputRaw(p)
  const existing = toolCards[callId]

  if (existing) {
    if (inputStr) existing.inputStr = inputStr
    if (p?.description) existing.description = String(p.description)
    if (ev.type === 'tool.pending') {
      existing.status = 'pending'
    } else if (ev.type === 'tool.running') {
      existing.status = 'running'
    } else if (ev.type === 'tool.completed') {
      existing.status = 'completed'
      existing.output = String(p?.output ?? '')
    } else if (ev.type === 'tool.error') {
      const errMsg = String(p?.error ?? '')
      existing.status = errMsg === 'cancelled' || /context canceled/i.test(errMsg) ? 'cancelled' : 'error'
      existing.error = errMsg
    }
    return
  }

  let status = 'pending'
  if (ev.type === 'tool.running') status = 'running'
  else if (ev.type === 'tool.completed') status = 'completed'
  else if (ev.type === 'tool.error') {
    const errMsg = String(p?.error ?? '')
    status = errMsg === 'cancelled' || /context canceled/i.test(errMsg) ? 'cancelled' : 'error'
  }

  toolCards[callId] = {
    callId,
    name: String(p?.name ?? ''),
    description: String(p?.description ?? ''),
    status,
    inputStr: inputStr || '',
    output: String(p?.output ?? ''),
    error: String(p?.error ?? ''),
    seq: ev.seq,
    stepNum: 0,
  }
}

function toolInputRaw(p: Record<string, unknown> | null): string {
  if (!p) return ''
  const input = p.input ?? p.arguments ?? p.args
  if (!input) return ''
  try {
    return JSON.stringify(input, null, 2)
  } catch {
    return String(input)
  }
}

function toolInputFields(inputStr: string): Array<{ key: string; value: string }> | null {
  if (!inputStr) return null
  try {
    const obj = JSON.parse(inputStr)
    if (typeof obj !== 'object' || obj === null || Array.isArray(obj)) return null
    return Object.entries(obj).map(([key, value]) => ({
      key,
      value: typeof value === 'string' ? value : JSON.stringify(value, null, 2),
    }))
  } catch {
    return null
  }
}

const currentTurnId = ref<string | null>(null)

watch(
  () => sessions.currentSession?.id,
  () => {
    currentTurnId.value = null
    toolGroupExpandOverride.value = new Map()
    toolCardExpandOverride.value = new Map()
    clearCollapseOverrides()
  },
)

const turnMap = computed(() => {
  const map: Record<string, Turn> = {}
  let activeTurnId: string | null = null
  const turnToolCards: Record<string, Record<string, ToolCard>> = {}

  for (const ev of sessions.streamEvents) {
    if (ev.type === 'turn.started') {
      const payload = asRecord(ev.payload)
      const turnId = ev.turnId || String(payload?.turnId ?? ev.seq)
      if (!map[turnId]) {
        map[turnId] = {
          id: turnId,
          goal: '',
          events: [],
          childTurnIds: [],
        }
      }
      map[turnId].goal = String(payload?.goal ?? map[turnId].goal)
      map[turnId].agentId = String(payload?.agentId ?? map[turnId].agentId)
      map[turnId].agentName = String(payload?.agentName ?? payload?.agentId ?? map[turnId].agentName ?? 'AI')
      map[turnId].events.push(ev)
      activeTurnId = turnId
      continue
    }

    const turnId = ev.turnId || activeTurnId
    if (!turnId) continue
    if (!map[turnId]) {
      // turn.started can be dropped from the SSE buffer; still surface interactive cards.
      if (ev.type !== 'permission.ask' && ev.type !== 'ask_user.pending') continue
      map[turnId] = {
        id: turnId,
        goal: '',
        events: [],
        childTurnIds: [],
        agentName: 'AI',
      }
    }

    if (ev.type.startsWith('tool.')) {
      if (!turnToolCards[turnId]) turnToolCards[turnId] = {}
      mergeToolCard(turnToolCards[turnId], ev)
      continue
    }

    map[turnId].events.push(ev)
    if (ev.type === 'user.message') {
      const payload = asRecord(ev.payload)
      map[turnId].userText = String(payload?.content ?? payload?.text ?? '')
      const rawAtts = payload?.attachments
      if (Array.isArray(rawAtts)) {
        map[turnId].userImages = rawAtts
          .map((a) => {
            const r = asRecord(a)
            const dataUrl = String(r?.dataUrl ?? '')
            if (!dataUrl) return null
            const att: UserImageAttachment = { dataUrl }
            if (r?.name) att.name = String(r.name)
            if (r?.mimeType) att.mimeType = String(r.mimeType)
            return att
          })
          .filter((x): x is UserImageAttachment => x !== null)
      }
    }
    if (ev.type === 'turn.ended' || ev.type === 'turn.failed') {
      const payload = asRecord(ev.payload)
      map[turnId].status = String(payload?.status ?? '')
      // Historical gap: cancel could drop tool.error from DB; close open cards.
      const openCards = turnToolCards[turnId]
      if (openCards) {
        const failed = ev.type === 'turn.failed'
        const kind = String(payload?.kind ?? '')
        for (const card of Object.values(openCards)) {
          if (card.status === 'running' || card.status === 'pending') {
            card.status = failed || kind === 'cancelled' ? 'cancelled' : 'error'
            if (!card.error) card.error = failed ? String(payload?.message ?? 'cancelled') : 'interrupted'
          }
        }
      }
      activeTurnId = null
    }
  }

  for (const turnId in turnToolCards) {
    const turn = map[turnId]
    if (!turn) continue

    // Build step number lookup: track step changes with seq positions
    const stepBoundaries: Array<{ seq: number; step: number }> = [{ seq: -1, step: 0 }]
    for (const ev of turn.events) {
      if (ev.type === 'step.started') {
        const p = asRecord(ev.payload)
        const step = Number(p?.step ?? 0)
        stepBoundaries.push({ seq: ev.seq, step })
      }
    }

    function stepAtSeq(targetSeq: number): number {
      let result = 0
      for (const b of stepBoundaries) {
        if (b.seq <= targetSeq) result = b.step
        else break
      }
      return result
    }

    for (const card of Object.values(turnToolCards[turnId])) {
      const idx = turn.events.findIndex((e) => e.seq > card.seq)
      const stepNum = stepAtSeq(card.seq)
      const synth = {
        seq: card.seq,
        type: '__tool_card__',
        sessionId: '',
        turnId,
        createdAt: '',
        payload: { ...card, stepNum },
      } as unknown as StreamEvent
      if (idx === -1) {
        turn.events.push(synth)
      } else {
        turn.events.splice(idx, 0, synth)
      }
    }
  }

  // Post-process: filter noise events, then aggregate consecutive tool cards
  const NOISE_TYPES = new Set(['turn.started', 'turn.ended', 'turn.failed', 'step.started', 'step.ended', 'llm.usage'])
  for (const turnId in map) {
    const turn = map[turnId]
    turn.events = groupConsecutiveToolCards(turn.events.filter((ev) => !NOISE_TYPES.has(ev.type)))
  }

  for (const ev of sessions.streamEvents) {
    if (ev.type === 'delegate.started') {
      const payload = asRecord(ev.payload)
      const childTurnId = String(payload?.childTurnId ?? '')
      const parentTurnId = ev.turnId
      if (childTurnId && map[childTurnId]) {
        map[childTurnId].parentTurnId = parentTurnId
        if (parentTurnId && map[parentTurnId] && !map[parentTurnId].childTurnIds.includes(childTurnId)) {
          map[parentTurnId].childTurnIds.push(childTurnId)
        }
      }
    }
  }

  return map
})

const rootTurns = computed(() => {
  return Object.values(turnMap.value)
    .filter((t) => !t.parentTurnId)
    .sort((a, b) => (a.events[0]?.seq ?? 0) - (b.events[0]?.seq ?? 0))
})

/** Turn whose todowrite PlanPanel should show (drill-in turn, else latest/running root). */
const planTurnId = computed(() => {
  if (currentTurnId.value) return currentTurnId.value
  const roots = rootTurns.value
  if (!roots.length) return null
  for (let i = roots.length - 1; i >= 0; i--) {
    if (!roots[i].status) return roots[i].id
  }
  return roots[roots.length - 1].id
})

/** Latest todowrite snapshot for the plan turn — badge shows completed/total. */
const planProgress = computed(() => {
  const turnId = planTurnId.value
  const events = sessions.streamEvents
  for (let i = events.length - 1; i >= 0; i--) {
    const ev = events[i]
    if (turnId && ev.turnId !== turnId) continue
    if (ev.type !== 'tool.running') continue
    const p = ev.payload as Record<string, unknown> | null
    if (p?.name !== 'todowrite') continue
    const input = p?.input as Record<string, unknown> | null
    const items = input?.todos
    if (!Array.isArray(items) || items.length === 0) continue
    const total = items.length
    const completed = items.filter(
      (item) => String((item as { status?: string })?.status ?? '') === 'completed',
    ).length
    return { completed, total }
  }
  return { completed: 0, total: 0 }
})

const visibleTurns = computed(() => {
  if (!currentTurnId.value) return rootTurns.value
  const turn = turnMap.value[currentTurnId.value]
  return turn ? [turn] : []
})

const { isTurnCollapsed, toggleTurnCollapse, ensureTurnExpanded, clearCollapseOverrides } = useTurnCollapse(() => visibleTurns.value)

const breadcrumbs = computed(() => {
  const path: { id: string | null; label: string }[] = [{ id: null, label: '全部 Turn' }]
  if (!currentTurnId.value) return path

  const stack: { id: string; label: string }[] = []
  let id: string | null = currentTurnId.value
  while (id) {
    const turn: Turn | undefined = turnMap.value[id]
    if (!turn) break
    stack.unshift({ id, label: formatTurnGoal(turn.goal) || turn.id })
    id = turn.parentTurnId ?? null
  }
  return [...path, ...stack]
})

function navigateToTurn(turnId: string | null) {
  currentTurnId.value = turnId
}

function childTurnIdFromDelegate(ev: StreamEvent): string | null {
  if (ev.type !== 'delegate.started') return null
  const p = asRecord(ev.payload)
  const id = String(p?.childTurnId ?? '')
  return id || null
}

function forEachToolCard(ev: StreamEvent, fn: (card: ToolCard) => void) {
  if (ev.type === '__tool_card__') {
    fn(ev.payload as ToolCard)
  } else if (ev.type === '__tool_group__') {
    const cards = toolGroupCards(ev)
    for (const c of cards) fn(c)
  }
}

function toolGroupCards(ev: StreamEvent): ToolCard[] {
  const p = ev.payload as { cards?: ToolCard[] } | null
  return Array.isArray(p?.cards) ? p.cards : []
}

const delegateLinkMap = computed(() => {
  const m = new Map<number, string>()
  for (const turn of Object.values(turnMap.value)) {
    // Parallel delegate_agent cards land in one __tool_group__; match each
    // delegate.started by callId when present, else FIFO by appearance order.
    const seqByCallId = new Map<string, number>()
    const pendingSeqs: number[] = []
    for (const ev of turn.events) {
      forEachToolCard(ev, (p) => {
        if (p.name !== 'delegate_agent') return
        pendingSeqs.push(p.seq)
        if (p.callId) seqByCallId.set(p.callId, p.seq)
      })
      if (ev.type !== 'delegate.started') continue
      const payload = asRecord(ev.payload)
      const childTurnId = String(payload?.childTurnId ?? '')
      if (!childTurnId) continue

      const callId = String(payload?.callId ?? '')
      let seq = -1
      if (callId && seqByCallId.has(callId)) {
        seq = seqByCallId.get(callId)!
        seqByCallId.delete(callId)
        const idx = pendingSeqs.indexOf(seq)
        if (idx >= 0) pendingSeqs.splice(idx, 1)
      } else if (pendingSeqs.length > 0) {
        seq = pendingSeqs.shift()!
        for (const [cid, s] of seqByCallId) {
          if (s === seq) {
            seqByCallId.delete(cid)
            break
          }
        }
      }
      if (seq >= 0) m.set(seq, childTurnId)
    }
  }
  return m
})

function delegateChildTurnId(seq: number): string | null {
  return delegateLinkMap.value.get(seq) ?? null
}

function groupCardAwaitingApproval(cards: ToolCard[], seq: number): boolean {
  const card = cards.find((c) => c.seq === seq)
  return !!card && card.name === 'delegate_agent' && delegateCardAwaiting(seq)
}

function groupCardShowChildLink(cards: ToolCard[], seq: number): boolean {
  const card = cards.find((c) => c.seq === seq)
  return !!card && card.name === 'delegate_agent' && !!delegateChildTurnId(seq)
}

/** Child turn has undecided permission.ask (same session stream, child turnId). */
function childTurnNeedsApproval(childTurnId: string | null): boolean {
  if (!childTurnId) return false
  return sessions.pendingApprovals.some((e) => e.turnId === childTurnId)
}

/** Child turn has unresolved ask_user.pending. */
function childTurnNeedsAsk(childTurnId: string | null): boolean {
  if (!childTurnId) return false
  return sessions.pendingAsks.some((e) => e.turnId === childTurnId && isAskActionable(e))
}

function childTurnNeedsAttention(childTurnId: string | null): boolean {
  return childTurnNeedsApproval(childTurnId) || childTurnNeedsAsk(childTurnId)
}

function delegateCardAwaiting(seq: number): boolean {
  return childTurnNeedsAttention(delegateChildTurnId(seq))
}

function delegateCardAwaitingLabel(seq: number): string {
  const childId = delegateChildTurnId(seq)
  if (childTurnNeedsApproval(childId)) return t('sessions.awaitingApproval')
  if (childTurnNeedsAsk(childId)) return t('sessions.awaitingAsk')
  return ''
}

function delegateCardLinkLabel(_seq: number): string {
  // Approvals/answers are acted on in Composer; drill-in is for context only.
  return t('sessions.viewExpertWork')
}

function drillIntoChildTurnBySeq(seq: number) {
  const childId = delegateChildTurnId(seq)
  if (childId) {
    currentTurnId.value = childId
  }
}

function drillIntoChildTurn(ev: StreamEvent) {
  const childId = childTurnIdFromDelegate(ev)
  if (childId) {
    currentTurnId.value = childId
  }
}

type ApprovalAnchor = {
  key: string
  seq: number
  turnId: string
  kind: 'permission' | 'ask'
  pending: boolean
  label: string
  topPercent: number
}

function approvalTool(payload: unknown) {
  const p = asRecord(payload)
  return String(p?.tool ?? p?.name ?? '')
}

/** Right-rail anchors for pending permission.ask / ask_user in the session stream. */
const approvalAnchors = computed((): ApprovalAnchor[] => {
  const events = sessions.streamEvents
  if (!events.length) return []
  const maxSeq = Math.max(1, events[events.length - 1]?.seq ?? 1)
  const out: ApprovalAnchor[] = []
  for (const e of events) {
    if (e.type === 'permission.ask') {
      const id = approvalId(e.payload)
      const pending = !!id && !sessions.decidedApprovalIds.has(id)
      if (!pending) continue
      const tool = approvalTool(e.payload)
      out.push({
        key: `perm-${id || e.seq}`,
        seq: e.seq,
        turnId: e.turnId || '',
        kind: 'permission',
        pending: true,
        label: tool ? `待审批 · ${tool}` : '待审批',
        topPercent: Math.min(92, Math.max(6, (e.seq / maxSeq) * 100)),
      })
    } else if (e.type === 'ask_user.pending') {
      if (!isAskActionable(e)) continue
      const q = askUserQuestion(e.payload)
      out.push({
        key: `ask-${askUserId(e.payload) || e.seq}`,
        seq: e.seq,
        turnId: e.turnId || '',
        kind: 'ask',
        pending: true,
        label: q ? `待回答 · ${q.slice(0, 36)}` : '待回答',
        topPercent: Math.min(92, Math.max(6, (e.seq / maxSeq) * 100)),
      })
    }
  }
  // Spread overlapping tops slightly so stacked asks stay clickable
  for (let i = 1; i < out.length; i++) {
    if (out[i].topPercent - out[i - 1].topPercent < 4) {
      out[i].topPercent = Math.min(94, out[i - 1].topPercent + 4)
    }
  }
  return out
})

async function jumpToApprovalAnchor(a: ApprovalAnchor) {
  if (a.turnId) {
    const turn = turnMap.value[a.turnId]
    if (turn?.parentTurnId) {
      currentTurnId.value = a.turnId
    } else if (currentTurnId.value && currentTurnId.value !== a.turnId) {
      currentTurnId.value = null
    }
    // Older turns default to collapsed — expand so the card is actually visible.
    ensureTurnExpanded(a.turnId)
  }
  userScrolledUp.value = true
  await nextTick()
  const root = scrollAreaRef.value
  let el = root?.querySelector(`[data-event-anchor="${a.seq}"]`) as HTMLElement | null
  // Card may land one frame later after turnMap / drill-in updates.
  if (!el) {
    await nextTick()
    el = scrollAreaRef.value?.querySelector(`[data-event-anchor="${a.seq}"]`) as HTMLElement | null
  }
  if (!el) {
    await new Promise<void>((r) => requestAnimationFrame(() => r()))
    el = scrollAreaRef.value?.querySelector(`[data-event-anchor="${a.seq}"]`) as HTMLElement | null
  }
  if (!el) return
  el.scrollIntoView({ behavior: 'smooth', block: 'center' })
  el.classList.add('is-anchor-flash')
  window.setTimeout(() => el.classList.remove('is-anchor-flash'), 1200)
}

function jumpToFirstPendingApproval() {
  const first = approvalAnchors.value.find((a) => a.pending)
  if (first) void jumpToApprovalAnchor(first)
}

const composerPermissionItems = computed(() =>
  sessions.pendingApprovals.map((e) => {
    const id = approvalId(e.payload)
    return {
      key: `perm-${id || e.seq}`,
      event: e,
      decided: isApprovalDecided(e.payload),
      deciding: isPermissionDeciding(e.payload),
      showActions: shouldShowApprovalActions(e.payload),
    }
  }),
)

const composerAskItems = computed(() =>
  sessions.pendingAsks
    .filter((e) => isAskActionable(e))
    .map((e) => ({
      key: `ask-${askUserId(e.payload) || e.seq}`,
      event: e,
      askId: askUserId(e.payload),
      question: askUserQuestion(e.payload),
      options: askUserOptions(e.payload),
      defaultOption: askUserDefaultOption(e.payload),
      formFields: askUserFormFields(e.payload),
      resolved: isAskResolved(askUserCallId(e.payload)),
      expired: isAskExpired(e),
      answering: answeringAskIds.value.has(askUserId(e.payload)),
      answer: askUserAnswer(e.payload),
    })),
)

/** When a new ask_user / permission card appears, keep composer-side cards in view (no timeline jump). */
watch(
  () => approvalAnchors.value.filter((a) => a.pending).map((a) => a.key).join('|'),
  async (keys, prev) => {
    if (!keys || keys === prev) return
    await nextTick()
    syncComposerLayout()
  },
)

const statusLabel = computed(() => {
  const s = sessions.currentSession?.status
  if (s === 'completed') return '已完成'
  if (s === 'failed') return '失败'
  if (s === 'active') return '运行中'
  if (s === 'archived') return '已归档'
  return s ?? ''
})

const statusType = computed(() => {
  const s = sessions.currentSession?.status
  if (s === 'completed') return 'success'
  if (s === 'failed') return 'danger'
  if (s === 'active') return 'info'
  if (s === 'archived') return 'default'
  return 'info'
})

function asRecord(v: unknown): Record<string, unknown> | null {
  if (v && typeof v === 'object' && !Array.isArray(v)) return v as Record<string, unknown>
  return null
}

function finalText(ev: StreamEvent): string {
  const p = asRecord(ev.payload)
  return String(p?.text ?? p?.summary ?? p?.content ?? '')
}

/** Skip events that would leave empty timeline rows (extra vertical gaps). */
function isTimelineEventVisible(ev: StreamEvent): boolean {
  switch (ev.type) {
    case '__tool_group__':
    case '__tool_card__':
    case 'agent.thinking':
    case 'permission.ask':
    case 'ask_user.pending':
    case 'context.compacted':
    case 'error':
    case 'capability.activated':
    case 'report':
      return true
    case 'agent.message':
      return Boolean(finalText(ev).trim())
    default:
      // user.message / delegate.* / llm.usage / turn.* are handled elsewhere or noise
      return false
  }
}

function timelineEvents(turn: StreamTurn): StreamEvent[] {
  return turn.events.filter(isTimelineEventVisible)
}

function toolName(ev: StreamEvent): string {
  const p = asRecord(ev.payload)
  return String(p?.tool ?? p?.name ?? ev.type)
}

function delegateLabel(ev: StreamEvent): string {
  const p = asRecord(ev.payload)
  const agentId = String(p?.agentId ?? p?.agent ?? '')
  const agent = agentId ? sessions.agents.find((a) => a.id === agentId) : undefined
  const name = agent?.name?.trim() || agentId
  const goal = String(p?.goal ?? '')
  const status = String(p?.status ?? '')
  if (ev.type === 'delegate.started') {
    if (!name) return t('sessions.delegateStartedFallback')
    return t('sessions.delegateStarted', { name, goal })
  }
  if (!name) return t('sessions.delegateCompletedFallback')
  const statusSuffix = status ? ` (${status})` : ''
  return t('sessions.delegateCompleted', { name, status: statusSuffix })
}

function delegateAgent(ev: StreamEvent): string {
  const p = asRecord(ev.payload)
  return String(p?.agentId ?? p?.agent ?? 'AI')
}

function delegateGoal(ev: StreamEvent): string {
  const p = asRecord(ev.payload)
  return String(p?.goal ?? '')
}

function compactionSummary(ev: StreamEvent): string {
  const p = asRecord(ev.payload)
  const turns = Number(p?.turnsCompacted ?? 0)
  const before = Number(p?.tokensBefore ?? 0)
  const after = Number(p?.tokensAfter ?? 0)
  const path = String(p?.filePath ?? '')
  return `压缩了 ${turns} 轮, tokens ${before} → ${after}, 文件: ${path}`
}

function usageText(ev: StreamEvent): string {
  const p = asRecord(ev.payload)
  const total = p?.totalTokens ?? p?.total_tokens ?? 0
  const prompt = p?.promptTokens ?? p?.prompt_tokens ?? 0
  const completion = p?.completionTokens ?? p?.completion_tokens ?? 0
  if (total) return String(total)
  if (prompt || completion) return `${prompt} + ${completion}`
  return '—'
}

function errorText(ev: StreamEvent): string {
  const p = asRecord(ev.payload)
  return String(p?.message ?? p?.error ?? '')
}

function toolStatus(ev: StreamEvent): string {
  if (ev.type === 'tool.running') return '执行中'
  if (ev.type === 'tool.completed') return '完成'
  if (ev.type === 'tool.error') return '错误'
  if (ev.type === 'tool.pending') return '待执行'
  return ''
}

function toolStatusType(ev: StreamEvent): 'info' | 'success' | 'danger' | 'warning' {
  if (ev.type === 'tool.running') return 'info'
  if (ev.type === 'tool.completed') return 'success'
  if (ev.type === 'tool.error') return 'danger'
  if (ev.type === 'tool.pending') return 'warning'
  return 'info'
}

function toolCardStatusLabel(status: string): string {
  if (status === 'running') return '执行中'
  if (status === 'completed') return '完成'
  if (status === 'error') return '错误'
  if (status === 'cancelled') return '已取消'
  return status
}

function toolCardStatusType(status: string): 'info' | 'success' | 'danger' | 'warning' {
  if (status === 'running') return 'info'
  if (status === 'completed') return 'success'
  if (status === 'error') return 'danger'
  if (status === 'cancelled') return 'warning'
  return 'info'
}

function toolPayload(ev: StreamEvent): Record<string, unknown> | null {
  return asRecord(ev.payload)
}

function toolInput(ev: StreamEvent): string {
  const p = toolPayload(ev)
  if (!p) return ''
  const input = p.input ?? p.arguments ?? p.args
  if (!input) return ''
  try {
    return JSON.stringify(input, null, 2)
  } catch {
    return String(input)
  }
}

function toolOutput(ev: StreamEvent): string {
  const p = toolPayload(ev)
  if (!p) return ''
  const out = p.output ?? p.result
  if (!out) return ''
  if (typeof out === 'string') return out
  try {
    return JSON.stringify(out, null, 2)
  } catch {
    return String(out)
  }
}

function toolError(ev: StreamEvent): string {
  const p = toolPayload(ev)
  if (!p) return ''
  return String(p.error ?? '')
}

function reportStatus(ev: StreamEvent): string {
  const p = asRecord(ev.payload)
  return String(p?.status ?? '')
}

function reportStatusType(ev: StreamEvent): string {
  const s = reportStatus(ev)
  if (s === 'done') return 'success'
  if (s === 'failed') return 'danger'
  if (s === 'blocked') return 'warning'
  return 'default'
}

function reportStatusLabel(ev: StreamEvent): string {
  const status = reportStatus(ev)
  if (status === 'done') return '已完成'
  if (status === 'failed') return '失败'
  if (status === 'blocked') return '已阻塞'
  return status || '未知'
}

function reportConfidence(ev: StreamEvent): number | null {
  const p = asRecord(ev.payload)
  const v = p?.confidence
  if (typeof v === 'number') return v
  return null
}

function reportSteps(ev: StreamEvent): number {
  const p = asRecord(ev.payload)
  const v = p?.stepsUsed
  if (typeof v === 'number') return v
  return 0
}

function reportSummary(ev: StreamEvent): string {
  const p = asRecord(ev.payload)
  return String(p?.summary ?? '')
}

/** Backend copies final assistant text into report.summary; skip re-rendering that body. */
function shouldShowReportSummary(turn: StreamTurn, ev: StreamEvent): boolean {
  const summary = reportSummary(ev).trim()
  if (!summary) return false
  return !turn.events.some(
    (e) => e.type === 'agent.message' && finalText(e).trim() === summary,
  )
}

function stepTitle(ev: StreamEvent): string {
  const p = asRecord(ev.payload)
  return String(p?.title ?? p?.step ?? 'Step')
}

function stepNumber(ev: StreamEvent): number {
  const p = asRecord(ev.payload)
  return Number(p?.step ?? 1)
}

function stepLabel(ev: StreamEvent, phase?: string): string {
  const p = asRecord(ev.payload)
  const title = String(p?.title ?? '')
  if (title) return title
  if (phase === 'failed') return '执行失败'
  if (phase === 'end') return '已完成'
  return '思考中…'
}

async function decide(ev: { payload: unknown }, approved: boolean, scope: 'once' | 'session' = 'once') {
  const p = asRecord(ev.payload)
  const id = String(p?.approvalId ?? p?.id ?? '')
  if (!id) {
    toast.error('审批 ID 缺失')
    return
  }
  decidingApprovalIds.value = new Set(decidingApprovalIds.value).add(id)
  try {
    await sessions.decideApproval(id, approved, scope)
    toast.success(approved ? (scope === 'session' ? '已允许本会话' : '已批准') : '已拒绝')
  } catch (e) {
    toast.error(e instanceof Error ? e.message : '审批失败')
  } finally {
    const next = new Set(decidingApprovalIds.value)
    next.delete(id)
    decidingApprovalIds.value = next
  }
}

function onPermissionDecide(
  ev: StreamEvent,
  payload: { decision: 'allow' | 'deny'; scope: 'once' | 'session' },
) {
  void decide(ev, payload.decision === 'allow', payload.scope)
}

function isPermissionDeciding(payload: unknown): boolean {
  const id = approvalId(payload)
  return id ? decidingApprovalIds.value.has(id) : false
}

function approvalId(payload: unknown): string {
  const p = asRecord(payload)
  return String(p?.approvalId ?? p?.id ?? '')
}

function isApprovalDecided(payload: unknown): boolean {
  const id = approvalId(payload)
  if (!id) return false
  return sessions.decidedApprovalIds.has(id)
}

/** Show approve/reject whenever this ask is still undecided in the live stream.
 *  Do not gate on session/turn status — after sendTurn those can stay stale
 *  (completed / no running turn) until poll/loadTurns, which hid the first ask's buttons. */
function shouldShowApprovalActions(payload: unknown): boolean {
  if (isApprovalDecided(payload)) return false
  const id = approvalId(payload)
  if (!id) return false
  return sessions.pendingApprovals.some((e) => approvalId(e.payload) === id)
}

const decidingApprovalIds = ref(new Set<string>())
/** Local answers before tool.completed arrives (or when stream never emits it). */
const answeredAskIds = ref(new Set<string>())
const resolvedAskAnswers = ref(new Map<string, string>())
const answeringAskIds = ref(new Set<string>())

watch(
  () => sessions.currentSession?.id,
  () => {
    answeredAskIds.value = new Set()
    resolvedAskAnswers.value = new Map()
    decidingApprovalIds.value = new Set()
    answeringAskIds.value = new Set()
  },
)

function askUserId(payload: unknown): string {
  const p = asRecord(payload)
  return String(p?.askId ?? p?.callId ?? '')
}

function askUserCallId(payload: unknown): string {
  const p = asRecord(payload)
  return String(p?.callId ?? p?.askId ?? '')
}

function askUserQuestion(payload: unknown): string {
  const p = asRecord(payload)
  return String(p?.question ?? '')
}

function askUserOptions(payload: unknown): string[] {
  const p = asRecord(payload)
  if (Array.isArray(p?.options)) return (p.options as unknown[]).map(String)
  return []
}

function askUserDefaultOption(payload: unknown): string {
  const p = asRecord(payload)
  return String(p?.defaultOption ?? '')
}

function askUserFormFields(payload: unknown): AskUserFormField[] {
  const p = asRecord(payload)
  if (Array.isArray(p?.formFields)) {
    return (p.formFields as unknown[]).map((item) => {
      const f = asRecord(item) ?? {}
      return {
        name: String(f.name ?? ''),
        label: String(f.label ?? ''),
        type: (String(f.type ?? 'text') as AskUserFormField['type']),
        required: Boolean(f.required),
        default: f.default,
        options: Array.isArray(f.options) ? (f.options as unknown[]).map(String) : undefined,
        placeholder: f.placeholder ? String(f.placeholder) : undefined,
      }
    }).filter((f) => f.name && f.label)
  }
  return []
}

async function onAskUserResolve(ev: StreamEvent, answer: string) {
  const trimmed = answer.trim()
  if (!trimmed) return
  const askId = askUserId(ev.payload)
  if (!askId) {
    toast.warning(t('sessions.askMissingId'))
    return
  }
  if (answeringAskIds.value.has(askId) || !isAskActionable(ev)) return
  await submitAskAnswer(askId, trimmed)
}

async function submitAskAnswer(askId: string, answer: string) {
  answeringAskIds.value = new Set(answeringAskIds.value).add(askId)
  try {
    await sessions.resolveAskUser(askId, answer)
    answeredAskIds.value = new Set(answeredAskIds.value).add(askId)
    resolvedAskAnswers.value = new Map(resolvedAskAnswers.value).set(askId, answer)
  } catch (e) {
    toast.error(e instanceof Error ? e.message : t('sessions.askResolveFailed'))
    // Stale ask after cancel/reload — stop showing interactive controls.
    if (String(e instanceof Error ? e.message : e).includes('not found') ||
        String(e instanceof Error ? e.message : e).includes('no longer waiting')) {
      answeredAskIds.value = new Set(answeredAskIds.value).add(askId)
    }
  } finally {
    const next = new Set(answeringAskIds.value)
    next.delete(askId)
    answeringAskIds.value = next
  }
}

function isAskResolved(callId: string): boolean {
  if (!callId) return false
  if (answeredAskIds.value.has(callId)) return true
  if (sessions.resolvedAskCallIds.has(callId)) return true
  return false
}

/** Turn already finished without completing this ask_user — buttons would no-op. */
function isAskExpired(ev: { seq: number; turnId?: string; payload: unknown }): boolean {
  const callId = askUserCallId(ev.payload)
  if (!callId || isAskResolved(callId)) return false
  const turnId = ev.turnId || ''
  for (const e of sessions.streamEvents) {
    if (e.seq <= ev.seq) continue
    if (turnId && e.turnId && e.turnId !== turnId) continue
    const p = asRecord(e.payload)
    if ((e.type === 'tool.completed' || e.type === 'tool.error') && String(p?.callId ?? '') === callId) {
      return e.type === 'tool.error'
    }
    // ResumeTurn re-publishes turn.started; prior ask_user waiters are gone.
    if (e.type === 'turn.started' || e.type === 'turn.failed' || e.type === 'turn.ended') return true
  }
  return false
}

function isAskActionable(ev: { seq: number; turnId?: string; payload: unknown }): boolean {
  const callId = askUserCallId(ev.payload)
  if (!callId || isAskResolved(callId)) return false
  if (isAskExpired(ev)) return false
  return true
}

function askUserAnswer(payload: unknown): string {
  const p = asRecord(payload)
  const cid = String(p?.askId ?? p?.callId ?? '')
  if (!cid) return ''
  for (const ev of sessions.streamEvents) {
    if (ev.type !== 'tool.completed') continue
    const tp = ev.payload as Record<string, unknown> | null
    if (tp?.name === 'ask_user' && tp?.callId === cid) {
      return String(tp?.output ?? '')
    }
  }
  return resolvedAskAnswers.value.get(cid) ?? ''
}

async function downloadTurnLog(turnId: string) {
  if (!sessions.currentSessionId) return
  try {
    const base = apiBaseUrl()
    const url = `${base}/api/v1/sessions/${sessions.currentSessionId}/turns/${turnId}/log`
    const res = await fetch(url)
    if (!res.ok) {
      const err = await res.json().catch(() => ({ error: res.statusText }))
      throw new Error((err as { error?: string }).error || `download failed (${res.status})`)
    }
    const blob = await res.blob()
    const result = await saveBlobAs(blob, `${turnId}.zip`, {
      filters: [{ name: 'Zip', extensions: ['zip'] }],
    })
    if (!result.ok) return // user cancelled save dialog
    if (result.method === 'download') {
      toast.success('已保存到浏览器默认下载目录')
    } else if (result.path) {
      toast.success(`已保存：${result.path}`)
    } else {
      toast.success('Turn Log 已保存')
    }
  } catch (e) {
    console.error('download turn log failed', e)
    toast.error(e instanceof Error ? e.message : '下载失败')
  }
}

function startEditTitle() {
  isEditingTitle.value = true
  editingTitle.value = sessions.currentSession?.title ?? sessions.currentSession?.content ?? ''
}

async function saveTitle() {
  if (!sessions.currentSession) return
  const title = editingTitle.value.trim()
  if (!title) {
    isEditingTitle.value = false
    return
  }
  try {
    await sessions.updateSession(sessions.currentSession.id, { title })
    toast.success('已保存')
  } catch (e) {
    toast.error(e instanceof Error ? e.message : '保存失败')
  }
  isEditingTitle.value = false
}

async function archiveSession() {
  if (!sessions.currentSession) return
  try {
    await sessions.updateSession(sessions.currentSession.id, { status: 'archived' })
    toast.success('已归档')
  } catch (e) {
    toast.error(e instanceof Error ? e.message : '归档失败')
  }
}

async function removeSession() {
  if (!sessions.currentSession) return
  try {
    await sessions.deleteSession(sessions.currentSession.id)
    toast.success('已删除')
  } catch (e) {
    toast.error(e instanceof Error ? e.message : '删除失败')
  }
}

async function cancelRunning() {
  if (!sessions.runningTurnId) return
  try {
    await sessions.cancelTurn(sessions.runningTurnId)
    toast.success('已取消')
  } catch (e) {
    toast.error(e instanceof Error ? e.message : '取消失败')
  }
}

async function copySessionId() {
  if (!sessions.currentSession) return
  try {
    await navigator.clipboard.writeText(sessions.currentSession.id)
    toast.success(t('sessions.copySessionIdDone'))
  } catch {
    toast.error(t('sessions.copySessionIdFailed'))
  }
}

async function resumeTurn(turnId: string) {
  try {
    await sessions.resumeTurn(turnId)
    toast.success('已恢复')
  } catch (e) {
    toast.error(e instanceof Error ? e.message : '恢复失败')
  }
}

function formatTurnGoal(goal: string) {
  return goal.trim().slice(0, 60) || '未命名 Turn'
}

// ── Turn summary computation ──
function turnSummary(turn: Turn): { toolCount: number; completedTools: number; errorTools: number; runningTools: number; tokensUsed: number } {
  let toolCount = 0
  let completedTools = 0
  let errorTools = 0
  let runningTools = 0
  for (const ev of turn.events) {
    forEachToolCard(ev, (card) => {
      toolCount++
      if (card.status === 'completed') completedTools++
      else if (card.status === 'error') errorTools++
      else if (card.status === 'running' || card.status === 'pending') runningTools++
    })
  }
  const tokensUsed = tokensForTurn(turn.id)
  return { toolCount, completedTools, errorTools, runningTools, tokensUsed }
}

function onWelcomePrompt(text: string) {
  composerRef.value?.appendContent?.(text)
  composerRef.value?.focusInput?.()
}

const WRITE_TOOL_NAMES = new Set(['write_file', 'edit_file', 'apply_patch', 'str_replace', 'create_file', 'delete_file', 'bash', 'shell', 'run_terminal'])

async function refreshChangesCount() {
  if (!sessions.selectedProjectId) {
    workspaceUi.changesCount = 0
    return
  }
  try {
    const data = await fetchJSON<{ changes?: { file: string }[] }>(`/projects/${sessions.selectedProjectId}/git-changes`)
    workspaceUi.changesCount = data?.changes?.length ?? 0
  } catch {
    /* ignore */
  }
}

watch(
  () => sessions.streamEvents.length,
  () => {
    const last = sessions.streamEvents[sessions.streamEvents.length - 1]
    if (!last) return
    if (last.type === 'tool.completed' || last.type === 'tool.running') {
      const p = asRecord(last.payload)
      const name = String(p?.name ?? '')
      if (WRITE_TOOL_NAMES.has(name) || name.includes('write') || name.includes('edit') || name.includes('patch')) {
        void refreshChangesCount().then(() => {
          if (workspaceUi.changesCount > 0 && rightTab.value !== 'changes') {
            // keep badge; optional soft nudge only when turn ends
          }
        })
      }
    }
    if (last.type === 'turn.ended' || last.type === 'report') {
      void refreshChangesCount().then(() => {
        if (
          workspaceUi.changesCount > 0 &&
          rightDrawerOpen.value &&
          rightTab.value === 'plan'
        ) {
          workspaceUi.setRightTab('changes')
        }
      })
    }
  },
)

watch(
  () => sessions.selectedProjectId,
  () => { void refreshChangesCount() },
  { immediate: true },
)

watch(
  () => approvalAnchors.value.filter((a) => a.pending).length,
  (n) => { workspaceUi.pendingApprovals = n },
  { immediate: true },
)

watch(
  () => ({
    sessionId: sessions.currentSessionId,
    askPending: approvalAnchors.value.some((a) => a.pending && a.kind === 'ask'),
  }),
  ({ sessionId, askPending }) => {
    if (sessionId && askPending) sessionActivity.setLocalAsk(sessionId)
    else sessionActivity.setLocalAsk(null)
    void sessionActivity.refresh()
  },
  { immediate: true },
)

function onTitleKeydown(e: KeyboardEvent) {
  if (e.key === 'Enter') {
    e.preventDefault()
    void saveTitle()
  }
  if (e.key === 'Escape') {
    isEditingTitle.value = false
  }
}
</script>

<template>
  <div class="session-workspace">
    <header v-if="sessions.currentSession" class="session-workspace__head">
      <div class="session-workspace__identity">
        <template v-if="isEditingTitle">
          <DqInput
            v-model="editingTitle"
            class="session-workspace__title-input"
            @blur="saveTitle"
            @keydown="onTitleKeydown"
          />
        </template>
        <template v-else>
          <h2 class="session-workspace__title" @click="startEditTitle">
            {{ sessions.currentSession.title || sessions.currentSession.content }}
          </h2>
        </template>
        <div class="session-workspace__status-id">
          <DqTag
            :type="statusType"
            :effect="sessions.currentSession?.status === 'active' ? 'dark' : 'light'"
          >{{ statusLabel }}</DqTag>
          <button
            type="button"
            class="session-workspace__copy-btn"
            :title="t('sessions.copySessionId')"
            :aria-label="t('sessions.copySessionId')"
            @click="copySessionId"
          >
            <svg viewBox="0 0 24 24" width="16" height="16" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true">
              <path d="M10 13a5 5 0 0 0 7.54.54l3-3a5 5 0 0 0-7.07-7.07l-1.72 1.71" />
              <path d="M14 11a5 5 0 0 0-7.54-.54l-3 3a5 5 0 0 0 7.07 7.07l1.71-1.71" />
            </svg>
          </button>
        </div>
        <ActiveSessionsBar
          class="session-workspace__active"
          @select="(id) => { sessions.selectSession(id); router.push({ name: 'sessions', params: { id } }) }"
          @jump-pending="jumpToFirstPendingApproval"
        />
      </div>
      <div class="session-workspace__actions">
        <DqButton v-if="sessions.runningTurnId" type="warning" size="sm" @click="cancelRunning">
          {{ t('sessions.cancelRunning') }}
        </DqButton>
        <div class="session-workspace__tools" role="toolbar" :aria-label="t('sessions.rightWorkspace')">
          <button
            v-for="item in rightIconItems"
            :key="item.value"
            type="button"
            class="session-workspace__tool"
            :class="{
              'is-active': rightDrawerOpen && rightTab === item.value,
              'has-badge': item.badge != null && item.badge !== '',
            }"
            :aria-label="item.badge != null && item.badge !== '' ? `${item.label} ${item.badge}` : item.label"
            :title="item.badge != null && item.badge !== '' ? `${item.label} · ${item.badge}` : item.label"
            :aria-pressed="rightDrawerOpen && rightTab === item.value"
            @click="onRightIconClick(item.value)"
          >
            <component :is="item.icon" class="session-workspace__tool-icon" :size="16" :stroke-width="2" aria-hidden="true" />
            <span
              v-if="item.badge != null && item.badge !== ''"
              class="session-workspace__tool-badge"
            >{{ item.badge }}</span>
          </button>
        </div>
      </div>
    </header>

    <div
      ref="bodyRef"
      class="session-workspace__body"
      :class="{
        'session-workspace__body--stage': isStageLayout,
        'session-workspace__body--immersive': layoutMode === 'immersive' && !!stage,
      }"
      :style="bodyGridStyle"
    >
      <div
        v-show="layoutMode !== 'immersive'"
        class="session-workspace__stream"
      >
      <div
        ref="scrollAreaRef"
        class="session-workspace__scroll"
        :class="{ 'has-approval-rail': approvalAnchors.length > 0 }"
        :style="{ paddingBottom: `${composerOverlayPx + 28}px` }"
        @scroll="onScrollAreaScroll"
      >
        <div v-if="sessions.composingNew && !sessions.currentSession" class="session-workspace__empty">
          <WelcomeEmpty @pick-prompt="onWelcomePrompt" />
        </div>

        <div v-else-if="!visibleTurns.length" class="session-workspace__empty">
          <DqEmpty :description="t('sessions.waitingFirstMessage')">
            <p class="session-workspace__hint">{{ t('sessions.waitingFirstHint') }}</p>
          </DqEmpty>
        </div>

        <div v-else class="session-workspace__turns">
          <nav v-if="breadcrumbs.length > 1" class="turn-breadcrumbs" aria-label="Turn 导航">
            <ol class="turn-breadcrumbs__list">
              <li
                v-for="(crumb, index) in breadcrumbs"
                :key="crumb.id ?? 'root'"
                class="turn-breadcrumbs__item"
              >
                <button
                  class="turn-breadcrumbs__link"
                  :class="{ 'turn-breadcrumbs__link--active': index === breadcrumbs.length - 1 }"
                  @click="navigateToTurn(crumb.id)"
                >
                  {{ crumb.label }}
                </button>
                <span v-if="index < breadcrumbs.length - 1" class="turn-breadcrumbs__sep">/</span>
              </li>
            </ol>
          </nav>

          <TurnSection
            v-for="(turn, turnIndex) in visibleTurns"
            :key="turn.id"
            :turn="turn"
            :turn-index="turnIndex"
            :collapsed="isTurnCollapsed(turn.id)"
            :summary="turnSummary(turn)"
            :show-divider="turnIndex > 0"
            @toggle-collapse="toggleTurnCollapse(turn.id)"
            @download="downloadTurnLog(turn.id)"
          >
            <template #timeline>
              <div v-for="ev in timelineEvents(turn)" :key="ev.seq" class="turn__event">
                <template v-if="ev.type === '__tool_group__'">
                  <ToolCardGroup
                    :cards="toolGroupCards(ev)"
                    :expanded="isToolGroupExpanded(ev.seq, toolGroupCards(ev))"
                    :is-card-expanded="isToolCardExpanded"
                    :card-awaiting-approval="(seq) => groupCardAwaitingApproval(toolGroupCards(ev), seq)"
                    :card-awaiting-label="delegateCardAwaitingLabel"
                    :card-show-child-link="(seq) => groupCardShowChildLink(toolGroupCards(ev), seq)"
                    :card-child-link-label="delegateCardLinkLabel"
                    @toggle="toggleToolGroup(ev.seq, toolGroupCards(ev))"
                    @toggle-card="toggleToolCard"
                    @open-child="drillIntoChildTurnBySeq"
                  />
                </template>

                <template v-else-if="ev.type === '__tool_card__'">
                  <ToolCardBlock
                    :card="ev.payload as ToolCard"
                    :expanded="isToolCardExpanded(ev.seq)"
                    :awaiting-approval="(ev.payload as ToolCard).name === 'delegate_agent' && delegateCardAwaiting(ev.seq)"
                    :awaiting-label="delegateCardAwaitingLabel(ev.seq)"
                    :show-child-link="(ev.payload as ToolCard).name === 'delegate_agent' && !!delegateChildTurnId(ev.seq)"
                    :child-link-label="delegateCardLinkLabel(ev.seq)"
                    @toggle="toggleToolCard(ev.seq)"
                    @open-child="drillIntoChildTurnBySeq(ev.seq)"
                  />
                </template>

                <template v-else-if="ev.type === 'agent.thinking'">
                  <ThinkingBlock
                    :text="finalText(ev)"
                    :expanded="isThinkingExpanded(ev.seq)"
                    :seq="ev.seq"
                    @toggle="toggleThinking"
                  />
                </template>

                <template v-else-if="ev.type === 'agent.message' && finalText(ev).trim()">
                  <AgentMessageBlock :html="renderMarkdown(finalText(ev))" />
                </template>

                <template v-else-if="ev.type === 'report'">
                  <div class="turn__report-meta">
                    <DqTag :type="reportStatusType(ev)">{{ reportStatusLabel(ev) }}</DqTag>
                    <span v-if="reportConfidence(ev) !== null" class="turn__report-meta-confidence">置信度 {{ reportConfidence(ev) }}</span>
                    <span v-if="reportSteps(ev)" class="turn__report-meta-steps">{{ reportSteps(ev) }} 步</span>
                    <div
                      v-if="shouldShowReportSummary(turn, ev)"
                      class="turn__report-meta-summary"
                      v-html="renderMarkdown(reportSummary(ev))"
                    />
                  </div>
                </template>

                <template v-else-if="ev.type === 'capability.activated'">
                  <div class="turn__skill">
                    <svg viewBox="0 0 24 24" width="14" height="14" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><polygon points="12 2 15.09 8.26 22 9.27 17 14.14 18.18 21.02 12 17.77 5.82 21.02 7 14.14 2 9.27 8.91 8.26 12 2"/></svg>
                    <span>{{ toolName(ev) }}</span>
                  </div>
                </template>

                <template v-else-if="ev.type === 'permission.ask'">
                  <PermissionAskBlock
                    :payload="ev.payload"
                    :decided="isApprovalDecided(ev.payload)"
                    :deciding="isPermissionDeciding(ev.payload)"
                    :show-actions="false"
                    :anchor-seq="ev.seq"
                  />
                </template>

                <template v-else-if="ev.type === 'ask_user.pending'">
                  <AskUserBlock
                    :payload="ev.payload"
                    :anchor-seq="ev.seq"
                    :ask-id="askUserId(ev.payload)"
                    :question="askUserQuestion(ev.payload)"
                    :options="askUserOptions(ev.payload)"
                    :default-option="askUserDefaultOption(ev.payload)"
                    :form-fields="askUserFormFields(ev.payload)"
                    :resolved="isAskResolved(askUserCallId(ev.payload))"
                    :expired="isAskExpired(ev)"
                    :answering="answeringAskIds.has(askUserId(ev.payload))"
                    :answer="askUserAnswer(ev.payload)"
                    :interactive="false"
                  />
                </template>

                <template v-else-if="ev.type === 'context.compacted'">
                  <div class="turn__compaction">
                    <svg class="turn__compaction-svg" viewBox="0 0 24 24" width="16" height="16" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><polyline points="17 1 21 5 17 9"/><path d="M3 11V9a4 4 0 0 1 4-4h14"/><polyline points="7 23 3 19 7 15"/><path d="M21 13v2a4 4 0 0 1-4 4H3"/></svg>
                    <div class="turn__compaction-body">
                      <div class="turn__compaction-title">上下文压缩</div>
                      <div class="turn__compaction-detail">{{ compactionSummary(ev) }}</div>
                    </div>
                  </div>
                </template>

                <template v-else-if="ev.type === 'error'">
                  <div class="turn__error">
                    <svg class="turn__error-svg" viewBox="0 0 24 24" width="16" height="16" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><circle cx="12" cy="12" r="10"/><line x1="12" y1="8" x2="12" y2="12"/><line x1="12" y1="16" x2="12.01" y2="16"/></svg>
                    <span class="turn__error-text">{{ errorText(ev) }}</span>
                  </div>
                </template>

                <template v-else-if="ev.type === 'llm.usage'">
                  <!-- hidden: tokens shown in turn summary -->
                </template>
              </div>
            </template>
          </TurnSection>
        </div>
      </div>

      <ApprovalRail :anchors="approvalAnchors" @jump="jumpToApprovalAnchor" />
      
      </div>

      <DocumentStage
        v-if="stage && sessions.selectedProjectId"
        class="session-workspace__stage"
        :project-id="sessions.selectedProjectId"
        @attach-element="onStageAttachElement"
        @attach-code-selection="onStageAttachCodeSelection"
        @attach-office-edit="onStageAttachOfficeEdit"
      />
    </div>

    <DqDrawer
      :open="rightDrawerOpen"
      class="session-workspace__drawer"
      direction="rtl"
      size="min(380px, 92vw)"
      :title="rightDrawerTitle"
      @update:open="workspaceUi.setRightDrawerOpen"
    >
      <RightWorkspacePanel
        ref="rightPanelRef"
        v-model:tab="rightTab"
        :stream-events="sessions.streamEvents"
        :plan-turn-id="planTurnId"
        :project-id="sessions.selectedProjectId"
        :changes-count="workspaceUi.changesCount"
        :agent-id="sessions.selectedAgentId"
        @open-in-office="openFileInOffice"
      />
    </DqDrawer>

    <div ref="composerWrapRef" class="session-workspace__composer" :style="composerStyle">
      <ComposerPendingDecisions
        :permissions="composerPermissionItems"
        :asks="composerAskItems"
        @decide="onPermissionDecide"
        @resolve="onAskUserResolve"
      />
      <ComposerPendingQueue />
      <FloatingComposer ref="composerRef" @jump-pending="jumpToFirstPendingApproval" />
    </div>
  </div>
</template>

<style scoped>
.session-workspace {
  position: relative;
  display: flex;
  flex-direction: column;
  height: 100%;
  min-width: 0;
  min-height: 0;
  overflow: hidden;
  background: transparent;
  border-radius: inherit;
}

.session-workspace__head {
  flex-shrink: 0;
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  padding: 10px 20px;
  border-bottom: 1px solid var(--dq-shell-divider);
  background: transparent;
}

.session-workspace__identity {
  display: flex;
  align-items: center;
  gap: 10px;
  min-width: 0;
  flex: 1;
  flex-wrap: wrap;
}

.session-workspace__active {
  flex-shrink: 0;
}

.session-workspace__title {
  flex: 0 1 auto;
  min-width: 0;
  max-width: min(100%, 42ch);
  margin: 0;
  font-size: var(--dq-font-size-title);
  font-weight: 600;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
  color: var(--dq-label-primary);
  cursor: pointer;
}

.session-workspace__title-input {
  flex: 0 1 auto;
  min-width: 160px;
  max-width: min(100%, 42ch);
}

.session-workspace__title:hover {
  color: var(--dq-accent);
}

.session-workspace__title-input :deep(.dq-input) {
  height: 28px;
  padding: 0 8px;
  font-size: var(--dq-font-size-secondary);
}

.session-workspace__actions {
  display: flex;
  align-items: center;
  gap: 8px;
  flex-shrink: 0;
}

.session-workspace__tools {
  display: flex;
  align-items: center;
  gap: 2px;
  padding: 2px;
  border-radius: 10px;
  background: color-mix(in srgb, var(--dq-label-primary) 4%, transparent);
  border: 1px solid color-mix(in srgb, var(--dq-label-primary) 8%, transparent);
}

.session-workspace__tool {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  gap: 4px;
  width: 30px;
  height: 28px;
  padding: 0;
  border: none;
  border-radius: 8px;
  background: transparent;
  color: var(--dq-label-secondary);
  cursor: pointer;
  transition: background 0.15s ease, color 0.15s ease;
}

.session-workspace__tool.has-badge {
  width: auto;
  min-width: 30px;
  padding: 0 7px 0 6px;
}

.session-workspace__tool-icon {
  flex-shrink: 0;
}

.session-workspace__tool:hover {
  color: var(--dq-label-primary);
  background: var(--dq-fill-on-glass);
}

.session-workspace__tool.is-active {
  color: var(--dq-accent);
  background: color-mix(in srgb, var(--dq-accent) 14%, transparent);
}

.session-workspace__tool-badge {
  flex-shrink: 0;
  font-size: 11px;
  font-weight: 600;
  font-variant-numeric: tabular-nums;
  line-height: 1;
  letter-spacing: -0.02em;
  color: inherit;
  opacity: 0.85;
}

.session-workspace__tool.is-active .session-workspace__tool-badge {
  opacity: 1;
}

.session-workspace__status-id {
  display: flex;
  align-items: center;
  gap: 6px;
  flex-shrink: 0;
}

.session-workspace__copy-btn {
  display: flex;
  align-items: center;
  justify-content: center;
  width: 28px;
  height: 28px;
  padding: 0;
  border: none;
  border-radius: var(--dq-radius-button);
  background: transparent;
  color: var(--dq-label-secondary);
  cursor: pointer;
  transition: background 0.2s, color 0.2s;
}

.session-workspace__copy-btn:hover {
  background: var(--dq-fill-on-glass);
  color: var(--dq-accent);
}

.session-workspace__body {
  flex: 1;
  min-height: 0;
  display: grid;
  overflow: hidden;
}

.session-workspace__body--stage {
  grid-template-columns: minmax(200px, 32%) minmax(0, 1fr);
}

/* Drawer layout lives in work.css — panel is teleported outside this scope. */

.session-workspace__body--immersive {
  grid-template-columns: 1fr;
}

.session-workspace__stage {
  min-width: 0;
  min-height: 0;
  overflow: hidden;
}

.session-workspace__stream {
  position: relative;
  min-width: 0;
  min-height: 0;
  display: flex;
  overflow: hidden;
}

.session-workspace__scroll {
  flex: 1;
  min-width: 0;
  min-height: 0;
  overflow: auto;
  padding: var(--dq-chat-gutter, 24px) var(--dq-chat-gutter, 24px) 120px;
  display: flex;
  flex-direction: column;
  align-items: center;
}


.session-workspace__scroll.has-approval-rail {
  padding-right: 64px;
}

/* Right-side anchors for pending approval / ask_user events */
.approval-rail {
  position: absolute;
  top: 16px;
  right: 6px;
  bottom: 140px;
  width: 28px;
  z-index: 4;
  pointer-events: none;
}

.approval-rail__track {
  position: absolute;
  top: 0;
  bottom: 0;
  left: 50%;
  width: 2px;
  transform: translateX(-50%);
  border-radius: 1px;
  background: color-mix(in srgb, var(--dq-label-primary) 10%, transparent);
}

.approval-rail__anchor {
  position: absolute;
  left: 50%;
  transform: translate(-50%, -50%);
  display: flex;
  align-items: center;
  gap: 4px;
  padding: 0;
  border: none;
  background: transparent;
  cursor: pointer;
  pointer-events: auto;
  color: var(--dq-warning, #d97706);
}

.approval-rail__dot {
  width: 10px;
  height: 10px;
  border-radius: 50%;
  background: currentColor;
  box-shadow: 0 0 0 3px color-mix(in srgb, currentColor 18%, transparent);
  flex-shrink: 0;
}

.approval-rail__anchor.is-pending .approval-rail__dot {
  animation: approval-rail-pulse 1.6s ease-in-out infinite;
}

.approval-rail__tip {
  position: absolute;
  right: 16px;
  top: 50%;
  transform: translateY(-50%);
  padding: 2px 6px;
  border-radius: 4px;
  font-size: var(--dq-font-size-caption);
  font-weight: 600;
  line-height: 1.3;
  white-space: nowrap;
  color: var(--dq-warning, #d97706);
  background: color-mix(in srgb, var(--dq-bg-base, #fff) 88%, var(--dq-warning, #d97706));
  border: 1px solid color-mix(in srgb, var(--dq-warning, #d97706) 35%, transparent);
  opacity: 0.95;
  pointer-events: none;
}

.approval-rail__anchor.is-ask {
  color: var(--dq-accent);
}

.approval-rail__anchor.is-ask .approval-rail__tip {
  color: var(--dq-accent);
  background: color-mix(in srgb, var(--dq-bg-base, #fff) 88%, var(--dq-accent));
  border-color: color-mix(in srgb, var(--dq-accent) 30%, transparent);
}

.approval-rail__anchor:hover .approval-rail__dot {
  transform: scale(1.15);
}

@keyframes approval-rail-pulse {
  0%, 100% { box-shadow: 0 0 0 3px color-mix(in srgb, currentColor 18%, transparent); }
  50% { box-shadow: 0 0 0 6px color-mix(in srgb, currentColor 10%, transparent); }
}

.turn__permission.is-anchor-flash,
.turn__ask-user.is-anchor-flash,
.permission-ask.is-anchor-flash,
.ask-user-block.is-anchor-flash {
  outline: 2px solid color-mix(in srgb, var(--dq-warning, #d97706) 70%, transparent);
  outline-offset: 2px;
  border-radius: 8px;
  transition: outline-color 0.3s ease;
}

.session-workspace__empty {
  height: 100%;
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  color: var(--dq-label-tertiary);
}

.session-workspace__hint {
  margin: 8px 0 0;
  font-size: var(--dq-font-size-body);
  color: var(--dq-label-tertiary);
}

.session-workspace__turns {
  display: flex;
  flex-direction: column;
  gap: var(--dq-chat-turn-gap, 12px);
  width: 100%;
  max-width: var(--dq-chat-column-max, 920px);
}

.turn-breadcrumbs {
  flex-shrink: 0;
  padding: 8px 12px;
  border-radius: 10px;
  background: color-mix(in srgb, var(--dq-label-primary) 4%, transparent);
}

.turn-breadcrumbs__list {
  list-style: none;
  margin: 0;
  padding: 0;
  display: flex;
  align-items: center;
  gap: 8px;
  flex-wrap: wrap;
}

.turn-breadcrumbs__item {
  display: flex;
  align-items: center;
  gap: 8px;
}

.turn-breadcrumbs__link {
  font-size: var(--dq-font-size-footnote);
  color: var(--dq-label-secondary);
  background: none;
  border: none;
  padding: 0;
  cursor: pointer;
  max-width: 160px;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.turn-breadcrumbs__link:hover {
  color: var(--dq-accent);
  text-decoration: underline;
}

.turn-breadcrumbs__link--active {
  color: var(--dq-label-primary);
  font-weight: 600;
  cursor: default;
}

.turn-breadcrumbs__link--active:hover {
  text-decoration: none;
}

.turn-breadcrumbs__sep {
  color: var(--dq-label-tertiary);
  font-size: var(--dq-font-size-caption);
}

.turn__skill {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  padding: 6px 12px;
  border-radius: 8px;
  background: color-mix(in srgb, var(--dq-accent) 8%, transparent);
  border: 1px solid color-mix(in srgb, var(--dq-accent) 16%, transparent);
  color: var(--dq-accent);
  font-size: var(--dq-font-size-footnote);
  font-weight: 500;
  width: fit-content;
}

.turn__skill svg {
  opacity: 0.7;
}

.turn__report-meta {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 8px 12px;
  padding: 8px 0;
  font-size: var(--dq-font-size-footnote);
  color: var(--dq-label-tertiary);
}


.turn__report-meta-confidence,
.turn__report-meta-steps {
  color: var(--dq-label-tertiary);
}

.turn__report-meta-summary {
  flex-basis: 100%;
  margin-top: 8px;
  font-size: var(--dq-font-size-body);
  line-height: 1.5;
  color: var(--dq-label-secondary);
}

.turn__report-meta-summary :deep(p) {
  margin: 0 0 6px;
}
.turn__report-meta-summary :deep(p:last-child) {
  margin-bottom: 0;
}
.turn__report-meta-summary :deep(table) {
  width: 100%;
  border-collapse: collapse;
  margin: 8px 0;
  font-size: inherit;
  border: 1px solid color-mix(in srgb, var(--dq-label-primary) 12%, transparent);
  border-radius: 8px;
  overflow: hidden;
}
.turn__report-meta-summary :deep(th),
.turn__report-meta-summary :deep(td) {
  border: 1px solid color-mix(in srgb, var(--dq-label-primary) 10%, transparent);
  padding: 6px 10px;
  text-align: left;
}
.turn__report-meta-summary :deep(th) {
  background: color-mix(in srgb, var(--dq-label-primary) 8%, transparent);
  font-weight: 600;
}
.turn__report-meta-summary :deep(ul),
.turn__report-meta-summary :deep(ol) {
  margin: 4px 0;
  padding-left: 1.5em;
}
.turn__report-meta-summary :deep(li) {
  margin: 2px 0;
  line-height: 1.5;
}
.turn__report-meta-summary :deep(code) {
  font-family: var(--dq-font-mono);
  font-size: var(--dq-font-size-caption);
  padding: 2px 5px;
  border-radius: 4px;
  background: color-mix(in srgb, var(--dq-label-primary) 8%, transparent);
}
.turn__report-meta-summary :deep(strong) {
  font-weight: 600;
  color: var(--dq-label-primary);
}
.turn__report-meta-summary :deep(h1),
.turn__report-meta-summary :deep(h2),
.turn__report-meta-summary :deep(h3),
.turn__report-meta-summary :deep(h4) {
  margin: 14px 0 6px;
  color: var(--dq-label-primary);
  line-height: 1.35;
  font-size: inherit;
}
.turn__report-meta-summary :deep(h1) { font-weight: 650; }
.turn__report-meta-summary :deep(h2),
.turn__report-meta-summary :deep(h3),
.turn__report-meta-summary :deep(h4) { font-weight: 600; }
.turn__report-meta-summary :deep(h1:first-child),
.turn__report-meta-summary :deep(h2:first-child),
.turn__report-meta-summary :deep(h3:first-child) {
  margin-top: 0;
}

.turn__step {
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 6px 0;
  font-size: var(--dq-font-size-body);
}

.turn__step-badge {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 22px;
  height: 22px;
  border-radius: 50%;
  font-size: var(--dq-font-size-caption);
  font-weight: 600;
  flex-shrink: 0;
  border: 1.5px solid var(--dq-accent);
  color: var(--dq-accent);
  background: transparent;
  transition: background 0.2s ease, color 0.2s ease, border-color 0.2s ease;
}

.turn__step-label {
  display: flex;
  align-items: center;
  gap: 6px;
  min-width: 0;
}

.turn__step-status-text {
  font-size: var(--dq-font-size-footnote);
  color: var(--dq-label-secondary);
  white-space: nowrap;
}

.turn__step[data-step-phase='start'] .turn__step-badge {
  animation: step-pulse 1.8s ease-in-out infinite;
}

.turn__step[data-step-phase='start'] .turn__step-status-text {
  color: var(--dq-accent);
}

.turn__step[data-step-phase='end'] .turn__step-badge {
  background: var(--dq-accent);
  color: var(--dq-on-accent);
  border-color: var(--dq-accent);
}

.turn__step[data-step-phase='end'] .turn__step-status-text {
  color: var(--dq-label-tertiary);
}

.turn__step[data-step-phase='failed'] .turn__step-badge {
  background: color-mix(in srgb, var(--dq-danger) 12%, transparent);
  border-color: var(--dq-danger);
  color: var(--dq-danger);
}

.turn__step[data-step-phase='failed'] .turn__step-status-text {
  color: var(--dq-danger);
  font-weight: 500;
}

@keyframes step-pulse {
  0%, 100% { opacity: 1; transform: scale(1); }
  50% { opacity: 0.6; transform: scale(0.92); }
}



.turn__delegate {
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 7px 4px;
  border-radius: 6px;
  border: none;
  background: transparent;
  color: var(--dq-label-secondary);
  font-size: var(--dq-font-size-body);
  transition: background 0.12s ease;
}

.turn__delegate:hover {
  background: color-mix(in srgb, var(--dq-label-primary) 5%, transparent);
}

.turn__delegate-avatar {
  flex-shrink: 0;
  display: flex;
  align-items: center;
  justify-content: center;
  width: 28px;
  height: 28px;
  border-radius: 7px;
  font-size: var(--dq-font-size-body);
  font-weight: 700;
  color: var(--dq-on-accent);
  background: var(--dq-accent);
}

.turn__delegate-body {
  flex: 1;
  min-width: 0;
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.turn__delegate-agent {
  font-size: var(--dq-font-size-footnote);
  font-weight: 600;
  color: var(--dq-accent);
}

.turn__delegate-goal {
  margin: 0;
  font-size: var(--dq-font-size-body);
  line-height: 1.5;
  color: var(--dq-label-primary);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.turn__delegate-hint {
  margin-left: auto;
  font-size: var(--dq-font-size-footnote);
  color: var(--dq-accent);
  font-weight: 600;
  cursor: pointer;
  user-select: none;
  padding: 4px 10px;
  border-radius: 6px;
  transition: background 0.12s ease;
  flex-shrink: 0;
}

.turn__delegate-hint:hover {
  background: color-mix(in srgb, var(--dq-accent) 10%, transparent);
}

.turn__usage {
  font-size: var(--dq-font-size-caption);
  color: var(--dq-label-tertiary);
  padding: 4px 0;
  text-align: right;
}

.turn__compaction {
  display: flex;
  align-items: flex-start;
  gap: 10px;
  padding: 12px 14px;
  border-radius: 8px;
  border: 1px solid color-mix(in srgb, var(--dq-label-primary) 12%, transparent);
  background: color-mix(in srgb, var(--dq-label-primary) 5%, transparent);
  font-size: var(--dq-font-size-body);
  line-height: 1.5;
}

.turn__compaction-svg {
  flex-shrink: 0;
  color: var(--dq-label-tertiary);
  margin-top: 2px;
}

.turn__compaction-body {
  flex: 1;
  min-width: 0;
}

.turn__compaction-title {
  font-weight: 600;
  color: var(--dq-accent);
  margin-bottom: 4px;
}

.turn__compaction-detail {
  color: var(--dq-label-secondary);
  word-break: break-word;
}

.turn__error {
  display: flex;
  align-items: flex-start;
  gap: 8px;
  padding: 6px 10px;
  border-radius: 8px;
  border: 1px solid color-mix(in srgb, var(--dq-danger) 25%, transparent);
  background: color-mix(in srgb, var(--dq-danger) 6%, transparent);
  color: var(--dq-danger);
  font-size: var(--dq-font-size-caption);
  line-height: 1.45;
}

.turn__error-svg {
  flex-shrink: 0;
  margin-top: 1px;
}

.turn__error-text {
  word-break: break-word;
}




.session-workspace__composer {
  position: fixed;
  bottom: 0;
  z-index: 10;
  display: flex;
  flex-direction: column;
  gap: 4px;
  padding: 16px 0 12px;
  pointer-events: none;
  /* Skill / @ picker renders above the composer; do not clip it. */
  overflow: visible;
}

.session-workspace__composer > * {
  pointer-events: auto;
}
</style>
