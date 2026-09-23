<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { useSessionsStore } from '@/stores/sessions'
import { useWorkspaceUiStore } from '@/stores/workspaceUi'
import { toast } from '@/utils/feedback'
import { renderMarkdown } from '@/utils/markdown-render'
import { useNovelBookLoader } from '@/composables/useNovelBookLoader'
import NovelShelf from '@/components/novel/NovelShelf.vue'
import NovelBookChrome from '@/components/novel/NovelBookChrome.vue'
import NovelBinder from '@/components/novel/NovelBinder.vue'
import NovelReader from '@/components/novel/NovelReader.vue'
import NovelInspector from '@/components/novel/NovelInspector.vue'
import type { DeskAction, InjectionPreview } from '@/components/novel/NovelInspector.vue'
import {
  buildConstrainedPrefill,
  buildUnitPhases,
  canRunAction,
  castLintIssues,
  computeBookPipeline,
  countPlainChars,
  isBookOutlineName,
  novelActionSkillId,
  novelCastCardPath,
  novelContinuityDir,
  novelFactsPath,
  novelOutlineDir,
  novelSummariesDir,
  novelUnitOutlinePath,
  novelUnitProsePath,
  setupDocLabel,
  splitUnitProseSections,
  volumeId,
  volumeNumFromName,
  type NovelBookContext,
  type NovelFileNode,
  type NovelPrimaryDecision,
  type NovelStageAction,
  type NovelUnitEntry,
  type NovelUnitPhase,
} from '@/types/novel-workbench'

type View = 'shelf' | 'book'
export type TreeKind = 'book' | 'volume' | 'unit' | 'cast' | 'setup' | 'ledger'
type TreeSel = {
  kind: TreeKind
  /** volume file name / unit id / cast stem / doc name */
  name?: string
  highlight?: number
}

const { t } = useI18n()
const sessions = useSessionsStore()
const workspaceUi = useWorkspaceUiStore()

const projectId = computed(() => sessions.selectedProjectId)
const loader = useNovelBookLoader(projectId)

const view = ref<View>('shelf')
const focusMode = ref(false)
const readPath = ref<string | null>(null)
const readTitle = ref('')
const readContent = ref('')
const readLoading = ref(false)
const readPane = ref<'outline' | 'prose' | null>(null)
const castPane = ref<'wall' | 'graph' | 'card'>('wall')
const treeSel = ref<TreeSel>({ kind: 'book' })

const {
  loading,
  books,
  selectedBookId,
  activeBookId,
  unitEntries,
  continuityFiles,
  summaryFiles,
  outlineFiles,
  canonFiles,
  castFiles,
  castCards,
  extendedState,
  outlineRaws,
  unitOutlines,
  reviewRaws,
  proseRaws,
  bookState,
  bookOutlineRows,
  volumes,
  legacyLayout,
  readFile,
  loadShelf,
  openBook: loaderOpenBook,
  clearBook,
  nodePath,
  unitNodePath,
} = loader

const selectedLead = computed(
  () => sessions.agents.find((a) => a.id === sessions.selectedAgentId) ?? null,
)
const canDelegate = computed(() => Boolean(selectedLead.value?.canDelegate))
const hasNovelExpert = computed(() =>
  sessions.agents.some((a) => a.id === 'novel' && a.mode === 'subagent'),
)

const bookOutlineFile = computed(
  () => outlineFiles.value.find((f) => isBookOutlineName(f.name)) ?? null,
)

const worldDocs = computed(() => canonFiles.value.filter((n) => !n.isDir))
const castDocs = computed(() => castFiles.value.filter((n) => !n.isDir))

const bookContext = computed((): NovelBookContext | null => {
  const bookId = selectedBookId.value
  if (!bookId || !extendedState.value) return null
  const unitPhases = buildUnitPhases(
    unitEntries.value,
    extendedState.value.lastCommittedCh,
    outlineRaws.value,
    reviewRaws.value,
  )
  return {
    bookId,
    state: extendedState.value,
    entries: unitEntries.value,
    unitPhases,
    unitOutlines: unitOutlines.value,
    volumes: volumes.value,
    cast: castCards.value,
    hasBookOutline: Boolean(bookOutlineFile.value),
    legacy: legacyLayout.value,
  }
})

