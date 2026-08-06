<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { asArray, fetchJSON } from '@/api/client'
import { useSessionsStore } from '@/stores/sessions'
import { useWorkspaceUiStore } from '@/stores/workspaceUi'
import { toast } from '@/utils/feedback'
import { renderMarkdown } from '@/utils/markdown-render'
import {
  buildNovelStagePrefill,
  chapterNumFromName,
  isNovelChapterPath,
  nextChapterNumber,
  novelBiblePath,
  novelBookDir,
  novelCanonDir,
  novelChapterSummariesPath,
  novelChaptersDir,
  novelContinuityDir,
  novelOutlineDir,
  novelReviewsDir,
  novelStatePath,
  parseNovelStateYaml,
  sortChapterNodes,
  sortMdNodes,
  type NovelFileNode,
  type NovelStageAction,
  type NovelStateSummary,
} from '@/types/novel-workbench'

function nodePath(bookId: string, dir: string, node: NovelFileNode): string {
  const p = (node.path || '').replace(/\\/g, '/')
  if (p) return p
  return `${dir}/${node.name}`
}

type View = 'shelf' | 'book' | 'read'

const { t } = useI18n()
const sessions = useSessionsStore()
const workspaceUi = useWorkspaceUiStore()

const view = ref<View>('shelf')
const loading = ref(false)
const books = ref<{ id: string; path: string; state: NovelStateSummary | null }[]>([])
const selectedBookId = ref<string | null>(null)
const chapters = ref<NovelFileNode[]>([])
const continuityFiles = ref<NovelFileNode[]>([])
const outlineFiles = ref<NovelFileNode[]>([])
const reviewFiles = ref<NovelFileNode[]>([])
const canonFiles = ref<NovelFileNode[]>([])
const bookState = ref<NovelStateSummary | null>(null)
const readPath = ref<string | null>(null)
const readTitle = ref('')
const readContent = ref('')
const readLoading = ref(false)

const projectId = computed(() => sessions.selectedProjectId)

const selectedLead = computed(() =>
  sessions.agents.find((a) => a.id === sessions.selectedAgentId) ?? null,
)
const canDelegate = computed(() => Boolean(selectedLead.value?.canDelegate))
const hasNovelExpert = computed(() =>
  sessions.agents.some((a) => a.id === 'novel' && a.mode === 'subagent'),
)

const nextCh = computed(() =>
  nextChapterNumber(bookState.value?.lastCommittedCh ?? 0, chapters.value),
)

