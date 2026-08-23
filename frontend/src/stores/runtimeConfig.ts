import { defineStore } from 'pinia'
import { ref } from 'vue'
import { fetchJSON } from '@/api/client'
import { toast } from '@/utils/feedback'
import { i18n } from '@/i18n'
import type { ConfigFile, UpdateConfigFileRequest, SandboxStatus, BrowserStatus, EnvironmentStatus } from '@/types/mission'

export interface RuntimeForm {
  autoApprove: boolean
  permissionMode: 'discuss' | 'plan' | 'interactive' | 'auto'
  sandboxEnabled: boolean
  sandboxMode: 'read-only' | 'workspace-write' | 'danger-full-access'
  sandboxNetwork: 'deny' | 'allow' | 'allowlist'
  sandboxAllowlistDomains: string
  sandboxBackend: string
  envImage: string
  envTarPath: string
  envWorkspaceMount: string
  envCpus: string
  envMemory: string
  browserEnabled: boolean
  browserExecutablePath: string
  browserCdpUrl: string
  doomLoopThreshold: number
  maxStepsDefault: number
  maxLLMFailures: number
  llmHttpTimeoutSec: number
  maxToolOutputChars: number
  maxDelegationDepth: number
  readTopK: number
  searchTopK: number
  chapterMaxTokens: number
  vectorHybrid: boolean
  compactionEnabled: boolean
  compactionMaxTokens: number
  compactionTriggerRatio: number
  compactionLowWaterRatio: number
  compactionCutTokens: number
  compactionTurnInterval: number
  compactionSubInterval: number
  compactionToolTruncate: number
  compactionKeepRecentToolSteps: number
}

const CONTAINER_BACKENDS = ['podman', 'docker', 'apple-container']

function formFromRuntime(rt: ConfigFile['runtime']): RuntimeForm {
  const sb = rt.sandbox
  const br = rt.browser
  return {
    autoApprove: rt.autoApprove,
    permissionMode: rt.permissionMode || 'interactive',
    sandboxEnabled: sb?.enabled ?? true,
    sandboxMode: sb?.mode ?? 'workspace-write',
    sandboxNetwork: sb?.network ?? 'deny',
    sandboxAllowlistDomains: (sb?.allowlistDomains ?? []).join('\n'),
    sandboxBackend: sb?.backend ?? '',
    envImage: sb?.image ?? '',
    envTarPath: sb?.tarPath ?? '',
    envWorkspaceMount: sb?.workspaceMount ?? '',
    envCpus: sb?.resources?.cpus ?? '',
    envMemory: sb?.resources?.memory ?? '',
    browserEnabled: br?.enabled ?? true,
    browserExecutablePath: br?.executablePath ?? '',
    browserCdpUrl: br?.cdpUrl ?? '',
    doomLoopThreshold: rt.turn.doomLoopThreshold,
    maxStepsDefault: rt.turn.maxStepsDefault,
    maxLLMFailures: rt.turn.maxLLMFailures ?? 3,
    llmHttpTimeoutSec: rt.turn.llmHttpTimeoutSec ?? 600,
    maxToolOutputChars: rt.tools?.maxOutputChars ?? 50000,
    maxDelegationDepth: rt.team.maxDelegationDepth,
    readTopK: rt.memory.readTopK,
    searchTopK: rt.knowledge.searchTopK,
    chapterMaxTokens: rt.knowledge.chapterMaxTokens ?? 512,
    vectorHybrid: rt.knowledge.vectorHybrid ?? false,
    compactionEnabled: rt.compaction?.enabled ?? true,
    compactionMaxTokens: rt.compaction?.maxTokens ?? 128000,
    compactionTriggerRatio: rt.compaction?.triggerRatio ?? 0.85,
    compactionLowWaterRatio: rt.compaction?.lowWaterRatio ?? 0.70,
    compactionCutTokens: rt.compaction?.cutTokens ?? 16000,
    compactionTurnInterval: rt.compaction?.turnInterval ?? 6,
    compactionSubInterval: rt.compaction?.subInterval ?? 4,
    compactionToolTruncate: rt.compaction?.toolTruncate ?? 8192,
    compactionKeepRecentToolSteps: rt.compaction?.keepRecentToolSteps ?? 3,
  }
}

