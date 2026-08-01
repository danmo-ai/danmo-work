<script setup lang="ts">
import { computed, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { useSessionActivityStore } from '@/stores/sessionActivity'
import { useSessionsStore } from '@/stores/sessions'
import type { SessionActivityState } from '@/types/session-activity'

const emit = defineEmits<{
  select: [sessionId: string]
  jumpPending: []
}>()

const { t } = useI18n()
const activity = useSessionActivityStore()
const sessions = useSessionsStore()
const open = ref(false)

const others = computed(() =>
  activity.activeItems.filter((i) => i.sessionId !== sessions.currentSessionId),
)

const count = computed(() => others.value.length)

const currentPending = computed(() => {
  if (sessions.composingNew || !sessions.currentSessionId) return false
  const st = activity.stateFor(sessions.currentSessionId)
  return st === 'awaiting_approval' || st === 'awaiting_ask'
})

function titleFor(id: string): string {
  const s = sessions.sessions.find((x) => x.id === id)
  if (!s) return id.slice(0, 8)
  return (s.title ?? s.content).trim().slice(0, 36) || t('navigation.untitledTask')
}

function labelFor(state: SessionActivityState | string): string {
  if (state === 'awaiting_approval') return t('navigation.sessionNeedsApproval')
  if (state === 'awaiting_ask') return t('navigation.sessionNeedsAsk')
  if (state === 'running') return t('navigation.sessionRunning')
  return ''
}

function onPick(id: string) {
  open.value = false
  emit('select', id)
}
</script>

<template>
  <div v-if="count > 0 || currentPending" class="active-sessions">
    <button
      v-if="count > 0"
      type="button"
      class="active-sessions__chip"
      :class="{ 'is-open': open }"
      @click="open = !open"
    >
      <span class="active-sessions__dot" />
      <span>{{ t('navigation.activeSessions', { n: count }) }}</span>
    </button>
    <button
      v-if="currentPending"
      type="button"
      class="active-sessions__chip active-sessions__chip--warn"
      @click="emit('jumpPending')"
    >
      {{ t('sessions.jumpToPending') }}
    </button>

    <div v-if="open && count" class="active-sessions__drawer" role="menu">
      <button
        v-for="item in others"
        :key="item.sessionId"
        type="button"
        class="active-sessions__row"
        role="menuitem"
        @click="onPick(item.sessionId)"
      >
        <span
          class="active-sessions__row-dot"
          :class="{
            'is-wait': item.state === 'awaiting_approval' || item.state === 'awaiting_ask',
            'is-run': item.state === 'running',
          }"
        />
        <span class="active-sessions__row-title">{{ titleFor(item.sessionId) }}</span>
        <span class="active-sessions__row-state">{{ labelFor(item.state) }}</span>
      </button>
    </div>
  </div>
</template>

<style scoped>
.active-sessions {
  position: relative;
  display: flex;
  align-items: center;
  gap: 8px;
  flex-wrap: wrap;
}

.active-sessions__chip {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  border: 1px solid var(--dq-separator-light);
  background: color-mix(in srgb, var(--dq-label-primary) 3%, transparent);
  color: var(--dq-label-secondary);
  border-radius: 999px;
  padding: 3px 10px;
  font-size: var(--dq-font-size-caption);
  font-weight: 600;
  cursor: pointer;
}

.active-sessions__chip:hover,
.active-sessions__chip.is-open {
  color: var(--dq-label-primary);
  border-color: color-mix(in srgb, var(--dq-accent) 35%, var(--dq-separator-light));
}

.active-sessions__chip--warn {
  color: var(--dq-system-orange);
  border-color: color-mix(in srgb, var(--dq-system-orange) 40%, transparent);
}

.active-sessions__dot {
  width: 6px;
  height: 6px;
  border-radius: 50%;
  background: var(--dq-accent);
  animation: active-pulse 1.4s ease-in-out infinite;
}

@keyframes active-pulse {
  0%,
  100% {
    opacity: 1;
  }
  50% {
    opacity: 0.35;
  }
}

.active-sessions__drawer {
  position: absolute;
  top: calc(100% + 6px);
  left: 0;
  z-index: 20;
  min-width: 240px;
  max-width: min(360px, 90vw);
  padding: 4px;
  border-radius: 10px;
  border: 1px solid var(--dq-separator-light);
  background: var(--dq-glass-popover-bg, var(--dq-bg-elevated, #fff));
  box-shadow: 0 10px 28px rgba(0, 0, 0, 0.12);
}

.active-sessions__row {
  width: 100%;
  display: flex;
  align-items: center;
  gap: 8px;
  border: none;
  background: transparent;
  text-align: left;
  padding: 8px 10px;
  border-radius: 6px;
  cursor: pointer;
  color: var(--dq-label-primary);
  font: inherit;
}

.active-sessions__row:hover {
  background: color-mix(in srgb, var(--dq-accent) 8%, transparent);
}

.active-sessions__row-dot {
  width: 7px;
  height: 7px;
  border-radius: 50%;
  flex-shrink: 0;
  background: var(--dq-label-tertiary);
}

.active-sessions__row-dot.is-run {
  background: var(--dq-accent);
}

.active-sessions__row-dot.is-wait {
  background: var(--dq-system-orange);
}

.active-sessions__row-title {
  flex: 1;
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  font-size: var(--dq-font-size-footnote);
}

.active-sessions__row-state {
  flex-shrink: 0;
  font-size: var(--dq-font-size-caption);
  font-weight: 600;
  color: var(--dq-label-tertiary);
}
</style>
