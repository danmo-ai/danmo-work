<script setup lang="ts">
import { computed } from 'vue'

export interface DiffLine {
  type: 'meta' | 'hunk' | 'add' | 'del' | 'ctx' | 'empty'
  text: string
  hunkIndex?: number
}

const props = defineProps<{
  patch: string
  /** When true, show per-hunk accept checkboxes. */
  selectableHunks?: boolean
  selectedHunks?: number[]
}>()

const emit = defineEmits<{
  'update:selectedHunks': [indexes: number[]]
}>()

const lines = computed<DiffLine[]>(() => {
  const patch = props.patch || ''
  if (!patch) return []
  let hunkIndex = -1
  return patch.split('\n').map((text) => {
    if (
      text.startsWith('+++') ||
      text.startsWith('---') ||
      text.startsWith('diff ') ||
      text.startsWith('index ') ||
      text.startsWith('new file') ||
      text.startsWith('deleted file')
    ) {
      return { type: 'meta' as const, text }
    }
    if (text.startsWith('@@')) {
      hunkIndex += 1
      return { type: 'hunk' as const, text, hunkIndex }
    }
    if (text.startsWith('+')) return { type: 'add' as const, text, hunkIndex: hunkIndex >= 0 ? hunkIndex : undefined }
    if (text.startsWith('-')) return { type: 'del' as const, text, hunkIndex: hunkIndex >= 0 ? hunkIndex : undefined }
    if (text === '') return { type: 'empty' as const, text: ' ' }
    return { type: 'ctx' as const, text, hunkIndex: hunkIndex >= 0 ? hunkIndex : undefined }
  })
})

const hunkCount = computed(() => {
  let n = 0
  for (const l of lines.value) if (l.type === 'hunk') n++
  return n
})

function toggleHunk(idx: number) {
  const cur = new Set(props.selectedHunks || [])
  if (cur.has(idx)) cur.delete(idx)
  else cur.add(idx)
  emit('update:selectedHunks', [...cur].sort((a, b) => a - b))
}

function isSelected(idx: number) {
  return (props.selectedHunks || []).includes(idx)
}

defineExpose({ hunkCount })
</script>

<template>
  <pre class="unified-diff" aria-label="diff"><template v-for="(line, i) in lines" :key="i"><span
      class="unified-diff__line"
      :class="'is-' + line.type"
    ><button
        v-if="selectableHunks && line.type === 'hunk' && line.hunkIndex != null"
        type="button"
        class="unified-diff__hunk-toggle"
        :class="{ 'is-on': isSelected(line.hunkIndex) }"
        :title="'Hunk ' + (line.hunkIndex + 1)"
        @click="toggleHunk(line.hunkIndex)"
      >{{ isSelected(line.hunkIndex) ? '✓' : '○' }}</button>{{ line.text }}
</span></template></pre>
</template>

<style scoped>
.unified-diff {
  margin: 0;
  padding: 10px 12px;
  overflow: auto;
  font: 12px/1.45 ui-monospace, SFMono-Regular, Menlo, Consolas, monospace;
  white-space: pre;
  flex: 1;
  min-height: 0;
}
.unified-diff__line {
  display: block;
}
.unified-diff__line.is-add {
  background: color-mix(in srgb, var(--dq-success, #22c55e) 14%, transparent);
  color: inherit;
}
.unified-diff__line.is-del {
  background: color-mix(in srgb, var(--dq-danger, #ef4444) 14%, transparent);
}
.unified-diff__line.is-hunk {
  color: var(--dq-accent, #60a5fa);
  font-weight: 600;
}
.unified-diff__line.is-meta {
  color: var(--dq-muted, #9ca3af);
}
.unified-diff__hunk-toggle {
  display: inline-block;
  width: 1.4em;
  margin-right: 6px;
  border: none;
  background: transparent;
  color: inherit;
  cursor: pointer;
  font: inherit;
  padding: 0;
}
.unified-diff__hunk-toggle.is-on {
  color: var(--dq-success, #22c55e);
}
</style>
