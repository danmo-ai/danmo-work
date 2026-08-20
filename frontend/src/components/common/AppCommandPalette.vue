<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { useRouter } from 'vue-router'
import { DqCommandPalette } from '@danqing/dq-shell'
import { useProjectsStore } from '@/stores/projects'
import { useSessionsStore } from '@/stores/sessions'
import { useWorkspaceUiStore, type RightWorkspaceTab } from '@/stores/workspaceUi'

interface PaletteAction {
  id: string
  title: string
  description?: string
  keywords?: string[]
  shortcut?: string
  disabled?: boolean
  group?: string
  order?: number
  run: () => void | Promise<void>
}

const { t } = useI18n()
const router = useRouter()
const sessions = useSessionsStore()
const projects = useProjectsStore()
const workspaceUi = useWorkspaceUiStore()

const open = computed({
  get: () => workspaceUi.paletteOpen,
  set: (v: boolean) => {
    workspaceUi.paletteOpen = v
  },
})

function goTab(tab: RightWorkspaceTab) {
  workspaceUi.openRightDrawer(tab)
  if (router.currentRoute.value.name !== 'sessions') {
    router.push({ name: 'sessions', params: sessions.currentSessionId ? { id: sessions.currentSessionId } : {} })
  }
}

const actions = computed<PaletteAction[]>(() => [
  {
    id: 'session.new',
    title: t('commandPalette.newSession'),
    group: t('commandPalette.groupSession'),
    shortcut: 'mod+n',
    keywords: ['new', 'session', t('commandPalette.kwNew')],
    run: () => {
      sessions.startCompose(projects.sortedProjects[0]?.id ?? null)
      router.push({ name: 'sessions' })
    },
  },
  {
    id: 'session.stop',
    title: t('commandPalette.stopTurn'),
    group: t('commandPalette.groupSession'),
    shortcut: 'mod+.',
    disabled: !sessions.runningTurnId,
    keywords: ['stop', 'cancel', t('commandPalette.kwStop')],
    run: async () => {
      if (sessions.runningTurnId) await sessions.cancelTurn(sessions.runningTurnId)
    },
  },
  {
    id: 'nav.settings',
    title: t('commandPalette.openSettings'),
    group: t('commandPalette.groupNavigate'),
    shortcut: 'mod+,',
    keywords: ['settings', t('commandPalette.kwSettings')],
    run: () => router.push({ name: 'settings' }),
  },
  {
    id: 'tab.plan',
    title: t('commandPalette.tabPlan'),
    group: t('commandPalette.groupNavigate'),
    run: () => goTab('plan'),
  },
  {
    id: 'tab.files',
    title: t('commandPalette.tabFiles'),
    group: t('commandPalette.groupNavigate'),
    run: () => goTab('files'),
  },
  {
    id: 'tab.memory',
    title: t('commandPalette.tabMemory'),
    group: t('commandPalette.groupNavigate'),
    run: () => goTab('memory'),
  },
  {
    id: 'tab.tables',
    title: t('commandPalette.tabTables'),
    group: t('commandPalette.groupNavigate'),
    run: () => goTab('tables'),
  },
  {
    id: 'tab.changes',
    title: t('commandPalette.tabChanges'),
    group: t('commandPalette.groupNavigate'),
    run: () => goTab('changes'),
  },
  {
    id: 'tab.terminal',
    title: t('commandPalette.tabTerminal'),
    group: t('commandPalette.groupNavigate'),
    run: () => goTab('terminal'),
  },
  {
    id: 'tab.trajectory',
    title: t('commandPalette.tabTrajectory'),
    group: t('commandPalette.groupNavigate'),
    run: () => goTab('trajectory'),
  },
  {
    id: 'stage.preview',
    title: t('commandPalette.openPreview'),
    group: t('commandPalette.groupNavigate'),
    keywords: ['preview', 'browser', 'url', t('commandPalette.kwPreview'), t('commandPalette.kwBrowser')],
    run: () => {
      if (!sessions.selectedProjectId) return
      workspaceUi.openStage({ kind: 'preview', path: '', mode: 'view', url: '' })
      if (router.currentRoute.value.name !== 'sessions') {
        router.push({
          name: 'sessions',
          params: sessions.currentSessionId ? { id: sessions.currentSessionId } : {},
        })
      }
    },
  },
])
</script>

<template>
  <DqCommandPalette
    v-model:open="open"
    :actions="actions"
    :title="t('commandPalette.title')"
    :placeholder="t('commandPalette.placeholder')"
    :empty-text="t('commandPalette.empty')"
  />
</template>
