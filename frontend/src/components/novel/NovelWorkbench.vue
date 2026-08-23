<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { asArray, fetchJSON } from '@/api/client'
import { useSessionsStore } from '@/stores/sessions'
import { useWorkspaceUiStore } from '@/stores/workspaceUi'
import { toast } from '@/utils/feedback'
import { renderMarkdown } from '@/utils/markdown-render'
import {
  buildChapterEntries,
  buildChapterPhases,
  buildConstrainedPrefill,
  buildNovelStagePrefill,
  canRunAction,
  chapterNumFromName,
  computeBookPipeline,
  inferChapterNextAction,
  inferNovelBookNextStep,
  isNovelChapterPath,
  isNovelContractName,
  isNovelContractPath,
  novelBatchFreezePath,
  novelBiblePath,
  novelBookDir,
  novelCanonDir,
  novelCastDir,
  novelChapterFilePath,
  novelChapterReviewPath,
  novelChapterSummariesPath,
  novelChaptersDir,
  novelContinuityDir,
  novelOutlineDir,
  novelReviewsDir,
  novelStatePath,
  novelVolumesDir,
  nextVolumeNumber,
  NOVEL_PIPELINE_STEPS,
  parseBatchFreezeYaml,
  parseNovelStateExtended,
  parseNovelStateYaml,
  sortWorkbenchDocNodes,
  type GateStatus,
  type NovelBookNextStep,
  type NovelChapterEntry,
  type NovelChapterPhase,
  type NovelExtendedState,
  type NovelFileNode,
  type NovelStageAction,
  type NovelStateSummary,
} from '@/types/novel-workbench'

function nodePath(bookId: string, dir: string, node: NovelFileNode): string {
  const p = (node.path || '').replace(/\\/g, '/')
  if (p) return p
  return `${dir}/${node.name}`
}

function chapterNodePath(bookId: string, node: NovelFileNode): string {
  const p = (node.path || '').replace(/\\/g, '/')
  if (p) return p
  return `${novelChaptersDir(bookId)}/${node.name}`
}

type View = 'shelf' | 'book' | 'read'

const { t } = useI18n()
const sessions = useSessionsStore()
const workspaceUi = useWorkspaceUiStore()

const view = ref<View>('shelf')
const loading = ref(false)
const books = ref<{ id: string; path: string; state: NovelStateSummary | null }[]>([])
const selectedBookId = ref<string | null>(null)
const chapterEntries = ref<NovelChapterEntry[]>([])
const continuityFiles = ref<NovelFileNode[]>([])
const outlineFiles = ref<NovelFileNode[]>([])
const volumeFiles = ref<NovelFileNode[]>([])
const reviewFiles = ref<NovelFileNode[]>([])
const canonFiles = ref<NovelFileNode[]>([])
const castFiles = ref<NovelFileNode[]>([])
const extendedState = ref<NovelExtendedState | null>(null)
const contractRaws = ref<Record<number, string>>({})
const reviewRaws = ref<Record<number, string>>({})
const batchFreezeFrozen = ref(false)
const bookState = ref<NovelStateSummary | null>(null)
const readPath = ref<string | null>(null)
const readTitle = ref('')
const readContent = ref('')
const readLoading = ref(false)
/** Which chapter pane is active on the detail page (drives actions + content). */
const readPane = ref<'contract' | 'prose' | null>(null)
/** Chapter number for detail page when pane has no file yet. */
const readChapterNum = ref<number | null>(null)

const projectId = computed(() => sessions.selectedProjectId)

const selectedLead = computed(() =>
  sessions.agents.find((a) => a.id === sessions.selectedAgentId) ?? null,
)
const canDelegate = computed(() => Boolean(selectedLead.value?.canDelegate))
const hasNovelExpert = computed(() =>
  sessions.agents.some((a) => a.id === 'novel' && a.mode === 'subagent'),
)

const bookNextStep = computed((): NovelBookNextStep =>
  inferNovelBookNextStep(bookState.value?.lastCommittedCh ?? 0, chapterEntries.value),
)

const bookContext = computed(() => {
  const bookId = selectedBookId.value
  if (!bookId || !extendedState.value) return null
  const chapterPhases = buildChapterPhases(
    chapterEntries.value,
    extendedState.value.lastCommittedCh,
    contractRaws.value,
    reviewRaws.value,
  )
  return {
    bookId,
    state: extendedState.value,
    entries: chapterEntries.value,
    chapterPhases,
    castFileCount: castFiles.value.length,
    hasBookOutline: Boolean(bookOutlineFile.value),
    hasVolumeOutline: volumeFiles.value.length > 0,
    hasBatchFreezeFile: continuityFiles.value.some((f) => f.name === 'batch-freeze.yaml'),
    batchFreezeFrozen: batchFreezeFrozen.value,
  }
})

const pipeline = computed(() => (bookContext.value ? computeBookPipeline(bookContext.value) : null))

function phaseLabel(phase: NovelChapterPhase): string {
  const map: Record<NovelChapterPhase, string> = {
    empty: 'phaseEmpty',
    contract_draft: 'phaseContractDraft',
    contract_ready: 'phaseContractReady',
    drafted: 'phaseDrafted',
    review_fail: 'phaseReviewFail',
    review_pass: 'phaseReviewPass',
    committed: 'phaseCommitted',
  }
  return t(`novelWorkbench.${map[phase]}`)
}

