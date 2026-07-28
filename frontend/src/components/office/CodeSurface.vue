<script setup lang="ts">
import { computed, nextTick, onMounted, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { fetchJSON } from '@/api/client'
import { toast } from '@/utils/feedback'
import { languageFromPath } from '@/utils/office-route'
import {
  createCodeSelectionAttachment,
  selectionLineRange,
  type CodeSelectionAttachment,
} from '@/types/code-attachment'
import ElementAnnotatePopover from '@/components/center/ElementAnnotatePopover.vue'

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
const loading = ref(false)
const saving = ref(false)
const content = ref('')
const dirty = ref(false)
const textareaRef = ref<HTMLTextAreaElement | null>(null)
const gutterRef = ref<HTMLElement | null>(null)
const selStart = ref(0)
const selEnd = ref(0)
const annotateOpen = ref(false)

const language = computed(() => languageFromPath(props.path))
const readOnly = computed(() => props.mode !== 'edit' || !!props.turnRunning)

const lineCount = computed(() => {
  const text = content.value
  if (!text) return 1
  let n = 1
  for (let i = 0; i < text.length; i++) {
    if (text.charCodeAt(i) === 10) n++
  }
  return n
})

const lineNumbers = computed(() => {
  const n = lineCount.value
  const parts: string[] = new Array(n)
  for (let i = 0; i < n; i++) parts[i] = String(i + 1)
  return parts.join('\n')
})

const selectionRange = computed(() =>
  selectionLineRange(content.value, selStart.value, selEnd.value),
)

const hasSelection = computed(() => selEnd.value > selStart.value)

const annotateSummary = computed(() => {
  const { startLine, endLine } = selectionRange.value
  const base = props.path.replace(/\\/g, '/').split('/').pop() || props.path
  if (startLine === endLine) return `${base}:${startLine}`
  return `${base}:${startLine}–${endLine}`
})

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
  } catch (e) {
    toast.error(e instanceof Error ? e.message : t('office.loadFailed'))
  } finally {
    loading.value = false
  }
}

