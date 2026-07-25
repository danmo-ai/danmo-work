<script setup lang="ts">
import { onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { useEditor, EditorContent } from '@tiptap/vue-3'
import StarterKit from '@tiptap/starter-kit'
import Link from '@tiptap/extension-link'
import Placeholder from '@tiptap/extension-placeholder'
import { fetchJSON } from '@/api/client'
import { htmlToMarkdown, markdownToEditorHTML } from '@/utils/md-bridge'
import { toast } from '@/utils/feedback'
import { useI18n } from 'vue-i18n'

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
}>()

const { t } = useI18n()
const loading = ref(false)
const saving = ref(false)
const sourceMarkdown = ref('')
const dirty = ref(false)

const editor = useEditor({
  extensions: [
    StarterKit,
    Link.configure({ openOnClick: false }),
    Placeholder.configure({ placeholder: '开始编写…' }),
  ],
  content: '',
  editable: props.mode === 'edit',
  onUpdate: () => {
    dirty.value = true
    emit('dirty', true)
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

async function load() {
  if (!props.projectId || !props.path) return
  loading.value = true
  try {
    const fc = await fetchJSON<{ content: string; binary?: boolean }>(
      `/projects/${props.projectId}/files/content?path=${encodeURIComponent(props.path)}`,
    )
    if (fc.binary) throw new Error('binary file')
    sourceMarkdown.value = fc.content || ''
    editor.value?.commands.setContent(markdownToEditorHTML(sourceMarkdown.value), false)
    dirty.value = false
    emit('dirty', false)
  } catch (e) {
    toast.error(e instanceof Error ? e.message : t('office.loadFailed'))
  } finally {
    loading.value = false
  }
}

async function save() {
  if (!editor.value || !props.projectId) return
  saving.value = true
  try {
    const md = htmlToMarkdown(editor.value.getHTML())
    await fetchJSON(`/projects/${props.projectId}/files/content`, {
      method: 'PUT',
      body: JSON.stringify({ path: props.path, content: md }),
    })
    sourceMarkdown.value = md
    dirty.value = false
    emit('dirty', false)
    emit('saved')
    toast.success(t('office.saved'))
  } catch (e) {
    toast.error(e instanceof Error ? e.message : t('office.saveFailed'))
  } finally {
    saving.value = false
  }
}

function getSelectionMarkdown(): string {
  const ed = editor.value
  if (!ed) return sourceMarkdown.value
  const { from, to, empty } = ed.state.selection
  if (empty) return htmlToMarkdown(ed.getHTML())
  const text = ed.state.doc.textBetween(from, to, '\n\n')
  if (text.trim()) return text.trimEnd() + '\n'
  return htmlToMarkdown(ed.getHTML())
}

watch(
  () => [props.projectId, props.path, props.reloadToken] as const,
  () => {
    load()
  },
  { immediate: true },
)

onMounted(() => {
  /* editor created by useEditor */
})

onBeforeUnmount(() => {
  editor.value?.destroy()
})

defineExpose({ save, getSelectionMarkdown, dirty, saving, loading })
</script>

<template>
  <div class="doc-surface" :class="{ 'is-readonly': mode !== 'edit' || turnRunning }">
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
  background: var(--dq-bg, #fff);
}
.doc-surface__status {
  padding: 24px;
  color: var(--dq-text-muted, #6b7280);
  font-size: 13px;
}
.doc-surface__editor :deep(.tiptap) {
  outline: none;
  min-height: 240px;
  max-width: 720px;
  margin: 0 auto;
  font-size: 15px;
  line-height: 1.65;
}
.doc-surface__editor :deep(.tiptap p.is-editor-empty:first-child::before) {
  color: #9ca3af;
  content: attr(data-placeholder);
  float: left;
  height: 0;
  pointer-events: none;
}
.doc-surface.is-readonly :deep(.tiptap) {
  caret-color: transparent;
}
</style>
