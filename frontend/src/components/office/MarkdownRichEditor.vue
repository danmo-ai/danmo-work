<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { MdEditor, type ExposeParam, type Themes } from 'md-editor-v3'
import 'md-editor-v3/lib/style.css'
import '@/styles/md-editor-overrides.css'
import { useI18n } from 'vue-i18n'
import { storeToRefs } from 'pinia'
import { useThemeStore, THEME_OPTIONS } from '@/stores/theme'

const props = withDefaults(
  defineProps<{
    content?: string
    editable?: boolean
    placeholder?: string
    /** Kept for API compat; md-editor always shows its toolbar when editable. */
    showToolbar?: boolean
    /** Show catalog outline when true. */
    showToc?: boolean
    /** Selection floating AI: polish / expand / modify (Office Doc). Works in edit + view. */
    enableSelectionAi?: boolean
  }>(),
  {
    content: '',
    editable: true,
    placeholder: '',
    showToolbar: true,
    showToc: false,
    enableSelectionAi: false,
  },
)

const emit = defineEmits<{
  update: []
  selectionEmpty: [empty: boolean]
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

const { t, locale } = useI18n()
const themeStore = useThemeStore()
const { currentTheme } = storeToRefs(themeStore)

const editorId = `md-doc-${Math.random().toString(36).slice(2, 10)}`
const rootRef = ref<HTMLElement | null>(null)
const mdRef = ref<ExposeParam>()
const text = ref(props.content || '')
const suppressUpdate = ref(false)

const aiAnnotateOpen = ref(false)
const aiInstruction = ref('')
const aiInputRef = ref<HTMLTextAreaElement | null>(null)
const aiSelectionMarkdown = ref('')
const aiSelectionLines = ref<{ startLine: number; endLine: number } | null>(null)
const floatPos = ref<{ top: number; left: number } | null>(null)
const hasSelection = ref(false)

const mdTheme = computed<Themes>(() => {
  const opt = THEME_OPTIONS.find((o) => o.id === currentTheme.value)
  return opt?.dark ? 'dark' : 'light'
})

const mdLanguage = computed(() => (String(locale.value).startsWith('zh') ? 'zh-CN' : 'en-US'))

const toolbarsExclude = computed(() => {
  const exclude: Array<
    | 'github'
    | 'save'
    | 'htmlPreview'
    | 'catalog'
  > = ['github', 'save', 'htmlPreview']
  if (!props.showToc) exclude.push('catalog')
  return exclude
})

function getMarkdown(): string {
  return text.value
}

function getPreviewEl(): HTMLElement | null {
  return rootRef.value?.querySelector('.md-editor-preview') as HTMLElement | null
}

/** DOM selection inside rendered preview (view / preview-only mode). */
function getDomSelectionInPreview(): { text: string; range: Range } | null {
  const preview = getPreviewEl()
  if (!preview) return null
  const sel = window.getSelection()
  if (!sel || sel.isCollapsed || sel.rangeCount === 0) return null
  const range = sel.getRangeAt(0)
  if (!preview.contains(range.commonAncestorContainer)) return null
  const text = sel.toString()
  if (!text.trim()) return null
  return { text, range }
}

function getSelectionMarkdown(): string {
  if (props.editable) {
    const selected = mdRef.value?.getSelectedText()
    if (selected) return selected
  } else {
    const dom = getDomSelectionInPreview()
    if (dom?.text) return dom.text
  }
  return aiSelectionMarkdown.value
}

function getSelectionLines(): { startLine: number; endLine: number } | null {
  if (!props.editable) return aiSelectionLines.value
  const view = mdRef.value?.getEditorView()
  if (!view) return aiSelectionLines.value
  const { from, to } = view.state.selection.main
  if (from === to) return null
  return {
    startLine: view.state.doc.lineAt(from).number,
    endLine: view.state.doc.lineAt(to).number,
  }
}

function isSelectionEmpty(): boolean {
  if (aiAnnotateOpen.value) return false
  if (props.editable) {
    const view = mdRef.value?.getEditorView()
    if (!view) return true
    return view.state.selection.main.empty
  }
  return !getDomSelectionInPreview()
}

function setContent(md: string, opts?: { emitUpdate?: boolean }) {
  const next = md || ''
  if (next === text.value) return
  suppressUpdate.value = !(opts?.emitUpdate ?? true)
  text.value = next
  void nextTick(() => {
    suppressUpdate.value = false
  })
}

function onChange(v: string) {
  text.value = v
  if (suppressUpdate.value) return
  emit('update')
  syncSelection()
}

function captureAiSelection(): string {
  if (props.editable) {
    const view = mdRef.value?.getEditorView()
    if (!view) return ''
    const { from, to, empty } = view.state.selection.main
    if (empty) return ''
    const md = view.state.sliceDoc(from, to)
    aiSelectionMarkdown.value = md
    aiSelectionLines.value = {
      startLine: view.state.doc.lineAt(from).number,
      endLine: view.state.doc.lineAt(to).number,
    }
    return md
  }
  // View mode: plain text from rendered preview (no reliable MD line map).
  const dom = getDomSelectionInPreview()
  if (!dom) return aiSelectionMarkdown.value
  aiSelectionMarkdown.value = dom.text
  aiSelectionLines.value = null
  return dom.text
}

function closeAiAnnotate() {
  aiAnnotateOpen.value = false
  aiInstruction.value = ''
  aiSelectionMarkdown.value = ''
  aiSelectionLines.value = null
}

function emitAiEdit(
  action: 'polish' | 'continue' | 'modify',
  instruction: string,
  selection: string,
  lineRange: { startLine: number; endLine: number } | null,
) {
  emit('aiEdit', {
    action,
    instruction,
    selection,
    startLine: lineRange?.startLine,
    endLine: lineRange?.endLine,
  })
}

function requestAiWithDefaultNote(action: 'polish' | 'continue') {
  const selection = captureAiSelection()
  if (!selection.trim()) return
  const lineRange = aiSelectionLines.value
  const note =
    action === 'polish' ? t('office.defaultPolishNote') : t('office.defaultExpandNote')
  closeAiAnnotate()
  floatPos.value = null
  emitAiEdit(action, note, selection, lineRange)
}

function openAiAnnotate() {
  const selection = captureAiSelection()
  if (!selection.trim()) return
  aiAnnotateOpen.value = true
  aiInstruction.value = ''
  void nextTick(() => aiInputRef.value?.focus())
}

function confirmAiModify() {
  const note = aiInstruction.value.trim()
  const selection = aiSelectionMarkdown.value || captureAiSelection()
  if (!selection.trim()) {
    closeAiAnnotate()
    return
  }
  const lineRange = aiSelectionLines.value
  closeAiAnnotate()
  floatPos.value = null
  emitAiEdit('modify', note, selection, lineRange)
}

function onAiAnnotateKeydown(e: KeyboardEvent) {
  if ((e.metaKey || e.ctrlKey) && e.key === 'Enter') {
    e.preventDefault()
    confirmAiModify()
    return
  }
  if (e.key === 'Escape') {
    e.preventDefault()
    closeAiAnnotate()
  }
}

function updateFloatFromCoords(startTop: number, startLeft: number, endRight: number, endTop: number) {
  const root = rootRef.value
  if (!root) {
    floatPos.value = null
    return
  }
  const rootRect = root.getBoundingClientRect()
  const midX = (startLeft + endRight) / 2
  const top = Math.min(startTop, endTop) - rootRect.top - 40
  floatPos.value = {
    top: Math.max(4, top),
    left: Math.max(8, Math.min(midX - rootRect.left, rootRect.width - 8)),
  }
}

function updateFloatPosition() {
  if (!props.enableSelectionAi) {
    floatPos.value = null
    return
  }
  if (props.editable) {
    const view = mdRef.value?.getEditorView()
    if (!view) {
      floatPos.value = null
      return
    }
    const { from, to, empty } = view.state.selection.main
    if (empty) {
      floatPos.value = null
      return
    }
    const start = view.coordsAtPos(from)
    const end = view.coordsAtPos(to)
    if (!start || !end) {
      floatPos.value = null
      return
    }
    updateFloatFromCoords(start.top, start.left, end.right, end.top)
    return
  }
  const dom = getDomSelectionInPreview()
  if (!dom) {
    floatPos.value = null
    return
  }
  const rect = dom.range.getBoundingClientRect()
  if (!rect.width && !rect.height) {
    floatPos.value = null
    return
  }
  updateFloatFromCoords(rect.top, rect.left, rect.right, rect.top)
}

function syncSelection() {
  if (!props.enableSelectionAi) {
    hasSelection.value = false
    floatPos.value = null
    emit('selectionEmpty', true)
    return
  }
  if (aiAnnotateOpen.value) {
    emit('selectionEmpty', false)
    hasSelection.value = true
    updateFloatPosition()
    return
  }
  const empty = isSelectionEmpty()
  hasSelection.value = !empty
  emit('selectionEmpty', empty)
  if (empty) {
    floatPos.value = null
    return
  }
  updateFloatPosition()
}

function onPreviewSelectionEvent() {
  if (props.editable || !props.enableSelectionAi) return
  void nextTick(syncSelection)
}

let previewListenersBound = false

function bindPreviewSelectionListeners() {
  if (previewListenersBound || props.editable || !props.enableSelectionAi) return
  const root = rootRef.value
  if (!root) return
  root.addEventListener('mouseup', onPreviewSelectionEvent)
  root.addEventListener('keyup', onPreviewSelectionEvent)
  document.addEventListener('selectionchange', onPreviewSelectionEvent)
  previewListenersBound = true
}

function unbindPreviewSelectionListeners() {
  if (!previewListenersBound) return
  const root = rootRef.value
  if (root) {
    root.removeEventListener('mouseup', onPreviewSelectionEvent)
    root.removeEventListener('keyup', onPreviewSelectionEvent)
  }
  document.removeEventListener('selectionchange', onPreviewSelectionEvent)
  previewListenersBound = false
}

function syncPreviewSelectionListeners() {
  if (!props.editable && props.enableSelectionAi) bindPreviewSelectionListeners()
  else unbindPreviewSelectionListeners()
}

/** md-editor keeps previewOnly in internal state; prop alone is not enough on mount. */
function applyEditableMode() {
  const md = mdRef.value
  if (!md) return
  if (props.editable) {
    // Source-only edit — split preview eats too much horizontal space.
    md.togglePreviewOnly(false)
    md.togglePreview(false)
    // Never force-open catalog: it steals width and often collapses into a useless strip.
    md.toggleCatalog(false)
  } else {
    md.togglePreviewOnly(true)
    md.toggleCatalog(false)
  }
}

function onRemount() {
  mdRef.value?.domEventHandlers({
    mouseup: () => {
      void nextTick(syncSelection)
    },
    keyup: () => {
      void nextTick(syncSelection)
    },
    blur: () => {
      if (!aiAnnotateOpen.value) {
        // Keep float briefly; clear if selection truly empty on next tick.
        void nextTick(syncSelection)
      }
    },
  })
  applyEditableMode()
  syncPreviewSelectionListeners()
  syncSelection()
}

watch(
  () => props.content,
  (md) => {
    if ((md || '') === text.value) return
    // Avoid clobbering while user is typing.
    const view = mdRef.value?.getEditorView()
    if (view?.hasFocus && props.editable) return
    setContent(md || '', { emitUpdate: false })
  },
)

watch(
  () => props.editable,
  () => {
    closeAiAnnotate()
    floatPos.value = null
    void nextTick(() => {
      applyEditableMode()
      syncPreviewSelectionListeners()
      syncSelection()
    })
  },
)

watch(
  () => props.enableSelectionAi,
  (enabled) => {
    if (!enabled) {
      closeAiAnnotate()
      floatPos.value = null
      hasSelection.value = false
      emit('selectionEmpty', true)
    }
    void nextTick(() => {
      syncPreviewSelectionListeners()
      if (enabled) syncSelection()
    })
  },
)

onMounted(() => {
  syncPreviewSelectionListeners()
})

onBeforeUnmount(() => {
  unbindPreviewSelectionListeners()
  closeAiAnnotate()
})

defineExpose({
  setContent,
  getMarkdown,
  getSelectionMarkdown,
  getSelectionLines,
  isSelectionEmpty,
})
</script>

<template>
  <div
    ref="rootRef"
    class="md-rich"
    :class="{ 'is-readonly': !editable }"
  >
    <MdEditor
      :id="editorId"
      ref="mdRef"
      v-model="text"
      :theme="mdTheme"
      :language="mdLanguage"
      :placeholder="placeholder || t('office.docPlaceholder')"
      :preview="false"
      :preview-only="!editable"
      :toolbars-exclude="toolbarsExclude"
      :footers="[]"
      :style="{ height: '100%' }"
      class="md-rich__editor"
      @on-change="onChange"
      @on-remount="onRemount"
    />

    <!-- Floating selection AI — edit (CM) + view (DOM preview selection) -->
    <div
      v-if="enableSelectionAi && hasSelection && floatPos && !aiAnnotateOpen"
      class="md-rich__ai-float"
      :style="{ top: `${floatPos.top}px`, left: `${floatPos.left}px` }"
      @mousedown.prevent
    >
      <button
        type="button"
        class="md-rich__ai-btn"
        :title="t('office.bubbleAiPolish')"
        @mousedown.prevent="requestAiWithDefaultNote('polish')"
      >
        {{ t('office.bubbleAiPolish') }}
      </button>
      <button
        type="button"
        class="md-rich__ai-btn"
        :title="t('office.bubbleAiExpand')"
        @mousedown.prevent="requestAiWithDefaultNote('continue')"
      >
        {{ t('office.bubbleAiExpand') }}
      </button>
      <button
        type="button"
        class="md-rich__ai-btn"
        :title="t('office.bubbleAiModify')"
        @mousedown.prevent="openAiAnnotate"
      >
        {{ t('office.bubbleAiModify') }}
      </button>
    </div>

    <div
      v-if="enableSelectionAi && aiAnnotateOpen"
      class="md-rich__ai-dialog"
      role="dialog"
      @keydown="onAiAnnotateKeydown"
    >
      <div class="md-rich__ai-dialog-backdrop" @click="closeAiAnnotate" />
      <div class="md-rich__ai-annotate">
        <div class="md-rich__ai-annotate-title">{{ t('office.selectionAnnotateTitle') }}</div>
        <textarea
          ref="aiInputRef"
          v-model="aiInstruction"
          class="md-rich__ai-annotate-input"
          rows="3"
          :placeholder="t('office.selectionAnnotatePlaceholder')"
        />
        <div class="md-rich__ai-annotate-actions">
          <button type="button" class="md-rich__ai-btn" @click="closeAiAnnotate">
            {{ t('common.cancel') }}
          </button>
          <button type="button" class="md-rich__ai-btn md-rich__ai-btn--primary" @click="confirmAiModify">
            {{ t('office.selectionAnnotateConfirm') }}
          </button>
        </div>
        <div class="md-rich__ai-annotate-hint">{{ t('office.selectionAnnotateHint') }}</div>
      </div>
    </div>
  </div>
</template>

<style scoped>
.md-rich {
  position: relative;
  display: flex;
  flex: 1;
  min-height: 0;
  min-width: 0;
  background: transparent;
  color: var(--dq-label-primary);
}
.md-rich__editor {
  flex: 1;
  min-height: 0;
  width: 100%;
  border: none !important;
  border-radius: 0 !important;
}
.md-rich :deep(.md-editor) {
  height: 100%;
  border: none;
  border-radius: 0;
  background: transparent !important;
  --md-color: var(--dq-label-primary);
  --md-hover-color: var(--dq-label-secondary);
  --md-bk-color: transparent;
  --md-bk-color-outstand: color-mix(in srgb, var(--dq-bg-elevated) 70%, transparent);
  --md-bk-hover-color: color-mix(in srgb, var(--dq-label-primary) 6%, transparent);
  --md-border-color: var(--dq-glass-border, var(--dq-separator-light));
  --md-scrollbar-bg-color: transparent;
  --md-scrollbar-thumb-color: color-mix(in srgb, var(--dq-label-quaternary) 50%, transparent);
}
.md-rich :deep(.md-editor-dark) {
  --md-color: var(--dq-label-primary);
  --md-bk-color: transparent;
  --md-bk-color-outstand: color-mix(in srgb, var(--dq-bg-elevated) 70%, transparent);
  background: transparent !important;
}
.md-rich :deep(.cm-editor),
.md-rich :deep(.cm-scroller),
.md-rich :deep(.cm-gutters) {
  background: transparent !important;
}
.md-rich :deep(.cm-content) {
  caret-color: var(--dq-accent);
}
.md-rich :deep(.cm-activeLine),
.md-rich :deep(.cm-activeLineGutter) {
  background: color-mix(in srgb, var(--dq-accent) 7%, transparent) !important;
}
.md-rich :deep(.cm-cursor) {
  border-left-color: var(--dq-accent) !important;
}
.md-rich :deep(.md-editor-content-wrapper),
.md-rich :deep(.md-editor-input-wrapper) {
  width: 100% !important;
  flex: 1 1 auto !important;
  background: transparent !important;
}
.md-rich :deep(.md-editor-catalog-editor),
.md-rich :deep(.md-editor-catalog-flat) {
  /* Catalog stays available via toolbar when showToc; keep it usable if opened. */
  min-width: 160px;
  width: 200px;
  background: color-mix(in srgb, var(--dq-bg-elevated) 55%, transparent) !important;
  border-inline-start: 1px solid var(--dq-glass-border, var(--dq-separator-light));
  color: var(--dq-label-secondary);
}
.md-rich :deep(.md-editor-toolbar-wrapper) {
  background: var(--dq-glass-bar-bg, color-mix(in srgb, var(--dq-bg-elevated) 55%, transparent));
  border-bottom: 1px solid var(--dq-glass-border, var(--dq-separator-light));
}
.md-rich.is-readonly :deep(.md-editor-toolbar-wrapper) {
  display: none;
}
.md-rich.is-readonly :deep(.md-editor-preview) {
  user-select: text;
  cursor: text;
}
.md-rich__ai-float {
  position: absolute;
  z-index: 20;
  transform: translateX(-50%);
  display: flex;
  align-items: center;
  gap: 2px;
  padding: 4px;
  border-radius: 8px;
  border: 1px solid var(--dq-glass-border, var(--dq-border));
  background: var(--dq-bg-elevated);
  box-shadow: 0 6px 20px color-mix(in srgb, var(--dq-mask) 18%, transparent);
  white-space: nowrap;
}
.md-rich__ai-btn {
  appearance: none;
  border: none;
  background: transparent;
  color: var(--dq-label-secondary);
  font: inherit;
  font-size: var(--dq-font-size-caption);
  font-weight: 500;
  padding: 4px 8px;
  border-radius: 6px;
  cursor: pointer;
}
.md-rich__ai-btn:hover {
  background: color-mix(in srgb, var(--dq-label-primary) 8%, transparent);
  color: var(--dq-label-primary);
}
.md-rich__ai-btn--primary {
  background: var(--dq-accent);
  color: var(--dq-on-accent);
  font-weight: 600;
}
.md-rich__ai-btn--primary:hover {
  background: var(--dq-accent-hover);
  color: var(--dq-on-accent);
}
.md-rich__ai-dialog {
  position: absolute;
  inset: 0;
  z-index: 30;
  display: flex;
  align-items: center;
  justify-content: center;
  padding: 24px;
}
.md-rich__ai-dialog-backdrop {
  position: absolute;
  inset: 0;
  background: color-mix(in srgb, var(--dq-mask) 35%, transparent);
}
.md-rich__ai-annotate {
  position: relative;
  z-index: 1;
  width: min(360px, 100%);
  display: flex;
  flex-direction: column;
  gap: 8px;
  padding: 12px;
  border-radius: 10px;
  border: 1px solid var(--dq-glass-border, var(--dq-border));
  background: var(--dq-bg-elevated);
  box-shadow: 0 12px 40px color-mix(in srgb, var(--dq-mask) 28%, transparent);
}
.md-rich__ai-annotate-title {
  font-size: var(--dq-font-size-caption);
  font-weight: 600;
  color: var(--dq-label-primary);
}
.md-rich__ai-annotate-input {
  width: 100%;
  resize: vertical;
  min-height: 64px;
  padding: 8px 10px;
  border: 1px solid var(--dq-border-subtle);
  border-radius: 6px;
  background: color-mix(in srgb, var(--dq-bg-elevated) 60%, transparent);
  color: var(--dq-label-primary);
  font-size: var(--dq-font-size-caption);
  font-family: inherit;
  box-sizing: border-box;
}
.md-rich__ai-annotate-input:focus {
  outline: none;
  border-color: var(--dq-accent);
  box-shadow: 0 0 0 3px color-mix(in srgb, var(--dq-accent) 12%, transparent);
}
.md-rich__ai-annotate-actions {
  display: flex;
  justify-content: flex-end;
  gap: 6px;
}
.md-rich__ai-annotate-hint {
  font-size: var(--dq-font-size-caption);
  color: var(--dq-label-quaternary);
}
</style>
