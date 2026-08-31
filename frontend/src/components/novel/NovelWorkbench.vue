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
  canRunAction,
  chapterNumFromName,
  computeBookPipeline,
  inferChapterNextAction,
  isBookOutlineName,
  isNovelChapterPath,
  isNovelContractName,
  isNovelContractPath,
  mergeVolumeOutlineFiles,
  novelActionSkillId,
  NOVEL_PIPELINE_STEPS,
  parseBookOutlineVolumeRows,
  parseVolumeUnitRows,
  novelBatchFreezePath,
  novelBiblePath,
  novelBookDir,
  novelCanonDir,
  novelCastDir,
  novelChapterFilePath,
  novelChapterReviewPath,
  novelChaptersDir,
  novelContinuityDir,
  novelActiveBookPath,
  novelOutlineDir,
  novelReviewsDir,
  novelStatePath,
  novelVolumesDir,
  nextVolumeNumber,
  parseBatchFreezeYaml,
  parseContractYaml,
  parseNovelStateExtended,
  setupDocLabel,
  sortWorkbenchDocNodes,
  volumeNumFromName,
  type BookOutlineVolumeRow,
  type NovelChapterEntry,
  type NovelChapterPhase,
  type NovelExtendedState,
  type NovelFileNode,
  type NovelPipelineStepId,
  type NovelStageAction,
  type NovelStateSummary,
  type VolumeUnitRow,
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

type View = 'shelf' | 'book'

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
const activeBookId = ref<string | null>(null)
const treeOpen = ref<string[]>(['outline', 'prose'])
const setupOpen = ref<string[]>(['world', 'cast'])
const bookOutlineRows = ref<BookOutlineVolumeRow[]>([])
const volumeUnitRows = ref<Record<string, VolumeUnitRow[]>>({})
const treeSel = ref<{ kind: 'book' | 'volume' | 'setup' | 'chapter'; name?: string; n?: number }>({
  kind: 'book',
})

const projectId = computed(() => sessions.selectedProjectId)

const visibleVolumeFiles = computed(() =>
  mergeVolumeOutlineFiles(outlineFiles.value, volumeFiles.value),
)

const selectedLead = computed(() =>
  sessions.agents.find((a) => a.id === sessions.selectedAgentId) ?? null,
)
const canDelegate = computed(() => Boolean(selectedLead.value?.canDelegate))
const hasNovelExpert = computed(() =>
  sessions.agents.some((a) => a.id === 'novel' && a.mode === 'subagent'),
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
    hasVolumeOutline: visibleVolumeFiles.value.length > 0,
    hasBatchFreezeFile: false,
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

function blockerText(key: string): string {
  const m = key.match(/^blocker\.(.+)$/)
  if (m) {
    const part = m[1]
    const camel = 'blocker' + part.charAt(0).toUpperCase() + part.slice(1)
    return t(`novelWorkbench.${camel}`)
  }
  return key
}

function stepperLabel(id: NovelPipelineStepId | string): string {
  const map: Record<string, string> = {
    init: 'stepperInit',
    setup: 'stepperSetup',
    outline: 'stepperBookOutline',
    volume: 'stepperVolume',
    contract: 'stepperContract',
    write: 'stepperWriting',
    review: 'stepperReview',
    commit: 'stepperCommit',
    idle: 'stepperWriting',
  }
  return t(`novelWorkbench.${map[id] ?? 'stepperWriting'}`)
}

function gateLabel(status: string): string {
  switch (status) {
    case 'pass':
      return t('novelWorkbench.gatePass')
    case 'fail':
      return t('novelWorkbench.gateFail')
    case 'skipped':
      return t('novelWorkbench.gateSkipped')
    default:
      return t('novelWorkbench.gateUnknown')
  }
}

const pipelineSteps = NOVEL_PIPELINE_STEPS

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
    case 'dialogue':
      return t('novelWorkbench.actionDialogue')
    case 'hook':
      return t('novelWorkbench.actionHook')
    case 'reversal':
      return t('novelWorkbench.actionReversal')
    case 'volume':
      return t('novelWorkbench.actionVolumeOutline', { n: chapter && chapter > 0 ? chapter : 'N' })
    case 'review':
      return t('novelWorkbench.actionReview')
    case 'polish':
      return t('novelWorkbench.actionPolish')
    case 'commit':
      return t('novelWorkbench.actionCommit')
    case 'review-polish-commit':
      return t('novelWorkbench.actionReviewPolishCommit')
    default:
      return action
  }
}

