<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import { formatTokenCount } from '@/composables/useSessionContextUsage'
import type { StreamTurn } from '@/composables/useStreamTurns'
import UserMessageBlock from '@/components/center/UserMessageBlock.vue'

const props = defineProps<{
  turn: StreamTurn
  turnIndex: number
  /** Whole-turn body folded (user + timeline). Separate from mid-process event fold. */
  collapsed: boolean
  summary: {
    toolCount: number
    completedTools: number
    errorTools: number
    runningTools: number
    tokensUsed: number
  }
  showDivider?: boolean
}>()

const emit = defineEmits<{
  'toggle-collapse': []
  download: []
}>()

const { t } = useI18n()

function turnStatusLabel(status?: string) {
  const map: Record<string, string> = {
    running: t('sessions.running'),
    completed: t('sessions.completed'),
    done: t('sessions.completed'),
    failed: t('sessions.failed'),
    cancelled: t('sessions.cancelled'),
    timeout: t('sessions.timeout'),
    blocked: t('sessions.blocked'),
  }
  return status ? (map[status] ?? status) : ''
}

const userImages = () => props.turn.userImages?.map((img) => img.dataUrl) ?? []

const showStatus = () => {
  const s = props.turn.status
  return Boolean(s && s !== 'completed' && s !== 'done')
}
</script>

<template>
  <section class="turn-section">
    <div v-if="showDivider" class="turn-section__divider" />

    <div
      class="turn-section__header"
      :class="{
        'is-running': turn.status === 'running' || summary.runningTools > 0,
        'is-failed': turn.status === 'failed' || turn.status === 'timeout' || summary.errorTools > 0,
      }"
      @click="emit('toggle-collapse')"
    >
      <div class="turn-section__header-left">
        <button
          type="button"
          class="turn-section__collapse-btn"
          :class="{ 'is-collapsed': collapsed }"
          :aria-label="collapsed ? t('sessions.expandTurn') : t('sessions.collapseTurn')"
          :title="collapsed ? t('sessions.expandTurn') : t('sessions.collapseTurn')"
          @click.stop="emit('toggle-collapse')"
        >
          <svg viewBox="0 0 24 24" width="12" height="12" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
            <polyline points="6 9 12 15 18 9" />
          </svg>
        </button>
        <span class="turn-section__number">Turn {{ turnIndex + 1 }}</span>
        <span
          v-if="showStatus()"
          class="turn-section__status"
        >{{ turnStatusLabel(turn.status) }}</span>
        <span v-if="summary.runningTools > 0" class="turn-section__live-dot" />
      </div>

      <div class="turn-section__header-right">
        <div class="turn-section__summary-strip">
          <span v-if="summary.toolCount > 0" class="turn-section__summary-item">
            {{ summary.toolCount }} tools
          </span>
          <span v-if="summary.tokensUsed > 0" class="turn-section__summary-item">
            {{ formatTokenCount(summary.tokensUsed) }}
          </span>
        </div>
        <button type="button" class="turn-section__download-btn" :title="t('sessions.downloadTurnLog')" @click.stop="emit('download')">
          <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
            <path d="M21 15v4a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2v-4" />
            <polyline points="7 10 12 15 17 10" />
            <line x1="12" y1="15" x2="12" y2="3" />
          </svg>
        </button>
      </div>
    </div>

    <!-- Whole-turn fold hides user + timeline; mid-process event fold lives in the timeline slot. -->
    <div v-show="!collapsed" class="turn-section__body">
      <UserMessageBlock
        v-if="turn.userText || turn.userImages?.length"
        :text="turn.userText"
        :images="userImages()"
      />

      <div class="turn-section__agent">
        <div class="turn-section__timeline">
          <slot name="timeline" />
        </div>
      </div>
    </div>
  </section>
</template>

<style scoped>
.turn-section {
  display: flex;
  flex-direction: column;
  gap: var(--dq-chat-block-gap, 4px);
  padding-bottom: 6px;
}

.turn-section__divider {
  height: 1px;
  border: none;
  background: var(--dq-shell-divider);
  opacity: 0.7;
}

.turn-section__header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
  cursor: pointer;
  padding: 1px 0;
  border-radius: 0;
  transition: color 0.12s ease;
  user-select: none;
}

.turn-section__header:hover .turn-section__number,
.turn-section__header.is-running .turn-section__number,
.turn-section__header.is-failed .turn-section__number {
  color: var(--dq-label-secondary);
}

