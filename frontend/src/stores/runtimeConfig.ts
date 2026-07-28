import { defineStore } from 'pinia'
import { ref } from 'vue'
import { fetchJSON } from '@/api/client'
import { toast } from '@/utils/feedback'
import type { ConfigFile, UpdateConfigFileRequest, SandboxStatus, BrowserStatus } from '@/types/mission'

export interface RuntimeForm {
  autoApprove: boolean
  permissionMode: 'discuss' | 'plan' | 'interactive' | 'auto'
  sandboxEnabled: boolean
  sandboxMode: 'read-only' | 'workspace-write' | 'danger-full-access'
  sandboxNetwork: 'deny' | 'allow' | 'allowlist'
  sandboxAllowlistDomains: string
  sandboxBackend: string
  sandboxShell: string
  browserEnabled: boolean
  browserExecutablePath: string
  browserCdpUrl: string
  doomLoopThreshold: number
  maxStepsDefault: number
  maxLLMFailures: number
  maxToolOutputChars: number
  maxDelegationDepth: number
  readTopK: number
  searchTopK: number
  chapterMaxTokens: number
  vectorHybrid: boolean
  compactionEnabled: boolean
  compactionMaxTokens: number
  compactionTriggerRatio: number
  compactionCutTokens: number
  compactionTurnInterval: number
  compactionSubInterval: number
  compactionToolTruncate: number
}

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
    sandboxShell: sb?.shell ?? 'auto',
    browserEnabled: br?.enabled ?? true,
    browserExecutablePath: br?.executablePath ?? '',
    browserCdpUrl: br?.cdpUrl ?? '',
    doomLoopThreshold: rt.turn.doomLoopThreshold,
    maxStepsDefault: rt.turn.maxStepsDefault,
    maxLLMFailures: rt.turn.maxLLMFailures ?? 3,
    maxToolOutputChars: rt.tools?.maxOutputChars ?? 50000,
    maxDelegationDepth: rt.team.maxDelegationDepth,
    readTopK: rt.memory.readTopK,
    searchTopK: rt.knowledge.searchTopK,
    chapterMaxTokens: rt.knowledge.chapterMaxTokens ?? 512,
    vectorHybrid: rt.knowledge.vectorHybrid ?? false,
    compactionEnabled: rt.compaction?.enabled ?? true,
    compactionMaxTokens: rt.compaction?.maxTokens ?? 128000,
    compactionTriggerRatio: rt.compaction?.triggerRatio ?? 0.85,
    compactionCutTokens: rt.compaction?.cutTokens ?? 16000,
    compactionTurnInterval: rt.compaction?.turnInterval ?? 6,
    compactionSubInterval: rt.compaction?.subInterval ?? 4,
    compactionToolTruncate: rt.compaction?.toolTruncate ?? 2000,
  }
}

export const useRuntimeConfigStore = defineStore('runtimeConfig', () => {
  const config = ref<RuntimeForm | null>(null)
  const sandboxStatus = ref<SandboxStatus | null>(null)
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
      await Promise.all([loadSandboxStatus(), loadBrowserStatus()])
    } catch {
      config.value = null
    } finally {
      loading.value = false
    }
  }

  async function saveConfig(form: RuntimeForm) {
    saving.value = true
    try {
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
          shell: form.sandboxShell || undefined,
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
          cutTokens: form.compactionCutTokens,
          turnInterval: form.compactionTurnInterval,
          subInterval: form.compactionSubInterval,
          toolTruncate: form.compactionToolTruncate,
        },
      }
      const req: UpdateConfigFileRequest = { runtime }
      const cfg = await fetchJSON<ConfigFile>('/config', {
        method: 'PUT',
        body: JSON.stringify(req),
      })
      config.value = formFromRuntime(cfg.runtime)
      await Promise.all([loadSandboxStatus(), loadBrowserStatus()])
      toast.success('运行时配置已保存')
    } catch (e) {
      toast.error(e instanceof Error ? e.message : '保存失败')
      throw e
    } finally {
      saving.value = false
    }
  }

  return {
    config,
    sandboxStatus,
    browserStatus,
    loading,
    saving,
    loadConfig,
    loadSandboxStatus,
    loadBrowserStatus,
    saveConfig,
  }
})
