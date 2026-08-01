<script setup lang="ts">
import { computed, ref, watch } from 'vue'
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
const answerExpanded = ref(false)

const pending = computed(() => !props.resolved && !props.expired)
const answerText = computed(() => (props.answer || '').trim())
const answerPreview = computed(() => {
  const raw = answerText.value
  if (!raw) return t('sessions.askAnswered')
  const oneLine = raw.replace(/\s+/g, ' ')
  if (oneLine.length <= 72) return oneLine
  return `${oneLine.slice(0, 72)}…`
})
const answerTruncated = computed(() => {
  const raw = answerText.value
  if (!raw) return false
  return raw.includes('\n') || raw.replace(/\s+/g, ' ').length > 72
})

/** Single text/number field → compact question + input row (no redundant label). */
const singleSimpleField = computed(() => {
  if (props.formFields.length !== 1) return null
  const f = props.formFields[0]
  if (f.type === 'text' || f.type === 'number') return f
  return null
})

function isRecommended(opt: string): boolean {
  const def = (props.defaultOption ?? '').trim()
  return Boolean(def && def === opt)
}

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
    formValues.value = {}
    selectedOption.value = ''
    textValue.value = ''
    answerExpanded.value = false
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
      toast.warning(t('sessions.askRequiredToast', { label: f.label }))
      return
    }
  }
  // Keep 是/否 in the submitted answer string so IM formatFormAnswer stays compatible.
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

function toggleAnswer() {
  if (!answerTruncated.value) return
  answerExpanded.value = !answerExpanded.value
}
</script>

<template>
  <div
    v-if="resolved && !pending"
    class="ask-user-block ask-user-block--settled"
    :data-event-anchor="anchorSeq"
    :title="answerText || t('sessions.askAnswered')"
  >
    <button
      v-if="answerTruncated"
      type="button"
      class="ask-user-block__settled-btn"
      @click="toggleAnswer"
    >
      <span class="ask-user-block__settled-text">
        {{ t('sessions.askAnswered') }} · {{ answerExpanded ? answerText : answerPreview }}
      </span>
    </button>
    <span v-else class="ask-user-block__settled-text">
      {{ t('sessions.askAnswered') }} · {{ answerPreview }}
    </span>
  </div>

  <div
    v-else-if="pending && !interactive"
    class="ask-user-block ask-user-block--timeline"
    :data-event-anchor="anchorSeq"
  >
    <span class="ask-user-block__timeline-text">
      {{ t('sessions.askBadge') }} · {{ question }}
    </span>
    <span class="ask-user-block__hint">{{ t('sessions.decideInComposer') }}</span>
  </div>

  <div
    v-else-if="expired"
    class="ask-user-block ask-user-block--timeline"
    :data-event-anchor="anchorSeq"
  >
    <span class="ask-user-block__timeline-text">{{ question }}</span>
    <span class="ask-user-block__hint">{{ t('sessions.askExpired') }}</span>
  </div>

  <div
    v-else
    class="ask-user-block is-interactive"
    :data-event-anchor="anchorSeq"
  >
    <p class="ask-user-block__question">{{ question }}</p>

    <!-- Single text field: input + submit on one row -->
    <div v-if="singleSimpleField" class="ask-user-block__combo">
      <input
        :type="singleSimpleField.type"
        :placeholder="singleSimpleField.placeholder || t('sessions.askPlaceholder')"
        :value="formValues[singleSimpleField.name]"
        class="ask-user-block__form-input"
        :disabled="answering"
        @input="formValues[singleSimpleField.name] = ($event.target as HTMLInputElement).value"
        @keydown.enter="submitForm()"
      />
      <DqButton type="primary" size="sm" :disabled="answering" @click="submitForm">
        {{ t('sessions.askSubmit') }}
      </DqButton>
    </div>

    <div v-else-if="formFields.length > 0" class="ask-user-block__form">
      <label
        v-for="field in formFields"
        :key="field.name"
        class="ask-user-block__form-field"
        :class="{ 'ask-user-block__form-field--bool': field.type === 'boolean' }"
      >
        <span class="ask-user-block__form-label">
          {{ field.label }}
          <span v-if="field.required" class="ask-user-block__form-required">*</span>
        </span>
        <input
          v-if="field.type === 'text' || field.type === 'number'"
          :type="field.type"
          :placeholder="field.placeholder || t('sessions.askPlaceholder')"
          :value="formValues[field.name]"
          class="ask-user-block__form-input"
          :disabled="answering"
          @input="formValues[field.name] = ($event.target as HTMLInputElement).value"
        />
        <div v-else-if="field.type === 'select'" class="ask-user-block__options">
          <DqButton
            v-for="opt in field.options ?? []"
            :key="opt"
            size="sm"
            :disabled="answering"
            :type="String(formValues[field.name] ?? '') === opt ? 'primary' : 'default'"
            @click="formValues[field.name] = opt"
          >
            {{ opt }}
          </DqButton>
        </div>
        <DqSwitch
          v-else-if="field.type === 'boolean'"
          :model-value="Boolean(formValues[field.name])"
          size="sm"
          :disabled="answering"
          @update:model-value="(v: boolean) => (formValues[field.name] = v)"
        />
      </label>
      <div class="ask-user-block__actions">
        <DqButton type="primary" size="sm" :disabled="answering" @click="submitForm">
          {{ t('sessions.askSubmit') }}
        </DqButton>
      </div>
    </div>

    <template v-else-if="options.length > 0">
      <div class="ask-user-block__options">
        <DqButton
          v-for="opt in options"
          :key="opt"
          size="sm"
          :disabled="answering"
          :type="isRecommended(opt) || selectedOption === opt ? 'primary' : 'default'"
          :title="isRecommended(opt) ? t('sessions.askRecommended') : undefined"
          @click="pickOption(opt)"
        >
          {{ opt }}
        </DqButton>
      </div>
      <div class="ask-user-block__combo ask-user-block__combo--secondary">
        <input
          v-model="textValue"
          :placeholder="t('sessions.askCustomPlaceholder')"
          :disabled="answering"
          @keydown.enter="submitText()"
        />
        <DqButton type="default" size="sm" :disabled="answering" @click="submitText()">
          {{ t('sessions.askReply') }}
        </DqButton>
      </div>
    </template>

    <div v-else class="ask-user-block__combo">
      <input
        v-model="textValue"
        :placeholder="t('sessions.askPlaceholder')"
        :disabled="answering"
        @keydown.enter="submitText()"
      />
      <DqButton type="primary" size="sm" :disabled="answering" @click="submitText()">
        {{ t('sessions.askReply') }}
      </DqButton>
    </div>
  </div>
