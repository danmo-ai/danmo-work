<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { useRouter, useRoute } from 'vue-router'
import LeftRail from '@/components/left/LeftRail.vue'
import AppCommandPalette from '@/components/common/AppCommandPalette.vue'
import { useSessionsStore } from '@/stores/sessions'
import { useProjectsStore } from '@/stores/projects'
import { useLLMStore } from '@/stores/llm'
import { useWorkspaceUiStore } from '@/stores/workspaceUi'
import { useSessionActivityStore } from '@/stores/sessionActivity'
import { initAppVersion, startSilentUpdateCheck } from '@/composables/useAppUpdater'
import { isTauriRuntime, waitForBackend } from '@/utils/desktop'
import type { AppModule } from '@/types/app-module'

const { t } = useI18n()
const router = useRouter()
const route = useRoute()
const sessions = useSessionsStore()
const projects = useProjectsStore()
const llm = useLLMStore()
const workspaceUi = useWorkspaceUiStore()
const sessionActivity = useSessionActivityStore()
const bootstrapping = ref(isTauriRuntime())
const bootError = ref('')

function onGlobalKeydown(e: KeyboardEvent) {
  if ((e.metaKey || e.ctrlKey) && e.key.toLowerCase() === 'k') {
    e.preventDefault()
    workspaceUi.togglePalette()
  }
}

const activeModule = computed<AppModule>(() => {
  const name = route.name as string
  if (name === 'sessions') return 'sessions'
  if (['workers', 'knowledge', 'skills', 'mcpServers', 'automations', 'settings'].includes(name)) {
    return name as AppModule
  }
  return 'sessions'
})

function navigateTo(module: AppModule) {
  if (module === 'sessions') {
    router.push({ name: 'sessions' })
  } else {
    router.push({ name: module })
  }
}

function onSelectSession(id: string) {
  sessions.selectSession(id)
  router.push({ name: 'sessions', params: { id } })
}

onMounted(async () => {
  window.addEventListener('keydown', onGlobalKeydown)
  void initAppVersion()
  startSilentUpdateCheck()
  try {
    if (isTauriRuntime()) {
      const ready = await waitForBackend()
      if (!ready) {
        bootError.value = t('desktop.backendStartTimeout')
        return
      }
    }
    await sessions.loadCatalog()
    await Promise.all([projects.loadProjects(), llm.loadConfigs(), llm.loadModels()])
    if (projects.sortedProjects.length) {
      sessions.selectedProjectId = projects.sortedProjects[0].id
    }
    sessions.syncModelSelection(llm.models, new Set())
    await sessions.loadSessions()
    sessionActivity.startPolling()
  } catch (e) {
    bootError.value = e instanceof Error ? e.message : t('desktop.backendStartFailed')
  } finally {
    bootstrapping.value = false
  }
})

onUnmounted(() => {
  sessionActivity.stopPolling()
  window.removeEventListener('keydown', onGlobalKeydown)
})

watch(() => llm.models, (newModels, oldModels) => {
  const oldIds = new Set((oldModels ?? []).map((m) => m.id))
  sessions.syncModelSelection(newModels, oldIds)
})
</script>

<template>
  <div v-if="bootstrapping || bootError" class="app-boot">
    <p v-if="bootstrapping" class="app-boot__status">{{ $t('desktop.startingBackend') }}</p>
    <p v-else class="app-boot__error">{{ bootError }}</p>
  </div>
  <div v-else class="app-layout teams-app">
    <LeftRail :active-module="activeModule" @navigate="navigateTo" @select-session="onSelectSession" @new-session="(pid?: string) => { sessions.startCompose(pid ?? projects.sortedProjects[0]?.id ?? null); router.push({ name: 'sessions' }) }" />
    <main class="app-workspace">
      <RouterView />
    </main>
    <AppCommandPalette />
  </div>
</template>

<style scoped>
.app-boot {
  display: flex;
  align-items: center;
  justify-content: center;
  height: 100vh;
  padding: 24px;
  background: var(--dq-shell-stream-bg);
}

.app-boot__status {
  margin: 0;
  font-size: var(--dq-font-size-body);
  color: var(--dq-label-secondary);
}

.app-boot__error {
  margin: 0;
  max-width: 480px;
  font-size: var(--dq-font-size-body);
  color: var(--dq-danger, #ff453a);
  text-align: center;
  line-height: 1.5;
  white-space: pre-wrap;
  word-break: break-word;
}

.app-layout {
  display: grid;
  grid-template-columns: auto minmax(0, 1fr);
  grid-template-rows: minmax(0, 1fr);
  height: 100vh;
  overflow: hidden;
  gap: 0;
  padding: 0;
  box-sizing: border-box;
  background: var(--dq-shell-stream-bg);
}

.app-workspace {
  position: relative;
  display: flex;
  flex-direction: column;
  min-width: 0;
  min-height: 0;
  overflow: hidden;
  padding: 0;
  background: var(--dq-shell-stream-bg);
  border: none;
  border-radius: 0;
  box-shadow: none;
}

.app-workspace :deep(.resource-shell) {
  height: 100%;
  border-radius: 0;
  border: none;
  box-shadow: none;
}
</style>
