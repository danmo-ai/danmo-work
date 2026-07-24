<script setup lang="ts">
import { computed, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { useModelConfigStore } from '@/stores/modelLimits'
import { useSessionsStore } from '@/stores/sessions'
import { formatTokenCount, useSessionContextUsage } from '@/composables/useSessionContextUsage'

const { t } = useI18n()
const sessions = useSessionsStore()
const modelConfig = useModelConfigStore()

const {
  usedTokens,
  contextWindow,
  usageRatio,
  usageLevel,
  compactionHistory,
} = useSessionContextUsage()

onMounted(() => {
  if (!modelConfig.models.length) void modelConfig.load()
})

const percentLabel = computed(() => `${Math.round(usageRatio.value * 100)}%`)
const hasData = computed(() => usedTokens.value > 0 || compactionHistory.value.length > 0)
</script>

<template>
  <div
    v-if="sessions.currentSession"
    class="context-usage"
    :class="[`is-${usageLevel}`, { 'is-empty': !hasData }]"
  >
    <div class="context-usage__main" :title="t('sessions.contextUsageHint')">
      <svg class="context-usage__icon" viewBox="0 0 24 24" width="14" height="14" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true">
        <path d="M12 2v4M12 18v4M4.93 4.93l2.83 2.83M16.24 16.24l2.83 2.83M2 12h4M18 12h4M4.93 19.07l2.83-2.83M16.24 7.76l2.83-2.83" />
        <circle cx="12" cy="12" r="3" />
      </svg>
      <div class="context-usage__track" aria-hidden="true">
        <div class="context-usage__fill" :style="{ width: `${Math.round(usageRatio * 100)}%` }" />
      </div>
      <span class="context-usage__label">
        {{ formatTokenCount(usedTokens) }} / {{ formatTokenCount(contextWindow) }}
        <span class="context-usage__pct">({{ percentLabel }})</span>
      </span>
    </div>
  </div>
</template>

<style scoped>
.context-usage {
  flex-shrink: 0;
  display: flex;
  flex-direction: column;
  align-items: flex-end;
  padding: 0;
  position: relative;
}

.context-usage__main {
  display: flex;
  align-items: center;
  justify-content: flex-end;
  gap: 6px;
  min-height: 22px;
}

.context-usage__icon {
  flex-shrink: 0;
  color: var(--dq-label-tertiary);
}

.context-usage.is-warn .context-usage__icon,
.context-usage.is-warn .context-usage__pct {
  color: var(--dq-system-orange);
}

.context-usage.is-critical .context-usage__icon,
.context-usage.is-critical .context-usage__pct {
  color: var(--dq-danger);
}

.context-usage__track {
  flex: 0 1 96px;
  max-width: 96px;
  height: 3px;
  border-radius: 2px;
  background: color-mix(in srgb, var(--dq-label-primary) 10%, transparent);
  overflow: hidden;
}

.context-usage__fill {
  height: 100%;
  border-radius: 2px;
  background: var(--dq-accent);
  transition: width 0.25s ease;
}

.context-usage.is-warn .context-usage__fill {
  background: var(--dq-system-orange);
}

.context-usage.is-critical .context-usage__fill {
  background: var(--dq-danger);
}

.context-usage__label {
  font-size: var(--dq-font-size-caption);
  font-family: var(--dq-font-mono, ui-monospace, monospace);
  color: var(--dq-label-secondary);
  white-space: nowrap;
}

.context-usage__pct {
  color: var(--dq-label-tertiary);
}

.context-usage.is-empty .context-usage__fill {
  width: 0 !important;
}
</style>
