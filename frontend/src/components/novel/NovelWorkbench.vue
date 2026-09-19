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
import type { DeskPrimary } from '@/components/novel/NovelInspector.vue'
import {
  buildConstrainedPrefill,
  buildUnitPhases,
  canRunAction,
  computeBookPipeline,
  countPlainChars,
  inferChapterNextAction,
  isBookOutlineName,
  mergeVolumeOutlineFiles,
  novelActionSkillId,
  novelCastDir,
  novelOutlineDir,
  novelUnitOutlinePath,
  novelUnitProsePath,
  nextVolumeNumber,
  setupDocLabel,
  splitUnitProseSections,
  volumeNumFromName,
  type NovelChapterPhase,
  type NovelFileNode,
  type NovelStageAction,
  type NovelUnitEntry,
} from '@/types/novel-workbench'

type View = 'shelf' | 'book'
type TreeSel = {
  kind: 'book' | 'volume' | 'setup' | 'unit' | 'dossier'
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
const readUnitId = ref<string | null>(null)
const treeOpen = ref<string[]>(['outline', 'prose', 'dossier'])
const setupOpen = ref<string[]>(['world', 'cast'])
const treeSel = ref<TreeSel>({ kind: 'book' })

const {
  loading,
  books,
  selectedBookId,
  activeBookId,
  unitEntries,
  continuityFiles,
  outlineFiles,
  volumeFiles,
  reviewFiles,
  canonFiles,
  castFiles,
  extendedState,
  outlineRaws,
  reviewRaws,
  proseRaws,
  bookState,
  bookOutlineRows,
  volumeUnitRows,
  readFile,
  loadShelf,
  openBook: loaderOpenBook,
  clearBook,
  nodePath,
  unitNodePath,
  volumeNodePath,
} = loader

const selectedLead = computed(
  () => sessions.agents.find((a) => a.id === sessions.selectedAgentId) ?? null,
)
const canDelegate = computed(() => Boolean(selectedLead.value?.canDelegate))
const hasNovelExpert = computed(() =>
  sessions.agents.some((a) => a.id === 'novel' && a.mode === 'subagent'),
)

const visibleVolumeFiles = computed(() =>
  mergeVolumeOutlineFiles(outlineFiles.value, volumeFiles.value),
)

const bookOutlineFile = computed(
  () => outlineFiles.value.find((f) => isBookOutlineName(f.name)) ?? null,
)

const worldDocs = computed(() => canonFiles.value.filter((n) => !n.isDir))
const castDocs = computed(() => castFiles.value.filter((n) => !n.isDir))

const bookContext = computed(() => {
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
    castFileCount: castFiles.value.length,
    hasBookOutline: Boolean(bookOutlineFile.value),
    hasVolumeOutline: visibleVolumeFiles.value.length > 0,
    hasBatchFreezeFile: false,
    batchFreezeFrozen: false,
  }
})

const pipeline = computed(() => (bookContext.value ? computeBookPipeline(bookContext.value) : null))

const unitPhases = computed(
  (): Record<string, NovelChapterPhase> => bookContext.value?.unitPhases ?? {},
)

const treeUnits = computed(() =>
  unitEntries.value.filter((e) => Boolean(e.outline || e.prose)),
)

const nextVolume = computed(() => nextVolumeNumber(visibleVolumeFiles.value))

const selectedVolumeNum = computed(() => {
  if (treeSel.value.kind !== 'volume' || !treeSel.value.name) return nextVolume.value
  return volumeNumFromName(treeSel.value.name) ?? nextVolume.value
})

const setupShowsGoldfinger = computed(() => {
  if (treeSel.value.kind !== 'setup' || !treeSel.value.name) return false
  const name = treeSel.value.name.toLowerCase()
  return name === 'book-bible.md' || name === 'world.md' || name.includes('goldfinger')
})