const selectedUnitId = computed(() =>
  treeSel.value.kind === 'unit' ? treeSel.value.name ?? null : null,
)

const pipeline = computed(() =>
  bookContext.value ? computeBookPipeline(bookContext.value, selectedUnitId.value) : null,
)

const unitPhases = computed(
  (): Record<string, NovelUnitPhase> => bookContext.value?.unitPhases ?? {},
)

const castIssues = computed(() => (bookContext.value ? castLintIssues(bookContext.value.cast) : []))

const selectedVolumeNum = computed(() => {
  if (treeSel.value.kind !== 'volume' || !treeSel.value.name) return null
  return volumeNumFromName(treeSel.value.name)
})

const selectedVolume = computed(() => {
  const n = selectedVolumeNum.value
  if (n == null) return null
  return volumes.value.find((v) => v.volume === n) ?? null
})

function blockerText(key: string): string {
  const m = key.match(/^blocker\.([A-Za-z]+)(?::(.+))?$/)
  if (m) {
    const camel = 'blocker' + m[1].charAt(0).toUpperCase() + m[1].slice(1)
    const base = t(`novelWorkbench.${camel}`)
    return m[2] ? `${base}：${m[2]}` : base
  }
  return key
}

function castIssueText(key: string): string {
  const m = key.match(/^cast\.([A-Za-z]+):(.+)$/)
  if (m) {
    const camel = 'castIssue' + m[1].charAt(0).toUpperCase() + m[1].slice(1)
    return `${t(`novelWorkbench.${camel}`)}：${m[2]}`
  }
  return blockerText(key)
}

function primaryLabel(d: NovelPrimaryDecision): string {
  switch (d.action) {
    case 'migrate':
      return t('novelWorkbench.actionMigrate')
    case 'plan':
      return selectedOrCurrentVolumeExists(d.volume)
        ? t('novelWorkbench.actionApproveVolume', { v: volumeId(d.volume ?? 1) })
        : t('novelWorkbench.actionPlan', { n: d.volume ?? 1 })
    case 'outline-batch':
      return t('novelWorkbench.actionOutlineBatch', { n: d.volume ?? 1, k: d.batchUnits?.length ?? 0 })
    case 'write':
      return t('novelWorkbench.actionWriteUnit', { unit: d.unitId ?? '' })
    case 'finalize':
      return t('novelWorkbench.actionFinalizeUnit', { unit: d.unitId ?? '' })
    case 'next-volume':
      return t('novelWorkbench.actionNextVolume', { n: d.volume ?? 1 })
    default:
      return d.action
  }
}

function selectedOrCurrentVolumeExists(volume?: number): boolean {
  if (!volume) return false
  return volumes.value.some((v) => v.volume === volume)
}

const primaryDesk = computed((): DeskAction | null => {
  const pipe = pipeline.value
  if (!pipe?.primary) return null
  const d = pipe.primary
  return {
    action: d.action,
    unitId: d.unitId,
    volume: d.volume,
    batchUnits: d.batchUnits,
    label: primaryLabel(d),
    allowed: d.allowed,
    blockers: d.blockers.map(blockerText),
  }
})

/** When the selected unit is already finalized, say where the primary button will go. */
const primaryJumpNote = computed(() => {
  const sel = selectedUnitId.value
  const d = pipeline.value?.primary
  if (!sel || !d) return ''
  if (unitPhases.value[sel] !== 'finalized') return ''
  if (d.unitId && d.unitId !== sel) return t('novelWorkbench.jumpToUnit', { from: sel, to: d.unitId })
  if (d.action === 'next-volume') return t('novelWorkbench.jumpToVolume', { from: sel, n: d.volume ?? 0 })
  return ''
})

