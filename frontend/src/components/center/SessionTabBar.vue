<script setup lang="ts">
import { computed, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { useRouter } from 'vue-router'
import { useSessionsStore } from '@/stores/sessions'
import { useProjectsStore } from '@/stores/projects'
import { useSessionActivityStore } from '@/stores/sessionActivity'
import { useOpenSessionTabsStore, COMPOSE_TAB_ID } from '@/stores/openSessionTabs'
import type { SessionActivityState } from '@/types/session-activity'

const { t } = useI18n()
const router = useRouter()
const sessions = useSessionsStore()
const projects = useProjectsStore()
const activity = useSessionActivityStore()
const openTabs = useOpenSessionTabsStore()

const overflowOpen = ref(false)

const tabItems = computed(() => {
  const items: Array<{ id: string; title: string; isCompose: boolean }> = openTabs.openSessions.map((s) => ({
    id: s.id,
    title: (s.title ?? s.content).trim() || t('navigation.untitledTask'),
    isCompose: false,
  }))
  if (sessions.composingNew) {
    items.push({
      id: COMPOSE_TAB_ID,
      title: t('navigation.newSession'),
      isCompose: true,
    })
  }
  return items
})

const activeId = computed(() => {
  if (sessions.composingNew) return COMPOSE_TAB_ID
  return sessions.currentSessionId
})

/** Tabs that don't fit visually — keep a simple overflow menu when > 8. */
const MAX_VISIBLE = 8

const visibleTabs = computed(() => {
  const items = tabItems.value
  if (items.length <= MAX_VISIBLE) return items
  const active = activeId.value
  const activeIdx = items.findIndex((x) => x.id === active)
  if (activeIdx < 0 || activeIdx < MAX_VISIBLE - 1) {
    return items.slice(0, MAX_VISIBLE - 1)
  }
  // Keep active tab visible near the end when overflowing.
  const head = items.slice(0, MAX_VISIBLE - 2)
  const activeTab = items[activeIdx]
  return [...head, activeTab]
})

const overflowTabs = computed(() => {
  const visibleIds = new Set(visibleTabs.value.map((t_) => t_.id))
  return tabItems.value.filter((t_) => !visibleIds.has(t_.id))
})

function stateFor(id: string): SessionActivityState | null {
  if (id === COMPOSE_TAB_ID) return null
  return activity.stateFor(id)
}

function titleFor(id: string, fallback: string): string {
  return fallback
}

async function activate(id: string) {
  overflowOpen.value = false
  if (id === COMPOSE_TAB_ID) {
    sessions.startCompose(sessions.selectedProjectId ?? projects.sortedProjects[0]?.id ?? null)
    router.push({ name: 'sessions' })
    return
  }
  openTabs.open(id)
  await sessions.selectSession(id)
  router.push({ name: 'sessions', params: { id } })
}

async function onClose(id: string, e?: Event) {
  e?.stopPropagation()
  e?.preventDefault()
  if (id === COMPOSE_TAB_ID) {
    // Closing compose: jump to last open tab or stay on empty compose.
    const last = openTabs.openIds[openTabs.openIds.length - 1]
    if (last) {
      await activate(last)
    }
    return
  }
  const wasCurrent = sessions.currentSessionId === id && !sessions.composingNew
  const next = openTabs.close(id)
  if (!wasCurrent) return
  if (next) {
    await sessions.selectSession(next)
    router.push({ name: 'sessions', params: { id: next } })
  } else {
    sessions.startCompose(sessions.selectedProjectId ?? projects.sortedProjects[0]?.id ?? null)
    router.push({ name: 'sessions' })
  }
}

function onNew() {
  sessions.startCompose(sessions.selectedProjectId ?? projects.sortedProjects[0]?.id ?? null)
  router.push({ name: 'sessions' })
}

function onAuxClick(id: string, e: MouseEvent) {
  // Middle-click closes tab (browser convention).
  if (e.button === 1) {
    void onClose(id, e)
  }
}

const currentPending = computed(() => {
  if (sessions.composingNew || !sessions.currentSessionId) return false
  const st = activity.stateFor(sessions.currentSessionId)
  return st === 'awaiting_approval' || st === 'awaiting_ask'
})

const emit = defineEmits<{
  jumpPending: []
}>()
</script>

<template>
  <div class="session-tab-bar" role="tablist" :aria-label="t('navigation.sessionTabsAria')">
    <div class="session-tab-bar__tabs">
      <button
        v-for="tab in visibleTabs"
        :key="tab.id"
        type="button"
        role="tab"
        class="session-tab"
        :class="{
          'is-active': activeId === tab.id,
          'is-compose': tab.isCompose,
        }"
        :aria-selected="activeId === tab.id"
        :title="titleFor(tab.id, tab.title)"
        @click="activate(tab.id)"
        @auxclick="onAuxClick(tab.id, $event)"
      >
        <span
          v-if="!tab.isCompose"
          class="session-tab__dot"
          :class="{
            'is-run': stateFor(tab.id) === 'running',
            'is-wait': stateFor(tab.id) === 'awaiting_approval' || stateFor(tab.id) === 'awaiting_ask',
          }"
        />
        <span class="session-tab__title">{{ tab.title }}</span>
        <button
          type="button"
          class="session-tab__close"
          :aria-label="t('navigation.closeTab')"
          :title="t('navigation.closeTab')"
          @click="onClose(tab.id, $event)"
        >
          <svg viewBox="0 0 24 24" width="12" height="12" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" aria-hidden="true">
            <path d="M18 6L6 18M6 6l12 12" />
          </svg>
        </button>
      </button>

      <div v-if="overflowTabs.length" class="session-tab-bar__overflow">
        <button
          type="button"
          class="session-tab session-tab--overflow"
          :class="{ 'is-open': overflowOpen }"
          :aria-expanded="overflowOpen"
          @click="overflowOpen = !overflowOpen"
        >
          <span class="session-tab__title">+{{ overflowTabs.length }}</span>
        </button>
        <div v-if="overflowOpen" class="session-tab-bar__overflow-menu" role="menu">
          <button
            v-for="tab in overflowTabs"
            :key="tab.id"
            type="button"
            class="session-tab-bar__overflow-row"
            role="menuitem"
            @click="activate(tab.id)"
          >
            <span
              class="session-tab__dot"
              :class="{
                'is-run': stateFor(tab.id) === 'running',
                'is-wait': stateFor(tab.id) === 'awaiting_approval' || stateFor(tab.id) === 'awaiting_ask',
              }"
            />
            <span class="session-tab-bar__overflow-title">{{ tab.title }}</span>
            <button
              type="button"
              class="session-tab__close"
              :aria-label="t('navigation.closeTab')"
              @click="onClose(tab.id, $event)"
            >
              <svg viewBox="0 0 24 24" width="12" height="12" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" aria-hidden="true">
                <path d="M18 6L6 18M6 6l12 12" />
              </svg>
            </button>
          </button>
        </div>
      </div>
    </div>

    <div class="session-tab-bar__actions">
      <button
        v-if="currentPending"
        type="button"
        class="session-tab-bar__pending"
        @click="emit('jumpPending')"
      >
        {{ t('sessions.jumpToPending') }}
      </button>
      <button
        type="button"
        class="session-tab-bar__new"
        :aria-label="t('navigation.newSession')"
        :title="t('navigation.newSession')"
        @click="onNew"
      >
        <svg viewBox="0 0 24 24" width="14" height="14" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" aria-hidden="true">
          <path d="M12 5v14M5 12h14" />
        </svg>
      </button>
    </div>
  </div>