.turn-section__header-left,
.turn-section__header-right {
  display: flex;
  align-items: center;
  gap: 6px;
  min-width: 0;
}

.turn-section__header-right {
  gap: 8px;
  flex-shrink: 0;
}

.turn-section__collapse-btn {
  display: flex;
  align-items: center;
  justify-content: center;
  width: 20px;
  height: 20px;
  padding: 0;
  border: none;
  border-radius: 4px;
  background: transparent;
  color: var(--dq-label-tertiary);
  cursor: pointer;
  flex-shrink: 0;
  transition: transform 0.2s ease, color 0.12s ease;
}

.turn-section__collapse-btn:hover {
  color: var(--dq-label-primary);
}

.turn-section__collapse-btn.is-collapsed svg {
  transform: rotate(-90deg);
}

.turn-section__live-dot {
  width: 5px;
  height: 5px;
  border-radius: 50%;
  background: var(--dq-accent);
  flex-shrink: 0;
  box-shadow: 0 0 0 3px color-mix(in srgb, var(--dq-accent) 18%, transparent);
}

.turn-section__number {
  font-size: var(--dq-font-size-caption);
  font-weight: 600;
  letter-spacing: 0.05em;
  text-transform: uppercase;
  color: var(--dq-label-tertiary);
}

.turn-section__status {
  font-size: var(--dq-font-size-caption);
  font-weight: 500;
  color: var(--dq-label-secondary);
}

.turn-section__header.is-running .turn-section__status {
  color: var(--dq-accent);
}

.turn-section__header.is-failed .turn-section__status {
  color: var(--dq-danger);
}

.turn-section__summary-strip {
  display: flex;
  align-items: center;
  gap: 8px;
  font-size: var(--dq-font-size-caption);
  font-weight: 500;
  font-variant-numeric: tabular-nums;
  color: var(--dq-label-tertiary);
}

.turn-section__summary-item {
  display: inline-flex;
  align-items: center;
  line-height: 1.4;
}

.turn-section__download-btn {
  display: flex;
  align-items: center;
  justify-content: center;
  width: 26px;
  height: 26px;
  padding: 0;
  border: none;
  border-radius: 6px;
  background: transparent;
  color: var(--dq-label-tertiary);
  cursor: pointer;
  opacity: 0;
  transition: background 0.12s ease, color 0.12s ease, opacity 0.12s ease;
}

.turn-section__header:hover .turn-section__download-btn,
.turn-section__header:focus-within .turn-section__download-btn {
  opacity: 1;
}

.turn-section__download-btn:hover {
  background: color-mix(in srgb, var(--dq-label-primary) 8%, transparent);
  color: var(--dq-label-primary);
}

.turn-section__body {
  display: flex;
  flex-direction: column;
  gap: var(--dq-chat-block-gap, 4px);
}

.turn-section__agent {
  display: flex;
  margin-block: 4px 2px;
}

.turn-section__timeline {
  flex: 1;
  min-width: 0;
  display: flex;
  flex-direction: column;
  gap: 0;
}

.turn-section__timeline :deep(.turn__event) {
  display: flex;
  align-items: stretch;
  margin-top: 10px;
}

.turn-section__timeline :deep(.turn__event:first-child) {
  margin-top: 0;
}

.turn-section__timeline :deep(.turn__event > *) {
  flex: 1;
  min-width: 0;
}

/*
 * Process rail — thinking / tool / skill footnotes share one continuous
 * guide line, so the agent's working steps read as a quiet sub-column and
 * the final answer stands free (Claude/Cursor-style step hierarchy).
 */
.turn-section__timeline :deep(.turn__event:has(> .thinking-block)),
.turn-section__timeline :deep(.turn__event:has(> .tool-group)),
.turn-section__timeline :deep(.turn__event:has(> .dq-tool-card)),
.turn-section__timeline :deep(.turn__event:has(> .turn__skill)) {
  margin-top: 0;
  padding: 3px 0 3px 12px;
  border-left: 2px solid color-mix(in srgb, var(--dq-label-primary) 9%, transparent);
}

/* Rail segments that follow prose keep breathing room before the line resumes */
.turn-section__timeline
  :deep(
    .turn__event:not(:has(> .thinking-block, > .tool-group, > .dq-tool-card, > .turn__skill))
      + .turn__event:has(> .thinking-block, > .tool-group, > .dq-tool-card, > .turn__skill)
  ) {
  margin-top: 10px;
}

.turn-section__timeline :deep(.agent-msg) {
  margin-block: 4px 2px;
}
</style>
