<script setup lang="ts">
import { ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { toast } from '@/utils/feedback'

export interface AskUserFormField {
  name: string
  label: string
  type: 'text' | 'number' | 'select' | 'boolean'
  required?: boolean
  default?: unknown
  options?: string[]
  placeholder?: string
}

const props = withDefaults(
  defineProps<{
    payload: unknown
    anchorSeq?: number
    question: string
    options: string[]
    defaultOption?: string
    formFields: AskUserFormField[]
    resolved: boolean
    expired: boolean
    answering?: boolean
    answer?: string
    askId: string
    /** When false, show read-only context (timeline); actions live in Composer. */
    interactive?: boolean
  }>(),
  { interactive: true },
)

const emit = defineEmits<{
  resolve: [answer: string]
}>()

const { t } = useI18n()

const textValue = ref('')
const selectedOption = ref('')
const formValues = ref<Record<string, unknown>>({})

function initFormValues() {
  if (Object.keys(formValues.value).length > 0) return
  const vals: Record<string, unknown> = {}
  for (const f of props.formFields) {
    if (f.default !== undefined) {
      vals[f.name] = f.default
    } else if (f.type === 'boolean') {
      vals[f.name] = false
    } else if (f.type === 'number') {
      vals[f.name] = 0
    } else {
      vals[f.name] = ''
    }
  }
  formValues.value = vals
}

function initSelectedOption() {
  if (selectedOption.value) return
  const def = props.defaultOption ?? ''
  if (def && props.options.includes(def)) {
    selectedOption.value = def
  }
}

watch(
  () => [props.askId, props.formFields, props.options, props.defaultOption] as const,
  () => {
    if (props.formFields.length > 0) initFormValues()
    if (props.options.length > 0) initSelectedOption()
  },
  { immediate: true },
)

function submitForm() {
  initFormValues()
  for (const f of props.formFields) {
    const v = formValues.value[f.name]
    if (f.required && (v === '' || v === undefined || v === null)) {
      toast.warning(`请填写 ${f.label}`)
      return
    }
  }
  const lines = props.formFields.map((f) => {
    const v = formValues.value[f.name]
    const display = f.type === 'boolean' ? (v ? '是' : '否') : String(v ?? '')
    return `${f.label}: ${display}`
  })
  emit('resolve', lines.join('\n'))
}

function submitText(raw?: string) {
  const trimmed = (raw ?? textValue.value).trim()
  if (!trimmed) return
  emit('resolve', trimmed)
}

function pickOption(opt: string) {
  selectedOption.value = opt
  emit('resolve', opt)
}
</script>

<template>
  <div class="ask-user-block" :data-event-anchor="anchorSeq">
    <div class="ask-user-block__header">
      <svg class="ask-user-block__icon" viewBox="0 0 24 24" width="16" height="16" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true">
        <path d="M21 15a2 2 0 0 1-2 2H7l-4 4V5a2 2 0 0 1 2-2h14a2 2 0 0 1 2 2z" />
      </svg>
      <span class="ask-user-block__question">{{ question }}</span>
    </div>

    <template v-if="resolved">
      <div class="ask-user-block__answer">
        <span class="ask-user-block__answer-label">你的回复</span>
        <p class="ask-user-block__answer-text">{{ answer || '已回复' }}</p>
      </div>
    </template>

    <template v-else-if="expired">
      <span class="ask-user-block__expired">{{ t('sessions.askExpired') }}</span>
    </template>

    <template v-else-if="!interactive">
      <span class="ask-user-block__hint">{{ t('sessions.decideInComposer') }}</span>
    </template>

    <template v-else>
      <div v-if="formFields.length > 0" class="ask-user-block__form">
        <label v-for="field in formFields" :key="field.name" class="ask-user-block__form-field">
          <span class="ask-user-block__form-label">
            {{ field.label }}
            <span v-if="field.required" class="ask-user-block__form-required">*</span>
          </span>
          <input
            v-if="field.type === 'text' || field.type === 'number'"
            :type="field.type"
            :placeholder="field.placeholder ?? ''"
            :value="formValues[field.name]"
            class="ask-user-block__form-input"
            @input="formValues[field.name] = ($event.target as HTMLInputElement).value"
          />
          <select
            v-else-if="field.type === 'select'"
            :value="String(formValues[field.name] ?? '')"
            class="ask-user-block__form-input"
            @change="formValues[field.name] = ($event.target as HTMLSelectElement).value"
          >
            <option value="" disabled>请选择...</option>
            <option v-for="opt in field.options ?? []" :key="opt" :value="opt">{{ opt }}</option>
          </select>
          <DqSwitch
            v-else-if="field.type === 'boolean'"
            :model-value="Boolean(formValues[field.name])"
            size="sm"
            @update:model-value="(v: boolean) => (formValues[field.name] = v)"
          />
        </label>
        <DqButton type="primary" size="sm" :disabled="answering" @click="submitForm">提交</DqButton>
      </div>

      <template v-else-if="options.length > 0">
        <div class="ask-user-block__options">
          <DqButton
            v-for="opt in options"
            :key="opt"
            size="sm"
            :disabled="answering"
            :type="selectedOption === opt ? 'primary' : 'default'"
            @click="pickOption(opt)"
          >
            {{ opt }}
          </DqButton>
        </div>
        <div class="ask-user-block__input-row">
          <input
            v-model="textValue"
            placeholder="或输入自定义回答..."
            :disabled="answering"
            @keydown.enter="submitText()"
          />
          <DqButton type="primary" size="sm" :disabled="answering" @click="submitText()">回复</DqButton>
        </div>
      </template>

      <template v-else>
        <div class="ask-user-block__input-row">
          <input
            v-model="textValue"
            placeholder="输入你的回答..."
            :disabled="answering"
            @keydown.enter="submitText()"
          />
          <DqButton type="primary" size="sm" :disabled="answering" @click="submitText()">回复</DqButton>
        </div>
      </template>
    </template>
  </div>
</template>

<style scoped>
.ask-user-block {
  display: flex;
  flex-direction: column;
  gap: 10px;
  padding: 12px 14px;
  border-radius: 10px;
  border: 1px solid color-mix(in srgb, var(--dq-label-primary) 12%, transparent);
  background: color-mix(in srgb, var(--dq-label-primary) 3%, transparent);
  color: var(--dq-label-primary);
  font-size: var(--dq-font-size-body);
  scroll-margin-bottom: 96px;
}

.ask-user-block__header {
  display: flex;
  align-items: flex-start;
  gap: 8px;
}

.ask-user-block__icon {
  flex-shrink: 0;
  margin-top: 2px;
  color: var(--dq-accent);
  opacity: 0.75;
}

.ask-user-block__question {
  font-weight: 500;
  line-height: 1.45;
}

.ask-user-block__expired {
  font-size: var(--dq-font-size-footnote);
  color: var(--dq-label-tertiary);
}

.ask-user-block__hint {
  font-size: var(--dq-font-size-footnote);
  color: var(--dq-label-tertiary);
}

.ask-user-block__answer {
  display: flex;
  flex-direction: column;
  gap: 4px;
  padding: 8px 10px;
  border-radius: 8px;
  background: color-mix(in srgb, var(--dq-label-primary) 4%, transparent);
  border: 1px solid color-mix(in srgb, var(--dq-label-primary) 8%, transparent);
}

.ask-user-block__answer-label {
  font-size: var(--dq-font-size-caption);
  font-weight: 600;
  color: var(--dq-label-tertiary);
}

.ask-user-block__answer-text {
  margin: 0;
  font-size: var(--dq-font-size-body);
  line-height: 1.5;
  color: var(--dq-label-secondary);
  white-space: pre-wrap;
  word-break: break-word;
}

.ask-user-block__options {
  display: flex;
  flex-wrap: wrap;
  gap: 6px;
}

.ask-user-block__input-row {
  display: flex;
  gap: 8px;
  align-items: center;
}

.ask-user-block__input-row input {
  flex: 1;
  min-width: 0;
  padding: 7px 10px;
  border-radius: 8px;
  border: 1px solid color-mix(in srgb, var(--dq-label-primary) 14%, transparent);
  background: var(--dq-bg-base, transparent);
  color: var(--dq-label-primary);
  font-size: var(--dq-font-size-body);
  outline: none;
}

.ask-user-block__input-row input:focus {
  border-color: color-mix(in srgb, var(--dq-accent) 45%, transparent);
}

.ask-user-block__form {
  display: flex;
  flex-direction: column;
  gap: 10px;
}

.ask-user-block__form-field {
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.ask-user-block__form-label {
  font-size: var(--dq-font-size-footnote);
  font-weight: 500;
  color: var(--dq-label-secondary);
}

.ask-user-block__form-required {
  color: var(--dq-danger);
}

.ask-user-block__form-input {
  padding: 7px 10px;
  border-radius: 8px;
  border: 1px solid color-mix(in srgb, var(--dq-label-primary) 14%, transparent);
  background: var(--dq-bg-base, transparent);
  color: var(--dq-label-primary);
  font-size: var(--dq-font-size-body);
  outline: none;
}

.ask-user-block__form-input:focus {
  border-color: color-mix(in srgb, var(--dq-accent) 45%, transparent);
}
</style>
