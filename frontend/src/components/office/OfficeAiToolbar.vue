<script setup lang="ts">
import { ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { useSessionsStore } from '@/stores/sessions'
import { buildOfficeEditPrompt, type OfficeKind } from '@/utils/office-route'
import { toast } from '@/utils/feedback'

const props = defineProps<{
  path: string
  kind: OfficeKind
  getSelectionMarkdown: () => string
  pageIndex?: number
  disabled?: boolean
}>()

const emit = defineEmits<{
  started: []
}>()

const { t } = useI18n()
const sessions = useSessionsStore()
const instruction = ref('')
const busy = ref(false)

async function run(action: 'polish' | 'modify' | 'continue' | 'slide-page' | 'sheet') {
  if (busy.value || props.disabled) return
  if (!sessions.currentSessionId) {
    toast.error(t('office.needSession'))
    return
  }
  let selection = props.getSelectionMarkdown()
  if (!selection.trim() && action !== 'continue') {
    toast.error(t('office.needSelection'))
    return
  }
  if (!selection.trim()) selection = '(cursor / end of document)'

  const mappedAction =
    props.kind === 'slides' && action === 'modify'
      ? 'slide-page'
      : props.kind === 'sheet' && action === 'modify'
        ? 'sheet'
        : action

  const prompt = buildOfficeEditPrompt({
    action: mappedAction,
    path: props.path,
    kind: props.kind,
    selection,
    instruction: instruction.value,
    pageIndex: props.pageIndex,
  })

  busy.value = true
  try {
    await sessions.sendTurn(prompt)
    instruction.value = ''
    emit('started')
    toast.success(t('office.turnStarted'))
  } catch (e) {
    toast.error(e instanceof Error ? e.message : t('office.turnFailed'))
  } finally {
    busy.value = false
  }
}
</script>

<template>
  <div class="office-ai">
    <input
      v-model="instruction"
      class="office-ai__input"
      :placeholder="t('office.instructionPlaceholder')"
      :disabled="disabled || busy"
      @keydown.enter.prevent="run('modify')"
    />
    <button class="office-ai__btn" :disabled="disabled || busy" @click="run('polish')">
      {{ t('office.polish') }}
    </button>
    <button class="office-ai__btn office-ai__btn--primary" :disabled="disabled || busy" @click="run('modify')">
      {{ t('office.modify') }}
    </button>
    <button v-if="kind === 'doc'" class="office-ai__btn" :disabled="disabled || busy" @click="run('continue')">
      {{ t('office.continue') }}
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
.office-ai__input {
  flex: 1 1 140px;
  min-width: 100px;
  height: 28px;
  padding: 0 8px;
  border: 1px solid var(--dq-border, #e5e7eb);
  border-radius: 6px;
  background: var(--dq-bg, #fff);
  color: inherit;
  font-size: 12px;
}
.office-ai__btn {
  height: 28px;
  padding: 0 10px;
  border: 1px solid var(--dq-border, #e5e7eb);
  border-radius: 6px;
  background: var(--dq-bg-subtle, #f9fafb);
  font-size: 12px;
  cursor: pointer;
  white-space: nowrap;
}
.office-ai__btn:disabled {
  opacity: 0.5;
  cursor: not-allowed;
}
.office-ai__btn--primary {
  background: var(--dq-accent, #2563eb);
  border-color: transparent;
  color: #fff;
}
</style>
