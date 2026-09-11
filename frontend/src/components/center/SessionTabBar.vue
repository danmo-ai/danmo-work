<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { useRouter } from 'vue-router'
import { useSessionsStore } from '@/stores/sessions'
import { useProjectsStore } from '@/stores/projects'
import { useSessionActivityStore } from '@/stores/sessionActivity'
import { useOpenSessionTabsStore, COMPOSE_TAB_ID } from '@/stores/openSessionTabs'
import { useWorkspaceUiStore } from '@/stores/workspaceUi'
import type { SessionActivityState } from '@/types/session-activity'

const { t } = useI18n()
const router = useRouter()
const sessions = useSessionsStore()
const projects = useProjectsStore()
const activity = useSessionActivityStore()
const openTabs = useOpenSessionTabsStore()
const workspaceUi = useWorkspaceUiStore()

const overflowOpen = ref(false)
const overflowRoot = ref<HTMLElement | null>(null)

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

async function activate(id: string) {
  overflowOpen.value = false
  if (id === COMPOSE_TAB_ID) {
    sessions.startCompose(sessions.selectedProjectId ?? projects.sortedProjects[0]?.id ?? null)
    router.push({ name: 'sessions' })
    return
  }
  openTabs.open(id)
  await sessions.selectSession(id)
  workspaceUi.revealSessionInLeftRail(id)
  router.push({ name: 'sessions', params: { id } })
}

async function onClose(id: string, e?: Event) {
  e?.stopPropagation()
  e?.preventDefault()
  if (id === COMPOSE_TAB_ID) {
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
    workspaceUi.revealSessionInLeftRail(next)
    router.push({ name: 'sessions', params: { id: next } })
  } else {
    sessions.startCompose(sessions.selectedProjectId ?? projects.sortedProjects[0]?.id ?? null)
    router.push({ name: 'sessions' })
  }
}

function onNew() {
  overflowOpen.value = false
  sessions.startCompose(sessions.selectedProjectId ?? projects.sortedProjects[0]?.id ?? null)
  router.push({ name: 'sessions' })
}

function onAuxClick(id: string, e: MouseEvent) {
  if (e.button === 1) {
    void onClose(id, e)
  }
}

const currentPending = computed(() => {
  if (sessions.composingNew || !sessions.currentSessionId) return false
  const st = activity.stateFor(sessions.currentSessionId)
  return st === 'awaiting_approval' || st === 'awaiting_ask'
})

function onDocPointerDown(e: PointerEvent) {
  if (!overflowOpen.value) return
  const root = overflowRoot.value
  if (root && e.target instanceof Node && root.contains(e.target)) return
  overflowOpen.value = false
}

onMounted(() => document.addEventListener('pointerdown', onDocPointerDown))
onUnmounted(() => document.removeEventListener('pointerdown', onDocPointerDown))

const emit = defineEmits<{
  jumpPending: []
}>()
</script>