function isActionAllowed(action: NovelStageAction, chapter?: number): boolean {
  if (!bookContext.value) return action === 'init'
  return canRunAction(action, bookContext.value, chapter).allowed
}

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

const deskBatchFreezeAllowed = computed(() => {
  if (!bookContext.value) return false
  return canRunAction('batch-freeze', bookContext.value).allowed
})

interface DeskPrimary {
  action: NovelStageAction
  chapter?: number
  label: string
  allowed: boolean
}

const deskPrimaryFromPipeline = computed((): DeskPrimary | null => {
  const pipe = pipeline.value
  if (!pipe?.primaryAction || treeSel.value.kind === 'chapter') return null
  return {
    action: pipe.primaryAction,
    chapter: pipe.primaryChapter,
    label: primaryActionLabel(pipe.primaryAction, pipe.primaryChapter),
    allowed: isActionAllowed(pipe.primaryAction, pipe.primaryChapter),
  }
})

const chapterDeskPrimary = computed((): DeskPrimary | null => {
  if (treeSel.value.kind !== 'chapter' || treeSel.value.n == null || !bookContext.value) return null
  const n = treeSel.value.n
  const phase = bookContext.value.chapterPhases[n]

  if (readingIsContract.value) {
    if (phase === 'empty' || phase === 'contract_draft' || phase === 'contract_ready') {
      return {
        action: 'contract',
        chapter: n,
        label: t('novelWorkbench.actionAskContract'),
        allowed: true,
      }
    }
    return null
  }

  if (!readingIsProse.value) return null

  const chapterAction = inferChapterNextAction(phase)
  if (chapterAction) {
    return {
      action: chapterAction,
      chapter: n,
      label: primaryActionLabel(chapterAction, n),
      allowed: isActionAllowed(chapterAction, n),
    }
  }

  const pipe = pipeline.value
  if (pipe?.primaryAction === 'continue' && phase === 'committed') {
    const lastCh = bookContext.value.state.lastCommittedCh
    if (n === lastCh) {
      return {
        action: 'continue',
        chapter: pipe.primaryChapter,
        label: primaryActionLabel('continue'),
        allowed: true,
      }
    }
  }
  return null
})

const bookOutlineFile = computed(() =>
  outlineFiles.value.find((f) => isBookOutlineName(f.name)) ?? null,
)

const worldDocs = computed(() => canonFiles.value.filter((n) => !n.isDir))
const castDocs = computed(() => castFiles.value.filter((n) => !n.isDir))

const SETUP_DOC_KEYS = new Set(['bible', 'world', 'glossary', 'reveal', 'rules', 'platform', 'goldfinger'])

function setupDocTitle(name: string): string {
  const id = setupDocLabel(name)
  if (SETUP_DOC_KEYS.has(id)) return t(`novelWorkbench.setupDoc_${id}`)
  return id
}

/** Contract-only chapters stay in the tree; they are the writing queue. */
const treeChapters = computed(() =>
  chapterEntries.value.filter((e) => Boolean(e.contract || e.prose)),
)

function chapterTreeName(entry: NovelChapterEntry): string {
  const title = parseContractYaml(contractRaws.value[entry.chapter] || '').title.trim()
  const pad = String(entry.chapter).padStart(3, '0')
  return title ? `第${pad}章_${title}` : t('novelWorkbench.chapterN', { n: entry.chapter })
}

const deskCrumb = computed(() => {
  if (treeSel.value.kind === 'book') return `${t('novelWorkbench.folderOutline')} / ${t('novelWorkbench.bookOutline')}`
  if (treeSel.value.kind === 'volume') {
    return `${t('novelWorkbench.folderOutline')} / ${volumeLabel(treeSel.value.name || '')}`
  }
  if (treeSel.value.kind === 'setup') {
    return `${t('novelWorkbench.folderSetup')} / ${setupDocTitle(treeSel.value.name || '')}`
  }
  if (treeSel.value.kind === 'chapter' && treeSel.value.n != null) {
    const e = chapterEntries.value.find((x) => x.chapter === treeSel.value.n)
    return `${t('novelWorkbench.folderProse')} / ${e ? chapterTreeName(e) : t('novelWorkbench.chapterN', { n: treeSel.value.n })}`
  }
  return ''
})

function volumeLabel(name: string): string {
  return name.replace(/\.md$/i, '')
}

async function selectBookOutline() {
  treeSel.value = { kind: 'book' }
  readPane.value = null
  readChapterNum.value = null
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
  readChapterNum.value = null
  await openRead(volumeNodePath(bookId, node), node.name)
}

