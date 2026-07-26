<script setup lang="ts">
import { computed, nextTick, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { fetchJSON } from '@/api/client'
import { apiBaseUrl } from '@/utils/desktop'
import { toast } from '@/utils/feedback'
import type { OfficeEditScope } from '@/utils/office-route'
import { useWorkspaceUiStore } from '@/stores/workspaceUi'
import {
  hashSlidesSource,
  isPlayableHtmlStale,
  joinSlidePages,
  renderPlayableSlidesHtml,
  siblingPlayableHtmlPath,
  siblingSlidesMarkdownPath,
  splitSlidesForEditor,
} from '@/utils/slides-render'

const props = defineProps<{
  projectId: string
  path: string
  mode: 'view' | 'edit' | 'present'
  reloadToken: number
  turnRunning?: boolean
}>()

const emit = defineEmits<{
  dirty: [value: boolean]
  saved: []
  updatePageIndex: [index: number]
}>()

const { t } = useI18n()
const workspaceUi = useWorkspaceUiStore()
const loading = ref(false)
const saving = ref(false)
const syncing = ref(false)
const source = ref('')
const frontmatterRaw = ref('')
const pages = ref<string[]>([])
const pageIndex = ref(0)
const dirty = ref(false)
const editorRef = ref<HTMLTextAreaElement | null>(null)
const thumbsRef = ref<HTMLElement | null>(null)
const editorScrollTop = ref(0)
const thumbsScrollTop = ref(0)

const isHtmlPresent = computed(() => /\.html?$/i.test(props.path))
const presentUrl = computed(() => {
  if (!props.projectId || !isHtmlPresent.value) return ''
  return `${apiBaseUrl()}/api/v1/projects/${props.projectId}/raw/${encodeURIComponent(props.path)}`
})

function currentMarkdown(): string {
  return joinSlidePages(pages.value, frontmatterRaw.value)
}

function captureScroll() {
  if (editorRef.value) editorScrollTop.value = editorRef.value.scrollTop
  if (thumbsRef.value) thumbsScrollTop.value = thumbsRef.value.scrollTop
}

async function restoreScroll() {
  await nextTick()
  if (editorRef.value) editorRef.value.scrollTop = editorScrollTop.value
  if (thumbsRef.value) thumbsRef.value.scrollTop = thumbsScrollTop.value
}

async function readFileContent(path: string): Promise<string | null> {
  try {
    const fc = await fetchJSON<{ content: string }>(
      `/projects/${props.projectId}/files/content?path=${encodeURIComponent(path)}`,
    )
    return fc.content ?? ''
  } catch {
    return null
  }
}

/** Deterministic Stage sync: rewrite sibling HTML only when md hash differs. */
async function syncPlayableHtml(md: string, opts?: { force?: boolean }): Promise<string> {
  const mdPath =
    /\.md$|\.markdown$/i.test(props.path) ? props.path : siblingSlidesMarkdownPath(props.path)
  const htmlPath = siblingPlayableHtmlPath(mdPath)

  const hash = await hashSlidesSource(md)
  if (!opts?.force) {
    const existing = await readFileContent(htmlPath)
    if (!isPlayableHtmlStale(existing, hash)) return htmlPath
  }
  const html = renderPlayableSlidesHtml(md, hash)
  await fetchJSON(`/projects/${props.projectId}/files/content`, {
    method: 'PUT',
    body: JSON.stringify({ path: htmlPath, content: html }),
  })
  return htmlPath
}

async function load(opts?: { resetPage?: boolean; syncHtml?: boolean }) {
  if (!props.projectId || !props.path) return
  captureScroll()
  const keepPage = opts?.resetPage ? 0 : pageIndex.value
  loading.value = true
  try {
    if (isHtmlPresent.value) {
      source.value = ''
      frontmatterRaw.value = ''
      pages.value = []
      pageIndex.value = 0
      emit('updatePageIndex', 0)
      return
    }
    const fc = await fetchJSON<{ content: string }>(
      `/projects/${props.projectId}/files/content?path=${encodeURIComponent(props.path)}`,
    )
    source.value = fc.content || ''
    const parsed = splitSlidesForEditor(source.value)
    frontmatterRaw.value = parsed.frontmatterRaw
    pages.value = parsed.pages
    pageIndex.value = Math.min(keepPage, Math.max(0, pages.value.length - 1))
    dirty.value = false
    emit('dirty', false)
    emit('updatePageIndex', pageIndex.value)
    await restoreScroll()
    if (opts?.syncHtml) {
      void syncPlayableHtml(source.value).catch(() => {
        /* silent background sync after AI reload */
      })
    }
  } catch (e) {
    toast.error(e instanceof Error ? e.message : t('office.loadFailed'))
  } finally {
    loading.value = false
  }
}

async function save(opts?: { quiet?: boolean }) {
  if (isHtmlPresent.value || !props.projectId) return
  saving.value = true
  try {
    const md = currentMarkdown()
    await fetchJSON(`/projects/${props.projectId}/files/content`, {
      method: 'PUT',
      body: JSON.stringify({ path: props.path, content: md }),
    })
    source.value = md
    dirty.value = false
    emit('dirty', false)
    emit('saved')
    try {
      await syncPlayableHtml(md, { force: true })
    } catch (syncErr) {
      toast.error(syncErr instanceof Error ? syncErr.message : t('office.slidesSyncFailed'))
    }
    if (!opts?.quiet) toast.success(t('office.saved'))
  } catch (e) {
    toast.error(e instanceof Error ? e.message : t('office.saveFailed'))
    throw e
  } finally {
    saving.value = false
  }
}

/** Ensure sibling HTML matches current md, then open it in Present mode. */
async function presentFromMarkdown(): Promise<boolean> {
  if (!props.projectId || isHtmlPresent.value) {
    workspaceUi.setStageMode('present')
    return true
  }
  syncing.value = true
  try {
    const md = dirty.value ? currentMarkdown() : source.value || currentMarkdown()
    if (dirty.value) {
      await fetchJSON(`/projects/${props.projectId}/files/content`, {
        method: 'PUT',
        body: JSON.stringify({ path: props.path, content: md }),
      })
      source.value = md
      dirty.value = false
      emit('dirty', false)
    }
    const htmlPath = await syncPlayableHtml(md)
    workspaceUi.setStagePath(htmlPath, 'present')
    return true
  } catch (e) {
    toast.error(e instanceof Error ? e.message : t('office.slidesSyncFailed'))
    return false
  } finally {
    syncing.value = false
  }
}

function selectPage(i: number) {
  pageIndex.value = i
  emit('updatePageIndex', i)
  editorScrollTop.value = 0
  void nextTick(() => {
    if (editorRef.value) editorRef.value.scrollTop = 0
  })
}

function onPageEdit(e: Event) {
  const el = e.target as HTMLTextAreaElement
  const next = [...pages.value]
  next[pageIndex.value] = el.value
  pages.value = next
  dirty.value = true
  emit('dirty', true)
}

function getEditScope(): OfficeEditScope {
  return 'slide'
}

function getSelectionMarkdown(): string {
  if (isHtmlPresent.value) return ''
  return pages.value[pageIndex.value] || ''
}

watch(
  () => [props.projectId, props.path] as const,
  () => load({ resetPage: true }),
  { immediate: true },
)

watch(
  () => props.reloadToken,
  () => load({ resetPage: false, syncHtml: !isHtmlPresent.value }),
)

defineExpose({
  save,
  presentFromMarkdown,
  getSelectionMarkdown,
  getEditScope,
  dirty,
  saving,
  loading,
  syncing,
  pageIndex,
})
</script>

<template>
  <div class="slides-surface">
    <div v-if="loading || syncing" class="slides-surface__status">
      {{ syncing ? t('office.slidesSyncing') : t('office.loading') }}
    </div>

    <iframe
      v-else-if="mode === 'present' && isHtmlPresent"
      class="slides-surface__frame"
      :src="presentUrl"
      title="slides"
    />

    <div v-else-if="!isHtmlPresent" class="slides-surface__edit">
      <aside ref="thumbsRef" class="slides-surface__thumbs">
        <button
          v-for="(p, i) in pages"
          :key="i"
          class="slides-surface__thumb"
          :class="{ 'is-active': i === pageIndex }"
          @click="selectPage(i)"
        >
          <span class="slides-surface__thumb-num">{{ i + 1 }}</span>
          <span class="slides-surface__thumb-title">{{ p.split('\n')[0]?.replace(/^#+\s*/, '') || t('office.emptySlide') }}</span>
        </button>
      </aside>
      <textarea
        ref="editorRef"
        class="slides-surface__editor"
        :value="pages[pageIndex] || ''"
        :readonly="mode === 'view' || turnRunning"
        :placeholder="t('office.slidePlaceholder')"
        @input="onPageEdit"
      />
    </div>

    <div v-else class="slides-surface__status">{{ t('office.slidesHtmlHint') }}</div>
  </div>
</template>

<style scoped>
.slides-surface {
  flex: 1;
  min-height: 0;
  display: flex;
  flex-direction: column;
  background: var(--dq-bg-base);
  color: var(--dq-label-primary);
}
.slides-surface__status {
  padding: 24px;
  font-size: 13px;
  color: var(--dq-label-tertiary);
}
.slides-surface__frame {
  flex: 1;
  border: 0;
  width: 100%;
  height: 100%;
  background: var(--dq-bg-page);
}
.slides-surface__edit {
  flex: 1;
  min-height: 0;
  display: grid;
  grid-template-columns: 180px 1fr;
}
.slides-surface__thumbs {
  border-right: 1px solid var(--dq-separator-light);
  background: color-mix(in srgb, var(--dq-bg-elevated) 40%, transparent);
  overflow: auto;
  padding: 8px;
  display: flex;
  flex-direction: column;
  gap: 6px;
}
.slides-surface__thumb {
  text-align: left;
  border: 1px solid var(--dq-border);
  border-radius: 6px;
  padding: 8px;
  background: var(--dq-fill-tertiary);
  color: var(--dq-label-primary);
  cursor: pointer;
  display: flex;
  flex-direction: column;
  gap: 4px;
}
.slides-surface__thumb:hover {
  background: color-mix(in srgb, var(--dq-label-primary) 8%, var(--dq-fill-tertiary));
}
.slides-surface__thumb.is-active {
  border-color: var(--dq-accent);
  background: color-mix(in srgb, var(--dq-accent) 14%, var(--dq-fill-tertiary));
  box-shadow: inset 2px 0 0 var(--dq-accent);
}
.slides-surface__thumb-num {
  font-size: 11px;
  color: var(--dq-label-tertiary);
}
.slides-surface__thumb-title {
  font-size: 12px;
  color: var(--dq-label-primary);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.slides-surface__editor {
  border: 0;
  resize: none;
  padding: 16px 20px;
  font-family: var(--dq-font-mono, ui-monospace, SFMono-Regular, Menlo, monospace);
  font-size: 13px;
  line-height: 1.55;
  outline: none;
  background: transparent;
  color: var(--dq-label-primary);
}
.slides-surface__editor::placeholder {
  color: var(--dq-label-quaternary);
}
</style>