</template>

<style scoped>
.session-tab-bar {
  flex-shrink: 0;
  display: flex;
  align-items: stretch;
  gap: 4px;
  min-height: 36px;
  padding: 0 8px 0 12px;
  border-bottom: 1px solid var(--dq-shell-divider);
  background: color-mix(in srgb, var(--dq-label-primary) 2.5%, transparent);
}

.session-tab-bar__tabs {
  display: flex;
  align-items: stretch;
  gap: 2px;
  min-width: 0;
  flex: 1;
  overflow: hidden;
}

.session-tab {
  position: relative;
  display: inline-flex;
  align-items: center;
  gap: 6px;
  max-width: 180px;
  min-width: 72px;
  margin: 4px 0 0;
  padding: 6px 8px 6px 10px;
  border: 1px solid transparent;
  border-bottom: none;
  border-radius: 8px 8px 0 0;
  background: transparent;
  color: var(--dq-label-secondary);
  font: inherit;
  font-size: var(--dq-font-size-footnote);
  cursor: pointer;
  transition: background 0.12s ease, color 0.12s ease;
}

.session-tab:hover {
  color: var(--dq-label-primary);
  background: color-mix(in srgb, var(--dq-label-primary) 5%, transparent);
}

.session-tab.is-active {
  color: var(--dq-label-primary);
  background: var(--dq-shell-stream-bg, var(--dq-bg-page));
  border-color: var(--dq-shell-divider);
  font-weight: 600;
}