<template>
  <div class="session-tab-bar" role="tablist" :aria-label="t('navigation.sessionTabsAria')">
    <div class="session-tab-bar__scroll">
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
          :title="tab.title"
          @click="activate(tab.id)"
          @auxclick="onAuxClick(tab.id, $event)"
        >
          <span
            class="session-tab__status"
            :class="{
              'is-idle': !tab.isCompose && !stateFor(tab.id),
              'is-run': stateFor(tab.id) === 'running',
              'is-wait': stateFor(tab.id) === 'awaiting_approval' || stateFor(tab.id) === 'awaiting_ask',
              'is-compose': tab.isCompose,
            }"
            aria-hidden="true"
          />
          <span class="session-tab__title">{{ tab.title }}</span>
          <span
            class="session-tab__close"
            role="button"
            tabindex="-1"
            :aria-label="t('navigation.closeTab')"
            :title="t('navigation.closeTab')"
            @click="onClose(tab.id, $event)"
          >
            <svg viewBox="0 0 16 16" width="10" height="10" fill="none" aria-hidden="true">
              <path d="M4 4l8 8M12 4l-8 8" stroke="currentColor" stroke-width="1.6" stroke-linecap="round" />
            </svg>
          </span>
        </button>

        <div v-if="overflowTabs.length" ref="overflowRoot" class="session-tab-bar__overflow">
          <button
            type="button"
            class="session-tab-bar__overflow-btn"
            :class="{ 'is-open': overflowOpen }"
            :aria-label="t('navigation.sessionTabsAria')"
            :aria-expanded="overflowOpen"
            @click="overflowOpen = !overflowOpen"
          >
            <span>{{ overflowTabs.length }}</span>
            <svg viewBox="0 0 16 16" width="10" height="10" fill="none" aria-hidden="true">
              <path d="M4 6l4 4 4-4" stroke="currentColor" stroke-width="1.6" stroke-linecap="round" stroke-linejoin="round" />
            </svg>
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
                class="session-tab__status"
                :class="{
                  'is-idle': !tab.isCompose && !stateFor(tab.id),
                  'is-run': stateFor(tab.id) === 'running',
                  'is-wait': stateFor(tab.id) === 'awaiting_approval' || stateFor(tab.id) === 'awaiting_ask',
                  'is-compose': tab.isCompose,
                }"
              />
              <span class="session-tab-bar__overflow-title">{{ tab.title }}</span>
              <span
                class="session-tab__close is-visible"
                role="button"
                tabindex="-1"
                :aria-label="t('navigation.closeTab')"
                @click="onClose(tab.id, $event)"
              >
                <svg viewBox="0 0 16 16" width="10" height="10" fill="none" aria-hidden="true">
                  <path d="M4 4l8 8M12 4l-8 8" stroke="currentColor" stroke-width="1.6" stroke-linecap="round" />
                </svg>
              </span>
            </button>
          </div>
        </div>

        <button
          type="button"
          class="session-tab-bar__new"
          :aria-label="t('navigation.newSession')"
          :title="t('navigation.newSession')"
          @click="onNew"
        >
          <svg viewBox="0 0 16 16" width="14" height="14" fill="none" aria-hidden="true">
            <path d="M8 3v10M3 8h10" stroke="currentColor" stroke-width="1.6" stroke-linecap="round" />
          </svg>
        </button>
      </div>
    </div>

    <button
      v-if="currentPending"
      type="button"
      class="session-tab-bar__pending"
      @click="emit('jumpPending')"
    >
      {{ t('sessions.jumpToPending') }}
    </button>
  </div>
</template>

<style scoped>
/* Agent / productivity pill strip — floating chips, quiet chrome, no faux browser ledge. */
.session-tab-bar {
  --tab-h: 30px;
  flex-shrink: 0;
  display: flex;
  align-items: center;
  gap: 10px;
  min-height: 40px;
  padding: 6px 12px;
  border-bottom: 1px solid color-mix(in srgb, var(--dq-shell-divider) 85%, transparent);
  background: color-mix(in srgb, var(--dq-label-primary) 2.2%, transparent);
}

.session-tab-bar__scroll {
  min-width: 0;
  flex: 1;
  overflow-x: auto;
  overflow-y: hidden;
  scrollbar-width: none;
}

.session-tab-bar__scroll::-webkit-scrollbar {
  display: none;
}

.session-tab-bar__tabs {
  display: flex;
  align-items: center;
  gap: 4px;
  min-width: 0;
  width: max-content;
  max-width: 100%;
}

.session-tab {
  position: relative;
  display: inline-flex;
  align-items: center;
  gap: 7px;
  max-width: 196px;
  min-width: 84px;
  height: var(--tab-h);
  margin: 0;
  padding: 0 5px 0 10px;
  border: 1px solid transparent;
  border-radius: 9px;
  background: transparent;
  color: var(--dq-label-tertiary);
  font: inherit;
  font-size: 12.5px;
  font-weight: 500;
  letter-spacing: -0.011em;
  line-height: 1;
  cursor: pointer;
  transition:
    color 0.14s ease,
    background 0.14s ease,
    border-color 0.14s ease,
    box-shadow 0.14s ease;
}

.session-tab:hover {
  color: var(--dq-label-secondary);
  background: color-mix(in srgb, var(--dq-label-primary) 6%, transparent);
}

