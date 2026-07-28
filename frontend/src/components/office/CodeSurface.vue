<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, ref, watch } from 'vue'
import { storeToRefs } from 'pinia'
import { useI18n } from 'vue-i18n'
import { fetchJSON } from '@/api/client'
import { toast } from '@/utils/feedback'
import { languageFromPath } from '@/utils/office-route'
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
const themeStore = useThemeStore()
const { currentTheme } = storeToRefs(themeStore)

const loading = ref(false)
const saving = ref(false)
const content = ref('')
const dirty = ref(false)
const hostEl = ref<HTMLElement | null>(null)
const selStart = ref(0)
const selEnd = ref(0)
const annotateOpen = ref(false)
let host: CodeMirrorHost | null = null
let suppressDocEvent = false

const language = computed(() => languageFromPath(props.path))
const readOnly = computed(() => props.mode !== 'edit' || !!props.turnRunning)
const isDark = computed(() => {
  const opt = THEME_OPTIONS.find((o) => o.id === currentTheme.value)
  return !!opt?.dark
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

function openAnnotate() {
  if (host) {
    const sel = getCodeMirrorSelection(host)
    selStart.value = sel.from
    selEnd.value = sel.to
  }
  if (!hasSelection.value) {
    toast.warning(t('office.codeNeedSelection'))
    return
  }
  annotateOpen.value = true
}

function onAnnotateConfirm(annotation: string) {
  annotateOpen.value = false
  const { startLine, endLine } = selectionRange.value
  const text = host
    ? getCodeMirrorSelection(host).text
    : content.value.slice(selStart.value, selEnd.value)
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

function onKeydown(e: KeyboardEvent) {
  if ((e.metaKey || e.ctrlKey) && e.key === 'Enter' && hasSelection.value) {
    e.preventDefault()
    openAnnotate()
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

onBeforeUnmount(() => {
  host?.view.destroy()
  host = null
})

defineExpose({ save })
</script>

<template>
  <div class="code-surface" @keydown="onKeydown">
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
    <div
      v-show="!loading"
      ref="hostEl"
      class="code-surface__editor"
      :class="{ 'is-readonly': readOnly }"
    />

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
</style>
