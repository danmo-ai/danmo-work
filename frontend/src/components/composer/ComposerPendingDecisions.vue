<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import PermissionAskBlock from '@/components/center/PermissionAskBlock.vue'
import AskUserBlock, { type AskUserFormField } from '@/components/center/AskUserBlock.vue'
import type { StreamEvent } from '@/types/mission'

export type PendingPermissionItem = {
  key: string
  event: StreamEvent
  decided: boolean
  deciding: boolean
  showActions: boolean
}

export type PendingAskItem = {
  key: string
  event: StreamEvent
  askId: string
  question: string
  options: string[]
  defaultOption?: string
  formFields: AskUserFormField[]
  resolved: boolean
  expired: boolean
  answering: boolean
  answer?: string
}

const props = defineProps<{
  permissions: PendingPermissionItem[]
  asks: PendingAskItem[]
  /** Max expanded decision cards; remainder collapses behind a count. */
  maxVisible?: number
}>()

const emit = defineEmits<{
  decide: [event: StreamEvent, payload: { decision: 'allow' | 'deny'; scope: 'once' | 'session' }]
  resolve: [event: StreamEvent, answer: string]
  'jump-timeline': []
}>()

const { t } = useI18n()

const maxVisible = computed(() => Math.max(1, props.maxVisible ?? 2))

const items = computed(() => {
  const out: Array<
    | { kind: 'permission'; item: PendingPermissionItem; seq: number }
    | { kind: 'ask'; item: PendingAskItem; seq: number }
  > = []
  for (const p of props.permissions) {
    out.push({ kind: 'permission', item: p, seq: p.event.seq })
  }
  for (const a of props.asks) {
    out.push({ kind: 'ask', item: a, seq: a.event.seq })
  }
  out.sort((a, b) => a.seq - b.seq)
  return out
})

const visible = computed(() => items.value.slice(0, maxVisible.value))
const hiddenCount = computed(() => Math.max(0, items.value.length - visible.value.length))

const empty = computed(() => items.value.length === 0)
</script>

<template>
  <div v-if="!empty" class="composer-decisions" role="region" :aria-label="t('composer.pendingDecisions')">
    <div class="composer-decisions__head">
      <span class="composer-decisions__title">{{ t('composer.pendingDecisions') }}</span>
      <span class="composer-decisions__count">{{ items.length }}</span>
      <button type="button" class="composer-decisions__jump" @click="emit('jump-timeline')">
        {{ t('sessions.jumpToPending') }}
      </button>
    </div>

    <div class="composer-decisions__list">
      <template v-for="entry in visible" :key="entry.kind + '-' + entry.item.key">
        <PermissionAskBlock
          v-if="entry.kind === 'permission'"
          :payload="entry.item.event.payload"
          :decided="entry.item.decided"
          :deciding="entry.item.deciding"
          :show-actions="entry.item.showActions"
          @decide="emit('decide', entry.item.event, $event)"
        />
        <AskUserBlock
          v-else
          :payload="entry.item.event.payload"
          :ask-id="entry.item.askId"
          :question="entry.item.question"
          :options="entry.item.options"
          :default-option="entry.item.defaultOption"
          :form-fields="entry.item.formFields"
          :resolved="entry.item.resolved"
          :expired="entry.item.expired"
          :answering="entry.item.answering"
          :answer="entry.item.answer"
          @resolve="emit('resolve', entry.item.event, $event)"
        />
      </template>
    </div>

    <p v-if="hiddenCount > 0" class="composer-decisions__more">
      {{ t('composer.pendingDecisionsMore', { n: hiddenCount }) }}
    </p>
  </div>
</template>

<style scoped>
.composer-decisions {
  display: flex;
  flex-direction: column;
  gap: 8px;
  margin-bottom: 8px;
  padding: 10px 12px;
  border-radius: 14px;
  border: 1px solid color-mix(in srgb, var(--dq-warning, #d97706) 28%, transparent);
  background: color-mix(in srgb, var(--dq-warning, #d97706) 6%, var(--dq-bg-elevated, var(--dq-bg-base)));
  box-shadow: 0 8px 24px color-mix(in srgb, var(--dq-label-primary) 6%, transparent);
}

.composer-decisions__head {
  display: flex;
  align-items: center;
  gap: 8px;
  min-width: 0;
}

.composer-decisions__title {
  font-size: var(--dq-font-size-footnote);
  font-weight: 600;
  color: var(--dq-label-primary);
}

.composer-decisions__count {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  min-width: 18px;
  height: 18px;
  padding: 0 6px;
  border-radius: 999px;
  font-size: var(--dq-font-size-caption);
  font-weight: 600;
  color: var(--dq-warning, #d97706);
  background: color-mix(in srgb, var(--dq-warning, #d97706) 16%, transparent);
}

.composer-decisions__jump {
  margin-left: auto;
  border: 0;
  background: transparent;
  color: var(--dq-accent);
  font-size: var(--dq-font-size-caption);
  font-weight: 500;
  cursor: pointer;
  padding: 2px 0;
}

.composer-decisions__jump:hover {
  text-decoration: underline;
}

.composer-decisions__list {
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.composer-decisions__more {
  margin: 0;
  font-size: var(--dq-font-size-caption);
  color: var(--dq-label-secondary);
}
</style>