.session-tab.is-active {
  color: var(--dq-label-primary);
  background: color-mix(in srgb, var(--dq-label-primary) 9%, transparent);
  border-color: color-mix(in srgb, var(--dq-label-primary) 10%, transparent);
  box-shadow:
    inset 0 1px 0 color-mix(in srgb, #fff 6%, transparent),
    0 1px 2px color-mix(in srgb, #000 18%, transparent);
  font-weight: 600;
}

.session-tab.is-active:hover {
  background: color-mix(in srgb, var(--dq-label-primary) 11%, transparent);
  color: var(--dq-label-primary);
}

.session-tab.is-compose:not(.is-active) {
  color: var(--dq-label-quaternary, var(--dq-label-tertiary));
}

.session-tab__status {
  width: 6px;
  height: 6px;
  border-radius: 50%;
  flex-shrink: 0;
  background: color-mix(in srgb, var(--dq-label-primary) 28%, transparent);
}

.session-tab__status.is-compose {
  background: transparent;
  box-shadow: inset 0 0 0 1.4px color-mix(in srgb, var(--dq-label-primary) 32%, transparent);
}

.session-tab.is-active .session-tab__status.is-idle {
  background: var(--dq-accent);
  opacity: 0.85;
}

.session-tab__status.is-run {
  background: var(--dq-accent);
  box-shadow: 0 0 0 3px color-mix(in srgb, var(--dq-accent) 20%, transparent);
  animation: tab-pulse 1.5s ease-in-out infinite;
}

.session-tab__status.is-wait {
  background: var(--dq-system-orange);
  box-shadow: 0 0 0 3px color-mix(in srgb, var(--dq-system-orange) 20%, transparent);
}

@keyframes tab-pulse {
  0%,
  100% {
    opacity: 1;
  }
  50% {
    opacity: 0.42;
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
  border-radius: 6px;
  color: var(--dq-label-tertiary);
  opacity: 0;
  transition: opacity 0.12s ease, background 0.12s ease, color 0.12s ease;
}

.session-tab:hover .session-tab__close,
.session-tab.is-active .session-tab__close,
.session-tab__close.is-visible {
  opacity: 1;
}

.session-tab__close:hover {
  color: var(--dq-label-primary);
  background: color-mix(in srgb, var(--dq-label-primary) 12%, transparent);
}

.session-tab-bar__new {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: var(--tab-h);
  height: var(--tab-h);
  margin: 0 0 0 2px;
  border: 1px solid transparent;
  border-radius: 9px;
  background: transparent;
  color: var(--dq-label-tertiary);
  cursor: pointer;
  flex-shrink: 0;
  transition: color 0.12s ease, background 0.12s ease, border-color 0.12s ease;
}

.session-tab-bar__new:hover {
  color: var(--dq-label-primary);
  background: color-mix(in srgb, var(--dq-label-primary) 7%, transparent);
  border-color: color-mix(in srgb, var(--dq-label-primary) 8%, transparent);
}

.session-tab-bar__overflow {
  position: relative;
  display: flex;
  align-items: center;
}

.session-tab-bar__overflow-btn {
  display: inline-flex;
  align-items: center;
  gap: 3px;
  height: var(--tab-h);
  margin: 0;
  padding: 0 8px;
  border: 1px solid transparent;
  border-radius: 9px;
  background: transparent;
  color: var(--dq-label-tertiary);
  font: inherit;
  font-size: 12px;
  font-weight: 600;
  cursor: pointer;
}

.session-tab-bar__overflow-btn:hover,
.session-tab-bar__overflow-btn.is-open {
  color: var(--dq-label-primary);
  background: color-mix(in srgb, var(--dq-label-primary) 7%, transparent);
}

.session-tab-bar__overflow-menu {
  position: absolute;
  top: calc(100% + 8px);
  left: 0;
  z-index: 40;
  min-width: 240px;
  max-width: min(340px, 90vw);
  padding: 6px;
  border-radius: 12px;
  border: 1px solid var(--dq-separator-light);
  background: var(--dq-glass-popover-bg, var(--dq-bg-elevated, #fff));
  box-shadow:
    0 14px 36px rgba(0, 0, 0, 0.18),
    0 0 0 1px color-mix(in srgb, var(--dq-label-primary) 4%, transparent);
}

.session-tab-bar__overflow-row {
  width: 100%;
  display: flex;
  align-items: center;
  gap: 8px;
  border: none;
  background: transparent;
  text-align: left;
  padding: 8px;
  border-radius: 8px;
  cursor: pointer;
  color: var(--dq-label-primary);
  font: inherit;
  font-size: 12.5px;
}

.session-tab-bar__overflow-row:hover {
  background: color-mix(in srgb, var(--dq-label-primary) 7%, transparent);
}

.session-tab-bar__overflow-title {
  flex: 1;
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.session-tab-bar__pending {
  display: inline-flex;
  align-items: center;
  border: 1px solid color-mix(in srgb, var(--dq-system-orange) 36%, transparent);
  background: color-mix(in srgb, var(--dq-system-orange) 10%, transparent);
  color: var(--dq-system-orange);
  border-radius: 999px;
  padding: 4px 10px;
  font-size: var(--dq-font-size-caption);
  font-weight: 600;
  cursor: pointer;
  white-space: nowrap;
  flex-shrink: 0;
}

.session-tab-bar__pending:hover {
  background: color-mix(in srgb, var(--dq-system-orange) 16%, transparent);
}
</style>