const injectionPreview = computed((): InjectionPreview | null => {
  const ctx = bookContext.value
  if (!ctx) return null
  const d = pipeline.value?.primary
  const unitId = selectedUnitId.value ?? d?.unitId ?? ''
  const outline = unitId ? ctx.unitOutlines[unitId] : undefined
  const lane = ctx.state.craftLane && ctx.state.craftLane !== 'default' ? ctx.state.craftLane : ''
  return {
    genre: ctx.state.genre || '',
    lane,
    unitId: d?.action === 'write' || d?.action === 'finalize' || d?.action === 'contract-one' || selectedUnitId.value ? unitId : '',
    onStage: outline?.onStage ?? [],
    pov: outline?.pov ?? '',
  }
})

const moreActions = computed((): DeskAction[] => {
  const ctx = bookContext.value
  if (!ctx) return []
  const items: DeskAction[] = []
  const push = (action: NovelStageAction, label: string, unitId?: string, stem?: string) => {
    const decision = canRunAction(action, ctx, unitId)
    items.push({
      action,
      unitId,
      stem,
      label,
      allowed: decision.allowed,
      blockers: decision.blockers.map(blockerText),
    })
  }
  const unitId = selectedUnitId.value
  if (unitId) {
    const ph = unitPhases.value[unitId]
    if (ph === 'pending_outline') push('contract-one', t('novelWorkbench.actionContractOne', { unit: unitId }), unitId)
    if (ph === 'drafted' || ph === 'review_fail') {
      push('expand', t('novelWorkbench.actionExpand'), unitId)
      push('review', t('novelWorkbench.actionReview'), unitId)
      push('polish', t('novelWorkbench.actionPolish'), unitId)
    }
  }
  if (treeSel.value.kind === 'cast') {
    const stem = treeSel.value.name
    const incomplete = ctx.cast.filter((c) => c.missing.length)
    const target = stem && ctx.cast.some((c) => c.stem === stem) ? stem : incomplete[0]?.stem
    if (target) push('cast-fix', t('novelWorkbench.actionCastFix', { stem: target }), undefined, target)
  }
  return items
})

const SETUP_DOC_KEYS = new Set(['bible', 'world', 'glossary', 'reveal', 'rules', 'platform', 'goldfinger', 'authorLore', 'style', 'lockedTerms'])

function setupDocTitle(name: string): string {
  const id = setupDocLabel(name)
  if (SETUP_DOC_KEYS.has(id)) return t(`novelWorkbench.setupDoc_${id}`)
  return id
}

function unitRowLabel(entry: NovelUnitEntry): string {
  if (entry.chapterFrom > 0 && entry.chapterTo >= entry.chapterFrom) {
    return t('novelWorkbench.unitRow', { n: entry.index, from: entry.chapterFrom, to: entry.chapterTo })
  }
  return entry.unitId
}

const deskCrumb = computed(() => {
  const sel = treeSel.value
  if (sel.kind === 'book') return `${t('novelWorkbench.folderOutline')} / ${t('novelWorkbench.bookOutline')}`
  if (sel.kind === 'volume') return `${t('novelWorkbench.folderOutline')} / ${volumeId(selectedVolumeNum.value ?? 0)}`
  if (sel.kind === 'setup') return `${t('novelWorkbench.folderSetup')} / ${setupDocTitle(sel.name || '')}`
  if (sel.kind === 'ledger') return `${t('novelWorkbench.ledger')} / ${sel.name || ''}`
  if (sel.kind === 'cast') return sel.name ? `${t('novelWorkbench.castWall')} / ${sel.name}` : t('novelWorkbench.castWall')
  if (sel.kind === 'unit' && sel.name) {
    const e = unitEntries.value.find((x) => x.unitId === sel.name)
    return `${volumeId(e?.volume ?? 0)} / ${e ? unitRowLabel(e) : sel.name}`
  }
  return ''
})

const readingEntry = computed((): NovelUnitEntry | null => {
  const id = selectedUnitId.value
  if (!id) return null
  return unitEntries.value.find((e) => e.unitId === id) ?? null
})

