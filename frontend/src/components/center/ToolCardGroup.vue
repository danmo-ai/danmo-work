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
    <div class="tool-group__header" @click="emit('toggle')">
      <div class="tool-group__meta">
        <span class="tool-group__title">{{ t('sessions.toolsGroup', { n: counts.total }) }}</span>
        <span v-if="!expanded && nameSummary" class="tool-group__names">{{ nameSummary }}</span>
      </div>
      <div class="tool-group__actions">
        <span v-if="counts.running > 0" class="tool-group__badge is-running">
          <span class="tool-group__spinner" />
          {{ counts.running }}
        </span>
        <span v-if="counts.completed > 0" class="tool-group__badge is-done">✓ {{ counts.completed }}</span>
        <span v-if="counts.error > 0" class="tool-group__badge is-error">✗ {{ counts.error }}</span>
        <span v-if="counts.cancelled > 0" class="tool-group__badge is-cancelled">{{ counts.cancelled }}</span>
        <svg
          class="tool-group__chevron"
          :class="{ 'is-open': expanded }"
          viewBox="0 0 24 24"
          width="14"
          height="14"
          fill="none"
          stroke="currentColor"
          stroke-width="2"
          stroke-linecap="round"
          stroke-linejoin="round"
        >
          <polyline points="6 9 12 15 18 9" />
        </svg>
      </div>
    </div>

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
.tool-group {
  border-radius: 10px;
  border: 1px solid color-mix(in srgb, var(--dq-label-primary) 8%, transparent);
  background: color-mix(in srgb, var(--dq-label-primary) 2%, transparent);
  overflow: hidden;
}

.tool-group.is-running {
  border-color: color-mix(in srgb, var(--dq-accent) 30%, transparent);
}

.tool-group.is-error {
  border-color: color-mix(in srgb, var(--dq-danger) 28%, transparent);
}

.tool-group__header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
  padding: 8px 10px;
  cursor: pointer;
  user-select: none;
}

.tool-group__meta {
  display: flex;
  align-items: center;
  gap: 8px;
  min-width: 0;
  flex: 1;
}

.tool-group__title {
  flex-shrink: 0;
  font-weight: 600;
  font-size: var(--dq-font-size-footnote);
  color: var(--dq-label-primary);
}

.tool-group__names {
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  font-size: var(--dq-font-size-caption);
  font-family: var(--dq-font-mono, ui-monospace, monospace);
  color: var(--dq-label-tertiary);
}

.tool-group__actions {
  display: flex;
  align-items: center;
  gap: 6px;
  flex-shrink: 0;
}

.tool-group__badge {
  display: inline-flex;
  align-items: center;
  gap: 4px;
  padding: 2px 7px;
  border-radius: 8px;
  font-size: var(--dq-font-size-caption);
  font-weight: 600;
}

.tool-group__badge.is-running {
  background: color-mix(in srgb, var(--dq-accent) 12%, transparent);
  color: var(--dq-accent);
}

.tool-group__badge.is-done {
  background: color-mix(in srgb, var(--dq-success) 12%, transparent);
  color: var(--dq-success);
}

.tool-group__badge.is-error {
  background: color-mix(in srgb, var(--dq-danger) 12%, transparent);
  color: var(--dq-danger);
}

.tool-group__badge.is-cancelled {
  background: color-mix(in srgb, var(--dq-label-primary) 8%, transparent);
  color: var(--dq-label-secondary);
}

.tool-group__spinner {
  width: 8px;
  height: 8px;
  border: 1.5px solid currentColor;
  border-right-color: transparent;
  border-radius: 50%;
  animation: tool-group-spin 0.7s linear infinite;
}

.tool-group__chevron {
  color: var(--dq-label-tertiary);
  transition: transform 0.15s ease;
}

.tool-group__chevron.is-open {
  transform: rotate(180deg);
}

.tool-group__body {
  display: flex;
  flex-direction: column;
  gap: 4px;
  padding: 0 6px 6px;
  border-top: 1px solid color-mix(in srgb, var(--dq-label-primary) 6%, transparent);
}

.tool-group__body :deep(.dq-tool-card) {
  border-radius: 8px;
  background: color-mix(in srgb, var(--dq-label-primary) 2.5%, transparent);
}

@keyframes tool-group-spin {
  to {
    transform: rotate(360deg);
  }
}
</style>