function gateStatusLabel(status: GateStatus): string {
  const map: Record<GateStatus, string> = {
    pass: 'gatePass',
    fail: 'gateFail',
    unknown: 'gateUnknown',
    skipped: 'gateSkipped',
  }
  return t(`novelWorkbench.${map[status]}`)
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

function stepperLabel(id: string): string {
  const map: Record<string, string> = {
    init: 'stepperInit',
    setup: 'stepperSetup',
    outline: 'stepperOutline',
    batch_freeze: 'stepperBatchFreeze',
    chapter_loop: 'stepperChapterLoop',
    continuation: 'stepperContinuation',
    idle: 'stepperChapterLoop',
  }
  return t(`novelWorkbench.${map[id] ?? 'stepperChapterLoop'}`)
}

function primaryActionLabel(action: NovelStageAction, chapter?: number): string {
  switch (action) {
    case 'init':
      return t('novelWorkbench.actionInit')
    case 'outline':
      return t('novelWorkbench.actionOutline')
    case 'assets':
      return t('novelWorkbench.actionAssets')
    case 'batch-freeze':
      return t('novelWorkbench.actionBatchFreeze')
    case 'continuation':
      return t('novelWorkbench.actionContinuation')
    case 'contract':
      return t('novelWorkbench.actionContract', { n: chapter ?? 'N' })
    case 'write':
      return t('novelWorkbench.actionWrite', { n: chapter ?? 'N' })
    case 'continue':
      return t('novelWorkbench.actionContinue')
    case 'review':
      return t('novelWorkbench.actionReview')
    case 'commit':
      return t('novelWorkbench.actionCommit')
    default:
      return action
  }
}

function isActionAllowed(action: NovelStageAction, chapter?: number): boolean {
  if (!bookContext.value) return action === 'init'
  return canRunAction(action, bookContext.value, chapter).allowed
}

const hasAnyProse = computed(() => chapterEntries.value.some((e) => Boolean(e.prose)))

const nextVolume = computed(() => nextVolumeNumber(volumeFiles.value))

const bookOutlineFile = computed(() =>
  outlineFiles.value.find((f) => f.name === 'book_outline.md') ?? null,
)

function runBookNextStep() {
  const step = bookNextStep.value
  if (step.action === 'continue') {
    runAction('continue')
    return
  }
  runAction(step.action, step.chapter)
}

const readingChapter = computed(() => {
  if (readChapterNum.value != null) return readChapterNum.value
  if (!readPath.value) return null
  if (!isNovelChapterPath(readPath.value) && !isNovelContractPath(readPath.value)) return null
  return chapterNumFromName(readTitle.value) ?? chapterNumFromName(readPath.value)
})

const readingEntry = computed((): NovelChapterEntry | null => {
  const ch = readingChapter.value
  if (ch == null) return null
  return chapterEntries.value.find((e) => e.chapter === ch) ?? null
})

const readingIsContract = computed(() => readPane.value === 'contract')
const readingIsProse = computed(() => readPane.value === 'prose')

const readingChapterIndex = computed(() => {
  const ch = readingChapter.value
  if (ch == null) return -1
  return chapterEntries.value.findIndex((e) => e.chapter === ch)
})

const prevChapterEntry = computed((): NovelChapterEntry | null => {
  const i = readingChapterIndex.value
  if (i <= 0) return null
  return chapterEntries.value[i - 1] ?? null
})

const nextChapterEntry = computed((): NovelChapterEntry | null => {
  const i = readingChapterIndex.value
  if (i < 0 || i >= chapterEntries.value.length - 1) return null
  return chapterEntries.value[i + 1] ?? null
})

watch(projectId, () => {
  view.value = 'shelf'
  selectedBookId.value = null
  void loadShelf()
}, { immediate: true })

async function listDir(path: string): Promise<NovelFileNode[]> {
  if (!projectId.value) return []
  const q = path ? `?path=${encodeURIComponent(path)}` : ''
  return asArray(
    await fetchJSON<NovelFileNode[]>(`/projects/${projectId.value}/files${q}`),
  )
}

async function listDirSoft(path: string): Promise<NovelFileNode[]> {
  try {
    return await listDir(path)
  } catch {
    return []
  }
}

async function readFile(path: string): Promise<string> {
  if (!projectId.value) return ''
  const fc = await fetchJSON<{ content: string }>(
    `/projects/${projectId.value}/files/content?path=${encodeURIComponent(path)}`,
  )
  return fc.content ?? ''
}

async function loadState(bookId: string): Promise<NovelStateSummary | null> {
  try {
    const raw = await readFile(novelStatePath(bookId))
    extendedState.value = parseNovelStateExtended(raw)
    bookState.value = extendedState.value
    return extendedState.value
  } catch {
    extendedState.value = null
    return null
  }
}

async function loadBatchFreezeStatus(bookId: string) {
  try {
    const raw = await readFile(novelBatchFreezePath(bookId))
    batchFreezeFrozen.value = parseBatchFreezeYaml(raw).status === 'frozen'
  } catch {
    batchFreezeFrozen.value = extendedState.value?.batchFreezeArtifact === 'frozen'
  }
}

async function loadChapterMeta(bookId: string, entries: NovelChapterEntry[]) {
  const contracts: Record<number, string> = {}
  const reviews: Record<number, string> = {}
  await Promise.all(
    entries.map(async (e) => {
      if (e.contract) {
        try {
          contracts[e.chapter] = await readFile(chapterNodePath(bookId, e.contract))
        } catch {
          /* ignore */
        }
      }
      if (e.prose) {
        try {
          reviews[e.chapter] = await readFile(novelChapterReviewPath(bookId, e.chapter))
        } catch {
          /* ignore */
        }
      }
    }),
  )
  contractRaws.value = contracts
  reviewRaws.value = reviews
}

async function loadShelf() {
  if (!projectId.value) {
    books.value = []
    return
  }
  loading.value = true
  try {
    const root = await listDir('')
    const novelDir = root.find((n) => n.isDir && n.name === 'novel')
    if (!novelDir) {
      books.value = []
      return
    }
    const kids = await listDir('novel')
    const dirs = kids.filter((n) => n.isDir)
    const rows = await Promise.all(
      dirs.map(async (d) => ({
        id: d.name,
        path: d.path || novelBookDir(d.name),
        state: await loadState(d.name),
      })),
    )
    rows.sort((a, b) => a.id.localeCompare(b.id, undefined, { sensitivity: 'base' }))
    books.value = rows
  } catch {
    books.value = []
  } finally {
    loading.value = false
  }
}

async function openBook(bookId: string, opts?: { keepView?: boolean }) {
  selectedBookId.value = bookId
  if (!opts?.keepView) view.value = 'book'
  loading.value = true
  try {
    bookState.value = await loadState(bookId)
    const [chNodes, contNodes, outNodes, volNodes, revNodes, canonNodes, castNodes] = await Promise.all([
      listDirSoft(novelChaptersDir(bookId)),
      listDirSoft(novelContinuityDir(bookId)),
      listDirSoft(novelOutlineDir(bookId)),
      listDirSoft(novelVolumesDir(bookId)),
      listDirSoft(novelReviewsDir(bookId)),
      listDirSoft(novelCanonDir(bookId)),
      listDirSoft(novelCastDir(bookId)),
    ])
    chapterEntries.value = buildChapterEntries(chNodes, outNodes)
    continuityFiles.value = sortWorkbenchDocNodes(contNodes)
    outlineFiles.value = sortWorkbenchDocNodes(outNodes).filter((n) => !isNovelContractName(n.name))
    volumeFiles.value = sortWorkbenchDocNodes(volNodes)
    reviewFiles.value = sortWorkbenchDocNodes(revNodes)
    canonFiles.value = sortWorkbenchDocNodes(canonNodes)
    castFiles.value = sortWorkbenchDocNodes(castNodes)
    await loadBatchFreezeStatus(bookId)
    await loadChapterMeta(bookId, chapterEntries.value)
  } catch {
    chapterEntries.value = []
    continuityFiles.value = []
    outlineFiles.value = []
    volumeFiles.value = []
    reviewFiles.value = []
    canonFiles.value = []
    castFiles.value = []
    extendedState.value = null
    contractRaws.value = {}
    reviewRaws.value = {}
    batchFreezeFrozen.value = false
    bookState.value = null
  } finally {
    loading.value = false
  }
}

async function openRead(path: string, title: string, pane?: 'contract' | 'prose') {
  readPath.value = path
  readTitle.value = title
  readContent.value = ''
  readChapterNum.value = chapterNumFromName(title) ?? chapterNumFromName(path)
  if (pane) {
    readPane.value = pane
  } else if (isNovelContractPath(path)) {
    readPane.value = 'contract'
  } else if (isNovelChapterPath(path)) {
    readPane.value = 'prose'
  } else {
    readPane.value = null
  }
  view.value = 'read'
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

function openChapterDoc(kind: 'contract' | 'prose') {
  const bookId = selectedBookId.value
  const entry = readingEntry.value
  const ch = readingChapter.value
  if (!bookId || ch == null) return
  readPane.value = kind
  readChapterNum.value = ch

  if (kind === 'contract') {
    if (entry?.contract) {
      void openRead(chapterNodePath(bookId, entry.contract), entry.contract.name, 'contract')
      return
    }
    readPath.value = null
    readTitle.value = t('novelWorkbench.badgeContract')
    readContent.value = ''
    view.value = 'read'
    return
  }

  if (entry?.prose) {
    void openRead(chapterNodePath(bookId, entry.prose), entry.prose.name, 'prose')
    return
  }
  // No prose file yet — stay on prose pane with empty body + write actions.
  readPath.value = novelChapterFilePath(bookId, ch)
  readTitle.value = `ch${String(ch).padStart(3, '0')}.md`
  readContent.value = ''
  view.value = 'read'
}

function openChapterFromList(chapter: number, kind: 'contract' | 'prose') {
  readChapterNum.value = chapter
  readPane.value = kind
  openChapterDoc(kind)
}

function goAdjacentChapter(dir: -1 | 1) {
  const entry = dir < 0 ? prevChapterEntry.value : nextChapterEntry.value
  if (!entry) return
  const kind = readPane.value === 'contract' || readPane.value === 'prose' ? readPane.value : 'prose'
  openChapterFromList(entry.chapter, kind)
}

function backToShelf() {
  view.value = 'shelf'
  selectedBookId.value = null
  void loadShelf()
}

function backToBook() {
  view.value = 'book'
  readPath.value = null
  readPane.value = null
  readChapterNum.value = null
  if (selectedBookId.value) void openBook(selectedBookId.value)
}

function runAction(action: NovelStageAction, chapter?: number, chapterPath?: string, volume?: number) {
  const bookId = selectedBookId.value ?? undefined
  const ctx = bookContext.value
  const pipe = pipeline.value

  if (ctx && pipe && action !== 'init') {
    const decision = canRunAction(action, ctx, chapter)
    if (!decision.allowed) {
      toast.warning(decision.blockers.map(blockerText).join(' · ') || t('novelWorkbench.actionBlocked'))
      return
    }
  }

  const batchFrom = pipe?.frozenBatch?.from ?? 1
  const batchTo = pipe?.frozenBatch?.to ?? Math.max(8, (chapter ?? 0) + 7)

  let text =
    pipe && action !== 'init'
      ? buildConstrainedPrefill(
          action,
          { bookId, chapter, chapterPath, volume, batchFrom, batchTo },
          pipe,
          ctx ? canRunAction(action, ctx, chapter).blockers : [],
        )
      : buildNovelStagePrefill(action, { bookId, chapter, chapterPath, volume, batchFrom, batchTo })

  if (!canDelegate.value) {
    text = `${t('novelWorkbench.needTeamHint')}\n\n${text}`
    toast.warning(t('composer.expertNeedDelegate'))
  } else if (hasNovelExpert.value) {
    workspaceUi.requestComposerSelectExperts(['novel'])
  }
  workspaceUi.prefillComposer(text)
}

function runPrimaryAction() {
  const pipe = pipeline.value
  if (!pipe?.primaryAction) return
  runAction(pipe.primaryAction, pipe.primaryChapter)
}

function runChapterNextAction(chapter: number) {
  const ctx = bookContext.value
  if (!ctx) return
  const phase = ctx.chapterPhases[chapter]
  if (!phase) return
  const action = inferChapterNextAction(phase)
  if (!action) return
  const chPath =
    action === 'write' || action === 'review' || action === 'polish' || action === 'commit'
      ? novelChapterFilePath(ctx.bookId, chapter)
      : undefined
  runAction(action, chapter, chPath)
}

async function onRefresh() {
  if (view.value === 'shelf') await loadShelf()
  else if (view.value === 'book' && selectedBookId.value) await openBook(selectedBookId.value)
  else if (view.value === 'read' && selectedBookId.value) {
    const pane = readPane.value
    await openBook(selectedBookId.value, { keepView: true })
    if (pane === 'contract' || pane === 'prose') openChapterDoc(pane)
    else if (readPath.value) await openRead(readPath.value, readTitle.value)
  }
}

const readHtml = computed(() => {
  if (!readContent.value) return ''
  if (/\.ya?ml$/i.test(readPath.value || '')) {
    return `<pre class="novel-wb__pre">${escapeHtml(readContent.value)}</pre>`
  }
  return renderMarkdown(readContent.value)
})

function escapeHtml(s: string) {
  return s
    .replace(/&/g, '&amp;')
    .replace(/</g, '&lt;')
    .replace(/>/g, '&gt;')
}
</script>

<template>
  <div class="novel-wb">
    <div class="novel-wb__toolbar">
      <button
        v-if="view !== 'shelf'"
        type="button"
        class="novel-wb__link"
        @click="view === 'read' ? backToBook() : backToShelf()"
      >
        ← {{ view === 'read' ? t('novelWorkbench.backBook') : t('novelWorkbench.backShelf') }}
      </button>
      <span class="novel-wb__heading">
        <template v-if="view === 'shelf'">{{ t('novelWorkbench.shelf') }}</template>
        <template v-else-if="view === 'book'">{{ selectedBookId }}</template>
        <template v-else>{{ readTitle }}</template>
      </span>
      <button type="button" class="novel-wb__link" :disabled="loading || readLoading" @click="onRefresh">
        {{ t('novelWorkbench.refresh') }}
      </button>
    </div>

    <div v-if="!projectId" class="novel-wb__empty">{{ t('novelWorkbench.needProject') }}</div>

    <template v-else-if="view === 'shelf'">
      <div v-if="loading" class="novel-wb__empty">{{ t('novelWorkbench.loading') }}</div>
      <div v-else-if="!books.length" class="novel-wb__empty">
        <p>{{ t('novelWorkbench.emptyShelf') }}</p>
        <p class="novel-wb__hint">{{ t('novelWorkbench.emptyHint') }}</p>
        <button type="button" class="novel-wb__btn" @click="runAction('init')">
          {{ t('novelWorkbench.actionInit') }}
        </button>
      </div>
      <ul v-else class="novel-wb__list">
        <li v-for="b in books" :key="b.id">
          <button type="button" class="novel-wb__row" @click="openBook(b.id)">
            <span class="novel-wb__row-title">{{ b.state?.title || b.id }}</span>
            <span class="novel-wb__row-meta">
              {{ b.id }}
              <template v-if="b.state">
                · {{ b.state.stage || '—' }}
                · ch{{ b.state.lastCommittedCh }}
              </template>
            </span>
            <span v-if="b.state?.lastCommittedCh" class="novel-wb__row-progress">
              {{ t('novelWorkbench.progressLabel', { committed: b.state.lastCommittedCh, total: b.state.lastCommittedCh }) }}
            </span>
          </button>
        </li>
      </ul>
      <div v-if="books.length" class="novel-wb__actions">
        <button type="button" class="novel-wb__btn novel-wb__btn--ghost" @click="runAction('init')">
          {{ t('novelWorkbench.actionInit') }}
        </button>
      </div>
    </template>

    <template v-else-if="view === 'book' && selectedBookId">
      <div class="novel-wb__scroll">
        <div v-if="bookState" class="novel-wb__state">
          <div>{{ bookState.title || selectedBookId }}</div>
          <div class="novel-wb__row-meta">
            stage={{ bookState.stage || '—' }}
            · last=ch{{ bookState.lastCommittedCh }}
            <template v-if="pipeline">
              · {{ t('novelWorkbench.progressLabel', { committed: pipeline.progress.committed, total: pipeline.progress.totalWithContract || pipeline.progress.committed }) }}
            </template>
            <template v-if="bookState.nextAction"> · {{ bookState.nextAction }}</template>
          </div>
        </div>

        <div v-if="pipeline" class="novel-wb__stepper" role="list">
          <div
            v-for="step in NOVEL_PIPELINE_STEPS"
            :key="step.id"
            role="listitem"
            class="novel-wb__step"
            :class="{
              'novel-wb__step--active': pipeline.phase === step.id,
              'novel-wb__step--done': NOVEL_PIPELINE_STEPS.findIndex((s) => s.id === pipeline.phase) > NOVEL_PIPELINE_STEPS.findIndex((s) => s.id === step.id),
            }"
          >
            {{ stepperLabel(step.id) }}
          </div>
        </div>

        <div v-if="pipeline" class="novel-wb__gates">
          <span class="novel-wb__gates-label">{{ t('novelWorkbench.gatePanel') }}</span>
          <span class="novel-wb__gate" :class="`novel-wb__gate--${pipeline.gates.knowledge}`">
            {{ t('novelWorkbench.gateKnowledge') }}: {{ gateStatusLabel(pipeline.gates.knowledge) }}
          </span>
          <span class="novel-wb__gate" :class="`novel-wb__gate--${pipeline.gates.asset}`">
            {{ t('novelWorkbench.gateAsset') }}: {{ gateStatusLabel(pipeline.gates.asset) }}
          </span>
          <span class="novel-wb__gate" :class="`novel-wb__gate--${pipeline.gates.qc}`">
            {{ t('novelWorkbench.gateQc') }}: {{ gateStatusLabel(pipeline.gates.qc) }}
          </span>
        </div>
        <p v-if="pipeline?.blockers.length" class="novel-wb__hint novel-wb__hint--pad">
          {{ t('novelWorkbench.blockersTitle') }}:
          {{ pipeline.blockers.map(blockerText).join(' · ') }}
        </p>

        <div v-if="pipeline?.primaryAction" class="novel-wb__group">
          <div class="novel-wb__group-label">{{ t('novelWorkbench.primaryCta') }}</div>
          <div class="novel-wb__actions novel-wb__actions--tight">
            <button type="button" class="novel-wb__btn" @click="runPrimaryAction">
              {{ primaryActionLabel(pipeline.primaryAction, pipeline.primaryChapter) }}
            </button>
            <button
              type="button"
              class="novel-wb__btn novel-wb__btn--ghost"
              :disabled="!isActionAllowed('batch-freeze')"
              @click="runAction('batch-freeze')"
            >
              {{ t('novelWorkbench.actionBatchFreeze') }}
            </button>
            <button
              type="button"
              class="novel-wb__btn novel-wb__btn--ghost"
              @click="runAction('continuation')"
            >
              {{ t('novelWorkbench.actionContinuation') }}
            </button>
            <button
              type="button"
              class="novel-wb__btn novel-wb__btn--ghost"
              :disabled="!isActionAllowed('batch-review')"
              @click="runAction('batch-review')"
            >
              {{ t('novelWorkbench.actionBatchReview') }}
            </button>
            <button
              type="button"
              class="novel-wb__btn novel-wb__btn--ghost"
              @click="runAction('preflight')"
            >
              {{ t('novelWorkbench.actionPreflight') }}
            </button>
          </div>
        </div>

        <div class="novel-wb__group">
          <div class="novel-wb__group-label">{{ t('novelWorkbench.groupSetup') }}</div>
          <div class="novel-wb__actions novel-wb__actions--tight">
            <button type="button" class="novel-wb__btn novel-wb__btn--ghost" @click="runAction('assets')">
              {{ t('novelWorkbench.actionAssets') }}
            </button>
            <button type="button" class="novel-wb__btn novel-wb__btn--ghost" @click="runAction('goldfinger')">
              {{ t('novelWorkbench.actionGoldfinger') }}
            </button>
          </div>
          <div v-if="canonFiles.length || castFiles.length" class="novel-wb__quick">
            <button
              v-for="f in canonFiles"
              :key="nodePath(selectedBookId, novelCanonDir(selectedBookId), f)"
              type="button"
              class="novel-wb__chip"
              @click="openRead(nodePath(selectedBookId, novelCanonDir(selectedBookId), f), f.name)"
            >
              canon/{{ f.name }}
            </button>
            <button
              v-for="f in castFiles"
              :key="nodePath(selectedBookId, novelCastDir(selectedBookId), f)"
              type="button"
              class="novel-wb__chip"
              @click="openRead(nodePath(selectedBookId, novelCastDir(selectedBookId), f), f.name)"
            >
              cast/{{ f.name }}
            </button>
          </div>
          <p v-else class="novel-wb__hint novel-wb__hint--pad">
            {{ t('novelWorkbench.noCanonYet') }}
          </p>
        </div>

        <div class="novel-wb__group">
          <div class="novel-wb__group-label">{{ t('novelWorkbench.groupVolumes') }}</div>
          <div class="novel-wb__actions novel-wb__actions--tight">
            <button type="button" class="novel-wb__btn novel-wb__btn--ghost" @click="runAction('outline')">
              {{ t('novelWorkbench.actionOutline') }}
            </button>
            <button
              type="button"
              class="novel-wb__btn novel-wb__btn--ghost"
              @click="runAction('volume', undefined, undefined, nextVolume)"
            >
              {{ t('novelWorkbench.actionVolumeOutline', { n: nextVolume }) }}
            </button>
          </div>
          <div class="novel-wb__quick">
            <button
              v-if="bookOutlineFile"
              type="button"
              class="novel-wb__chip"
              @click="openRead(nodePath(selectedBookId, novelOutlineDir(selectedBookId), bookOutlineFile), bookOutlineFile.name)"
            >
              outline/book_outline.md
            </button>
            <button
              v-for="f in volumeFiles"
              :key="nodePath(selectedBookId, novelVolumesDir(selectedBookId), f)"
              type="button"
              class="novel-wb__chip"
              @click="openRead(nodePath(selectedBookId, novelVolumesDir(selectedBookId), f), f.name)"
            >
              volumes/{{ f.name }}
            </button>
          </div>
          <p v-if="!bookOutlineFile && !volumeFiles.length" class="novel-wb__hint novel-wb__hint--pad">
            {{ t('novelWorkbench.noVolumesYet') }}
          </p>
        </div>

        <div class="novel-wb__group">
          <div class="novel-wb__group-label">{{ t('novelWorkbench.groupNext') }}</div>
          <div class="novel-wb__actions novel-wb__actions--tight">
            <button type="button" class="novel-wb__btn" @click="runBookNextStep">
              <template v-if="bookNextStep.action === 'write'">
                {{ t('novelWorkbench.actionWrite', { n: bookNextStep.chapter }) }}
              </template>
              <template v-else-if="bookNextStep.action === 'contract'">
                {{ t('novelWorkbench.actionContract', { n: bookNextStep.chapter }) }}
              </template>
              <template v-else>
                {{ t('novelWorkbench.actionContinue') }}
              </template>
            </button>
            <button
              v-if="hasAnyProse && bookNextStep.action !== 'continue'"
              type="button"
              class="novel-wb__btn novel-wb__btn--ghost"
              @click="runAction('continue')"
            >
              {{ t('novelWorkbench.actionContinue') }}
            </button>
          </div>
          <p v-if="bookState?.nextAction" class="novel-wb__hint novel-wb__hint--pad">
            {{ bookState.nextAction }}
          </p>
        </div>

        <div class="novel-wb__group">
          <div class="novel-wb__group-label">{{ t('novelWorkbench.groupFiles') }}</div>
          <div class="novel-wb__quick">
            <button
              type="button"
              class="novel-wb__chip"
              @click="openRead(novelBiblePath(selectedBookId), 'book-bible.md')"
            >
              bible
            </button>
            <button
              type="button"
              class="novel-wb__chip"
              @click="openRead(novelStatePath(selectedBookId), 'novel-state.yaml')"
            >
              state
            </button>
            <button
              type="button"
              class="novel-wb__chip"
              @click="openRead(novelChapterSummariesPath(selectedBookId), 'chapter_summaries.md')"
            >
              summaries
            </button>
          </div>
          <div v-if="outlineFiles.length" class="novel-wb__quick">
            <button
              v-for="f in outlineFiles"
              :key="nodePath(selectedBookId, novelOutlineDir(selectedBookId), f)"
              type="button"
              class="novel-wb__chip"
              @click="openRead(nodePath(selectedBookId, novelOutlineDir(selectedBookId), f), f.name)"
            >
              outline/{{ f.name }}
            </button>
          </div>
          <div v-if="continuityFiles.length" class="novel-wb__quick">
            <button
              v-for="f in continuityFiles"
              :key="nodePath(selectedBookId, novelContinuityDir(selectedBookId), f)"
              type="button"
              class="novel-wb__chip"
              @click="openRead(nodePath(selectedBookId, novelContinuityDir(selectedBookId), f), f.name)"
            >
              continuity/{{ f.name }}
            </button>
          </div>
          <div v-if="reviewFiles.length" class="novel-wb__quick">
            <button
              v-for="f in reviewFiles.slice(-6)"
              :key="nodePath(selectedBookId, novelReviewsDir(selectedBookId), f)"
              type="button"
              class="novel-wb__chip"
              @click="openRead(nodePath(selectedBookId, novelReviewsDir(selectedBookId), f), f.name)"
            >
              reviews/{{ f.name }}
            </button>
          </div>
        </div>

        <div class="novel-wb__group">
          <div class="novel-wb__group-label">{{ t('novelWorkbench.chapters') }}</div>
          <div v-if="loading" class="novel-wb__empty">{{ t('novelWorkbench.loading') }}</div>
          <ul v-else-if="chapterEntries.length" class="novel-wb__list novel-wb__list--embedded">
            <li
              v-for="entry in chapterEntries"
              :key="`${selectedBookId}-${entry.chapter}`"
              class="novel-wb__chapter"
            >
              <div class="novel-wb__chapter-head">
                <span class="novel-wb__row-title">{{
                  t('novelWorkbench.chapterN', { n: entry.chapter })
                }}</span>
                <span
                  v-if="bookContext?.chapterPhases[entry.chapter]"
                  class="novel-wb__phase-badge"
                  :class="`novel-wb__phase-badge--${bookContext.chapterPhases[entry.chapter]}`"
                >
                  {{ phaseLabel(bookContext.chapterPhases[entry.chapter]) }}
                </span>
                <span class="novel-wb__row-meta">{{ entry.label }}</span>
              </div>
              <div class="novel-wb__chapter-files">
                <button
                  v-if="entry.contract"
                  type="button"
                  class="novel-wb__chip"
                  @click="openRead(chapterNodePath(selectedBookId, entry.contract!), entry.contract!.name, 'contract')"
                >
                  {{ t('novelWorkbench.badgeContract') }}
                </button>
                <button
                  v-if="entry.prose"
                  type="button"
                  class="novel-wb__chip"
                  @click="openRead(chapterNodePath(selectedBookId, entry.prose!), entry.prose!.name, 'prose')"
                >
                  {{ t('novelWorkbench.badgeProse') }}
                </button>
                <button
                  v-else
                  type="button"
                  class="novel-wb__chip novel-wb__chip--muted"
                  @click="openChapterFromList(entry.chapter, 'prose')"
                >
                  {{ t('novelWorkbench.badgeProse') }} · {{ t('novelWorkbench.noProseYet') }}
                </button>
                <button
                  v-if="bookContext && inferChapterNextAction(bookContext.chapterPhases[entry.chapter])"
                  type="button"
                  class="novel-wb__chip novel-wb__chip--action"
                  @click="runChapterNextAction(entry.chapter)"
                >
                  →
                  {{
                    primaryActionLabel(
                      inferChapterNextAction(bookContext.chapterPhases[entry.chapter])!,
                      entry.chapter,
                    )
                  }}
                </button>
              </div>
            </li>
          </ul>
          <div v-else class="novel-wb__empty">{{ t('novelWorkbench.noChapters') }}</div>
        </div>
      </div>
    </template>

    <template v-else-if="view === 'read'">
      <div v-if="readingChapter != null" class="novel-wb__tabs" role="tablist">
        <button
          type="button"
          role="tab"
          class="novel-wb__tab"
          :class="{
            'novel-wb__tab--active': readingIsContract,
            'novel-wb__tab--missing': !readingEntry?.contract,
          }"
          :aria-selected="readingIsContract"
          @click="openChapterDoc('contract')"
        >
          {{ t('novelWorkbench.badgeContract') }}
          <span v-if="!readingEntry?.contract" class="novel-wb__tab-hint">{{
            t('novelWorkbench.noContractYet')
          }}</span>
        </button>
        <button
          type="button"
          role="tab"
          class="novel-wb__tab"
          :class="{
            'novel-wb__tab--active': readingIsProse,
            'novel-wb__tab--missing': !readingEntry?.prose,
          }"
          :aria-selected="readingIsProse"
          @click="openChapterDoc('prose')"
        >
          {{ t('novelWorkbench.badgeProse') }}
          <span v-if="!readingEntry?.prose" class="novel-wb__tab-hint">{{
            t('novelWorkbench.noProseYet')
          }}</span>
        </button>
      </div>
      <div v-if="readingChapter != null" class="novel-wb__actions">
        <template v-if="readingIsContract">
          <button
            type="button"
            class="novel-wb__btn"
            @click="runAction('contract', readingChapter, readPath || undefined)"
          >
            {{
              readingEntry?.contract
                ? t('novelWorkbench.actionAskContract')
                : t('novelWorkbench.actionContract', { n: readingChapter })
            }}
          </button>
        </template>
        <template v-else-if="readingIsProse">
          <button
            type="button"
            class="novel-wb__btn"
            :disabled="!isActionAllowed('write', readingChapter)"
            @click="runAction('write', readingChapter, readPath || undefined)"
          >
            {{
              readingEntry?.prose
                ? t('novelWorkbench.actionAskRewrite')
                : t('novelWorkbench.actionAskWrite', { n: readingChapter })
            }}
          </button>
          <button
            v-if="readingEntry?.prose"
            type="button"
            class="novel-wb__btn novel-wb__btn--ghost"
            :disabled="!isActionAllowed('review', readingChapter)"
            @click="runAction('review', readingChapter, readPath || undefined)"
          >
            {{ t('novelWorkbench.actionReview') }}
          </button>
          <button
            v-if="readingEntry?.prose"
            type="button"
            class="novel-wb__btn novel-wb__btn--ghost"
            :disabled="!isActionAllowed('polish', readingChapter)"
            @click="runAction('polish', readingChapter, readPath || undefined)"
          >
            {{ t('novelWorkbench.actionPolish') }}
          </button>
          <button
            v-if="readingEntry?.prose"
            type="button"
            class="novel-wb__btn novel-wb__btn--ghost"
            :disabled="!isActionAllowed('commit', readingChapter)"
            @click="runAction('commit', readingChapter, readPath || undefined)"
          >
            {{ t('novelWorkbench.actionCommit') }}
          </button>
        </template>
      </div>
      <div v-if="readLoading" class="novel-wb__empty">{{ t('novelWorkbench.loading') }}</div>
      <div
        v-else-if="readingIsProse && !readingEntry?.prose"
        class="novel-wb__empty"
      >
        {{ t('novelWorkbench.noProseYet') }}
      </div>
      <div
        v-else-if="readingIsContract && !readingEntry?.contract"
        class="novel-wb__empty"
      >
        {{ t('novelWorkbench.noContractYet') }}
      </div>
      <div v-else class="novel-wb__reader" v-html="readHtml" />
      <nav
        v-if="readingChapter != null"
        class="novel-wb__chapter-nav"
        :aria-label="t('novelWorkbench.chapterNav')"
      >
        <button
          type="button"
          class="novel-wb__link"
          :disabled="!prevChapterEntry"
          @click="goAdjacentChapter(-1)"
        >
          ← {{
            prevChapterEntry
              ? t('novelWorkbench.prevChapter', { n: prevChapterEntry.chapter })
              : t('novelWorkbench.prevChapterNone')
          }}
        </button>
        <button
          type="button"
          class="novel-wb__link"
          :disabled="!nextChapterEntry"
          @click="goAdjacentChapter(1)"
        >
          {{
            nextChapterEntry
              ? t('novelWorkbench.nextChapter', { n: nextChapterEntry.chapter })
              : t('novelWorkbench.nextChapterNone')
          }} →
        </button>
      </nav>
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