const readingIsOutline = computed(() => treeSel.value.kind === 'unit' && readPane.value === 'outline')
const readingIsProse = computed(() => treeSel.value.kind === 'unit' && readPane.value === 'prose')

const readingUnitIndex = computed(() => {
  const id = readingEntry.value?.unitId
  if (!id) return -1
  return unitEntries.value.findIndex((e) => e.unitId === id)
})

const prevUnitEntry = computed((): NovelUnitEntry | null => {
  const i = readingUnitIndex.value
  if (i <= 0) return null
  return unitEntries.value[i - 1] ?? null
})

const nextUnitEntry = computed((): NovelUnitEntry | null => {
  const i = readingUnitIndex.value
  if (i < 0 || i >= unitEntries.value.length - 1) return null
  return unitEntries.value[i + 1] ?? null
})

const currentOutline = computed(() => {
  const id = readingEntry.value?.unitId
  if (!id) return null
  return unitOutlines.value[id] ?? null
})

const currentOutlineRaw = computed(() => {
  const id = readingEntry.value?.unitId
  return id ? outlineRaws.value[id] || '' : ''
})

const currentUnitPhase = computed((): NovelUnitPhase | null => {
  const id = readingEntry.value?.unitId
  return id ? unitPhases.value[id] ?? null : null
})

const currentProse = computed(() => {
  const id = readingEntry.value?.unitId
  if (!id) return ''
  if (readingIsProse.value && readContent.value) return readContent.value
  return proseRaws.value[id] || ''
})

const chapterSections = computed(() => {
  if (!readingIsProse.value) return []
  return splitUnitProseSections(currentProse.value)
})

const chapterChips = computed(() =>
  chapterSections.value.map((s) => ({ chapter: s.chapter, title: s.title })),
)

const chapterChars = computed(() => {
  const out: Record<number, number> = {}
  for (const s of chapterSections.value) out[s.chapter] = countPlainChars(s.body)
  return out
})

const proseChars = computed(() =>
  Object.values(chapterChars.value).reduce((sum, n) => sum + n, 0),
)

const readHtml = computed(() => {
  if (!readContent.value) return ''
  if (/\.ya?ml$/i.test(readPath.value || '')) {
    return `<pre class="novel-wb__pre">${escapeHtml(readContent.value)}</pre>`
  }
  if (readingIsProse.value) return renderUnitSections(readContent.value, treeSel.value.highlight)
  return renderMarkdown(readContent.value)
})

function escapeHtml(s: string) {
  return s.replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;')
}

function renderUnitSections(md: string, highlight?: number): string {
  const sections = splitUnitProseSections(md)
  if (!sections.length) return renderMarkdown(md)
  const copyLabel = escapeHtml(t('novelWorkbench.copyShort'))
  return sections
    .map((s, i) => {
      const title = s.title
        ? `${t('novelWorkbench.chapterN', { n: s.chapter })} ${escapeHtml(s.title)}`
        : t('novelWorkbench.chapterN', { n: s.chapter })
      const on = highlight === s.chapter ? ' novel-unit-ch--on' : ''
      const hr = i > 0 ? '<hr class="novel-unit-cut" />' : ''
      const copy = `<button type="button" class="novel-unit-ch__copy" data-copy-chapter="${s.chapter}">${copyLabel}</button>`
      return `${hr}<section id="unit-ch-${s.chapter}" class="novel-unit-ch${on}"><div class="novel-unit-ch__head"><h2>${title}</h2>${copy}</div>${renderMarkdown(s.body)}</section>`
    })
    .join('\n')
}

watch(
  projectId,
  () => {
    view.value = 'shelf'
    focusMode.value = false
    clearBook()
    void loadShelf()
  },
  { immediate: true },
)

const runningTurnCount = computed(
  () => sessions.turns.filter((turn) => turn.status === 'running').length,
)
watch(runningTurnCount, (n, prev) => {
  if ((prev ?? 0) > 0 && n === 0) void onRefresh()
})
watch(
  () => workspaceUi.filesReloadToken,
  () => {
    void onRefresh()
  },
)