function primaryActionLabel(action: NovelStageAction): string {
  switch (action) {
    case 'init':
      return t('novelWorkbench.actionInit')
    case 'outline':
      return t('novelWorkbench.actionOutline')
    case 'assets':
      return t('novelWorkbench.actionAssets')
    case 'continuation':
      return t('novelWorkbench.actionContinuation')
    case 'contract':
      return t('novelWorkbench.actionContract')
    case 'write':
      return t('novelWorkbench.actionWrite')
    case 'continue':
      return t('novelWorkbench.actionContinue')
    case 'expand':
      return t('novelWorkbench.actionExpand')
    case 'volume':
      return t('novelWorkbench.actionVolumeOutline', { n: nextVolume.value })
    case 'review':
      return t('novelWorkbench.actionReview')
    case 'polish':
      return t('novelWorkbench.actionPolish')
    case 'commit':
      return t('novelWorkbench.actionCommit')
    case 'review-polish-commit':
      return t('novelWorkbench.actionReviewPolishCommit')
    case 'goldfinger':
      return t('novelWorkbench.actionGoldfinger')
    case 'preflight':
      return t('novelWorkbench.actionPreflight')
    default:
      return action
  }
}

function isActionAllowed(action: NovelStageAction, unitId?: string): boolean {
  if (!bookContext.value) return action === 'init'
  return canRunAction(action, bookContext.value, unitId).allowed
}

function blockerText(key: string): string {
  const m = key.match(/^blocker\.(.+)$/)
  if (m) {
    const part = m[1]
    const camel = 'blocker' + part.charAt(0).toUpperCase() + part.slice(1)
    return t(`novelWorkbench.${camel}`)
  }
  return key
}

const deskPrimaryFromPipeline = computed((): DeskPrimary | null => {
  const pipe = pipeline.value
  if (!pipe?.primaryAction || treeSel.value.kind === 'unit') return null
  return {
    action: pipe.primaryAction,
    unitId: pipe.primaryUnit,
    label: primaryActionLabel(pipe.primaryAction),
    allowed: isActionAllowed(pipe.primaryAction, pipe.primaryUnit),
  }
})

const unitDeskPrimary = computed((): DeskPrimary | null => {
  if (treeSel.value.kind !== 'unit' || !treeSel.value.name || !bookContext.value) return null
  const id = treeSel.value.name
  const phase = bookContext.value.unitPhases[id] ?? 'empty'
  const next = inferChapterNextAction(phase)
  if (next) {
    return {
      action: next,
      unitId: id,
      label: primaryActionLabel(next),
      allowed: next === 'contract' || isActionAllowed(next, id),
    }
  }
  const pipe = pipeline.value
  if (pipe?.primaryAction === 'continue' && phase === 'committed' && pipe.primaryUnit) {
    return {
      action: 'continue',
      unitId: pipe.primaryUnit,
      label: primaryActionLabel('continue'),
      allowed: true,
    }
  }
  return null
})

const focusPrimary = computed(() =>
  treeSel.value.kind === 'unit' ? unitDeskPrimary.value : deskPrimaryFromPipeline.value,
)

const moreActions = computed(() => {
  const entry = readingEntry.value
  if (!entry || !readingIsProse.value || !entry.prose) return []
  const primary = unitDeskPrimary.value?.action
  const items: DeskPrimary[] = []
  const push = (action: NovelStageAction, label: string) => {
    if (primary === action) return
    items.push({
      action,
      unitId: entry.unitId,
      label,
      allowed: isActionAllowed(action, entry.unitId),
    })
  }
  push('expand', t('novelWorkbench.actionExpand'))
  push('review', t('novelWorkbench.actionReview'))
  push('polish', t('novelWorkbench.actionPolish'))
  push('commit', t('novelWorkbench.actionCommit'))
  return items
})

const inspectorBlockers = computed(() => {
  const primary = focusPrimary.value
  if (!primary || !bookContext.value || primary.action === 'init') {
    return (pipeline.value?.blockers ?? []).map(blockerText)
  }
  return canRunAction(primary.action, bookContext.value, primary.unitId).blockers.map(blockerText)
})

const SETUP_DOC_KEYS = new Set(['bible', 'world', 'glossary', 'reveal', 'rules', 'platform', 'goldfinger'])

function setupDocTitle(name: string): string {
  const id = setupDocLabel(name)
  if (SETUP_DOC_KEYS.has(id)) return t(`novelWorkbench.setupDoc_${id}`)
  return id
}

function unitRowLabel(entry: NovelUnitEntry): string {
  if (entry.chapterFrom > 0 && entry.chapterTo >= entry.chapterFrom) {
    return t('novelWorkbench.unitRow', {
      n: entry.index,
      from: entry.chapterFrom,
      to: entry.chapterTo,
    })
  }
  return entry.unitId
}

