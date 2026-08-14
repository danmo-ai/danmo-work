<script setup lang="ts">
import { computed, onBeforeUnmount, ref, watch } from 'vue'

const props = defineProps<{
  text: string
  expanded: boolean
  seq: number
  running?: boolean
}>()

const emit = defineEmits<{
  toggle: [seq: number]
}>()

function formatLen(s: string): string {
  const n = s.length
  if (n < 1000) return String(n)
  return (n / 1000).toFixed(1).replace(/\.0$/, '') + 'k'
}

function latestLine(s: string): string {
  const t = s.trimEnd()
  const i = t.lastIndexOf('\n')
  return i === -1 ? t : t.slice(i + 1)
}

/** Full text for CSS ellipsis — fills the row to the trailing meta. */
const preview = computed(() => {
  const clean = props.text.replace(/\s+/g, ' ').trim()
  return props.running ? latestLine(clean) : clean
})

const previewRef = ref<HTMLElement | null>(null)

/** Frame-throttled tail follow (every 3 frames) — streams never fight the render loop. */
let pendingFrame: number | null = null
function scheduleTailScroll() {
  if (pendingFrame !== null) return
  let remaining = 3
  const advance = () => {
    remaining -= 1
    if (remaining > 0) {
      pendingFrame = requestAnimationFrame(advance)
      return
    }
    pendingFrame = null
    const el = previewRef.value
    if (el) el.scrollLeft = el.scrollWidth - el.clientWidth
  }
  pendingFrame = requestAnimationFrame(advance)
}

watch(preview, () => {
  if (!props.running) {
    const el = previewRef.value
    if (el) el.scrollLeft = 0
    return
  }
  scheduleTailScroll()
})

onBeforeUnmount(() => {
  if (pendingFrame !== null) cancelAnimationFrame(pendingFrame)
})
</script>

<template>
  <div class="thinking-block" :class="{ 'is-expanded': expanded, 'is-running': running }">
    <button type="button" class="thinking-block__header" @click="emit('toggle', seq)">
      <span v-if="!expanded" ref="previewRef" class="thinking-block__preview">{{ preview }}</span>
      <span v-else class="thinking-block__hint">思考过程</span>
      <span class="thinking-block__trail">
        <span v-if="running" class="thinking-block__live-dot" />
        <span class="thinking-block__meta">{{ formatLen(text) }}</span>
        <svg
          class="thinking-block__chevron"
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
      </span>
    </button>
    <div v-if="expanded" class="thinking-block__body">{{ text }}</div>
  </div>
</template>

<style scoped>
.thinking-block {
  margin: 0;
}

.thinking-block__header {
  display: flex;
  align-items: center;
  gap: 6px;
  width: 100%;
  min-height: 22px;
  padding: 1px 0;
  border: none;
  border-radius: 4px;
  background: transparent;
  color: var(--dq-label-primary);
  cursor: pointer;
  text-align: left;
  font: inherit;
  line-height: 1.35;
}

.thinking-block__header:hover {
  background: color-mix(in srgb, var(--dq-label-primary) 4%, transparent);
}

.thinking-block__preview {
  /* Grow with text, shrink with ellipsis — trail stays glued after text (no far-right gap) */
  flex: 0 1 auto;
  min-width: 0;
  max-width: calc(100% - 52px);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  font-size: var(--dq-font-size-body);
  color: var(--dq-label-primary);
}

.thinking-block__hint {
  flex: 0 1 auto;
  min-width: 0;
  font-size: var(--dq-font-size-body);
  color: var(--dq-label-primary);
}

.thinking-block__trail {
  display: inline-flex;
  align-items: center;
  gap: 4px;
  flex-shrink: 0;
}

.thinking-block__live-dot {
  width: 5px;
  height: 5px;
  border-radius: 50%;
  background: var(--dq-accent);
  box-shadow: 0 0 0 3px color-mix(in srgb, var(--dq-accent) 18%, transparent);
  animation: thinking-pulse 1.6s ease-in-out infinite;
}

@keyframes thinking-pulse {
  0%,
  100% {
    opacity: 1;
  }
  50% {
    opacity: 0.35;
  }
}

.thinking-block__meta {
  font-size: var(--dq-font-size-caption);
  color: var(--dq-label-quaternary, var(--dq-label-tertiary));
  font-variant-numeric: tabular-nums;
  opacity: 0.85;
}

.thinking-block__chevron {
  opacity: 0.4;
  transition: transform 0.15s ease;
}

.thinking-block__chevron.is-open {
  transform: rotate(180deg);
}

.thinking-block__body {
  max-height: 280px;
  overflow-y: auto;
  margin: 2px 0 4px;
  padding: 6px 8px;
  border-radius: 6px;
  background: color-mix(in srgb, var(--dq-label-primary) 3%, transparent);
  font-size: var(--dq-font-size-body);
  line-height: 1.45;
  color: var(--dq-label-primary);
  white-space: pre-wrap;
  word-break: break-word;
}
</style>
