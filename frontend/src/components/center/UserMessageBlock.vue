<script setup lang="ts">
import { computed } from 'vue'
import {
  officeActionLabel,
  parseUserMessageParts,
  type UserMessagePart,
} from '@/utils/user-message-parts'

const props = defineProps<{
  text?: string
  images?: string[]
}>()

const parts = computed(() => parseUserMessageParts(props.text || ''))

const structured = computed(() => parts.value.filter((p) => p.type !== 'text'))
const plainText = computed(() =>
  parts.value
    .filter((p): p is Extract<UserMessagePart, { type: 'text' }> => p.type === 'text')
    .map((p) => p.text)
    .join('\n\n')
    .trim(),
)

function fileBase(path: string) {
  return path.replace(/\\/g, '/').split('/').pop() || path
}

function officeTitle(p: Extract<UserMessagePart, { type: 'office-edit' }>) {
  const act = officeActionLabel(p.action)
  const base = fileBase(p.path)
  if (p.lines) return `${base}:${p.lines} · ${act}`
  if (p.page != null && p.page !== '') return `${base} · p${Number(p.page) + 1} · ${act}`
  return `${base} · ${act}`
}

function rowTitle(part: UserMessagePart): string {
  if (part.type === 'office-edit') return officeTitle(part)
  if (part.type === 'selected-code') {
    return part.lines ? `${fileBase(part.path)}:${part.lines}` : fileBase(part.path)
  }
  if (part.type === 'selected-element') return part.summary
  if (part.type === 'preview-console') return 'Console / network'
  return ''
}

function rowAsk(part: UserMessagePart): string {
  if (part.type === 'office-edit') return part.ask
  if (part.type === 'selected-code') return part.request || ''
  if (part.type === 'selected-element') return part.request || ''
  if (part.type === 'preview-console') return part.request || part.preview || ''
  return ''
}
</script>

<template>
  <div v-if="text || images?.length" class="user-msg dq-msg-enter">
    <div class="user-msg__bubble">
      <div v-if="images?.length" class="user-msg__images">
        <img
          v-for="(src, i) in images"
          :key="`img-${i}`"
          class="user-msg__image"
          :src="src"
          alt="attachment"
        />
      </div>

      <p v-if="plainText" class="user-msg__text">{{ plainText }}</p>

      <div v-if="structured.length" class="user-msg__rows">
        <div
          v-for="(part, i) in structured"
          :key="i"
          class="user-msg__row"
          :class="{ 'is-divided': i > 0 }"
        >
          <div class="user-msg__row-title">{{ rowTitle(part) }}</div>
          <div v-if="rowAsk(part)" class="user-msg__row-ask">{{ rowAsk(part) }}</div>
        </div>
      </div>
    </div>
  </div>
</template>

<style scoped>
.user-msg {
  display: flex;
  justify-content: flex-end;
}

.user-msg__bubble {
  max-width: min(85%, 480px);
  padding: 10px 14px;
  border-radius: var(--dq-msg-user-radius);
  font-size: var(--dq-font-size-body);
  line-height: var(--dq-line-height-prose, 1.55);
  color: var(--dq-msg-user-fg);
  background: var(--dq-msg-user-bg);
  border: 1px solid var(--dq-msg-user-border);
  box-shadow: var(--dq-msg-user-shadow, none);
  word-break: break-word;
}

.user-msg__text {
  margin: 0;
  white-space: pre-wrap;
}

.user-msg__text + .user-msg__rows {
  margin-top: 8px;
  padding-top: 8px;
  border-top: 1px solid color-mix(in srgb, var(--dq-msg-user-fg) 14%, transparent);
}

.user-msg__rows {
  display: flex;
  flex-direction: column;
}

.user-msg__row.is-divided {
  margin-top: 8px;
  padding-top: 8px;
  border-top: 1px solid color-mix(in srgb, var(--dq-msg-user-fg) 14%, transparent);
}

.user-msg__row-title {
  font-size: var(--dq-font-size-body);
  font-weight: 600;
  line-height: 1.35;
}

.user-msg__row-ask {
  margin-top: 2px;
  font-size: var(--dq-font-size-caption);
  opacity: 0.78;
  line-height: 1.4;
}

.user-msg__images {
  display: flex;
  flex-wrap: wrap;
  gap: 6px;
  margin-bottom: 6px;
}

.user-msg__images:last-child {
  margin-bottom: 0;
}

.user-msg__image {
  max-width: min(220px, 100%);
  max-height: 160px;
  border-radius: var(--dq-code-radius, 8px);
  object-fit: contain;
  background: color-mix(in srgb, var(--dq-label-primary) 4%, transparent);
}
</style>