async function openRead(path: string, title: string) {
  readPath.value = path
  readTitle.value = title
  readContent.value = ''
  view.value = 'book'
  readLoading.value = true
  try {
    readContent.value = await readFile(path)
  } catch {
    readContent.value = ''
    toast.error(t('novelWorkbench.readFailed'))
  } finally {
    readLoading.value = false
  }
}

function clearRead() {
  readPath.value = null
  readTitle.value = ''
  readContent.value = ''
}

function openUnitDoc(kind: 'outline' | 'prose') {
  const bookId = selectedBookId.value
  const entry = readingEntry.value
  if (!bookId || !entry) return
  readPane.value = kind
  if (kind === 'outline') {
    clearRead()
    readPath.value = entry.outline ? unitNodePath(bookId, entry.outline, 'outline') : novelUnitOutlinePath(bookId, entry.unitId)
    readTitle.value = `${entry.unitId}.yaml`
    readContent.value = currentOutlineRaw.value
    view.value = 'book'
    return
  }
  if (entry.prose) {
    void openRead(unitNodePath(bookId, entry.prose, 'prose'), entry.prose.name)
    return
  }
  clearRead()
  readPath.value = novelUnitProsePath(bookId, entry.unitId)
  readTitle.value = `${entry.unitId}.md`
  view.value = 'book'
}

async function selectBookOutline() {
  treeSel.value = { kind: 'book' }
  readPane.value = null
  const bookId = selectedBookId.value
  const f = bookOutlineFile.value
  if (bookId && f) {
    await openRead(nodePath(bookId, novelOutlineDir(bookId), f), f.name)
    return
  }
  clearRead()
  readTitle.value = t('novelWorkbench.bookOutline')
}

async function selectVolume(volume: number) {
  const bookId = selectedBookId.value
  if (!bookId) return
  const info = volumes.value.find((v) => v.volume === volume)
  treeSel.value = { kind: 'volume', name: info?.fileName ?? `${volumeId(volume)}.md` }
  readPane.value = null
  if (!info) {
    clearRead()
    readTitle.value = `${volumeId(volume)}.md`
    return
  }
  const node = { name: info.fileName, path: '', isDir: false }
  await openRead(loader.volumeNodePath(bookId, node), info.fileName)
}

async function selectSetupDoc(path: string, name: string) {
  treeSel.value = { kind: 'setup', name }
  readPane.value = null
  await openRead(path, name)
}

async function selectLedger(path: string, name: string) {
  treeSel.value = { kind: 'ledger', name }
  readPane.value = null
  await openRead(path, name)
}

function selectCast(stem?: string) {
  const bookId = selectedBookId.value
  if (!bookId) return
  treeSel.value = { kind: 'cast', name: stem }
  readPane.value = null
  if (stem) {
    castPane.value = 'card'
    const node = castDocs.value.find((f) => f.name === `${stem}.md`)
    void openRead(node?.path || novelCastCardPath(bookId, stem), `${stem}.md`)
    return
  }
  if (castPane.value === 'card') castPane.value = 'wall'
  clearRead()
}

function setCastPane(pane: 'wall' | 'graph') {
  castPane.value = pane
  if (treeSel.value.kind === 'cast' && treeSel.value.name) {
    treeSel.value = { kind: 'cast' }
    clearRead()
  }
}

function selectUnit(unitId: string) {
  const entry = unitEntries.value.find((e) => e.unitId === unitId)
  treeSel.value = { kind: 'unit', name: unitId }
  openUnitDoc(entry?.prose ? 'prose' : 'outline')
}

function goAdjacentUnit(dir: -1 | 1) {
  const entry = dir < 0 ? prevUnitEntry.value : nextUnitEntry.value
  if (!entry) return
  const kind = readPane.value === 'outline' || readPane.value === 'prose' ? readPane.value : 'prose'
  treeSel.value = { kind: 'unit', name: entry.unitId }
  openUnitDoc(kind)
}

