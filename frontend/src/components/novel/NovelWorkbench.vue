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
  buildChapterPhases,
  buildConstrainedPrefill,
  canRunAction,
  chapterNumFromName,
  computeBookPipeline,
  inferChapterNextAction,
  isBookOutlineName,
  isNovelChapterPath,
  isNovelContractPath,
  mergeVolumeOutlineFiles,
  novelActionSkillId,
  novelCanonDir,
  novelCastDir,
  novelChapterFilePath,
  novelOutlineDir,
  nextVolumeNumber,
  parseContractYaml,
  setupDocLabel,
  volumeNumFromName,
  type NovelChapterEntry,
  type NovelChapterPhase,
  type NovelFileNode,
  type NovelStageAction,
} from '@/types/novel-workbench'

type View = 'shelf' | 'book'
type TreeSel = {
  kind: 'book' | 'volume' | 'setup' | 'chapter' | 'dossier'
  name?: string
  n?: number
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
const readPane = ref<'contract' | 'prose' | null>(null)
const readChapterNum = ref<number | null>(null)
const treeOpen = ref<string[]>(['outline', 'prose', 'dossier'])
const setupOpen = ref<string[]>(['world', 'cast'])
const treeSel = ref<TreeSel>({ kind: 'book' })

const {
  loading,
  books,
  selectedBookId,
  activeBookId,
  chapterEntries,
  continuityFiles,
  outlineFiles,
  volumeFiles,
  reviewFiles,
  canonFiles,
  castFiles,
  extendedState,
  contractRaws,
  reviewRaws,
  batchFreezeFrozen,
  bookState,
  bookOutlineRows,
  volumeUnitRows,
  readFile,
  loadShelf,
  openBook: loaderOpenBook,
  clearBook,
  nodePath,
  chapterNodePath,
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

const chapterPhases = computed(
  (): Record<number, NovelChapterPhase> => bookContext.value?.chapterPhases ?? {},
)

const treeChapters = computed(() =>
  chapterEntries.value.filter((e) => Boolean(e.contract || e.prose)),
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

const deskBatchFreezeAllowed = computed(() => {
  if (!bookContext.value) return false
  return canRunAction('batch-freeze', bookContext.value).allowed
})

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
    case 'goldfinger':
      return t('novelWorkbench.actionGoldfinger')
    default:
      return action
  }
}

function isActionAllowed(action: NovelStageAction, chapter?: number): boolean {
  if (!bookContext.value) return action === 'init'
  return canRunAction(action, bookContext.value, chapter).allowed
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

const focusPrimary = computed(() =>
  treeSel.value.kind === 'chapter' ? chapterDeskPrimary.value : deskPrimaryFromPipeline.value,
)

const moreActions = computed(() => {
  const ch = readingChapter.value
  if (ch == null || !readingIsProse.value || !readingEntry.value?.prose) return []
  const primary = chapterDeskPrimary.value?.action
  const items: DeskPrimary[] = []
  const push = (action: NovelStageAction, label: string) => {
    if (primary === action) return
    items.push({
      action,
      chapter: ch,
      label,
      allowed: isActionAllowed(action, ch),
    })
  }
  push('write', t('novelWorkbench.actionAskRewrite'))
  push('dialogue', t('novelWorkbench.actionDialogue'))
  push('hook', t('novelWorkbench.actionHook'))
  push('reversal', t('novelWorkbench.actionReversal'))
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
  return canRunAction(primary.action, bookContext.value, primary.chapter).blockers.map(blockerText)
})

const SETUP_DOC_KEYS = new Set(['bible', 'world', 'glossary', 'reveal', 'rules', 'platform', 'goldfinger'])

function setupDocTitle(name: string): string {
  const id = setupDocLabel(name)
  if (SETUP_DOC_KEYS.has(id)) return t(`novelWorkbench.setupDoc_${id}`)
  return id
}

function chapterTreeName(entry: NovelChapterEntry): string {
  const title = parseContractYaml(contractRaws.value[entry.chapter] || '').title.trim()
  if (title) return t('novelWorkbench.chapterTitle', { n: entry.chapter, title })
  return t('novelWorkbench.chapterN', { n: entry.chapter })
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
  if (treeSel.value.kind === 'chapter' && treeSel.value.n != null) {
    const e = chapterEntries.value.find((x) => x.chapter === treeSel.value.n)
    return `${t('novelWorkbench.folderProse')} / ${e ? chapterTreeName(e) : t('novelWorkbench.chapterN', { n: treeSel.value.n })}`
  }
  return ''
})

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

const currentVolumeUnits = computed(() => {
  if (treeSel.value.kind !== 'volume' || !treeSel.value.name) return []
  return volumeUnitRows.value[treeSel.value.name] ?? []
})

const currentContractRaw = computed(() => {
  const ch = readingChapter.value
  if (ch == null) return ''
  return contractRaws.value[ch] || ''
})

const currentChapterPhase = computed((): NovelChapterPhase | null => {
  const ch = readingChapter.value
  if (ch == null) return null
  return chapterPhases.value[ch] ?? null
})

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

async function selectSetupDoc(path: string, name: string) {
  treeSel.value = { kind: 'setup', name }
  readPane.value = null
  readChapterNum.value = null
  await openRead(path, name)
}

async function selectDossier(path: string, name: string) {
  const bookId = selectedBookId.value
  if (!bookId) return
  treeSel.value = { kind: 'dossier', name }
  readPane.value = null
  readChapterNum.value = null
  const full = path.includes('/') ? path : `novel/${bookId}/continuity/${name}`
  // Prefer actual node path from continuity/reviews lists
  const hit =
    continuityFiles.value.find((f) => f.name === name) ||
    reviewFiles.value.find((f) => f.name === name)
  await openRead(hit?.path || full, name)
}

function selectChapter(n: number, pane: 'contract' | 'prose' = 'prose') {
  treeSel.value = { kind: 'chapter', n }
  openChapterFromList(n, pane)
}

function goAdjacentChapter(dir: -1 | 1) {
  const entry = dir < 0 ? prevChapterEntry.value : nextChapterEntry.value
  if (!entry) return
  const kind =
    readPane.value === 'contract' || readPane.value === 'prose' ? readPane.value : 'prose'
  selectChapter(entry.chapter, kind)
}

async function openBook(bookId: string) {
  focusMode.value = false
  view.value = 'book'
  await loaderOpenBook(bookId)
  treeOpen.value = chapterEntries.value.some((e) => Boolean(e.prose))
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

function runAction(
  action: NovelStageAction,
  chapter?: number,
  chapterPath?: string,
  volume?: number,
) {
  const bookId = selectedBookId.value ?? undefined
  const ctx = bookContext.value
  const pipe = pipeline.value

  if (ctx && pipe && action !== 'init') {
    const decision = canRunAction(action, ctx, chapter)
    if (!decision.allowed) {
      toast.warning(
        decision.blockers.map(blockerText).join(' · ') || t('novelWorkbench.actionBlocked'),
      )
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

function onInspectorAction(
  action: NovelStageAction,
  chapter?: number,
  opts?: { volume?: number; chapterPath?: boolean },
) {
  const bookId = selectedBookId.value
  let chapterPath: string | undefined
  if (opts?.chapterPath && bookId && chapter != null) {
    chapterPath = readPath.value || novelChapterFilePath(bookId, chapter)
  }
  runAction(action, chapter, chapterPath, opts?.volume)
}

function runFocusPrimary() {
  const primary = focusPrimary.value
  if (!primary?.allowed) return
  onInspectorAction(primary.action, primary.chapter, {
    volume: primary.action === 'volume' ? nextVolume.value : undefined,
    chapterPath: treeSel.value.kind === 'chapter',
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
  if (pane === 'contract' || pane === 'prose') openChapterDoc(pane)
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
          :tree-chapters="treeChapters"
          :chapter-phases="chapterPhases"
          :contract-raws="contractRaws"
          :continuity-files="continuityFiles"
          :review-files="reviewFiles"
          :next-volume="nextVolume"
          @update:tree-open="treeOpen = $event"
          @update:setup-open="setupOpen = $event"
          @select-book-outline="selectBookOutline"
          @select-volume="selectVolume"
          @select-setup="selectSetupDoc"
          @select-chapter="selectChapter"
          @select-dossier="selectDossier"
          @add-volume="runAction('volume', undefined, undefined, nextVolume)"
        />

        <NovelReader
          :crumb="deskCrumb"
          :read-title="readTitle"
          :read-html="readHtml"
          :read-loading="readLoading"
          :read-content="readContent"
          :tree-kind="treeSel.kind"
          :reading-is-contract="readingIsContract"
          :reading-is-prose="readingIsProse"
          :reading-entry="readingEntry"
          :has-book-outline="Boolean(bookOutlineFile)"
          :book-outline-rows="bookOutlineRows"
          :volume-units="currentVolumeUnits"
          :prev-chapter="prevChapterEntry"
          :next-chapter="nextChapterEntry"
          @open-contract="openChapterDoc('contract')"
          @open-prose="openChapterDoc('prose')"
          @prev-chapter="goAdjacentChapter(-1)"
          @next-chapter="goAdjacentChapter(1)"
        />

        <NovelInspector
          v-show="!focusMode"
          :pipeline="pipeline"
          :tree-kind="treeSel.kind"
          :reading-is-contract="readingIsContract"
          :reading-is-prose="readingIsProse"
          :reading-chapter="readingChapter"
          :contract-raw="currentContractRaw"
          :chapter-phase="currentChapterPhase"
          :cast-docs="castDocs"
          :continuity-files="continuityFiles"
          :desk-primary="deskPrimaryFromPipeline"
          :chapter-primary="chapterDeskPrimary"
          :more-actions="moreActions"
          :blockers="inspectorBlockers"
          :setup-shows-goldfinger="setupShowsGoldfinger"
          :has-book-outline="Boolean(bookOutlineFile)"
          :desk-batch-freeze-allowed="deskBatchFreezeAllowed"
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
