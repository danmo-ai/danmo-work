<script setup lang="ts">
import { computed } from 'vue'

const props = defineProps<{
  text: string
  expanded: boolean
  seq: number
}>()

const emit = defineEmits<{
  toggle: [seq: number]
}>()

function truncatePreview(s: string, max = 80): string {
  if (s.length <= max) return s
  return s.slice(0, max) + '…'
}

function formatLen(s: string): string {
  const n = s.length
  if (n < 1000) return String(n)
  return (n / 1000).toFixed(1).replace(/\.0$/, '') + 'k'
}

const preview = computed(() => truncatePreview(props.text))
</script>

<template>
  <div class="thinking-block" :class="{ 'is-expanded': expanded }">
    <button type="button" class="thinking-block__header" @click="emit('toggle', seq)">
      <span v-if="!expanded" class="thinking-block__preview">{{ preview }}</span>
      <span v-else class="thinking-block__hint">思考过程</span>
      <span class="thinking-block__meta">{{ formatLen(text) }}</span>
      <svg
        class="thinking-block__chevron"
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
    </button>
    <div v-if="expanded" class="thinking-block__body">{{ text }}</div>
  </div>
</template>

<style scoped>
.thinking-block {
  margin: 2px 0;
}

.thinking-block__header {
  display: flex;
  align-items: center;
  gap: 8px;
  width: 100%;
  padding: 5px 6px;
  border: none;
  border-radius: 6px;
  background: transparent;
  color: var(--dq-label-secondary);
  cursor: pointer;
  text-align: left;
  font: inherit;
}

.thinking-block__header:hover {
  background: color-mix(in srgb, var(--dq-label-primary) 4%, transparent);
}

.thinking-block__preview {
  flex: 1;
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  font-size: var(--dq-font-size-body);
  color: var(--dq-label-secondary);
  font-style: normal;
}

.thinking-block__hint {
  flex: 1;
  min-width: 0;
  font-size: var(--dq-font-size-body);
  color: var(--dq-label-secondary);
}

.thinking-block__meta {
  flex-shrink: 0;
  font-size: var(--dq-font-size-caption);
  color: var(--dq-label-secondary);
  font-variant-numeric: tabular-nums;
}

.thinking-block__chevron {
  flex-shrink: 0;
  opacity: 0.45;
  transition: transform 0.15s ease;
}

.thinking-block__chevron.is-open {
  transform: rotate(180deg);
}

.thinking-block__body {
  max-height: 240px;
  overflow-y: auto;
  margin: 0 6px 4px;
  padding: 8px 10px;
  border-radius: 6px;
  background: color-mix(in srgb, var(--dq-label-primary) 3%, transparent);
  font-size: var(--dq-font-size-body);
  line-height: 1.5;
  color: var(--dq-label-secondary);
  white-space: pre-wrap;
  word-break: break-word;
}
</style>
