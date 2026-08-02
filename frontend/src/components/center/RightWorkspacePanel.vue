<script setup lang="ts">
import { computed, nextTick, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import {
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
import type { RightWorkspaceTab } from '@/stores/workspaceUi'
import { useWorkspaceUiStore } from '@/stores/workspaceUi'

export type RightTab = RightWorkspaceTab

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
}>()

const { t } = useI18n()
const workspaceUi = useWorkspaceUiStore()
const fileTreeRef = ref<InstanceType<typeof FileTree> | null>(null)
const changesPanelRef = ref<InstanceType<typeof ChangesPanel> | null>(null)
const memoryPanelRef = ref<InstanceType<typeof MemoryPanel> | null>(null)
const tablesPanelRef = ref<InstanceType<typeof TablesPanel> | null>(null)
const terminalPanelRef = ref<InstanceType<typeof TerminalPanel> | null>(null)

watch(rightTab, async (tab) => {
  if (tab === 'terminal') {
    await nextTick()
    requestAnimationFrame(() => terminalPanelRef.value?.refit?.())
  }
  if (tab === 'changes') changesPanelRef.value?.refresh?.()
  if (tab === 'memory') memoryPanelRef.value?.refresh?.()
  if (tab === 'tables') tablesPanelRef.value?.refresh?.()
})

const tabTitle = computed(() => {
  const map: Record<RightTab, string> = {
    plan: t('sessions.tabPlan'),
    files: t('sessions.tabFiles'),
    memory: t('sessions.tabMemory'),
    tables: t('sessions.tabTables'),
    changes: t('sessions.tabChanges'),
    terminal: t('sessions.tabTerminal'),
  }
  return map[rightTab.value]
})

/** Shared icon defs for session header toolbar. */
const iconItems = computed(() => [
  { value: 'plan' as const, label: t('sessions.tabPlan'), icon: MagicStick },
  { value: 'files' as const, label: t('sessions.tabFiles'), icon: Document },
  {
    value: 'memory' as const,
    label: t('sessions.tabMemory'),
    icon: Library,
    badge: workspaceUi.memoryCount > 0 ? workspaceUi.memoryCount : undefined,
  },
  { value: 'tables' as const, label: t('sessions.tabTables'), icon: Grid },
  {
    value: 'changes' as const,
    label: t('sessions.tabChanges'),
    icon: FolderChecked,
    badge: props.changesCount && props.changesCount > 0 ? props.changesCount : undefined,
  },
  { value: 'terminal' as const, label: t('sessions.tabTerminal'), icon: Terminal },
])

defineExpose({
  changesPanelRef,
  tabTitle,
  iconItems,
  refreshChanges: () => changesPanelRef.value?.refresh?.(),
  refreshMemory: () => memoryPanelRef.value?.refresh?.(),
  refreshTables: () => tablesPanelRef.value?.refresh?.(),
})
</script>

<template>
  <div class="right-workspace">
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

      <MemoryPanel
        v-else-if="rightTab === 'memory'"
        ref="memoryPanelRef"
        :project-id="projectId"
        :agent-id="agentId ?? null"
        :stream-events="streamEvents"
        @loaded="workspaceUi.memoryCount = $event"
      />

      <template v-else-if="rightTab === 'terminal'">
        <div v-if="!projectId" class="right-workspace__empty">{{ t('sessions.noProjectLinked') }}</div>
        <TerminalPanel
          v-else
          ref="terminalPanelRef"
          class="right-workspace__terminal"
          :key="projectId"
          :project-id="projectId"
          :active="true"
        />
      </template>
    </div>
  </div>
</template>

<style scoped>
.right-workspace {
  display: flex;
  flex-direction: column;
  min-width: 0;
  min-height: 0;
  flex: 1;
  height: 100%;
  background: transparent;
}

.right-workspace__body {
  position: relative;
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