.session-tab.is-compose {
  font-style: italic;
}

.session-tab__dot {
  width: 6px;
  height: 6px;
  border-radius: 50%;
  flex-shrink: 0;
  background: var(--dq-label-quaternary, var(--dq-label-tertiary));
  opacity: 0.45;
}

.session-tab__dot.is-run {
  opacity: 1;
  background: var(--dq-accent);
  animation: tab-pulse 1.4s ease-in-out infinite;
}

.session-tab__dot.is-wait {
  opacity: 1;
  background: var(--dq-system-orange);
}

@keyframes tab-pulse {
  0%,
  100% {
    opacity: 1;
  }
  50% {
    opacity: 0.35;
  }
}

.session-tab__title {
  min-width: 0;
  flex: 1;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  text-align: left;
}

.session-tab__close {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 18px;
  height: 18px;
  flex-shrink: 0;
  border: none;
  border-radius: 4px;
  background: transparent;
  color: var(--dq-label-tertiary);
  opacity: 0;
  cursor: pointer;
  padding: 0;
}

.session-tab:hover .session-tab__close,
.session-tab.is-active .session-tab__close {
  opacity: 1;
}

.session-tab__close:hover {
  color: var(--dq-label-primary);
  background: color-mix(in srgb, var(--dq-label-primary) 10%, transparent);
}

.session-tab-bar__overflow {
  position: relative;
  display: flex;
  align-items: stretch;
}

.session-tab--overflow {
  min-width: 40px;
  max-width: 56px;
  justify-content: center;
}

.session-tab-bar__overflow-menu {
  position: absolute;
  top: calc(100% + 4px);
  right: 0;
  z-index: 30;
  min-width: 220px;
  max-width: min(320px, 90vw);
  padding: 4px;
  border-radius: 10px;
  border: 1px solid var(--dq-separator-light);
  background: var(--dq-glass-popover-bg, var(--dq-bg-elevated, #fff));
  box-shadow: 0 10px 28px rgba(0, 0, 0, 0.12);
}

.session-tab-bar__overflow-row {
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

.session-tab-bar__overflow-row:hover {
  background: color-mix(in srgb, var(--dq-accent) 8%, transparent);
}

.session-tab-bar__overflow-title {
  flex: 1;
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  font-size: var(--dq-font-size-footnote);
}

.session-tab-bar__actions {
  display: flex;
  align-items: center;
  gap: 6px;
  flex-shrink: 0;
  padding: 4px 0;
}

.session-tab-bar__new {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 28px;
  height: 28px;
  border: none;
  border-radius: 6px;
  background: transparent;
  color: var(--dq-label-secondary);
  cursor: pointer;
}

.session-tab-bar__new:hover {
  color: var(--dq-label-primary);
  background: color-mix(in srgb, var(--dq-label-primary) 8%, transparent);
}

.session-tab-bar__pending {
  display: inline-flex;
  align-items: center;
  border: 1px solid color-mix(in srgb, var(--dq-system-orange) 40%, transparent);
  background: color-mix(in srgb, var(--dq-system-orange) 8%, transparent);
  color: var(--dq-system-orange);
  border-radius: 999px;
  padding: 3px 10px;
  font-size: var(--dq-font-size-caption);
  font-weight: 600;
  cursor: pointer;
  white-space: nowrap;
}
</style>