const readingChapter = computed(() => {
  if (!readPath.value || !isNovelChapterPath(readPath.value)) return null
  return chapterNumFromName(readTitle.value)
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
    return parseNovelStateYaml(raw)
  } catch {
    return null
  }
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

async function openBook(bookId: string) {
  selectedBookId.value = bookId
  view.value = 'book'
  loading.value = true
  try {
    bookState.value = await loadState(bookId)
    const [chNodes, contNodes, outNodes, revNodes, canonNodes] = await Promise.all([
      listDirSoft(novelChaptersDir(bookId)),
      listDirSoft(novelContinuityDir(bookId)),
      listDirSoft(novelOutlineDir(bookId)),
      listDirSoft(novelReviewsDir(bookId)),
      listDirSoft(novelCanonDir(bookId)),
    ])
    chapters.value = sortChapterNodes(chNodes)
    continuityFiles.value = sortMdNodes(contNodes)
    outlineFiles.value = sortMdNodes(outNodes)
    reviewFiles.value = sortMdNodes(revNodes)
    canonFiles.value = sortMdNodes(canonNodes)
  } catch {
    chapters.value = []
    continuityFiles.value = []
    outlineFiles.value = []
    reviewFiles.value = []
    canonFiles.value = []
    bookState.value = null
  } finally {
    loading.value = false
  }
}

async function openRead(path: string, title: string) {
  readPath.value = path
  readTitle.value = title
  readContent.value = ''
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

function backToShelf() {
  view.value = 'shelf'
  selectedBookId.value = null
  void loadShelf()
}

function backToBook() {
  view.value = 'book'
  readPath.value = null
  if (selectedBookId.value) void openBook(selectedBookId.value)
}

function runAction(action: NovelStageAction, chapter?: number, chapterPath?: string) {
  const bookId = selectedBookId.value ?? undefined
  let text = buildNovelStagePrefill(action, { bookId, chapter, chapterPath })
  if (!canDelegate.value) {
    text = `${t('novelWorkbench.needTeamHint')}\n\n${text}`
    toast.warning(t('composer.expertNeedDelegate'))
  } else if (hasNovelExpert.value) {
    workspaceUi.requestComposerSelectExperts(['novel'])
  }
  workspaceUi.prefillComposer(text)
}

async function onRefresh() {
  if (view.value === 'shelf') await loadShelf()
  else if (view.value === 'book' && selectedBookId.value) await openBook(selectedBookId.value)
  else if (view.value === 'read' && readPath.value) await openRead(readPath.value, readTitle.value)
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
            <template v-if="bookState.nextAction"> · {{ bookState.nextAction }}</template>
          </div>
        </div>

        <div class="novel-wb__group">
          <div class="novel-wb__group-label">{{ t('novelWorkbench.groupSetup') }}</div>
          <div class="novel-wb__actions novel-wb__actions--tight">
            <button type="button" class="novel-wb__btn novel-wb__btn--ghost" @click="runAction('outline')">
              {{ t('novelWorkbench.actionOutline') }}
            </button>
            <button type="button" class="novel-wb__btn novel-wb__btn--ghost" @click="runAction('assets')">
              {{ t('novelWorkbench.actionAssets') }}
            </button>
            <button type="button" class="novel-wb__btn novel-wb__btn--ghost" @click="runAction('goldfinger')">
              {{ t('novelWorkbench.actionGoldfinger') }}
            </button>
          </div>
        </div>

        <div class="novel-wb__group">
          <div class="novel-wb__group-label">{{ t('novelWorkbench.groupChapter') }}</div>
          <div class="novel-wb__actions novel-wb__actions--tight">
            <button type="button" class="novel-wb__btn" @click="runAction('continue')">
              {{ t('novelWorkbench.actionContinue') }}
            </button>
            <button
              type="button"
              class="novel-wb__btn novel-wb__btn--ghost"
              @click="runAction('contract', nextCh)"
            >
              {{ t('novelWorkbench.actionContract', { n: nextCh }) }}
            </button>
            <button
              type="button"
              class="novel-wb__btn novel-wb__btn--ghost"
              @click="runAction('write', nextCh)"
            >
              {{ t('novelWorkbench.actionWrite', { n: nextCh }) }}
            </button>
          </div>
        </div>

        <div class="novel-wb__group">
          <div class="novel-wb__group-label">{{ t('novelWorkbench.groupQuality') }}</div>
          <div class="novel-wb__actions novel-wb__actions--tight">
            <button
              type="button"
              class="novel-wb__btn novel-wb__btn--ghost"
              @click="runAction('commit')"
            >
              {{ t('novelWorkbench.actionCommit') }}
            </button>
          </div>
          <p class="novel-wb__hint novel-wb__hint--pad">{{ t('novelWorkbench.qualityHint') }}</p>
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
          <div v-if="canonFiles.length" class="novel-wb__quick">
            <button
              v-for="f in canonFiles"
              :key="nodePath(selectedBookId, novelCanonDir(selectedBookId), f)"
              type="button"
              class="novel-wb__chip"
              @click="openRead(nodePath(selectedBookId, novelCanonDir(selectedBookId), f), f.name)"
            >
              canon/{{ f.name }}
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
          <ul v-else-if="chapters.length" class="novel-wb__list novel-wb__list--embedded">
            <li v-for="ch in chapters" :key="nodePath(selectedBookId, novelChaptersDir(selectedBookId), ch)">
              <button
                type="button"
                class="novel-wb__row"
                @click="openRead(nodePath(selectedBookId, novelChaptersDir(selectedBookId), ch), ch.name)"
              >
                <span class="novel-wb__row-title">{{ ch.name }}</span>
                <span class="novel-wb__row-meta">{{
                  nodePath(selectedBookId, novelChaptersDir(selectedBookId), ch)
                }}</span>
              </button>
            </li>
          </ul>
          <div v-else class="novel-wb__empty">{{ t('novelWorkbench.noChapters') }}</div>
        </div>
      </div>
    </template>

    <template v-else-if="view === 'read'">
      <div class="novel-wb__actions">
        <template v-if="readingChapter != null && readPath">
          <button
            type="button"
            class="novel-wb__btn novel-wb__btn--ghost"
            @click="runAction('contract', readingChapter, readPath)"
          >
            {{ t('novelWorkbench.actionContract', { n: readingChapter }) }}
          </button>
          <button
            type="button"
            class="novel-wb__btn novel-wb__btn--ghost"
            @click="runAction('review', readingChapter, readPath)"
          >
            {{ t('novelWorkbench.actionReview') }}
          </button>
          <button
            type="button"
            class="novel-wb__btn novel-wb__btn--ghost"
            @click="runAction('polish', readingChapter, readPath)"
          >
            {{ t('novelWorkbench.actionPolish') }}
          </button>
          <button
            type="button"
            class="novel-wb__btn"
            @click="runAction('commit', readingChapter, readPath)"
          >
            {{ t('novelWorkbench.actionCommit') }}
          </button>
        </template>
        <button
          v-if="selectedBookId"
          type="button"
          class="novel-wb__btn novel-wb__btn--ghost"
          @click="runAction('continue')"
        >
          {{ t('novelWorkbench.actionContinue') }}
        </button>
      </div>
      <div v-if="readLoading" class="novel-wb__empty">{{ t('novelWorkbench.loading') }}</div>
      <div v-else class="novel-wb__reader" v-html="readHtml" />
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