function volumeLabel(name: string): string {
  return name.replace(/\.md$/i, '')
}

const deskCrumb = computed(() => {
  if (treeSel.value.kind === 'book') {
    return `${t('novelWorkbench.folderOutline')} / ${t('novelWorkbench.bookOutline')}`
  }
  if (treeSel.value.kind === 'volume') {
    return `${t('novelWorkbench.folderOutline')} / ${volumeLabel(treeSel.value.name || '')}`
  }
  if (treeSel.value.kind === 'setup') {
    return `${t('novelWorkbench.folderSetup')} / ${setupDocTitle(treeSel.value.name || '')}`
  }
  if (treeSel.value.kind === 'dossier') {
    return `${t('novelWorkbench.dossier')} / ${treeSel.value.name || ''}`
  }
  if (treeSel.value.kind === 'unit' && treeSel.value.name) {
    const e = unitEntries.value.find((x) => x.unitId === treeSel.value.name)
    return `${t('novelWorkbench.folderProse')} / ${e ? unitRowLabel(e) : treeSel.value.name}`
  }
  return ''
})

const readingEntry = computed((): NovelUnitEntry | null => {
  const id = treeSel.value.kind === 'unit' ? treeSel.value.name || readUnitId.value : readUnitId.value
  if (!id) return null
  return unitEntries.value.find((e) => e.unitId === id) ?? null
})

const readingIsOutline = computed(() => readPane.value === 'outline')
const readingIsProse = computed(() => readPane.value === 'prose')

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

const currentVolumeUnits = computed(() => {
  if (treeSel.value.kind !== 'volume' || !treeSel.value.name) return []
  return volumeUnitRows.value[treeSel.value.name] ?? []
})

const currentOutlineRaw = computed(() => {
  const id = readingEntry.value?.unitId
  if (!id) return ''
  return outlineRaws.value[id] || ''
})

const currentUnitPhase = computed((): NovelChapterPhase | null => {
  const id = readingEntry.value?.unitId
  if (!id) return null
  return unitPhases.value[id] ?? null
})

const currentProse = computed(() => {
  const id = readingEntry.value?.unitId
  if (!id) return ''
  if (readingIsProse.value && readContent.value) return readContent.value
  return proseRaws.value[id] || ''
})

const chapterChips = computed(() => {
  if (!readingIsProse.value) return []
  return splitUnitProseSections(currentProse.value).map((s) => ({ chapter: s.chapter, title: s.title }))
})

const chapterChars = computed(() => {
  const out: Record<number, number> = {}
  for (const s of splitUnitProseSections(currentProse.value)) {
    out[s.chapter] = countPlainChars(s.body)
  }
  return out
})

const proseChars = computed(() =>
  Object.values(chapterChars.value).reduce((sum, n) => sum + n, 0),
)

const readHtml = computed(() => {
  if (!readContent.value) return ''
  if (/\.ya?ml$/i.test(readPath.value || '') || readingIsOutline.value) {
    return `<pre class="novel-wb__pre">${escapeHtml(readContent.value)}</pre>`
  }
  if (readingIsProse.value && treeSel.value.kind === 'unit') {
    return renderUnitSections(readContent.value, treeSel.value.highlight)
  }
  return renderMarkdown(readContent.value)
})

function escapeHtml(s: string) {
  return s
    .replace(/&/g, '&amp;')
    .replace(/</g, '&lt;')
    .replace(/>/g, '&gt;')
}