</template>

<style scoped>
.ask-user-block {
  display: flex;
  flex-direction: column;
  gap: 10px;
  color: var(--dq-label-primary);
  font-size: var(--dq-font-size-body);
  scroll-margin-bottom: 96px;
}

.ask-user-block.is-interactive {
  padding: 0;
  border: none;
  background: transparent;
}

.ask-user-block--timeline {
  gap: 2px;
  padding: 0;
  border: none;
  background: transparent;
  font-size: var(--dq-font-size-caption);
}

.ask-user-block__timeline-text {
  color: var(--dq-label-secondary);
  line-height: 1.45;
  word-break: break-word;
}

.ask-user-block__question {
  margin: 0;
  font-size: 15px;
  font-weight: 600;
  line-height: 1.5;
  letter-spacing: 0.01em;
  word-break: break-word;
}

.ask-user-block__hint {
  margin: 0;
  font-size: var(--dq-font-size-footnote);
  color: var(--dq-label-tertiary);
}

.ask-user-block--settled {
  display: flex;
  flex-direction: row;
  align-items: center;
  min-height: 22px;
  padding: 0;
  border: none;
  background: transparent;
  font-size: var(--dq-font-size-caption);
}

.ask-user-block__settled-text {
  color: var(--dq-label-secondary);
  white-space: pre-wrap;
  word-break: break-word;
  line-height: 1.45;
}

.ask-user-block__settled-btn {
  margin: 0;
  padding: 0;
  border: 0;
  background: transparent;
  text-align: left;
  cursor: pointer;
  color: inherit;
  font: inherit;
}

.ask-user-block__settled-btn:hover .ask-user-block__settled-text {
  color: var(--dq-label-primary);
}

.ask-user-block__options {
  display: flex;
  flex-wrap: wrap;
  gap: 6px;
}

.ask-user-block__combo {
  display: flex;
  gap: 8px;
  align-items: center;
}

.ask-user-block__combo--secondary {
  padding-top: 2px;
}

.ask-user-block__combo input,
.ask-user-block__form-input {
  flex: 1;
  min-width: 0;
  height: 36px;
  padding: 0 12px;
  border-radius: 9px;
  border: 1px solid color-mix(in srgb, var(--dq-accent) 28%, transparent);
  background: color-mix(in srgb, var(--dq-bg-base, #000) 55%, transparent);
  color: var(--dq-label-primary);
  font-size: var(--dq-font-size-body);
  outline: none;
}

.ask-user-block__combo input:focus,
.ask-user-block__form-input:focus {
  border-color: color-mix(in srgb, var(--dq-accent) 65%, transparent);
  box-shadow: 0 0 0 3px color-mix(in srgb, var(--dq-accent) 16%, transparent);
}

.ask-user-block__combo input:disabled,
.ask-user-block__form-input:disabled {
  opacity: 0.6;
  cursor: not-allowed;
}

.ask-user-block__form {
  display: flex;
  flex-direction: column;
  gap: 10px;
}

.ask-user-block__form-field {
  display: flex;
  flex-direction: column;
  gap: 5px;
}

.ask-user-block__form-field--bool {
  flex-direction: row;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  min-height: 32px;
}

.ask-user-block__form-label {
  font-size: var(--dq-font-size-footnote);
  font-weight: 600;
  color: var(--dq-label-secondary);
}

.ask-user-block__form-required {
  color: var(--dq-danger);
}

.ask-user-block__actions {
  display: flex;
  flex-wrap: wrap;
  gap: 6px;
}
</style>
