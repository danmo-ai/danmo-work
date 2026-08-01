<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import ToolCardBlock, { type ToolCardPayload } from '@/components/center/ToolCardBlock.vue'
import { useSessionsStore } from '@/stores/sessions'
import { friendlyToolDisplayName } from '@/utils/tool-display'

export interface ToolGroupCard extends ToolCardPayload {
  callId: string
  seq: number
}

const props = defineProps<{
  cards: ToolGroupCard[]
  expanded: boolean
  isCardExpanded: (seq: number) => boolean
  cardAwaitingApproval?: (seq: number) => boolean
  cardAwaitingLabel?: (seq: number) => string
  cardShowChildLink?: (seq: number) => boolean
  cardChildLinkLabel?: (seq: number) => string
}>()

const emit = defineEmits<{
  toggle: []
  toggleCard: [seq: number]
  openChild: [seq: number]
}>()

const { t } = useI18n()
const sessions = useSessionsStore()

const counts = computed(() => {
  let completed = 0
  let error = 0
  let running = 0
  let cancelled = 0
  for (const c of props.cards) {
    if (c.status === 'completed') completed++
    else if (c.status === 'error') error++
    else if (c.status === 'cancelled') cancelled++
    else if (c.status === 'running' || c.status === 'pending') running++
  }
  return { completed, error, running, cancelled, total: props.cards.length }
})

const hasRunning = computed(() => counts.value.running > 0)

const nameSummary = computed(() => {
  const names = props.cards
    .map((c) => friendlyToolDisplayName(c.name, c.inputStr, sessions.agents, t))
    .filter(Boolean)
  if (names.length <= 3) return names.join(', ')
  return `${names.slice(0, 3).join(', ')} +${names.length - 3}`
})

const statusHint = computed(() => {
  if (counts.value.running > 0) return t('sessions.toolsRunning', { n: counts.value.running })
  if (counts.value.error > 0) return t('sessions.toolsError', { n: counts.value.error })
  return ''
})
</script>

<template>
  <div
    class="tool-group"
    :class="{
      'is-expanded': expanded,
      'is-running': hasRunning,
      'is-error': counts.error > 0 && !hasRunning,
    }"
  >
    <button
      type="button"
      class="tool-group__header"
      :title="nameSummary"
      @click="emit('toggle')"
    >
      <span class="tool-group__dot" aria-hidden="true" />
      <span class="tool-group__label">
        <span class="tool-group__title">{{ t('sessions.toolsGroup', { n: counts.total }) }}</span>
        <span v-if="!expanded && nameSummary" class="tool-group__names">{{ nameSummary }}</span>
        <span v-if="statusHint" class="tool-group__hint">{{ statusHint }}</span>
      </span>
      <svg
        class="tool-group__chevron"
        :class="{ 'is-open': expanded }"
        viewBox="0 0 24 24"
        width="12"
        height="12"
        fill="none"
        stroke="currentColor"
        stroke-width="2"
        stroke-linecap="round"
        stroke-linejoin="round"
        aria-hidden="true"
      >
        <polyline points="6 9 12 15 18 9" />
      </svg>
    </button>

    <div v-show="expanded" class="tool-group__body">
      <ToolCardBlock
        v-for="card in cards"
        :key="card.seq"
        :card="card"
        :expanded="isCardExpanded(card.seq)"
        :awaiting-approval="cardAwaitingApproval?.(card.seq)"
        :awaiting-label="cardAwaitingLabel?.(card.seq)"
        :show-child-link="cardShowChildLink?.(card.seq)"
        :child-link-label="cardChildLinkLabel?.(card.seq)"
        @toggle="emit('toggleCard', card.seq)"
        @open-child="emit('openChild', card.seq)"
      />
    </div>
  </div>
</template>

<style scoped>
/* Quiet footnote row — not a dashboard card wall */
.tool-group {
  border: none;
  background: transparent;
  border-radius: 0;
}

.tool-group__header {
  display: flex;
  align-items: center;
  gap: 7px;
  width: 100%;
  min-height: 24px;
  padding: 0;
  border: none;
  background: transparent;
  color: inherit;
  font: inherit;
  text-align: left;
  cursor: pointer;
  transition: color 0.12s ease;
}

.tool-group__dot {
  width: 5px;
  height: 5px;
  border-radius: 50%;
  flex-shrink: 0;
  background: var(--dq-label-quaternary);
}

.tool-group.is-running .tool-group__dot {
  background: var(--dq-accent);
  box-shadow: 0 0 0 3px color-mix(in srgb, var(--dq-accent) 22%, transparent);
}

.tool-group.is-error .tool-group__dot {
  background: var(--dq-danger);
}

.tool-group:not(.is-running):not(.is-error) .tool-group__dot {
  background: var(--dq-label-quaternary);
}

.tool-group__label {
  display: flex;
  align-items: baseline;
  gap: 8px;
  min-width: 0;
  flex: 1;
}

.tool-group__title {
  flex-shrink: 0;
  font-size: var(--dq-font-size-body);
  font-weight: 500;
  color: var(--dq-label-tertiary);
}

.tool-group__names {
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  font-size: var(--dq-font-size-caption);
  color: var(--dq-label-tertiary);
  font-family: var(--dq-font-mono, ui-monospace, monospace);
}

.tool-group__header:hover .tool-group__title,
.tool-group.is-expanded .tool-group__title {
  color: var(--dq-label-secondary);
}

.tool-group.is-running .tool-group__title,
.tool-group.is-error .tool-group__title {
  font-weight: 600;
  color: var(--dq-label-primary);
}

.tool-group.is-running .tool-group__names,
.tool-group.is-error .tool-group__names {
  font-size: var(--dq-font-size-body);
  color: var(--dq-label-secondary);
}

.tool-group__hint {
  flex-shrink: 0;
  font-size: var(--dq-font-size-caption);
  color: var(--dq-label-secondary);
  font-weight: 500;
}

.tool-group.is-running .tool-group__hint {
  color: var(--dq-accent);
}

.tool-group.is-error .tool-group__hint {
  color: var(--dq-danger);
}

.tool-group__chevron {
  flex-shrink: 0;
  color: var(--dq-label-quaternary);
  transition: transform 0.15s ease;
}

.tool-group__chevron.is-open {
  transform: rotate(180deg);
}

.tool-group__body {
  display: flex;
  flex-direction: column;
  gap: 2px;
  margin: 4px 0 8px;
  padding: 4px 0 4px 13px;
  border-left: 1px solid color-mix(in srgb, var(--dq-label-primary) 8%, transparent);
}

.tool-group__body :deep(.dq-tool-card),
.tool-group__body :deep(.dq-tool-card.is-running),
.tool-group__body :deep(.dq-tool-card.is-awaiting),
.tool-group__body :deep(.dq-tool-card.is-error) {
  border: none !important;
  background: transparent;
  border-radius: 0;
  box-shadow: none;
}

.tool-group__body :deep(.dq-tool-card__header) {
  min-height: 24px;
  padding: 1px 0;
}

.tool-group__body :deep(.dq-tool-card__header:hover) {
  background: transparent;
}

.tool-group__body :deep(.dq-tool-card__name) {
  font-weight: 500;
  color: var(--dq-label-tertiary);
}

.tool-group__body :deep(.dq-tool-card__body) {
  border-top: none;
  padding: 4px 0 8px 0;
}

.tool-group__body :deep(.dq-tool-card__badge.is-completed) {
  color: var(--dq-label-quaternary);
}
</style>
