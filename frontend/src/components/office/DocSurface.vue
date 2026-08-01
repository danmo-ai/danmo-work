<script setup lang="ts">
import { computed, nextTick, ref, watch } from 'vue'
import { fetchJSON } from '@/api/client'
import { toast } from '@/utils/feedback'
import { useI18n } from 'vue-i18n'
import type { OfficeEditScope } from '@/utils/office-route'
import MarkdownRichEditor from '@/components/office/MarkdownRichEditor.vue'

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
  aiEdit: [
    payload: {
      action: 'polish' | 'continue' | 'modify'
      instruction: string
      selection: string
      startLine?: number
      endLine?: number
    },
  ]
}>()

const { t } = useI18n()
const rootRef = ref<HTMLElement | null>(null)
const editorRef = ref<InstanceType<typeof MarkdownRichEditor> | null>(null)
const loading = ref(false)
const saving = ref(false)
const sourceMarkdown = ref('')
const dirty = ref(false)
const scrollTop = ref(0)

const editable = computed(() => props.mode === 'edit' && !props.turnRunning)
/** Selection AI in edit (CM) and view (rendered preview DOM). */
const selectionAiEnabled = computed(() => !props.turnRunning && props.mode !== 'present')

function publishScope(empty: boolean) {
  emit('scope', empty ? 'document' : 'selection')
}

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
    await nextTick()
    editorRef.value?.setContent(sourceMarkdown.value, { emitUpdate: false })
    dirty.value = false
    emit('dirty', false)
    publishScope(editorRef.value?.isSelectionEmpty() ?? true)
    await nextTick()
    const scrollEl =
      (rootRef.value?.querySelector('.cm-scroller') as HTMLElement | null) ||
      (rootRef.value?.querySelector('.md-editor-content') as HTMLElement | null)
    if (scrollEl) scrollEl.scrollTop = scrollTop.value
  } catch (e) {
    toast.error(e instanceof Error ? e.message : t('office.loadFailed'))
  } finally {
    loading.value = false
  }
}

async function save(opts?: { quiet?: boolean }) {
  if (!editorRef.value || !props.projectId) return
  saving.value = true
  try {
    const md = editorRef.value.getMarkdown()
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

function onEditorUpdate() {
  dirty.value = true
  emit('dirty', true)
}

function hasTextSelection(): boolean {
  return !(editorRef.value?.isSelectionEmpty() ?? true)
}

function getEditScope(): OfficeEditScope {
  return hasTextSelection() ? 'selection' : 'document'
}

function getSelectionMarkdown(): string {
  return editorRef.value?.getSelectionMarkdown() || sourceMarkdown.value
}

function getSelectionLines(): { startLine: number; endLine: number } | null {
  return editorRef.value?.getSelectionLines() ?? null
}

function getMarkdown(): string {
  return editorRef.value?.getMarkdown() || sourceMarkdown.value
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

defineExpose({
  save,
  getMarkdown,
  getSelectionMarkdown,
  getSelectionLines,
  getEditScope,
  dirty,
  saving,
  loading,
})
</script>

<template>
  <div ref="rootRef" class="doc-surface" :class="{ 'is-readonly': !editable }">
    <div v-if="loading" class="doc-surface__status">{{ t('office.loading') }}</div>
    <MarkdownRichEditor
      v-show="!loading"
      ref="editorRef"
      :editable="editable"
      :show-toolbar="editable"
      :show-toc="true"
      :enable-selection-ai="selectionAiEnabled"
      :placeholder="t('office.docPlaceholder')"
      @update="onEditorUpdate"
      @selection-empty="publishScope"
      @ai-edit="emit('aiEdit', $event)"
    />
  </div>
</template>

<style scoped>
.doc-surface {
  flex: 1;
  min-height: 0;
  display: flex;
  flex-direction: column;
  background: var(--dq-bg-base);
  color: var(--dq-label-primary);
}
.doc-surface__status {
  padding: 24px;
  color: var(--dq-label-tertiary);
  font-size: var(--dq-font-size-body);
}
</style>