function renderUnitSections(md: string, highlight?: number): string {
  const sections = splitUnitProseSections(md)
  if (!sections.length) return renderMarkdown(md)
  return sections
    .map((s, i) => {
      const title = s.title
        ? `${t('novelWorkbench.chapterN', { n: s.chapter })} ${escapeHtml(s.title)}`
        : t('novelWorkbench.chapterN', { n: s.chapter })
      const on = highlight === s.chapter ? ' novel-unit-ch--on' : ''
      const hr = i > 0 ? '<hr class="novel-unit-cut" />' : ''
      return `${hr}<section id="unit-ch-${s.chapter}" class="novel-unit-ch${on}"><h2>${title}</h2>${renderMarkdown(s.body)}</section>`
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

async function openRead(path: string, title: string, pane?: 'outline' | 'prose') {
  readPath.value = path
  readTitle.value = title
  readContent.value = ''
  if (pane) readPane.value = pane
  else if (treeSel.value.kind !== 'unit') readPane.value = null
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

function openUnitDoc(kind: 'outline' | 'prose') {
  const bookId = selectedBookId.value
  const entry = readingEntry.value
  if (!bookId || !entry) return
  readPane.value = kind
  readUnitId.value = entry.unitId

  if (kind === 'outline') {
    if (entry.outline) {
      void openRead(unitNodePath(bookId, entry.outline, 'outline'), entry.outline.name, 'outline')
      return
    }
    readPath.value = novelUnitOutlinePath(bookId, entry.unitId)
    readTitle.value = `${entry.unitId}.yaml`
    readContent.value = ''
    return
  }

  if (entry.prose) {
    void openRead(unitNodePath(bookId, entry.prose, 'prose'), entry.prose.name, 'prose')
    return
  }
  readPath.value = novelUnitProsePath(bookId, entry.unitId)
  readTitle.value = `${entry.unitId}.md`
  readContent.value = ''
  view.value = 'book'
}

async function selectBookOutline() {
  treeSel.value = { kind: 'book' }
  readPane.value = null
  readUnitId.value = null
  const bookId = selectedBookId.value
  const f = bookOutlineFile.value
  if (bookId && f) {
    await openRead(nodePath(bookId, novelOutlineDir(bookId), f), f.name)
    return
  }
  readPath.value = null
  readTitle.value = t('novelWorkbench.bookOutline')
  readContent.value = ''
}

async function selectVolume(node: NovelFileNode) {
  const bookId = selectedBookId.value
  if (!bookId) return
  treeSel.value = { kind: 'volume', name: node.name }
  readPane.value = null
  readUnitId.value = null
  await openRead(volumeNodePath(bookId, node), node.name)
}

async function selectSetupDoc(path: string, name: string) {
  treeSel.value = { kind: 'setup', name }
  readPane.value = null
  readUnitId.value = null
  await openRead(path, name)
}

async function selectDossier(path: string, name: string) {
  const bookId = selectedBookId.value
  if (!bookId) return
  treeSel.value = { kind: 'dossier', name }
  readPane.value = null
  readUnitId.value = null
  const full = path.includes('/') ? path : `novel/${bookId}/continuity/${name}`
  const hit =
    continuityFiles.value.find((f) => f.name === name) ||
    reviewFiles.value.find((f) => f.name === name)
  await openRead(hit?.path || full, name)
}

function selectUnit(unitId: string) {
  const entry = unitEntries.value.find((e) => e.unitId === unitId)
  treeSel.value = { kind: 'unit', name: unitId }
  readUnitId.value = unitId
  openUnitDoc(entry?.prose ? 'prose' : 'outline')
}

function goAdjacentUnit(dir: -1 | 1) {
  const entry = dir < 0 ? prevUnitEntry.value : nextUnitEntry.value
  if (!entry) return
  const kind = readPane.value === 'outline' || readPane.value === 'prose' ? readPane.value : 'prose'
  treeSel.value = { kind: 'unit', name: entry.unitId }
  readUnitId.value = entry.unitId
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
  treeOpen.value = unitEntries.value.some((e) => Boolean(e.prose))
    ? ['prose', 'dossier']
    : ['outline', 'prose', 'dossier']
  setupOpen.value = ['world', 'cast']
  await selectBookOutline()
}

function backToShelf() {
  view.value = 'shelf'
  focusMode.value = false
  clearBook()
  void loadShelf()
}

function runAction(action: NovelStageAction, unitId?: string, volume?: number) {
  const bookId = selectedBookId.value ?? undefined
  const ctx = bookContext.value
  const pipe = pipeline.value

  if (ctx && pipe && action !== 'init') {
    const decision = canRunAction(action, ctx, unitId)
    if (!decision.allowed) {
      toast.warning(
        decision.blockers.map(blockerText).join(' · ') || t('novelWorkbench.actionBlocked'),
      )
      return
    }
  }

  const entry = unitId ? unitEntries.value.find((e) => e.unitId === unitId) : undefined
  const unitPath =
    entry?.prose
      ? unitNodePath(bookId || '', entry.prose, 'prose')
      : bookId && unitId
        ? novelUnitProsePath(bookId, unitId)
        : undefined

  let text = buildConstrainedPrefill(
    action,
    { bookId, unitId, unitPath, volume },
    pipe && action !== 'init' ? pipe : undefined,
    ctx && action !== 'init' ? canRunAction(action, ctx, unitId).blockers : [],
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

function onInspectorAction(
  action: NovelStageAction,
  unitId?: string,
  opts?: { volume?: number },
) {
  runAction(action, unitId, opts?.volume)
}

function runFocusPrimary() {
  const primary = focusPrimary.value
  if (!primary?.allowed) return
  onInspectorAction(primary.action, primary.unitId, {
    volume: primary.action === 'volume' ? nextVolume.value : undefined,
  })
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
  if ((pane === 'outline' || pane === 'prose') && sel.kind === 'unit') openUnitDoc(pane)
  else if (path) await openRead(path, title)
}

async function openCast(node: NovelFileNode) {
  const bookId = selectedBookId.value
  if (!bookId) return
  const path = node.path || `${novelCastDir(bookId)}/${node.name}`
  await selectSetupDoc(path, node.name)
}

async function openLedger(node: NovelFileNode) {
  await selectDossier(node.path || node.name, node.name)
}
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
        @init="runAction('init')"
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
          :tree-open="treeOpen"
          :setup-open="setupOpen"
          :tree-sel="treeSel"
          :book-outline-selected="treeSel.kind === 'book'"
          :visible-volume-files="visibleVolumeFiles"
          :world-docs="worldDocs"
          :cast-docs="castDocs"
          :tree-units="treeUnits"
          :unit-phases="unitPhases"
          :continuity-files="continuityFiles"
          :review-files="reviewFiles"
          :next-volume="nextVolume"
          @update:tree-open="treeOpen = $event"
          @update:setup-open="setupOpen = $event"
          @select-book-outline="selectBookOutline"
          @select-volume="selectVolume"
          @select-setup="selectSetupDoc"
          @select-unit="selectUnit"
          @select-dossier="selectDossier"
          @add-volume="runAction('volume', undefined, nextVolume)"
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
          :chapter-chips="chapterChips"
          :highlight-chapter="treeSel.highlight ?? null"
          :has-book-outline="Boolean(bookOutlineFile)"
          :book-outline-rows="bookOutlineRows"
          :volume-units="currentVolumeUnits"
          :prev-unit="prevUnitEntry"
          :next-unit="nextUnitEntry"
          @open-outline="openUnitDoc('outline')"
          @open-prose="openUnitDoc('prose')"
          @prev-unit="goAdjacentUnit(-1)"
          @next-unit="goAdjacentUnit(1)"
          @highlight-chapter="highlightChapter"
        />

        <NovelInspector
          v-show="!focusMode"
          :pipeline="pipeline"
          :tree-kind="treeSel.kind"
          :reading-is-outline="readingIsOutline"
          :reading-is-prose="readingIsProse"
          :unit-id="readingEntry?.unitId ?? null"
          :outline-raw="currentOutlineRaw"
          :unit-phase="currentUnitPhase"
          :highlight-chapter="treeSel.highlight ?? null"
          :prose-chars="proseChars"
          :chapter-chars="chapterChars"
          :cast-docs="castDocs"
          :continuity-files="continuityFiles"
          :desk-primary="deskPrimaryFromPipeline"
          :unit-primary="unitDeskPrimary"
          :more-actions="moreActions"
          :blockers="inspectorBlockers"
          :setup-shows-goldfinger="setupShowsGoldfinger"
          :has-book-outline="Boolean(bookOutlineFile)"
          :selected-volume-num="selectedVolumeNum"
          :next-volume="nextVolume"
          @action="onInspectorAction"
          @open-cast="openCast"
          @open-ledger="openLedger"
        />
      </div>

      <div v-if="focusMode && focusPrimary" class="novel-wb__focus-cta">
        <button
          type="button"
          class="novel-wb-btn novel-wb-btn--cta"
          :disabled="!focusPrimary.allowed"
          @click="runFocusPrimary"
        >
          {{ focusPrimary.label }}
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

.novel-wb {
  position: relative;
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
</style>