export const useRuntimeConfigStore = defineStore('runtimeConfig', () => {
  const config = ref<RuntimeForm | null>(null)
  const sandboxStatus = ref<SandboxStatus | null>(null)
  const environmentStatus = ref<EnvironmentStatus | null>(null)
  const browserStatus = ref<BrowserStatus | null>(null)
  const loading = ref(false)
  const saving = ref(false)

  async function loadSandboxStatus() {
    try {
      sandboxStatus.value = await fetchJSON<SandboxStatus>('/sandbox/status')
    } catch {
      sandboxStatus.value = null
    }
  }

  async function loadEnvironmentStatus() {
    try {
      environmentStatus.value = await fetchJSON<EnvironmentStatus>('/environment/status')
    } catch {
      environmentStatus.value = null
    }
  }

  const downloadingTar = ref<string | null>(null)

  async function downloadEnvTar(arch?: string) {
    const key = arch || 'auto'
    downloadingTar.value = key
    try {
      const res = await fetchJSON<{ status: EnvironmentStatus }>('/environment/tar/download', {
        method: 'POST',
        body: JSON.stringify(arch ? { arch } : {}),
      })
      environmentStatus.value = res.status
      toast.success(arch ? i18n.global.t('settings.envTarDownloadedArch', { arch }) : i18n.global.t('settings.envTarDownloaded'))
      return res.status
    } catch (e) {
      toast.error(e instanceof Error ? e.message : i18n.global.t('common.downloadFailed'))
      throw e
    } finally {
      downloadingTar.value = null
    }
  }

  async function loadBrowserStatus() {
    try {
      browserStatus.value = await fetchJSON<BrowserStatus>('/browser/status')
    } catch {
      browserStatus.value = null
    }
  }

  async function loadConfig() {
    loading.value = true
    try {
      const cfg = await fetchJSON<ConfigFile>('/config')
      config.value = formFromRuntime(cfg.runtime)
      await Promise.all([loadSandboxStatus(), loadEnvironmentStatus(), loadBrowserStatus()])
    } catch {
      config.value = null
    } finally {
      loading.value = false
    }
  }

  async function saveConfig(form: RuntimeForm) {
    saving.value = true
    try {
      const containerSelected = CONTAINER_BACKENDS.includes(form.sandboxBackend)
      const runtime: ConfigFile['runtime'] = {
        autoApprove: form.autoApprove,
        permissionMode: form.permissionMode,
        sandbox: {
          enabled: form.sandboxEnabled,
          mode: form.sandboxMode,
          network: form.sandboxNetwork,
          allowlistDomains: form.sandboxAllowlistDomains
            .split(/[\n,]+/)
            .map((s) => s.trim())
            .filter(Boolean),
          backend: form.sandboxBackend || undefined,
          image: containerSelected && form.envImage.trim() ? form.envImage.trim() : undefined,
          tarPath: containerSelected && form.envTarPath.trim() ? form.envTarPath.trim() : undefined,
          workspaceMount: containerSelected && form.envWorkspaceMount.trim() ? form.envWorkspaceMount.trim() : undefined,
          resources: containerSelected
            ? {
                cpus: form.envCpus.trim() || undefined,
                memory: form.envMemory.trim() || undefined,
              }
            : undefined,
        },
        browser: {
          enabled: form.browserEnabled,
          executablePath: form.browserExecutablePath || undefined,
          cdpUrl: form.browserCdpUrl || undefined,
        },
        turn: {
          doomLoopThreshold: form.doomLoopThreshold,
          maxStepsDefault: form.maxStepsDefault,
          maxLLMFailures: form.maxLLMFailures,
          llmHttpTimeoutSec: form.llmHttpTimeoutSec,
        },
        tools: {
          maxOutputChars: form.maxToolOutputChars,
        },
        team: {
          maxDelegationDepth: form.maxDelegationDepth,
        },
        memory: {
          readTopK: form.readTopK,
        },
        knowledge: {
          searchTopK: form.searchTopK,
          chapterMaxTokens: form.chapterMaxTokens,
          vectorHybrid: form.vectorHybrid,
        },
        compaction: {
          enabled: form.compactionEnabled,
          model: '',
          maxTokens: form.compactionMaxTokens,
          triggerRatio: form.compactionTriggerRatio,
          lowWaterRatio: form.compactionLowWaterRatio,
          cutTokens: form.compactionCutTokens,
          turnInterval: form.compactionTurnInterval,
          subInterval: form.compactionSubInterval,
          toolTruncate: form.compactionToolTruncate,
          keepRecentToolSteps: form.compactionKeepRecentToolSteps,
        },
      }
      const req: UpdateConfigFileRequest = { runtime }
      const cfg = await fetchJSON<ConfigFile>('/config', {
        method: 'PUT',
        body: JSON.stringify(req),
      })
      config.value = formFromRuntime(cfg.runtime)
      await Promise.all([loadSandboxStatus(), loadEnvironmentStatus(), loadBrowserStatus()])
      toast.success(i18n.global.t('settings.runtimeSaved'))
    } catch (e) {
      toast.error(e instanceof Error ? e.message : i18n.global.t('common.saveFailed'))
      throw e
    } finally {
      saving.value = false
    }
  }

  return {
    config,
    sandboxStatus,
    environmentStatus,
    browserStatus,
    loading,
    saving,
    downloadingTar,
    loadConfig,
    loadSandboxStatus,
    loadEnvironmentStatus,
    loadBrowserStatus,
    downloadEnvTar,
    saveConfig,
  }
})
