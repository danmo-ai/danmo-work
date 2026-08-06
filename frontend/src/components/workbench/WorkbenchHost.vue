<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { useWorkspaceUiStore } from '@/stores/workspaceUi'
import { WORKBENCH_REGISTRY, type WorkbenchId } from '@/types/workbench'
import NovelWorkbench from '@/components/novel/NovelWorkbench.vue'

const { t } = useI18n()
const workspaceUi = useWorkspaceUiStore()

const entries = WORKBENCH_REGISTRY

const title = computed(() => {
  const e = entries.find((x) => x.id === workspaceUi.activeWorkbenchId)
  return e ? t(e.labelKey) : t('workbench.title')
})

function onSelect(id: WorkbenchId) {
  workspaceUi.setActiveWorkbenchId(id)
}

function onClose() {
  workspaceUi.closeWorkbench()
}
</script>

<template>
  <aside class="workbench-host" :aria-label="t('workbench.title')">
    <header class="workbench-host__bar">
      <div class="workbench-host__tabs" role="tablist" :aria-label="t('workbench.title')">
        <button
          v-for="e in entries"
          :key="e.id"
          type="button"
          role="tab"
          class="workbench-host__tab"
          :class="{ 'is-active': workspaceUi.activeWorkbenchId === e.id }"
          :aria-selected="workspaceUi.activeWorkbenchId === e.id"
          @click="onSelect(e.id)"
        >
          {{ t(e.labelKey) }}
        </button>
      </div>
      <button
        type="button"
        class="workbench-host__close"
        :aria-label="t('common.close')"
        :title="t('common.close')"
        @click="onClose"
      >
        ×
      </button>
    </header>
    <div class="workbench-host__body">
      <NovelWorkbench v-if="workspaceUi.activeWorkbenchId === 'novel'" />
      <div v-else class="workbench-host__empty">{{ title }}</div>
    </div>
  </aside>
</template>

<style scoped>
.workbench-host {
  display: flex;
  flex-direction: column;
  min-width: 0;
  min-height: 0;
  height: 100%;
  overflow: hidden;
  border-left: 1px solid var(--dq-shell-divider, rgba(0, 0, 0, 0.08));
  background: color-mix(in srgb, var(--dq-glass-popover-bg, #fff) 92%, transparent);
}

.workbench-host__bar {
  flex-shrink: 0;
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 8px 10px;
  border-bottom: 1px solid var(--dq-shell-divider, rgba(0, 0, 0, 0.08));
}

.workbench-host__tabs {
  flex: 1;
  min-width: 0;
  display: flex;
  flex-wrap: wrap;
  gap: 4px;
}

.workbench-host__tab {
  margin: 0;
  padding: 5px 10px;
  border: none;
  border-radius: 6px;
  background: transparent;
  color: var(--dq-label-secondary, inherit);
  font: inherit;
  font-size: var(--dq-font-size-secondary, 12px);
  font-weight: 600;
  cursor: pointer;
}

.workbench-host__tab:hover {
  background: var(--dq-fill-on-glass, rgba(0, 0, 0, 0.04));
  color: var(--dq-label-primary, inherit);
}

.workbench-host__tab.is-active {
  background: color-mix(in srgb, var(--dq-accent) 14%, transparent);
  color: var(--dq-accent);
}

.workbench-host__close {
  flex-shrink: 0;
  width: 28px;
  height: 28px;
  margin: 0;
  padding: 0;
  border: none;
  border-radius: 6px;
  background: transparent;
  color: inherit;
  opacity: 0.55;
  font-size: 18px;
  line-height: 1;
  cursor: pointer;
}

.workbench-host__close:hover {
  opacity: 1;
  background: var(--dq-fill-on-glass, rgba(0, 0, 0, 0.04));
}

.workbench-host__body {
  flex: 1;
  min-height: 0;
  overflow: hidden;
  display: flex;
  flex-direction: column;
}

.workbench-host__empty {
  padding: 24px;
  opacity: 0.6;
  font-size: var(--dq-font-size-body);
}
</style>
