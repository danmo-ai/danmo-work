<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, ref, watch } from 'vue'
import { storeToRefs } from 'pinia'
import { useI18n } from 'vue-i18n'
import { fetchJSON } from '@/api/client'
import { toast } from '@/utils/feedback'
import { languageFromPath } from '@/utils/file-route'
import {
  createCodeSelectionAttachment,
  selectionLineRange,
  type CodeSelectionAttachment,
} from '@/types/code-attachment'
import {
  createCodeMirror,
  getCodeMirrorSelection,
  loadLanguageExtension,
  setCodeMirrorDoc,
  setCodeMirrorLanguage,
  setCodeMirrorReadOnly,
  setCodeMirrorTheme,
  type CodeMirrorHost,
} from '@/utils/codemirror-setup'
import { useThemeStore, THEME_OPTIONS } from '@/stores/theme'

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
  attachCodeSelection: [att: CodeSelectionAttachment]
}>()

const { t } = useI18n()
const themeStore = useThemeStore()
const { currentTheme } = storeToRefs(themeStore)

const loading = ref(false)
const saving = ref(false)
const content = ref('')
const dirty = ref(false)
const rootRef = ref<HTMLElement | null>(null)
const hostEl = ref<HTMLElement | null>(null)
const selStart = ref(0)
const selEnd = ref(0)
const floatPos = ref<{ top: number; left: number } | null>(null)
const aiAnnotateOpen = ref(false)
const aiInstruction = ref('')
const aiInputRef = ref<HTMLTextAreaElement | null>(null)
const capturedText = ref('')
const capturedRange = ref<{ startLine: number; endLine: number } | null>(null)
let host: CodeMirrorHost | null = null
let suppressDocEvent = false

const language = computed(() => languageFromPath(props.path))
const readOnly = computed(() => props.mode !== 'edit' || !!props.turnRunning)
const selectionAiEnabled = computed(() => !props.turnRunning && props.mode !== 'present')
const isDark = computed(() => {
  const opt = THEME_OPTIONS.find((o) => o.id === currentTheme.value)
  return !!opt?.dark
})

const selectionRange = computed(() =>
  selectionLineRange(content.value, selStart.value, selEnd.value),
)

const hasSelection = computed(() => selEnd.value > selStart.value)

async function ensureEditor() {
  if (host || !hostEl.value) return
  const languageExt = await loadLanguageExtension(props.path, language.value)
  host = createCodeMirror({
    parent: hostEl.value,
    doc: content.value,
    readOnly: readOnly.value,
    dark: isDark.value,
    languageExt,
    onDocChanged: (doc) => {
      if (suppressDocEvent) return
      content.value = doc
      if (!dirty.value) {
        dirty.value = true
        emit('dirty', true)
      }
    },
    onSelectionChanged: (from, to) => {
      selStart.value = from
      selEnd.value = to
      syncFloat()
    },
  })
}

async function load() {
  if (!props.projectId || !props.path) return
  loading.value = true
  try {
    const fc = await fetchJSON<{ content: string; binary?: boolean }>(
      `/projects/${props.projectId}/files/content?path=${encodeURIComponent(props.path)}`,
    )
    if (fc.binary) throw new Error(t('office.codeBinary'))
    content.value = fc.content || ''
    dirty.value = false
    emit('dirty', false)
    selStart.value = 0
    selEnd.value = 0
    floatPos.value = null
    closeAiAnnotate()
    await nextTick()
    await ensureEditor()
    if (host) {
      suppressDocEvent = true
      setCodeMirrorDoc(host, content.value)
      suppressDocEvent = false
      await setCodeMirrorLanguage(host, props.path, language.value)
      setCodeMirrorReadOnly(host, readOnly.value)
    }
  } catch (e) {
    toast.error(e instanceof Error ? e.message : t('office.loadFailed'))
  } finally {
    loading.value = false
  }
}

