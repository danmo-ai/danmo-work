<script setup lang="ts">
import { computed, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import {
  type EditableFileKind,
  type FileEditScope,
  type FileEngine,
} from '@/utils/file-route'
import {
  createOfficeEditAttachment,
  type OfficeEditAttachment,
  type OfficeEditAction,
} from '@/types/office-edit-attachment'
import { toast } from '@/utils/feedback'

const props = defineProps<{
  path: string
  kind: EditableFileKind
  engine?: FileEngine
  getSelectionMarkdown: () => string
  getSelectionLines?: () => { startLine: number; endLine: number } | null
  getEditScope: () => FileEditScope
  ensureSaved: () => Promise<boolean>
  scope: FileEditScope
  pageIndex?: number
  disabled?: boolean
}>()

const emit = defineEmits<{
  attachOfficeEdit: [att: OfficeEditAttachment]
}>()

const { t } = useI18n()
const instruction = ref('')
const busy = ref(false)

const scopeHint = computed(() => {
  if (props.scope === 'selection') return t('office.scopeSelection')
  if (props.scope === 'document') return t('office.scopeDocument')
  if (props.scope === 'slide') return t('office.scopeSlide')
  return t('office.scopeSheet')
})

async function run(
  action: OfficeEditAction,
  opts?: {
    instruction?: string
    selection?: string
    scope?: FileEditScope
    startLine?: number
    endLine?: number
  },
) {
  if (busy.value || props.disabled) return

  busy.value = true
  try {
    const saved = await props.ensureSaved()
    if (!saved) return

    let selection = opts?.selection ?? props.getSelectionMarkdown()
    const scope = opts?.scope ?? props.getEditScope()
    if (!selection.trim()) {
      if (action === 'continue') {
        selection = '(cursor / end of document)'
      } else {
        toast.error(t('office.needContent'))
        return
      }
    }

    const mappedAction: OfficeEditAction =
      props.kind === 'slides' && action === 'modify'
        ? 'slide-page'
        : props.kind === 'sheet' && action === 'modify'
          ? 'sheet'
          : action

    const lineRange =
      opts?.startLine != null && opts?.endLine != null
        ? { startLine: opts.startLine, endLine: opts.endLine }
        : scope === 'selection'
          ? props.getSelectionLines?.() ?? null
          : null

    let note = opts?.instruction !== undefined ? opts.instruction : instruction.value
    // Polish / expand carry a default editable note when the user left the field empty.
    if (!note.trim()) {
      if (mappedAction === 'polish') note = t('office.defaultPolishNote')
      else if (mappedAction === 'continue') note = t('office.defaultExpandNote')
    }
    const att = createOfficeEditAttachment({
      action: mappedAction,
      path: props.path,
      officeKind: props.kind,
      scope,
      selection,
      instruction: note,
      pageIndex: props.pageIndex,
      startLine: lineRange?.startLine,
      endLine: lineRange?.endLine,
      engine: props.engine,
    })

    emit('attachOfficeEdit', att)
    instruction.value = ''
    toast.success(t('office.officeAttached'))
  } catch (e) {
    toast.error(e instanceof Error ? e.message : t('office.attachFailed'))
  } finally {
    busy.value = false
  }
}

defineExpose({ run })
</script>

<template>
  <div class="office-ai">
    <span class="office-ai__scope" :title="scopeHint">{{ scopeHint }}</span>
    <input
      v-model="instruction"
      class="office-ai__input"
      :placeholder="t('office.instructionPlaceholder')"
      :disabled="disabled || busy"
      @keydown.enter.prevent="run('modify')"
    />
    <button class="office-ai__btn" :disabled="disabled || busy" @click="run('polish')">
      {{ t('office.bubbleAiPolish') }}
    </button>
    <button class="office-ai__btn" :disabled="disabled || busy" @click="run('continue')">
      {{ t('office.bubbleAiExpand') }}
    </button>
    <button class="office-ai__btn" :disabled="disabled || busy" @click="run('modify')">
      {{ t('office.bubbleAiModify') }}
    </button>
  </div>
</template>

<style scoped>
.office-ai {
  display: flex;
  align-items: center;
  gap: 6px;
  flex-wrap: wrap;
  min-width: 0;
}
.office-ai__scope {
  flex: 0 0 auto;
  font-size: var(--dq-font-size-caption);
  color: var(--dq-label-tertiary);
  white-space: nowrap;
  padding: 0 2px;
}
.office-ai__input {
  flex: 1 1 140px;
  min-width: 100px;
  height: 28px;
  padding: 0 8px;
  border: 1px solid var(--dq-border);
  border-radius: 6px;
  background: var(--dq-bg-elevated);
  color: var(--dq-label-primary);
  font-size: var(--dq-font-size-caption);
}
.office-ai__input::placeholder {
  color: var(--dq-label-quaternary);
}
.office-ai__input:focus {
  outline: none;
  border-color: var(--dq-accent);
  box-shadow: 0 0 0 3px color-mix(in srgb, var(--dq-accent) 12%, transparent);
}
.office-ai__btn {
  height: 28px;
  padding: 0 10px;
  border: 1px solid var(--dq-border);
  border-radius: 6px;
  background: var(--dq-fill-tertiary);
  color: var(--dq-label-primary);
  font-size: var(--dq-font-size-caption);
  cursor: pointer;
  white-space: nowrap;
}
.office-ai__btn:hover:not(:disabled) {
  background: color-mix(in srgb, var(--dq-label-primary) 8%, var(--dq-fill-tertiary));
}
.office-ai__btn:disabled {
  opacity: 0.5;
  cursor: not-allowed;
}
.office-ai__btn--primary {
  background: var(--dq-accent);
  border-color: transparent;
  color: var(--dq-on-accent);
}
.office-ai__btn--primary:hover:not(:disabled) {
  background: var(--dq-accent-hover);
}
</style>