function highlightChapter(n: number) {
  if (treeSel.value.kind !== 'unit') return
  treeSel.value = { ...treeSel.value, highlight: n }
}

async function openBook(bookId: string) {
  focusMode.value = false
  view.value = 'book'
  await loaderOpenBook(bookId)
  const d = pipeline.value?.primary
  if (d?.unitId && (d.action === 'write' || d.action === 'finalize')) {
    selectUnit(d.unitId)
  } else if (d?.volume && volumes.value.some((v) => v.volume === d.volume)) {
    await selectVolume(d.volume)
  } else {
    await selectBookOutline()
  }
}

function backToShelf() {
  view.value = 'shelf'
  focusMode.value = false
  clearBook()
  void loadShelf()
}

function runAction(desk: DeskAction) {
  const bookId = selectedBookId.value ?? undefined
  const ctx = bookContext.value
  const pipe = pipeline.value
  const action = desk.action

  if (!desk.allowed && action !== 'init') {
    toast.warning(desk.blockers.join(' · ') || t('novelWorkbench.actionBlocked'))
    return
  }

  const entry = desk.unitId ? unitEntries.value.find((e) => e.unitId === desk.unitId) : undefined
  const unitPath =
    entry?.prose && bookId
      ? unitNodePath(bookId, entry.prose, 'prose')
      : bookId && desk.unitId
        ? novelUnitProsePath(bookId, desk.unitId)
        : undefined

  let text = buildConstrainedPrefill(
    action,
    {
      bookId,
      unitId: desk.unitId,
      unitPath,
      volume: desk.volume,
      batchUnits: desk.batchUnits,
      stem: desk.stem,
      volumeOutlineExists: selectedOrCurrentVolumeExists(desk.volume),
    },
    pipe && ctx && action !== 'init' ? pipe : undefined,
    desk.blockers,
  )

  if (!canDelegate.value) {
    text = `${t('novelWorkbench.needTeamHint')}\n\n${text}`
    toast.warning(t('composer.expertNeedDelegate'))
  } else if (hasNovelExpert.value) {
    workspaceUi.requestComposerSelectExperts(['novel'])
  }
  workspaceUi.requestComposerSelectSkills([novelActionSkillId(action)])
  workspaceUi.prefillComposer(text)
}

function runInit() {
  runAction({ action: 'init', label: t('novelWorkbench.actionInit'), allowed: true, blockers: [] })
}

function runFocusPrimary() {
  const p = primaryDesk.value
  if (!p?.allowed) return
  runAction(p)
}

async function onRefresh() {
  if (view.value === 'shelf') {
    await loadShelf()
    return
  }
  if (!selectedBookId.value) return
  const pane = readPane.value
  const path = readPath.value
  const title = readTitle.value
  const sel = { ...treeSel.value }
  await loaderOpenBook(selectedBookId.value)
  treeSel.value = sel
  if (sel.kind === 'unit' && (pane === 'outline' || pane === 'prose')) openUnitDoc(pane)
  else if (sel.kind === 'cast' && !sel.name) clearRead()
  else if (path) await openRead(path, title)
}

function openLedgerDoc(node: NovelFileNode, kind: 'facts' | 'summary') {
  const bookId = selectedBookId.value
  if (!bookId) return
  const dir = kind === 'summary' ? novelSummariesDir(bookId) : novelContinuityDir(bookId)
  void selectLedger(node.path || `${dir}/${node.name}`, node.name)
}

function openFacts() {
  const bookId = selectedBookId.value
  if (!bookId) return
  const facts = continuityFiles.value.find((f) => f.name.toLowerCase() === 'facts.md')
  void selectLedger(facts?.path || novelFactsPath(bookId), 'facts.md')
}

const ledgerFiles = computed(() => continuityFiles.value.filter((f) => !f.isDir && f.name.toLowerCase() !== 'facts.md'))
</script>

