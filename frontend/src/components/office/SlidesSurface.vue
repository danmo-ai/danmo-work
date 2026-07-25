<script setup lang="ts">
import { computed, nextTick, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { fetchJSON } from '@/api/client'
import { apiBaseUrl } from '@/utils/desktop'
import { toast } from '@/utils/feedback'
import type { OfficeEditScope } from '@/utils/office-route'

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
const loading = ref(false)
const saving = ref(false)
const source = ref('')
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

function splitSlides(md: string): string[] {
  const trimmed = md.replace(/^\uFEFF/, '')
  // Strip YAML frontmatter
  let body = trimmed
  if (body.startsWith('---')) {
    const end = body.indexOf('\n---', 3)
    if (end !== -1) body = body.slice(end + 4).replace(/^\r?\n/, '')
  }
  const parts = body.split(/^\s*---\s*$/m).map((p) => p.trim()).filter(Boolean)
  return parts.length ? parts : [body.trim() || '']
}

function joinSlides(parts: string[]): string {
  return parts.map((p) => p.trimEnd()).join('\n\n---\n\n') + '\n'
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

async function load(opts?: { resetPage?: boolean }) {
  if (!props.projectId || !props.path) return
  captureScroll()
  const keepPage = opts?.resetPage ? 0 : pageIndex.value
  loading.value = true
  try {
    if (isHtmlPresent.value) {
      source.value = ''
      pages.value = []
      pageIndex.value = 0
      emit('updatePageIndex', 0)
      return
    }
    const fc = await fetchJSON<{ content: string }>(
      `/projects/${props.projectId}/files/content?path=${encodeURIComponent(props.path)}`,
    )
    source.value = fc.content || ''
    pages.value = splitSlides(source.value)
    pageIndex.value = Math.min(keepPage, Math.max(0, pages.value.length - 1))
    dirty.value = false
    emit('dirty', false)
    emit('updatePageIndex', pageIndex.value)
    await restoreScroll()
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
    const md = joinSlides(pages.value)
    await fetchJSON(`/projects/${props.projectId}/files/content`, {
      method: 'PUT',
      body: JSON.stringify({ path: props.path, content: md }),
    })
    source.value = md
    dirty.value = false
    emit('dirty', false)
    emit('saved')
    if (!opts?.quiet) toast.success(t('office.saved'))
  } catch (e) {
    toast.error(e instanceof Error ? e.message : t('office.saveFailed'))
    throw e
  } finally {
    saving.value = false
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
  () => load({ resetPage: false }),
)

defineExpose({ save, getSelectionMarkdown, getEditScope, dirty, saving, loading, pageIndex })
</script>

<template>
  <div class="slides-surface">
    <div v-if="loading" class="slides-surface__status">{{ t('office.loading') }}</div>

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
  background: var(--dq-bg, #fff);
}
.slides-surface__status {
  padding: 24px;
  font-size: 13px;
  color: var(--dq-text-muted, #6b7280);
}
.slides-surface__frame {
  flex: 1;
  border: 0;
  width: 100%;
  height: 100%;
  background: #111;
}
.slides-surface__edit {
  flex: 1;
  min-height: 0;
  display: grid;
  grid-template-columns: 180px 1fr;
}
.slides-surface__thumbs {
  border-right: 1px solid var(--dq-border, #e5e7eb);
  overflow: auto;
  padding: 8px;
  display: flex;
  flex-direction: column;
  gap: 6px;
}
.slides-surface__thumb {
  text-align: left;
  border: 1px solid var(--dq-border, #e5e7eb);
  border-radius: 6px;
  padding: 8px;
  background: var(--dq-bg-subtle, #f9fafb);
  cursor: pointer;
  display: flex;
  flex-direction: column;
  gap: 4px;
}
.slides-surface__thumb.is-active {
  border-color: var(--dq-accent, #2563eb);
  background: #eff6ff;
}
.slides-surface__thumb-num {
  font-size: 11px;
  color: #6b7280;
}
.slides-surface__thumb-title {
  font-size: 12px;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.slides-surface__editor {
  border: 0;
  resize: none;
  padding: 16px 20px;
  font-family: ui-monospace, SFMono-Regular, Menlo, monospace;
  font-size: 13px;
  line-height: 1.55;
  outline: none;
  background: transparent;
  color: inherit;
}
</style>
