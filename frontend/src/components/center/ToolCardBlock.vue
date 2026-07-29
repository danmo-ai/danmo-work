<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { DqToolCard } from '@danqing/dq-shell'
import { useSessionsStore } from '@/stores/sessions'
import { friendlyToolDisplayName } from '@/utils/tool-display'

export interface ToolCardPayload {
  name: string
  description?: string
  status: string
  inputStr?: string
  output?: string
  error?: string
  stepNum?: number
}

const props = defineProps<{
  card: ToolCardPayload
  expanded: boolean
  awaitingApproval?: boolean
  awaitingLabel?: string
  childLinkLabel?: string
  showChildLink?: boolean
}>()

const emit = defineEmits<{
  toggle: []
  openChild: []
}>()

const { t } = useI18n()
const sessions = useSessionsStore()

const toolDisplayName = computed(() =>
  friendlyToolDisplayName(props.card.name, props.card.inputStr, sessions.agents, t),
)

function truncateText(s: string, max = 200): string {
  if (s.length <= max) return s
  return s.slice(0, max) + '…'
}

function toolInputFields(inputStr?: string): { key: string; value: string }[] | null {
  if (!inputStr) return null
  try {
    const obj = JSON.parse(inputStr) as unknown
    if (!obj || typeof obj !== 'object' || Array.isArray(obj)) return null
    return Object.entries(obj as Record<string, unknown>).map(([key, value]) => ({
      key,
      value: typeof value === 'string' ? value : JSON.stringify(value),
    }))
  } catch {
    return null
  }
}

const inputSummary = computed(() => {
  const fields = toolInputFields(props.card.inputStr)
  if (fields?.length) {
    const first = fields[0]
    return `${first.key}: ${truncateText(first.value, 80)}`
  }
  if (props.card.inputStr) return truncateText(props.card.inputStr, 100)
  return props.card.description || ''
})

const outputSummary = computed(() => {
  if (props.card.error) return truncateText(props.card.error, 100)
  if (props.card.output) return truncateText(props.card.output, 100)
  return ''
})

const fields = computed(() => toolInputFields(props.card.inputStr))

const statusLabel = computed(() => {
  if (props.awaitingApproval) return props.awaitingLabel || t('sessions.awaitingApproval')
  if (props.card.status === 'running') return t('sessions.running')
  if (props.card.status === 'cancelled') return t('sessions.cancelled')
  if (props.card.status === 'error') return t('sessions.failed')
  if (props.card.status === 'completed') return t('sessions.completed')
  return ''
})
</script>

<template>
  <DqToolCard
    :name="toolDisplayName"
    :status="card.status"
    :summary="inputSummary"
    :preview="outputSummary"
    :expanded="expanded"
    :awaiting="awaitingApproval"
    :awaiting-label="awaitingLabel || t('sessions.awaitingApproval')"
    :status-label="statusLabel"
    :show-link="showChildLink"
    :link-label="childLinkLabel || t('sessions.viewSubagent')"
    @toggle="emit('toggle')"
    @link="emit('openChild')"
  >
    <div v-if="card.inputStr" class="dq-tool-card__section">
      <span class="dq-tool-card__section-label">{{ t('sessions.toolInput') }}</span>
      <div v-if="fields" class="dq-tool-card__fields">
        <div v-for="field in fields" :key="field.key" class="dq-tool-card__field">
          <span class="dq-tool-card__field-key">{{ field.key }}</span>
          <span class="dq-tool-card__field-val" :title="field.value">{{ truncateText(field.value) }}</span>
        </div>
      </div>
      <pre v-else class="dq-code-block">{{ card.inputStr }}</pre>
    </div>

    <div v-if="card.output" class="dq-tool-card__section">
      <span class="dq-tool-card__section-label">{{ t('sessions.toolOutput') }}</span>
      <pre class="dq-code-block">{{ card.output }}</pre>
    </div>

    <div v-if="card.error" class="dq-tool-card__section dq-tool-card__section--error">
      <span class="dq-tool-card__section-label">{{ t('sessions.toolError') }}</span>
      <pre class="dq-code-block">{{ card.error }}</pre>
    </div>
  </DqToolCard>
</template>