<template>
  <div class="novel-wb" :class="{ 'novel-wb--focus': focusMode }">
    <template v-if="view === 'shelf'">
      <div class="novel-wb__toolbar">
        <span class="novel-wb__heading">{{ t('novelWorkbench.shelf') }}</span>
        <button type="button" class="novel-wb-link" :disabled="loading" @click="onRefresh">
          {{ t('novelWorkbench.refresh') }}
        </button>
      </div>
      <div v-if="!projectId" class="novel-wb__empty">{{ t('novelWorkbench.needProject') }}</div>
      <NovelShelf
        v-else
        :books="books"
        :active-book-id="activeBookId"
        :loading="loading"
        @open="openBook"
        @init="runInit"
      />
    </template>

    <template v-else-if="selectedBookId">
      <NovelBookChrome
        :title="bookState?.title || selectedBookId"
        :pipeline="pipeline"
        :loading="loading || readLoading"
        :focus-mode="focusMode"
        @back="backToShelf"
        @refresh="onRefresh"
        @toggle-focus="focusMode = !focusMode"
      />

      <div class="novel-wb__desk">
        <NovelBinder
          v-show="!focusMode"
          :book-id="selectedBookId"
          :tree-sel="treeSel"
          :volumes="volumes"
          :units="unitEntries"
          :unit-phases="unitPhases"
          :primary-unit="pipeline?.primary?.unitId ?? null"
          :world-docs="worldDocs"
          :ledger-files="ledgerFiles"
          :summary-files="summaryFiles"
          :cast-count="castCards.length"
          :cast-issue-count="castIssues.length"
          @select-book-outline="selectBookOutline"
          @select-volume="selectVolume"
          @select-unit="selectUnit"
          @select-cast="selectCast()"
          @select-setup="selectSetupDoc"
          @select-facts="openFacts"
          @select-ledger="openLedgerDoc"
        />

        <NovelReader
          :crumb="deskCrumb"
          :read-title="readTitle"
          :read-html="readHtml"
          :read-loading="readLoading"
          :read-content="readContent"
          :tree-kind="treeSel.kind"
          :reading-is-outline="readingIsOutline"
          :reading-is-prose="readingIsProse"
          :reading-entry="readingEntry"
          :unit-outline="currentOutline"
          :unit-phase="currentUnitPhase"
          :chapter-chips="chapterChips"
          :chapter-sections="chapterSections"
          :chapter-chars="chapterChars"
          :highlight-chapter="treeSel.highlight ?? null"
          :has-book-outline="Boolean(bookOutlineFile)"
          :book-outline-rows="bookOutlineRows"
          :volume="selectedVolume"
          :cast="castCards"
          :cast-pane="castPane"
          :cast-issues="castIssues.map(castIssueText)"
          :prev-unit="prevUnitEntry"
          :next-unit="nextUnitEntry"
          @open-outline="openUnitDoc('outline')"
          @open-prose="openUnitDoc('prose')"
          @prev-unit="goAdjacentUnit(-1)"
          @next-unit="goAdjacentUnit(1)"
          @highlight-chapter="highlightChapter"
          @select-unit="selectUnit"
          @open-cast="selectCast"
          @set-cast-pane="setCastPane"
        />

        <NovelInspector
          v-show="!focusMode"
          :pipeline="pipeline"
          :primary="primaryDesk"
          :jump-note="primaryJumpNote"
          :injection="injectionPreview"
          :more-actions="moreActions"
          :unit-phase="currentUnitPhase"
          :prose-chars="proseChars"
          :word-target="currentOutline?.wordTarget ?? ''"
          :cast-issues="treeSel.kind === 'cast' ? castIssues.map(castIssueText) : []"
          @action="runAction"
        />
      </div>

      <div v-if="focusMode && primaryDesk" class="novel-wb__focus-cta">
        <button
          type="button"
          class="novel-wb-btn novel-wb-btn--cta"
          :disabled="!primaryDesk.allowed"
          @click="runFocusPrimary"
        >
          {{ primaryDesk.label }}
        </button>
        <button type="button" class="novel-wb-link" @click="focusMode = false">
          {{ t('novelWorkbench.exitFocus') }}
        </button>
      </div>
    </template>
  </div>