.novel-wb__link {
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

.novel-wb__link:disabled {
  opacity: 0.4;
  cursor: not-allowed;
}

.novel-wb__scroll {
  flex: 1;
  min-height: 0;
  overflow: auto;
  display: flex;
  flex-direction: column;
}

.novel-wb__empty {
  padding: 20px 14px;
  font-size: var(--dq-font-size-body);
  opacity: 0.7;
  line-height: 1.45;
}

.novel-wb__hint {
  margin: 8px 0 14px;
  font-size: var(--dq-font-size-caption);
  opacity: 0.85;
}

.novel-wb__hint--pad {
  margin: 0;
  padding: 0 12px 8px;
}

.novel-wb__group {
  padding-top: 4px;
}

.novel-wb__group-label {
  padding: 10px 12px 0;
  font-size: var(--dq-font-size-caption);
  font-weight: 650;
  opacity: 0.65;
  letter-spacing: 0.02em;
}

.novel-wb__list {
  list-style: none;
  margin: 0;
  padding: 6px;
  overflow: auto;
  flex: 1;
  min-height: 0;
}

.novel-wb__list--embedded {
  flex: none;
  max-height: none;
  overflow: visible;
}

.novel-wb__row {
  width: 100%;
  display: flex;
  flex-direction: column;
  align-items: flex-start;
  gap: 2px;
  margin: 0 0 4px;
  padding: 10px 10px;
  border: none;
  border-radius: 8px;
  background: transparent;
  color: inherit;
  font: inherit;
  text-align: left;
  cursor: pointer;
}

.novel-wb__row:hover {
  background: color-mix(in srgb, var(--dq-accent) 10%, transparent);
}

.novel-wb__row-title {
  font-size: var(--dq-font-size-body);
  font-weight: 600;
}

.novel-wb__row-meta {
  font-size: var(--dq-font-size-caption);
  opacity: 0.55;
  word-break: break-all;
}

.novel-wb__state {
  padding: 10px 12px 0;
  font-size: var(--dq-font-size-body);
}

.novel-wb__tabs {
  flex-shrink: 0;
  display: flex;
  align-items: stretch;
  gap: 0;
  padding: 0 12px;
  border-bottom: 1px solid color-mix(in srgb, var(--dq-border-subtle, #000) 50%, transparent);
}

.novel-wb__tab {
  margin: 0;
  padding: 8px 12px;
  border: none;
  border-bottom: 2px solid transparent;
  margin-bottom: -1px;
  background: transparent;
  color: inherit;
  font: inherit;
  font-size: var(--dq-font-size-caption);
  font-weight: 600;
  cursor: pointer;
  opacity: 0.65;
}

.novel-wb__tab:hover:not(:disabled) {
  opacity: 1;
}

.novel-wb__tab--active {
  opacity: 1;
  border-bottom-color: var(--dq-accent);
  color: var(--dq-accent);
}

.novel-wb__tab--missing {
  opacity: 0.45;
}

.novel-wb__tab-hint {
  margin-left: 6px;
  font-weight: 400;
  opacity: 0.8;
}

.novel-wb__chip--muted {
  opacity: 0.55;
  border-style: dashed;
}

.novel-wb__chip--action {
  border-color: var(--dq-accent);
  color: var(--dq-accent);
  font-weight: 600;
}

.novel-wb__row-progress {
  font-size: var(--dq-font-size-caption);
  opacity: 0.5;
}

.novel-wb__stepper {
  display: flex;
  flex-wrap: wrap;
  gap: 4px;
  padding: 8px 12px 4px;
}

.novel-wb__step {
  padding: 4px 8px;
  border-radius: 999px;
  font-size: var(--dq-font-size-caption);
  opacity: 0.45;
  background: color-mix(in srgb, var(--dq-border-subtle, #000) 30%, transparent);
}

.novel-wb__step--active {
  opacity: 1;
  background: color-mix(in srgb, var(--dq-accent) 18%, transparent);
  color: var(--dq-accent);
  font-weight: 650;
}

.novel-wb__step--done {
  opacity: 0.7;
}

.novel-wb__gates {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 8px;
  padding: 4px 12px 8px;
  font-size: var(--dq-font-size-caption);
}

.novel-wb__gates-label {
  font-weight: 650;
  opacity: 0.65;
}

.novel-wb__gate--pass {
  color: var(--dq-success, #2a7);
}

.novel-wb__gate--fail {
  color: var(--dq-danger, #c33);
}

.novel-wb__gate--unknown {
  opacity: 0.55;
}

.novel-wb__phase-badge {
  font-size: var(--dq-font-size-caption);
  padding: 2px 6px;
  border-radius: 4px;
  background: color-mix(in srgb, var(--dq-accent) 10%, transparent);
}

.novel-wb__phase-badge--review_fail {
  background: color-mix(in srgb, var(--dq-danger, #c33) 15%, transparent);
}

.novel-wb__phase-badge--committed {
  opacity: 0.65;
}

.novel-wb__actions {
  flex-shrink: 0;
  display: flex;
  flex-wrap: wrap;
  gap: 6px;
  padding: 10px 12px;
}

.novel-wb__actions--tight {
  padding-top: 6px;
  padding-bottom: 4px;
}

.novel-wb__btn {
  margin: 0;
  padding: 6px 10px;
  border: none;
  border-radius: 6px;
  background: var(--dq-accent);
  color: var(--dq-on-accent, #fff);
  font: inherit;
  font-size: var(--dq-font-size-caption);
  font-weight: 600;
  cursor: pointer;
}

.novel-wb__btn--ghost {
  background: color-mix(in srgb, var(--dq-accent) 12%, transparent);
  color: var(--dq-accent);
}

.novel-wb__btn:disabled {
  opacity: 0.4;
  cursor: not-allowed;
}

.novel-wb__chapter {
  margin: 0 0 4px;
  padding: 10px 10px;
  border-radius: 8px;
}

.novel-wb__chapter:hover {
  background: color-mix(in srgb, var(--dq-accent) 10%, transparent);
}

.novel-wb__chapter-head {
  display: flex;
  flex-direction: column;
  align-items: flex-start;
  gap: 2px;
  margin-bottom: 6px;
}

.novel-wb__chapter-files {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 6px;
}

.novel-wb__quick {
  display: flex;
  flex-wrap: wrap;
  gap: 6px;
  padding: 6px 12px 8px;
}

.novel-wb__chip {
  margin: 0;
  padding: 3px 8px;
  border: 1px solid color-mix(in srgb, var(--dq-accent) 30%, transparent);
  border-radius: 999px;
  background: transparent;
  color: inherit;
  font: inherit;
  font-size: var(--dq-font-size-caption);
  cursor: pointer;
}

.novel-wb__chapter-nav {
  flex-shrink: 0;
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
  padding: 8px 12px 12px;
  border-top: 1px solid color-mix(in srgb, var(--dq-border-subtle, #000) 50%, transparent);
}

.novel-wb__reader {
  flex: 1;
  min-height: 0;
  overflow: auto;
  padding: 12px 14px 24px;
  font-size: var(--dq-font-size-body);
  line-height: 1.55;
}

.novel-wb__reader :deep(.novel-wb__pre),
.novel-wb__reader :deep(pre) {
  white-space: pre-wrap;
  word-break: break-word;
  margin: 0;
  font-size: var(--dq-font-size-caption);
}

.novel-wb__reader :deep(h1),
.novel-wb__reader :deep(h2),
.novel-wb__reader :deep(h3) {
  margin: 0.8em 0 0.4em;
  line-height: 1.3;
}

.novel-wb__reader :deep(p) {
  margin: 0.5em 0;
}
</style>