async function save(opts?: { quiet?: boolean }) {
  if (!props.projectId || readOnly.value) return
  if (host) content.value = host.view.state.doc.toString()
  saving.value = true
  try {
    await fetchJSON(`/projects/${props.projectId}/files/content`, {
      method: 'PUT',
      body: JSON.stringify({ path: props.path, content: content.value }),
    })
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

function syncFloat() {
  if (!selectionAiEnabled.value || aiAnnotateOpen.value) {
    if (!aiAnnotateOpen.value) floatPos.value = null
    return
  }
  if (!host || !hasSelection.value) {
    floatPos.value = null
    return
  }
  const { from, to } = host.view.state.selection.main
  if (from === to) {
    floatPos.value = null
    return
  }
  const start = host.view.coordsAtPos(from)
  const end = host.view.coordsAtPos(to)
  if (!start || !end) {
    floatPos.value = null
    return
  }
  updateFloatFromCoords(start.top, start.left, end.right, end.top)
}

function captureSelection(): { text: string; startLine: number; endLine: number } | null {
  if (host) {
    const sel = getCodeMirrorSelection(host)
    selStart.value = sel.from
    selEnd.value = sel.to
    if (sel.from === sel.to || !sel.text.trim()) return null
    const range = selectionLineRange(content.value, sel.from, sel.to)
    capturedText.value = sel.text
    capturedRange.value = range
    return { text: sel.text, ...range }
  }
  if (!hasSelection.value) return null
  const text = content.value.slice(selStart.value, selEnd.value)
  if (!text.trim()) return null
  const range = selectionRange.value
  capturedText.value = text
  capturedRange.value = range
  return { text, ...range }
}

function closeAiAnnotate(opts?: { restoreFloat?: boolean }) {
  aiAnnotateOpen.value = false
  aiInstruction.value = ''
  if (opts?.restoreFloat) void nextTick(syncFloat)
}

function emitAttachment(annotation: string) {
  const text = capturedText.value
  const range = capturedRange.value
  if (!text.trim() || !range) return
  const att = createCodeSelectionAttachment({
    path: props.path,
    language: language.value,
    startLine: range.startLine,
    endLine: range.endLine,
    text,
    annotation,
  })
  emit('attachCodeSelection', att)
  toast.success(t('office.codeAttached'))
  floatPos.value = null
  closeAiAnnotate()
}

function requestAiWithDefaultNote(action: 'polish' | 'continue') {
  if (!captureSelection()) return
  const note =
    action === 'polish' ? t('office.defaultPolishNote') : t('office.defaultExpandNote')
  emitAttachment(note)
}

function openAiAnnotate() {
  if (!captureSelection()) return
  aiAnnotateOpen.value = true
  aiInstruction.value = ''
  floatPos.value = null
  void nextTick(() => aiInputRef.value?.focus())
}

function confirmAiModify() {
  const note = aiInstruction.value.trim()
  if (!capturedText.value.trim()) {
    closeAiAnnotate()
    return
  }
  emitAttachment(note)
}

function onAiAnnotateKeydown(e: KeyboardEvent) {
  if ((e.metaKey || e.ctrlKey) && e.key === 'Enter') {
    e.preventDefault()
    confirmAiModify()
    return
  }
  if (e.key === 'Escape') {
    e.preventDefault()
    closeAiAnnotate({ restoreFloat: true })
  }
}

function onKeydown(e: KeyboardEvent) {
  if ((e.metaKey || e.ctrlKey) && e.key === 'Enter' && hasSelection.value && selectionAiEnabled.value) {
    e.preventDefault()
    openAiAnnotate()
  }
}

watch(
  () => [props.projectId, props.path, props.reloadToken] as const,
  () => {
    void load()
  },
  { immediate: true },
)

watch(readOnly, (v) => {
  if (host) setCodeMirrorReadOnly(host, v)
})

watch(isDark, (v) => {
  if (host) setCodeMirrorTheme(host, v)
})

watch(
  () => [props.path, language.value] as const,
  ([, lang]) => {
    if (host) void setCodeMirrorLanguage(host, props.path, lang)
  },
)

watch(selectionAiEnabled, (enabled) => {
  if (!enabled) {
    closeAiAnnotate()
    floatPos.value = null
  } else {
    void nextTick(syncFloat)
  }
})

onBeforeUnmount(() => {
  host?.view.destroy()
  host = null
})

defineExpose({ save })
</script>

<template>
  <div ref="rootRef" class="code-surface" @keydown="onKeydown">
    <div class="code-surface__toolbar">
      <span class="code-surface__lang">{{ language }}</span>
      <span v-if="hasSelection" class="code-surface__sel">
        L{{ selectionRange.startLine
        }}<template v-if="selectionRange.endLine !== selectionRange.startLine"
          >–{{ selectionRange.endLine }}</template
        >
      </span>
    </div>

    <div v-if="loading" class="code-surface__status">{{ t('office.loading') }}</div>
    <div
      v-show="!loading"
      ref="hostEl"
      class="code-surface__editor"
      :class="{ 'is-readonly': readOnly }"
    />

    <!-- Floating selection AI — same pattern as MarkdownRichEditor / Doc -->
    <div
      v-if="selectionAiEnabled && hasSelection && floatPos && !aiAnnotateOpen"
      class="code-surface__ai-float"
      :style="{ top: `${floatPos.top}px`, left: `${floatPos.left}px` }"
      @mousedown.prevent
    >
      <button
        type="button"
        class="code-surface__ai-btn"
        :title="t('office.bubbleAiPolish')"
        @mousedown.prevent="requestAiWithDefaultNote('polish')"
      >
        {{ t('office.bubbleAiPolish') }}
      </button>
      <button
        type="button"
        class="code-surface__ai-btn"
        :title="t('office.bubbleAiExpand')"
        @mousedown.prevent="requestAiWithDefaultNote('continue')"
      >
        {{ t('office.bubbleAiExpand') }}
      </button>
      <button
        type="button"
        class="code-surface__ai-btn"
        :title="t('office.bubbleAiModify')"
        @mousedown.prevent="openAiAnnotate"
      >
        {{ t('office.bubbleAiModify') }}
      </button>
    </div>

    <div
      v-if="selectionAiEnabled && aiAnnotateOpen"
      class="code-surface__ai-dialog"
      role="dialog"
      @keydown="onAiAnnotateKeydown"
    >
      <div
        class="code-surface__ai-dialog-backdrop"
        @click="closeAiAnnotate({ restoreFloat: true })"
      />
      <div class="code-surface__ai-annotate">
        <div class="code-surface__ai-annotate-title">{{ t('office.selectionAnnotateTitle') }}</div>
        <textarea
          ref="aiInputRef"
          v-model="aiInstruction"
          class="code-surface__ai-annotate-input"
          rows="3"
          :placeholder="t('office.selectionAnnotatePlaceholder')"
        />
        <div class="code-surface__ai-annotate-actions">
          <button
            type="button"
            class="code-surface__ai-btn"
            @click="closeAiAnnotate({ restoreFloat: true })"
          >
            {{ t('common.cancel') }}
          </button>
          <button
            type="button"
            class="code-surface__ai-btn code-surface__ai-btn--primary"
            @click="confirmAiModify"
          >
            {{ t('office.selectionAnnotateConfirm') }}
          </button>
        </div>
        <div class="code-surface__ai-annotate-hint">{{ t('office.selectionAnnotateHint') }}</div>
      </div>
    </div>
  </div>
</template>

<style scoped>
.code-surface {
  position: relative;
  display: flex;
  flex-direction: column;
  min-height: 0;
  flex: 1;
  background: var(--dq-bg-base);
}
.code-surface__toolbar {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 6px 10px;
  border-bottom: 1px solid var(--dq-separator-light);
  background: color-mix(in srgb, var(--dq-bg-elevated) 40%, transparent);
  flex-wrap: wrap;
}
.code-surface__lang {
  font-size: var(--dq-font-size-caption);
  font-weight: 600;
  text-transform: uppercase;
  letter-spacing: 0.04em;
  color: var(--dq-label-tertiary);
}
.code-surface__sel {
  font-size: var(--dq-font-size-caption);
  font-variant-numeric: tabular-nums;
  color: var(--dq-accent);
}
.code-surface__status {
  padding: 24px;
  color: var(--dq-label-tertiary);
  font-size: var(--dq-font-size-body);
}
.code-surface__editor {
  min-height: 0;
  flex: 1;
  overflow: hidden;
}
.code-surface__editor :deep(.cm-editor) {
  height: 100%;
  outline: none;
}
.code-surface__editor :deep(.cm-editor.cm-focused) {
  outline: none;
}
.code-surface__editor.is-readonly :deep(.cm-content) {
  caret-color: transparent;
}
.code-surface__ai-float {
  position: absolute;
  z-index: 20;
  transform: translateX(-50%);
  display: flex;
  align-items: center;
  gap: 2px;
  padding: 4px;
  border-radius: 8px;
  border: 1px solid var(--dq-border);
  background: var(--dq-bg-elevated);
  box-shadow: 0 6px 20px color-mix(in srgb, #000 18%, transparent);
  white-space: nowrap;
}
.code-surface__ai-btn {
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
.code-surface__ai-btn:hover {
  background: color-mix(in srgb, var(--dq-label-primary) 8%, transparent);
  color: var(--dq-label-primary);
}
.code-surface__ai-btn--primary {
  background: var(--dq-accent);
  color: var(--dq-on-accent);
  font-weight: 600;
}
.code-surface__ai-btn--primary:hover {
  background: var(--dq-accent-hover);
  color: var(--dq-on-accent);
}
.code-surface__ai-dialog {
  position: absolute;
  inset: 0;
  z-index: 30;
  display: flex;
  align-items: center;
  justify-content: center;
  padding: 24px;
}
.code-surface__ai-dialog-backdrop {
  position: absolute;
  inset: 0;
  background: color-mix(in srgb, #000 35%, transparent);
}
.code-surface__ai-annotate {
  position: relative;
  z-index: 1;
  width: min(360px, 100%);
  display: flex;
  flex-direction: column;
  gap: 8px;
  padding: 12px;
  border-radius: 10px;
  border: 1px solid var(--dq-border);
  background: var(--dq-bg-elevated);
  box-shadow: 0 12px 40px color-mix(in srgb, #000 28%, transparent);
}
.code-surface__ai-annotate-title {
  font-size: var(--dq-font-size-caption);
  font-weight: 600;
  color: var(--dq-label-primary);
}
.code-surface__ai-annotate-input {
  width: 100%;
  resize: vertical;
  min-height: 64px;
  padding: 8px 10px;
  border: 1px solid var(--dq-border);
  border-radius: 6px;
  background: var(--dq-bg-base);
  color: var(--dq-label-primary);
  font-size: var(--dq-font-size-caption);
  font-family: inherit;
  box-sizing: border-box;
}
.code-surface__ai-annotate-input:focus {
  outline: none;
  border-color: var(--dq-accent);
  box-shadow: 0 0 0 3px color-mix(in srgb, var(--dq-accent) 12%, transparent);
}
.code-surface__ai-annotate-actions {
  display: flex;
  justify-content: flex-end;
  gap: 6px;
}
.code-surface__ai-annotate-hint {
  font-size: var(--dq-font-size-caption);
  color: var(--dq-label-quaternary);
}
</style>