async function save(opts?: { quiet?: boolean }) {
  if (!props.projectId || readOnly.value) return
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

function onInput(e: Event) {
  const el = e.target as HTMLTextAreaElement
  content.value = el.value
  if (!dirty.value) {
    dirty.value = true
    emit('dirty', true)
  }
  syncSelection(el)
}

function syncSelection(el?: HTMLTextAreaElement | null) {
  const ta = el ?? textareaRef.value
  if (!ta) return
  selStart.value = ta.selectionStart
  selEnd.value = ta.selectionEnd
}

function onScroll() {
  const ta = textareaRef.value
  const gut = gutterRef.value
  if (ta && gut) gut.scrollTop = ta.scrollTop
}

function onKeydown(e: KeyboardEvent) {
  if (e.key === 'Tab' && !readOnly.value) {
    e.preventDefault()
    const ta = textareaRef.value
    if (!ta) return
    const start = ta.selectionStart
    const end = ta.selectionEnd
    const v = content.value
    content.value = v.slice(0, start) + '  ' + v.slice(end)
    dirty.value = true
    emit('dirty', true)
    void nextTick(() => {
      ta.selectionStart = ta.selectionEnd = start + 2
      syncSelection(ta)
    })
    return
  }
  if ((e.metaKey || e.ctrlKey) && e.key === 'Enter' && hasSelection.value) {
    e.preventDefault()
    openAnnotate()
  }
}

function openAnnotate() {
  syncSelection()
  if (!hasSelection.value) {
    toast.warning(t('office.codeNeedSelection'))
    return
  }
  annotateOpen.value = true
}

function onAnnotateConfirm(annotation: string) {
  annotateOpen.value = false
  const { startLine, endLine } = selectionRange.value
  const text = content.value.slice(selStart.value, selEnd.value)
  const att = createCodeSelectionAttachment({
    path: props.path,
    language: language.value,
    startLine,
    endLine,
    text,
    annotation,
  })
  emit('attachCodeSelection', att)
  toast.success(t('office.codeAttached'))
}

function onAnnotateCancel() {
  annotateOpen.value = false
}

watch(
  () => [props.projectId, props.path, props.reloadToken] as const,
  () => {
    void load()
  },
  { immediate: true },
)

onMounted(() => {
  void nextTick(() => textareaRef.value?.focus())
})

defineExpose({ save })
</script>

<template>
  <div class="code-surface">
    <div class="code-surface__toolbar">
      <span class="code-surface__lang">{{ language }}</span>
      <span v-if="hasSelection" class="code-surface__sel">
        L{{ selectionRange.startLine
        }}<template v-if="selectionRange.endLine !== selectionRange.startLine"
          >–{{ selectionRange.endLine }}</template
        >
      </span>
      <span class="code-surface__hint">{{ t('office.codeAnnotateHint') }}</span>
      <button
        type="button"
        class="code-surface__btn"
        :disabled="!hasSelection"
        @click="openAnnotate"
      >
        {{ t('office.codeAnnotate') }}
      </button>
    </div>

    <div v-if="loading" class="code-surface__status">{{ t('office.loading') }}</div>
    <div v-else class="code-surface__editor">
      <pre ref="gutterRef" class="code-surface__gutter" aria-hidden="true">{{ lineNumbers }}</pre>
      <textarea
        ref="textareaRef"
        class="code-surface__textarea"
        :value="content"
        :readonly="readOnly"
        spellcheck="false"
        wrap="off"
        @input="onInput"
        @scroll="onScroll"
        @keydown="onKeydown"
        @keyup="syncSelection()"
        @click="syncSelection()"
        @select="syncSelection()"
      />
    </div>

    <ElementAnnotatePopover
      :open="annotateOpen"
      :payload="null"
      :summary="annotateSummary"
      @confirm="onAnnotateConfirm"
      @cancel="onAnnotateCancel"
    />
  </div>
</template>

<style scoped>
.code-surface {
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
  font-size: 11px;
  font-weight: 600;
  text-transform: uppercase;
  letter-spacing: 0.04em;
  color: var(--dq-label-tertiary);
}
.code-surface__sel {
  font-size: 12px;
  font-variant-numeric: tabular-nums;
  color: var(--dq-accent);
}
.code-surface__hint {
  flex: 1;
  font-size: 11px;
  color: var(--dq-label-tertiary);
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.code-surface__btn {
  height: 26px;
  padding: 0 10px;
  border: 1px solid var(--dq-border);
  border-radius: 6px;
  background: var(--dq-fill-tertiary);
  color: var(--dq-label-primary);
  font-size: 12px;
  cursor: pointer;
  flex-shrink: 0;
}
.code-surface__btn:hover:not(:disabled) {
  background: color-mix(in srgb, var(--dq-label-primary) 8%, var(--dq-fill-tertiary));
}
.code-surface__btn:disabled {
  opacity: 0.45;
  cursor: default;
}
.code-surface__status {
  padding: 24px;
  color: var(--dq-label-tertiary);
  font-size: 13px;
}
.code-surface__editor {
  display: flex;
  min-height: 0;
  flex: 1;
  overflow: hidden;
  font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, 'Liberation Mono', monospace;
  font-size: 13px;
  line-height: 1.5;
}
.code-surface__gutter {
  margin: 0;
  padding: 12px 8px 12px 12px;
  min-width: 3.2em;
  text-align: right;
  color: var(--dq-label-quaternary, var(--dq-label-tertiary));
  background: color-mix(in srgb, var(--dq-label-primary) 3%, transparent);
  border-right: 1px solid var(--dq-separator-light);
  overflow: hidden;
  user-select: none;
  white-space: pre;
  font-variant-numeric: tabular-nums;
}
.code-surface__textarea {
  flex: 1;
  min-width: 0;
  margin: 0;
  padding: 12px;
  border: 0;
  resize: none;
  outline: none;
  background: transparent;
  color: var(--dq-label-primary);
  font: inherit;
  line-height: inherit;
  white-space: pre;
  overflow: auto;
  tab-size: 2;
}
.code-surface__textarea:read-only {
  cursor: default;
}
</style>
