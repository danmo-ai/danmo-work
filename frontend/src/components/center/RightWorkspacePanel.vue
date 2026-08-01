<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import {
  DqPillTabs,
  Document,
  FolderChecked,
  MagicStick,
  Terminal,
  Library,
  Grid,
} from '@danqing/dq-shell'
import PlanPanel from '@/components/center/PlanPanel.vue'
import FileTree from '@/components/center/FileTree.vue'
import MemoryPanel from '@/components/center/MemoryPanel.vue'
import TablesPanel from '@/components/center/TablesPanel.vue'
import ChangesPanel from '@/components/center/ChangesPanel.vue'
import TerminalPanel from '@/components/center/TerminalPanel.vue'
import type { StreamEvent } from '@/types/mission'

export type RightTab = 'plan' | 'files' | 'memory' | 'tables' | 'changes' | 'terminal'

const props = defineProps<{
  streamEvents: StreamEvent[]
  planTurnId?: string | null
  projectId: string | null
  agentId?: string | null
  changesCount?: number
}>()

const rightTab = defineModel<RightTab>('tab', { required: true })

const emit = defineEmits<{
  openInOffice: [path: string]
  collapse: []
}>()

const { t } = useI18n()
const terminalOpened = ref(false)
const fileTreeRef = ref<InstanceType<typeof FileTree> | null>(null)
const changesPanelRef = ref<InstanceType<typeof ChangesPanel> | null>(null)
const memoryPanelRef = ref<InstanceType<typeof MemoryPanel> | null>(null)
const tablesPanelRef = ref<InstanceType<typeof TablesPanel> | null>(null)
const memoryCount = ref(0)

watch(rightTab, (tab) => {
  if (tab === 'terminal') terminalOpened.value = true
  if (tab === 'changes') changesPanelRef.value?.refresh?.()
  if (tab === 'memory') memoryPanelRef.value?.refresh?.()
  if (tab === 'tables') tablesPanelRef.value?.refresh?.()
})

const pillItems = computed(() => [
  { value: 'plan', label: t('sessions.tabPlan'), icon: MagicStick },
  { value: 'files', label: t('sessions.tabFiles'), icon: Document },
  {
    value: 'memory',
    label: t('sessions.tabMemory'),
    icon: Library,
    badge: memoryCount.value > 0 ? memoryCount.value : undefined,
  },
  {
    value: 'tables',
    label: t('sessions.tabTables'),
    icon: Grid,
  },
  {
    value: 'changes',
    label: t('sessions.tabChanges'),
    icon: FolderChecked,
    badge: props.changesCount && props.changesCount > 0 ? props.changesCount : undefined,
  },
  { value: 'terminal', label: t('sessions.tabTerminal'), icon: Terminal },
])

defineExpose({
  changesPanelRef,
  refreshChanges: () => changesPanelRef.value?.refresh?.(),
  refreshMemory: () => memoryPanelRef.value?.refresh?.(),
  refreshTables: () => tablesPanelRef.value?.refresh?.(),
})
</script>

<template>
  <div class="right-workspace">
    <div class="right-workspace__tabs">
      <DqPillTabs v-model="rightTab" size="sm" :items="pillItems" />
      <DqIconButton
        class="right-workspace__collapse"
        :aria-label="t('navigation.collapseRightPanel')"
        :title="t('navigation.collapseRightPanel')"
        @click="emit('collapse')"
      >
        <svg viewBox="0 0 24 24" width="14" height="14" fill="none" stroke="currentColor" stroke-width="2">
          <path d="M9 6l6 6-6 6" stroke-linecap="round" stroke-linejoin="round" />
        </svg>
      </DqIconButton>
    </div>

    <div class="right-workspace__body">
      <PlanPanel v-if="rightTab === 'plan'" :stream-events="streamEvents" :plan-turn-id="planTurnId" />

      <template v-else-if="rightTab === 'files'">
        <FileTree
          v-if="projectId"
          ref="fileTreeRef"
          :project-id="projectId"
          @select-file="emit('openInOffice', $event)"
        />
        <div v-else class="right-workspace__empty">{{ t('sessions.noProjectLinked') }}</div>
      </template>

      <ChangesPanel v-else-if="rightTab === 'changes'" ref="changesPanelRef" />

      <TablesPanel
        v-else-if="rightTab === 'tables'"
        ref="tablesPanelRef"
        :project-id="projectId"
        :agent-id="agentId ?? null"
      />

      <template v-else-if="rightTab === 'terminal'">
        <div v-if="!projectId" class="right-workspace__empty">{{ t('sessions.noProjectLinked') }}</div>
      </template>

      <MemoryPanel
        v-show="rightTab === 'memory'"
        ref="memoryPanelRef"
        :project-id="projectId"
        :agent-id="agentId ?? null"
        :stream-events="streamEvents"
        @loaded="memoryCount = $event"
      />

      <TerminalPanel
        v-if="terminalOpened && projectId"
        v-show="rightTab === 'terminal'"
        :key="projectId"
        :project-id="projectId"
      />
    </div>
  </div>
</template>

<style scoped>
.right-workspace {
  display: flex;
  flex-direction: column;
  min-width: 0;
  min-height: 0;
  height: 100%;
  background: transparent;
}

.right-workspace__tabs {
  flex-shrink: 0;
  display: flex;
  align-items: center;
  gap: 4px;
  padding: 6px 6px 6px 8px;
  border-bottom: 1px solid var(--dq-shell-divider);
  min-width: 0;
}

.right-workspace__tabs :deep(.dq-pill-tabs) {
  flex: 1;
  min-width: 0;
}

.right-workspace__collapse {
  flex-shrink: 0;
}

.right-workspace__body {
  flex: 1;
  min-height: 0;
  overflow: hidden;
  display: flex;
  flex-direction: column;
}

.right-workspace__empty {
  padding: 24px 16px;
  font-size: var(--dq-font-size-body);
  color: var(--dq-label-tertiary);
  text-align: center;
}
</style>