</template>

<style scoped>
.novel-wb {
  position: relative;
  display: flex;
  flex-direction: column;
  min-height: 0;
  height: 100%;
  overflow: hidden;
}

.novel-wb__toolbar {
  flex-shrink: 0;
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 8px 12px;
  border-bottom: 1px solid color-mix(in srgb, var(--dq-border-subtle, #000) 50%, transparent);
}

.novel-wb__heading {
  flex: 1;
  min-width: 0;
  font-size: var(--dq-font-size-body);
  font-weight: 650;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.novel-wb__empty {
  padding: 20px 14px;
  font-size: var(--dq-font-size-body);
  opacity: 0.7;
  line-height: 1.45;
}

.novel-wb__desk {
  display: flex;
  flex: 1;
  min-height: 0;
  overflow: hidden;
}

.novel-wb--focus .novel-wb__desk {
  background: color-mix(in srgb, var(--dq-glass-popover-bg, #fff) 92%, transparent);
}

.novel-wb__focus-cta {
  position: absolute;
  right: 16px;
  bottom: 16px;
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 10px 12px;
  border-radius: 10px;
  border: 1px solid color-mix(in srgb, var(--dq-border-subtle, #000) 45%, transparent);
  background: color-mix(in srgb, var(--dq-glass-popover-bg, #fff) 92%, transparent);
  box-shadow: 0 8px 24px color-mix(in srgb, #000 12%, transparent);
  z-index: 2;
}
</style>

<style>
/* Shared action chrome used by child novel components */
.novel-wb-link {
  flex-shrink: 0;
  margin: 0;
  padding: 4px 6px;
  border: none;
  border-radius: 4px;
  background: transparent;
  color: var(--dq-accent);
  font: inherit;
  font-size: var(--dq-font-size-caption);
  cursor: pointer;
}

.novel-wb-link:disabled {
  opacity: 0.4;
  cursor: not-allowed;
}

.novel-wb-btn {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  margin: 0;
  padding: 7px 12px;
  border: 1px solid color-mix(in srgb, var(--dq-accent) 55%, transparent);
  border-radius: 7px;
  background: color-mix(in srgb, var(--dq-accent) 12%, transparent);
  color: inherit;
  font: inherit;
  font-size: var(--dq-font-size-caption);
  font-weight: 650;
  cursor: pointer;
}

.novel-wb-btn:disabled {
  opacity: 0.45;
  cursor: not-allowed;
}

.novel-wb-btn--ghost {
  border-color: color-mix(in srgb, var(--dq-border-subtle, #000) 50%, transparent);
  background: transparent;
  font-weight: 550;
}

.novel-wb-btn--cta {
  border-color: var(--dq-accent);
  background: color-mix(in srgb, var(--dq-accent) 22%, transparent);
  color: var(--dq-accent);
}

.novel-wb__pre {
  overflow: auto;
  padding: 10px 12px;
  border-radius: 8px;
  background: color-mix(in srgb, var(--dq-border-subtle, #000) 12%, transparent);
  font-size: 12.5px;
  line-height: 1.45;
}

/* Unit phase pills shared by binder / reader / inspector */
.novel-phase {
  display: inline-block;
  padding: 1px 6px;
  border-radius: 3px;
  font-size: 10px;
  font-weight: 650;
  background: color-mix(in srgb, var(--dq-border-subtle, #000) 28%, transparent);
  opacity: 0.9;
  white-space: nowrap;
}

.novel-phase--ready,
.novel-phase--drafted {
  background: color-mix(in srgb, var(--dq-accent) 16%, transparent);
  color: var(--dq-accent);
}

.novel-phase--review_fail {
  background: color-mix(in srgb, var(--dq-danger, #dc2626) 16%, transparent);
  color: var(--dq-danger, #dc2626);
}

.novel-phase--finalized {
  background: color-mix(in srgb, var(--dq-success, #16a34a) 16%, transparent);
  color: var(--dq-success, #16a34a);
}
</style>