async function selectSetupDoc(path: string, title: string) {
  treeSel.value = { kind: 'setup', name: title }
  readPane.value = null
  readChapterNum.value = null
  await openRead(path, title)
}

function selectChapter(n: number, pane: 'contract' | 'prose' = 'prose') {
  treeSel.value = { kind: 'chapter', n }
  openChapterFromList(n, pane)
}

function volumeNodePath(bookId: string, node: NovelFileNode): string {
  const p = (node.path || '').replace(/\\/g, '/')
  if (p) return p
  if (volumeFiles.value.some((v) => v.name === node.name)) {
    return `${novelVolumesDir(bookId)}/${node.name}`
  }
  return `${novelOutlineDir(bookId)}/${node.name}`
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

const runningTurnCount = computed(
  () => sessions.turns.filter((turn) => turn.status === 'running').length,
)
watch(runningTurnCount, (n, prev) => {
  if ((prev ?? 0) > 0 && n === 0) void onRefresh()
})
watch(() => workspaceUi.filesReloadToken, () => {
  void onRefresh()
})

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

async function loadBatchFreezeStatus(_bookId: string) {
  // Freeze lives in novel-state; legacy batch-freeze.yaml is optional soft-read
  if (extendedState.value?.batchFreezeArtifact === 'frozen') {
    batchFreezeFrozen.value = true
    return
  }
  try {
    const raw = await readFile(novelBatchFreezePath(_bookId))
    batchFreezeFrozen.value = parseBatchFreezeYaml(raw).status === 'frozen'
  } catch {
    batchFreezeFrozen.value = Boolean(extendedState.value?.frozenBatch?.from)
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
    try {
      const raw = await readFile(novelActiveBookPath())
      activeBookId.value = raw.trim().split('\n')[0] || null
    } catch {
      activeBookId.value = null
    }
    const dirs = kids.filter((n) => n.isDir && !n.name.startsWith('.'))
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
    await loadOutlinePreviews(bookId)
    treeOpen.value = chapterEntries.value.some((e) => Boolean(e.prose))
      ? ['prose']
      : ['outline', 'prose']
    setupOpen.value = ['world', 'cast']
    void persistActiveBook(bookId)
    if (!opts?.keepView) void selectBookOutline()
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
    bookOutlineRows.value = []
    volumeUnitRows.value = {}
  } finally {
    loading.value = false
  }
}

async function loadOutlinePreviews(bookId: string) {
  bookOutlineRows.value = []
  volumeUnitRows.value = {}
  const bookFile = outlineFiles.value.find((f) => isBookOutlineName(f.name))
  if (bookFile) {
    try {
      const raw = await readFile(nodePath(bookId, novelOutlineDir(bookId), bookFile))
      bookOutlineRows.value = parseBookOutlineVolumeRows(raw)
    } catch {
      /* ignore */
    }
  }
  const previews: Record<string, VolumeUnitRow[]> = {}
  await Promise.all(
    mergeVolumeOutlineFiles(outlineFiles.value, volumeFiles.value).map(async (f) => {
      try {
        previews[f.name] = parseVolumeUnitRows(await readFile(volumeNodePath(bookId, f)))
      } catch {
        previews[f.name] = []
      }
    }),
  )
  volumeUnitRows.value = previews
}

async function persistActiveBook(bookId: string) {
  if (!projectId.value) return
  try {
    await fetchJSON(`/projects/${projectId.value}/files/content`, {
      method: 'PUT',
      body: JSON.stringify({ path: novelActiveBookPath(), content: `${bookId}\n` }),
    })
    activeBookId.value = bookId
  } catch {
    /* console is read-mostly; missing write permission should not block */
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
  view.value = 'book'
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

  let text = buildConstrainedPrefill(
    action,
    { bookId, chapter, chapterPath, volume, batchFrom, batchTo },
    pipe && action !== 'init' ? pipe : undefined,
    ctx && action !== 'init' ? canRunAction(action, ctx, chapter).blockers : [],
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

function runChapterDeskPrimary() {
  const primary = chapterDeskPrimary.value
  if (!primary?.allowed) return
  const bookId = selectedBookId.value
  if (!bookId) return
  const chapter = primary.chapter
  let chapterPath: string | undefined
  if (chapter != null) {
    const proseActions: NovelStageAction[] = [
      'write',
      'review',
      'polish',
      'commit',
      'review-polish-commit',
      'dialogue',
      'hook',
      'reversal',
      'continue',
    ]
    if (proseActions.includes(primary.action) && primary.action !== 'continue') {
      chapterPath = readPath.value || novelChapterFilePath(bookId, chapter)
    }
  }
  runAction(primary.action, chapter, chapterPath)
}

function runDeskVolume(vol?: number) {
  runAction('volume', undefined, undefined, vol ?? selectedVolumeNum.value)
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
  await openBook(selectedBookId.value, { keepView: true })
  treeSel.value = sel
  if (pane === 'contract' || pane === 'prose') openChapterDoc(pane)
  else if (path) await openRead(path, title)
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
        @click="backToShelf()"
      >
        ← {{ t('novelWorkbench.backShelf') }}
      </button>
      <span class="novel-wb__heading">
        <template v-if="view === 'shelf'">{{ t('novelWorkbench.shelf') }}</template>
        <template v-else>{{ bookState?.title || selectedBookId }}</template>
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
          <button
            type="button"
            class="novel-wb__row"
            :class="{ 'novel-wb__row--active': activeBookId === b.id }"
            @click="openBook(b.id)"
          >
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
      <div v-if="pipeline" class="novel-wb__rail">
        <div class="novel-wb__stepper novel-wb__stepper--rail" role="list">
          <button
            v-for="s in pipelineSteps"
            :key="s.id"
            type="button"
            role="listitem"
            class="novel-wb__step"
            :class="{
              'novel-wb__step--current': pipeline.step === s.id,
              'novel-wb__step--done':
                pipelineSteps.findIndex((x) => x.id === pipeline.step) >
                pipelineSteps.findIndex((x) => x.id === s.id),
            }"
            :disabled="!s.action"
            @click="s.action && runAction(s.action, pipeline.primaryChapter, undefined, s.id === 'volume' ? nextVolume : undefined)"
          >
            {{ stepperLabel(s.id) }}
          </button>
        </div>
        <div class="novel-wb__rail-meta">
          <div class="novel-wb__gates" :aria-label="t('novelWorkbench.gatePanel')">
            <span
              class="novel-wb__gate"
              :class="'novel-wb__gate--' + pipeline.gates.knowledge"
              :title="t('novelWorkbench.gateKnowledge')"
            >{{ t('novelWorkbench.gateKnowledge') }} · {{ gateLabel(pipeline.gates.knowledge) }}</span>
            <span
              class="novel-wb__gate"
              :class="'novel-wb__gate--' + pipeline.gates.asset"
              :title="t('novelWorkbench.gateAsset')"
            >{{ t('novelWorkbench.gateAsset') }} · {{ gateLabel(pipeline.gates.asset) }}</span>
            <span
              class="novel-wb__gate"
              :class="'novel-wb__gate--' + pipeline.gates.qc"
              :title="t('novelWorkbench.gateQc')"
            >{{ t('novelWorkbench.gateQc') }} · {{ gateLabel(pipeline.gates.qc) }}</span>
          </div>
          <button
            v-if="deskPrimaryFromPipeline"
            type="button"
            class="novel-wb__btn novel-wb__btn--cta"
            :disabled="!deskPrimaryFromPipeline.allowed"
            @click="runAction(deskPrimaryFromPipeline.action, deskPrimaryFromPipeline.chapter, undefined, deskPrimaryFromPipeline.action === 'volume' ? nextVolume : undefined)"
          >
            {{ t('novelWorkbench.primaryCta') }} · {{ deskPrimaryFromPipeline.label }}
          </button>
        </div>
        <p class="novel-wb__model-tip">{{ t('novelWorkbench.modelTip') }}</p>
      </div>
      <div class="novel-wb__book">
        <aside class="novel-wb__tree">
          <DqCollapse v-model="treeOpen">
            <DqCollapseItem name="outline" :title="t('novelWorkbench.folderOutline')">
              <button
                type="button"
                class="novel-wb__tree-item"
                :class="{ 'novel-wb__tree-item--on': treeSel.kind === 'book' }"
                @click="selectBookOutline()"
              >
                {{ t('novelWorkbench.bookOutline') }}
              </button>
              <button
                v-for="v in visibleVolumeFiles"
                :key="v.name"
                type="button"
                class="novel-wb__tree-item"
                :class="{ 'novel-wb__tree-item--on': treeSel.kind === 'volume' && treeSel.name === v.name }"
                @click="selectVolume(v)"
              >
                {{ volumeLabel(v.name) }}
              </button>
              <button type="button" class="novel-wb__tree-item novel-wb__tree-item--ghost" @click="runAction('volume', undefined, undefined, nextVolume)">
                + {{ t('novelWorkbench.actionVolumeOutline', { n: nextVolume }) }}
              </button>
            </DqCollapseItem>

            <DqCollapseItem name="setup" :title="t('novelWorkbench.folderSetup')">
              <button
                type="button"
                class="novel-wb__tree-item"
                :class="{ 'novel-wb__tree-item--on': treeSel.kind === 'setup' && treeSel.name === 'book-bible.md' }"
                @click="selectSetupDoc(novelBiblePath(selectedBookId), 'book-bible.md')"
              >
                {{ t('novelWorkbench.setupDoc_bible') }}
              </button>
              <DqCollapse v-model="setupOpen" class="novel-wb__tree-sub">
                <DqCollapseItem name="world" :title="t('novelWorkbench.folderSetupWorld')">
                  <button
                    v-for="f in worldDocs"
                    :key="f.name"
                    type="button"
                    class="novel-wb__tree-item novel-wb__tree-item--nested"
                    :class="{ 'novel-wb__tree-item--on': treeSel.kind === 'setup' && treeSel.name === f.name }"
                    @click="selectSetupDoc(f.path || `${novelCanonDir(selectedBookId)}/${f.name}`, f.name)"
                  >
                    {{ setupDocTitle(f.name) }}
                  </button>
                  <p v-if="!worldDocs.length" class="novel-wb__tree-unit">{{ t('novelWorkbench.noCanonYet') }}</p>
                </DqCollapseItem>
                <DqCollapseItem name="cast" :title="t('novelWorkbench.folderSetupCast')">
                  <button
                    v-for="f in castDocs"
                    :key="f.name"
                    type="button"
                    class="novel-wb__tree-item novel-wb__tree-item--nested"
                    :class="{ 'novel-wb__tree-item--on': treeSel.kind === 'setup' && treeSel.name === f.name }"
                    @click="selectSetupDoc(f.path || `${novelCastDir(selectedBookId)}/${f.name}`, f.name)"
                  >
                    {{ setupDocTitle(f.name) }}
                  </button>
                </DqCollapseItem>
              </DqCollapse>
            </DqCollapseItem>

            <DqCollapseItem name="prose">
              <template #title>
                <span>{{ t('novelWorkbench.folderProse') }}</span>
                <span class="novel-wb__tree-meta">{{ treeChapters.length }}</span>
              </template>
              <button
                v-for="entry in treeChapters"
                :key="entry.chapter"
                type="button"
                class="novel-wb__tree-item"
                :class="{
                  'novel-wb__tree-item--on': treeSel.kind === 'chapter' && treeSel.n === entry.chapter,
                  'novel-wb__tree-item--dim': !entry.prose,
                }"
                @click="selectChapter(entry.chapter, entry.prose ? 'prose' : 'contract')"
              >
                <span>{{ chapterTreeName(entry) }}</span>
                <span v-if="!entry.prose && entry.contract" class="novel-wb__tree-meta">
                  {{ t('novelWorkbench.contractOnly') }}
                </span>
              </button>
              <p v-if="!treeChapters.length" class="novel-wb__tree-unit">{{ t('novelWorkbench.noChapters') }}</p>
            </DqCollapseItem>
          </DqCollapse>
        </aside>

        <div class="novel-wb__preview">
          <div class="novel-wb__preview-bar">
            <div class="novel-wb__preview-head">
              <div class="novel-wb__row-title">{{ deskCrumb }}</div>
              <div class="novel-wb__row-meta">{{ readTitle }}</div>
            </div>
          </div>

          <div v-if="treeSel.kind === 'book'" class="novel-wb__actions novel-wb__actions--desk">
            <button type="button" class="novel-wb__btn" @click="runAction('outline')">
              {{ bookOutlineFile ? t('novelWorkbench.actionReviseBookOutline') : t('novelWorkbench.actionOutline') }}
            </button>
            <button
              type="button"
              class="novel-wb__btn novel-wb__btn--ghost"
              @click="runAction('volume', undefined, undefined, nextVolume)"
            >
              {{ t('novelWorkbench.actionVolumeOutline', { n: nextVolume }) }}
            </button>
            <button type="button" class="novel-wb__btn novel-wb__btn--ghost" @click="runAction('assets')">
              {{ t('novelWorkbench.actionAssets') }}
            </button>
            <button
              v-if="deskBatchFreezeAllowed"
              type="button"
              class="novel-wb__btn novel-wb__btn--ghost"
              @click="runAction('batch-freeze')"
            >
              {{ t('novelWorkbench.actionBatchFreeze') }}
            </button>
          </div>

          <div v-else-if="treeSel.kind === 'volume'" class="novel-wb__actions novel-wb__actions--desk">
            <button type="button" class="novel-wb__btn" @click="runDeskVolume(selectedVolumeNum)">
              {{ t('novelWorkbench.actionReviseVolumeOutline', { n: selectedVolumeNum }) }}
            </button>
            <button type="button" class="novel-wb__btn novel-wb__btn--ghost" @click="runAction('outline')">
              {{ bookOutlineFile ? t('novelWorkbench.actionReviseBookOutline') : t('novelWorkbench.actionOutline') }}
            </button>
            <button
              v-if="deskBatchFreezeAllowed"
              type="button"
              class="novel-wb__btn novel-wb__btn--ghost"
              @click="runAction('batch-freeze')"
            >
              {{ t('novelWorkbench.actionBatchFreeze') }}
            </button>
          </div>

          <div v-else-if="treeSel.kind === 'setup'" class="novel-wb__actions novel-wb__actions--desk">
            <button type="button" class="novel-wb__btn" @click="runAction('assets')">
              {{ t('novelWorkbench.actionAssets') }}
            </button>
            <button
              v-if="setupShowsGoldfinger"
              type="button"
              class="novel-wb__btn novel-wb__btn--ghost"
              @click="runAction('goldfinger')"
            >
              {{ t('novelWorkbench.actionGoldfinger') }}
            </button>
            <button type="button" class="novel-wb__btn novel-wb__btn--ghost" @click="runAction('outline')">
              {{ bookOutlineFile ? t('novelWorkbench.actionReviseBookOutline') : t('novelWorkbench.actionOutline') }}
            </button>
          </div>

          <div v-if="treeSel.kind === 'chapter'" class="novel-wb__tabs" role="tablist">
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
            </button>
          </div>

          <div v-if="treeSel.kind === 'chapter'" class="novel-wb__actions">
            <button
              v-if="chapterDeskPrimary"
              type="button"
              class="novel-wb__btn"
              :disabled="!chapterDeskPrimary.allowed"
              @click="runChapterDeskPrimary()"
            >
              {{ chapterDeskPrimary.label }}
            </button>
            <template v-if="readingIsContract">
              <button
                v-if="!chapterDeskPrimary"
                type="button"
                class="novel-wb__btn novel-wb__btn--ghost"
                @click="runAction('contract', readingChapter || treeSel.n, readPath || undefined)"
              >
                {{ t('novelWorkbench.actionAskContract') }}
              </button>
            </template>
            <template v-else-if="readingIsProse && readingChapter">
              <button
                type="button"
                class="novel-wb__btn novel-wb__btn--ghost"
                :disabled="!isActionAllowed('write', readingChapter)"
                @click="runAction('write', readingChapter, readPath || undefined)"
              >
                {{ readingEntry?.prose ? t('novelWorkbench.actionAskRewrite') : t('novelWorkbench.actionAskWrite', { n: readingChapter }) }}
              </button>
              <template v-if="readingEntry?.prose">
                <button
                  type="button"
                  class="novel-wb__btn novel-wb__btn--ghost"
                  @click="runAction('dialogue', readingChapter, readPath || undefined)"
                >
                  {{ t('novelWorkbench.actionDialogue') }}
                </button>
                <button
                  type="button"
                  class="novel-wb__btn novel-wb__btn--ghost"
                  @click="runAction('hook', readingChapter, readPath || undefined)"
                >
                  {{ t('novelWorkbench.actionHook') }}
                </button>
                <button
                  type="button"
                  class="novel-wb__btn novel-wb__btn--ghost"
                  @click="runAction('reversal', readingChapter, readPath || undefined)"
                >
                  {{ t('novelWorkbench.actionReversal') }}
                </button>
              </template>
              <button
                v-if="chapterDeskPrimary?.action !== 'review'"
                type="button"
                class="novel-wb__btn novel-wb__btn--ghost"
                :disabled="!isActionAllowed('review', readingChapter)"
                @click="runAction('review', readingChapter, readPath || undefined)"
              >
                {{ t('novelWorkbench.actionReview') }}
              </button>
              <button
                v-if="chapterDeskPrimary?.action !== 'polish'"
                type="button"
                class="novel-wb__btn novel-wb__btn--ghost"
                :disabled="!isActionAllowed('polish', readingChapter)"
                @click="runAction('polish', readingChapter, readPath || undefined)"
              >
                {{ t('novelWorkbench.actionPolish') }}
              </button>
              <button
                v-if="chapterDeskPrimary?.action !== 'commit'"
                type="button"
                class="novel-wb__btn novel-wb__btn--ghost"
                :disabled="!isActionAllowed('commit', readingChapter)"
                @click="runAction('commit', readingChapter, readPath || undefined)"
              >
                {{ t('novelWorkbench.actionCommit') }}
              </button>
              <button
                v-if="readingEntry?.prose && chapterDeskPrimary?.action !== 'review-polish-commit'"
                type="button"
                class="novel-wb__btn novel-wb__btn--ghost"
                :disabled="!isActionAllowed('review-polish-commit', readingChapter)"
                @click="runAction('review-polish-commit', readingChapter, readPath || undefined)"
              >
                {{ t('novelWorkbench.actionReviewPolishCommit') }}
              </button>
            </template>
          </div>

          <div v-if="readLoading" class="novel-wb__empty">{{ t('novelWorkbench.loading') }}</div>
          <div v-else-if="treeSel.kind === 'book' && !bookOutlineFile" class="novel-wb__empty">
            {{ t('novelWorkbench.noBookOutline') }}
          </div>
          <div v-else-if="treeSel.kind === 'volume' && !readContent" class="novel-wb__empty">
            {{ t('novelWorkbench.volumeUnitsEmpty') }}
          </div>
          <div v-else-if="readingIsProse && !readingEntry?.prose" class="novel-wb__empty">
            {{ t('novelWorkbench.noProseYet') }}
          </div>
          <div v-else-if="readingIsContract && !readingEntry?.contract" class="novel-wb__empty">
            {{ t('novelWorkbench.noContractYet') }}
          </div>
          <div v-else class="novel-wb__reader novel-wb__reader--desk" v-html="readHtml" />
        </div>
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

.novel-wb__book {
  display: flex;
  flex: 1;
  min-height: 0;
  overflow: hidden;
}

.novel-wb__tree {
  flex: 0 0 220px;
  min-width: 188px;
  max-width: 260px;
  overflow: auto;
  border-right: 1px solid color-mix(in srgb, var(--dq-border-subtle, #000) 50%, transparent);
  padding: 6px 0 16px;
}

.novel-wb__tree :deep(.dq-collapse-item__header) {
  padding: 6px 12px;
  font-size: var(--dq-font-size-caption);
  font-weight: 650;
  border-radius: 6px;
}

.novel-wb__tree :deep(.dq-collapse-item__header:hover) {
  background: color-mix(in srgb, var(--dq-accent) 8%, transparent);
}

.novel-wb__tree :deep(.dq-collapse-item__title) {
  display: flex;
  align-items: baseline;
  justify-content: space-between;
  gap: 8px;
}

.novel-wb__tree-sub {
  margin-left: 4px;
}

.novel-wb__tree-sub :deep(.dq-collapse-item__header) {
  padding-left: 28px;
  font-weight: 600;
}

.novel-wb__tree-item--nested {
  padding-left: 40px;
}

.novel-wb__tree-item--dim {
  opacity: 0.72;
}

.novel-wb__tree-item {
  display: flex;
  align-items: baseline;
  justify-content: space-between;
  gap: 6px;
  width: 100%;
  margin: 0;
  padding: 6px 12px 6px 28px;
  border: none;
  background: transparent;
  color: inherit;
  font: inherit;
  font-size: var(--dq-font-size-caption);
  text-align: left;
  cursor: pointer;
}

.novel-wb__tree-item:hover {
  background: color-mix(in srgb, var(--dq-accent) 10%, transparent);
}

.novel-wb__tree-item--on {
  background: color-mix(in srgb, var(--dq-accent) 14%, transparent);
  font-weight: 650;
}

.novel-wb__tree-item--ghost {
  opacity: 0.65;
}

.novel-wb__tree-meta {
  opacity: 0.5;
  font-weight: 400;
}

.novel-wb__tree-unit {
  margin: 0;
  padding: 2px 12px 2px 20px;
  font-size: var(--dq-font-size-caption);
  opacity: 0.55;
  line-height: 1.35;
}

.novel-wb__preview {
  flex: 1;
  min-width: 0;
  display: flex;
  flex-direction: column;
  overflow: hidden;
}

.novel-wb__preview-bar {
  flex-shrink: 0;
  padding: 10px 12px 6px;
}

.novel-wb__preview-head {
  min-width: 0;
}

.novel-wb__actions--desk {
  padding-top: 0;
  padding-bottom: 8px;
  border-bottom: 1px solid color-mix(in srgb, var(--dq-border-subtle, #000) 40%, transparent);
}

.novel-wb__reader--desk {
  flex: 1;
  min-height: 0;
}

.novel-wb__stage {
  flex: 1;
  min-width: 0;
}

.novel-wb__inject {
  margin: 0;
  padding: 0 12px 8px;
  font-size: var(--dq-font-size-caption);
  opacity: 0.7;
}

.novel-wb__more {
  margin: 4px 12px 8px;
  font-size: var(--dq-font-size-caption);
}

.novel-wb__more summary {
  cursor: pointer;
  opacity: 0.7;
  font-weight: 650;
}

.novel-wb__row--active {
  background: color-mix(in srgb, var(--dq-accent) 12%, transparent);
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

.novel-wb__rail {
  flex-shrink: 0;
  border-bottom: 1px solid color-mix(in srgb, var(--dq-border-subtle, #000) 45%, transparent);
  padding-bottom: 8px;
}

.novel-wb__stepper--rail {
  flex-direction: row;
  flex-wrap: wrap;
  align-items: center;
  padding: 8px 12px 4px;
  gap: 4px;
}

.novel-wb__rail-meta {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 8px;
  padding: 4px 12px;
}

.novel-wb__gates {
  display: flex;
  flex-wrap: wrap;
  gap: 6px;
  font-size: var(--dq-font-size-caption);
}

.novel-wb__gate {
  padding: 2px 8px;
  border-radius: 4px;
  opacity: 0.75;
  background: color-mix(in srgb, var(--dq-border-subtle, #000) 25%, transparent);
}

.novel-wb__gate--pass {
  opacity: 1;
  color: var(--dq-success, #1a7f37);
}

.novel-wb__gate--fail {
  opacity: 1;
  color: var(--dq-danger, #cf222e);
}

.novel-wb__btn--cta {
  font-weight: 650;
}

.novel-wb__model-tip {
  margin: 0;
  padding: 0 12px 4px;
  font-size: var(--dq-font-size-caption);
  opacity: 0.65;
  line-height: 1.35;
}

.novel-wb__step {
  padding: 4px 8px;
  border: none;
  border-radius: 999px;
  font-size: var(--dq-font-size-caption);
  opacity: 0.45;
  background: color-mix(in srgb, var(--dq-border-subtle, #000) 30%, transparent);
  color: inherit;
  cursor: pointer;
}

.novel-wb__step--done {
  opacity: 0.7;
}

.novel-wb__step--btn {
  width: 100%;
  border: none;
  color: inherit;
  font: inherit;
  text-align: left;
  cursor: pointer;
}

.novel-wb__step--btn:hover {
  opacity: 0.85;
}

.novel-wb__step--current {
  opacity: 1;
  font-weight: 650;
  box-shadow: inset 0 0 0 1px color-mix(in srgb, var(--dq-accent) 45%, transparent);
}

.novel-wb__nav {
  display: block;
  width: calc(100% - 16px);
  margin: 0 8px 4px;
  padding: 6px 8px;
  border: none;
  border-radius: 6px;
  background: transparent;
  color: inherit;
  font: inherit;
  font-size: var(--dq-font-size-caption);
  text-align: left;
  cursor: pointer;
  opacity: 0.75;
}

.novel-wb__nav:hover {
  background: color-mix(in srgb, var(--dq-accent) 10%, transparent);
  opacity: 1;
}

.novel-wb__nav--on {
  background: color-mix(in srgb, var(--dq-accent) 12%, transparent);
  opacity: 1;
  font-weight: 650;
}

.novel-wb__cards {
  display: flex;
  flex-direction: column;
  gap: 8px;
  padding: 0 12px 8px;
}

.novel-wb__card {
  margin: 6px 12px 8px;
  padding: 10px;
  border-radius: 8px;
  background: color-mix(in srgb, var(--dq-border-subtle, #000) 18%, transparent);
}

.novel-wb__cards .novel-wb__card {
  margin: 0;
}

.novel-wb__card-head {
  display: flex;
  align-items: center;
  flex-wrap: wrap;
  gap: 6px;
}

.novel-wb__mini {
  list-style: none;
  margin: 8px 0 0;
  padding: 0;
}

.novel-wb__mini li {
  display: flex;
  gap: 8px;
  padding: 3px 0;
  font-size: var(--dq-font-size-caption);
  opacity: 0.85;
}

.novel-wb__mini strong {
  flex: 0 0 auto;
  font-weight: 650;
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
