<script setup lang="ts">
import { nextTick, onBeforeUnmount, ref, watch } from 'vue'
import { useEditor, EditorContent } from '@tiptap/vue-3'
import StarterKit from '@tiptap/starter-kit'
import { Placeholder } from '@tiptap/extensions'
import { Markdown } from '@tiptap/markdown'
import { fetchJSON } from '@/api/client'
import { editorToMarkdown, selectionToMarkdown } from '@/utils/tiptap-markdown'
import { toast } from '@/utils/feedback'
import { useI18n } from 'vue-i18n'
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
  scope: [value: OfficeEditScope]
}>()

const { t } = useI18n()
const rootRef = ref<HTMLElement | null>(null)
const loading = ref(false)
const saving = ref(false)
const sourceMarkdown = ref('')
const dirty = ref(false)
const scrollTop = ref(0)

function publishScope(empty: boolean) {
  emit('scope', empty ? 'document' : 'selection')
}

const editor = useEditor({
  extensions: [
    StarterKit.configure({
      link: { openOnClick: false },
    }),
    Placeholder.configure({ placeholder: '开始编写…' }),
    Markdown,
  ],
  content: '',
  contentType: 'markdown',
  editable: props.mode === 'edit',
  onUpdate: () => {
    dirty.value = true
    emit('dirty', true)
  },
  onSelectionUpdate: ({ editor: ed }) => {
    publishScope(ed.state.selection.empty)
  },
})

watch(
  () => props.mode,
  (mode) => {
    editor.value?.setEditable(mode === 'edit' && !props.turnRunning)
  },
)

watch(
  () => props.turnRunning,
  (running) => {
    editor.value?.setEditable(props.mode === 'edit' && !running)
  },
)

async function load(opts?: { resetScroll?: boolean }) {
  if (!props.projectId || !props.path) return
  if (opts?.resetScroll) scrollTop.value = 0
  else if (rootRef.value) scrollTop.value = rootRef.value.scrollTop
  loading.value = true
  try {
    const fc = await fetchJSON<{ content: string; binary?: boolean }>(
      `/projects/${props.projectId}/files/content?path=${encodeURIComponent(props.path)}`,
    )
    if (fc.binary) throw new Error('binary file')
    sourceMarkdown.value = fc.content || ''
    editor.value?.commands.setContent(sourceMarkdown.value, {
      contentType: 'markdown',
      emitUpdate: false,
    })
    dirty.value = false
    emit('dirty', false)
    publishScope(editor.value?.state.selection.empty ?? true)
    await nextTick()
    if (rootRef.value) rootRef.value.scrollTop = scrollTop.value
  } catch (e) {
    toast.error(e instanceof Error ? e.message : t('office.loadFailed'))
  } finally {
    loading.value = false
  }
}

async function save(opts?: { quiet?: boolean }) {
  if (!editor.value || !props.projectId) return
  saving.value = true
  try {
    const md = editorToMarkdown(editor.value)
    await fetchJSON(`/projects/${props.projectId}/files/content`, {
      method: 'PUT',
      body: JSON.stringify({ path: props.path, content: md }),
    })
    sourceMarkdown.value = md
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

function hasTextSelection(): boolean {
  const ed = editor.value
  if (!ed) return false
  return !ed.state.selection.empty
}

function getEditScope(): OfficeEditScope {
  return hasTextSelection() ? 'selection' : 'document'
}

function getSelectionMarkdown(): string {
  const ed = editor.value
  if (!ed) return sourceMarkdown.value
  return selectionToMarkdown(ed)
}

watch(
  () => [props.projectId, props.path] as const,
  () => {
    void load({ resetScroll: true })
  },
  { immediate: true },
)

watch(
  () => props.reloadToken,
  () => {
    void load({ resetScroll: false })
  },
)

onBeforeUnmount(() => {
  editor.value?.destroy()
})

defineExpose({ save, getSelectionMarkdown, getEditScope, dirty, saving, loading })
</script>

<template>
  <div ref="rootRef" class="doc-surface" :class="{ 'is-readonly': mode !== 'edit' || turnRunning }">
    <div v-if="loading" class="doc-surface__status">{{ t('office.loading') }}</div>
    <EditorContent v-else :editor="editor" class="doc-surface__editor dq-prose" />
  </div>
</template>

<style scoped>
.doc-surface {
  flex: 1;
  min-height: 0;
  overflow: auto;
  padding: 16px 20px 48px;
  background: var(--dq-bg-base);
  color: var(--dq-label-primary);
}
.doc-surface__status {
  padding: 24px;
  color: var(--dq-label-tertiary);
  font-size: 13px;
}
.doc-surface__editor :deep(.tiptap) {
  outline: none;
  min-height: 240px;
  max-width: 720px;
  margin: 0 auto;
  font-size: 15px;
  line-height: 1.65;
  color: var(--dq-label-primary);
}
.doc-surface__editor :deep(.tiptap p.is-editor-empty:first-child::before) {
  color: var(--dq-label-quaternary);
  content: attr(data-placeholder);
  float: left;
  height: 0;
  pointer-events: none;
}
.doc-surface.is-readonly :deep(.tiptap) {
  caret-color: transparent;
}
</style>
