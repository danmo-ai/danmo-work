<script setup lang="ts">
import { ref, reactive, computed, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { Setting, Cpu, Search, Brush, Monitor } from '@danqing/dq-shell'
import { useLLMStore } from '@/stores/llm'
import { useSearchConfigStore } from '@/stores/searchConfig'
import { useRuntimeConfigStore } from '@/stores/runtimeConfig'
import { useMarketConfigStore } from '@/stores/marketConfig'
import { useModelConfigStore } from '@/stores/modelLimits'
import { useWeixinStore } from '@/stores/weixin'
import { useFeishuStore } from '@/stores/feishu'
import { useWecomStore } from '@/stores/wecom'
import { useQQStore } from '@/stores/qq'
import { useSessionsStore } from '@/stores/sessions'
import { useProjectsStore } from '@/stores/projects'
import { useThemeStore, THEME_OPTIONS } from '@/stores/theme'
import type { ThemeId } from '@/stores/theme'
import { toast, confirm } from '@/utils/feedback'
import Skeleton from '@/components/common/Skeleton.vue'
import { useAppUpdater } from '@/composables/useAppUpdater'
import { isTauriRuntime } from '@/utils/desktop'
import type { LLMProviderType, LLMProviderConfig, LLMModelRef, LLMProviderPreset, SearchProvider, ModelConfig, ConfigMarketSection, MarketSourceConfig } from '@/types/mission'

type SettingsTab = 'runtime' | 'models' | 'modelConfig' | 'search' | 'market' | 'weixin' | 'feishu' | 'wecom' | 'qq' | 'appearance' | 'about'

const { t } = useI18n()
const activeTab = ref<SettingsTab>('models')
const llm = useLLMStore()
const searchConfig = useSearchConfigStore()
const runtimeConfig = useRuntimeConfigStore()
const marketConfig = useMarketConfigStore()
const modelConfig = useModelConfigStore()
const weixin = useWeixinStore()
const feishu = useFeishuStore()
const wecom = useWecomStore()
const qq = useQQStore()
const sessions = useSessionsStore()
const projects = useProjectsStore()
const themeStore = useThemeStore()

const weixinForm = ref({
  enabled: false,
  defaultAgentId: '',
  defaultModelId: '',
  autoApprove: false,
})
const weixinLoginProjectId = ref('')
const weixinPolling = ref(false)
const showWeixinAddDialog = ref(false)
const weixinAddCancelled = ref(false)
const feishuForm = ref({
  enabled: false,
  defaultAgentId: '',
  defaultModelId: '',
  autoApprove: false,
  domain: 'feishu' as 'feishu' | 'lark',
  appId: '',
  appSecret: '',
  projectId: '',
  richProgress: true,
})
const wecomForm = ref({
  enabled: false,
  defaultAgentId: '',
  defaultModelId: '',
  autoApprove: true,
  botId: '',
  secret: '',
  wsUrl: '',
  projectId: '',
})
const qqForm = ref({
  enabled: false,
  defaultAgentId: '',
  defaultModelId: '',
  autoApprove: false,
  appId: '',
  clientSecret: '',
  projectId: '',
  groupDenyTools: '',
  requireMention: true,
  nativeC2cStream: true,
})
const {
  appVersion,
  status: updaterStatus,
  availableVersion,
  updateNotes,
  errorMessage: updaterError,
  downloadPercent,
  hasUpdate,
  isBusy: updaterBusy,
  initAppVersion,
  checkForUpdates,
  downloadAndInstallUpdate,
} = useAppUpdater()
const isDesktop = isTauriRuntime()

async function handleCheckUpdate() {
  if (!isDesktop) {
    toast.info(t('updater.desktopOnly'))
    return
  }
  try {
    const found = await checkForUpdates()
    if (found) {
      toast.info(t('updater.availableToast', { version: availableVersion.value }))
    } else if (updaterStatus.value === 'upToDate') {
      toast.success(t('updater.upToDate'))
    } else if (updaterStatus.value === 'error') {
      toast.error(updaterError.value || t('updater.failed'))
    }
  } catch (e) {
    toast.error(e instanceof Error ? e.message : t('updater.failed'))
  }
}

async function handleInstallUpdate() {
  if (!isDesktop) return
  try {
    toast.info(t('updater.downloading'))
    await downloadAndInstallUpdate()
  } catch (e) {
    toast.error(e instanceof Error ? e.message : t('updater.failed'))
  }
}

const providerOptions = computed<{ value: LLMProviderType; label: string }[]>(() => [
  { value: 'openai', label: 'OpenAI' },
  { value: 'anthropic', label: 'Anthropic' },
  { value: 'local', label: t('settings.localProvider') },
  { value: 'mock', label: t('settings.mockProvider') },
])

const editingId = ref<string | null>(null)
const showForm = ref(false)
const refreshingModels = ref(false)
const dialogStep = ref<'choose' | 'configure'>('choose')
const manualModelName = ref('')

const form = ref({
  provider: 'openai' as LLMProviderType,
  name: '',
  apiKey: '',
  baseUrl: '',
  models: [] as LLMModelRef[],
})

const searchForm = ref({
  provider: 'duckduckgo' as SearchProvider,
  baseUrl: '',
  apiKey: '',
  timeoutMs: 15000,
  maxResults: 5,
  proxy: '',
  userAgent: '',
  htmlFallback: true,
})

const marketForm = ref<ConfigMarketSection>({
  cacheTtlHours: 6,
  sources: [],
})

const runtimeForm = ref({
  autoApprove: false,
  permissionMode: 'interactive' as 'discuss' | 'plan' | 'interactive' | 'auto',
  sandboxEnabled: true,
  sandboxMode: 'workspace-write' as 'read-only' | 'workspace-write' | 'danger-full-access',
  sandboxNetwork: 'deny' as 'deny' | 'allow' | 'allowlist',
  sandboxAllowlistDomains: '',
  sandboxBackend: '',
  sandboxShell: 'auto',
  envBackend: 'local' as 'local' | 'container',
  envEngine: 'auto' as 'auto' | 'podman' | 'docker' | 'apple-container',
  envCpus: '',
  envMemory: '',
  browserEnabled: true,
  browserExecutablePath: '',
  browserCdpUrl: '',
  doomLoopThreshold: 10,
  maxStepsDefault: 200,
  maxLLMFailures: 3,
  maxToolOutputChars: 50000,
  maxDelegationDepth: 3,
  readTopK: 10,
  searchTopK: 3,
  chapterMaxTokens: 512,
  vectorHybrid: false,
  compactionEnabled: true,
  compactionMaxTokens: 128000,
  compactionTriggerRatio: 0.85,
  compactionCutTokens: 16000,
  compactionTurnInterval: 6,
  compactionSubInterval: 4,
  compactionToolTruncate: 2000,
})

const sandboxStatusText = computed(() => {
  const st = runtimeConfig.sandboxStatus
  if (!st) return ''
  const caps = st.capabilities?.length ? ` (${st.capabilities.join(', ')})` : ''
  const shell = st.shell ? ` · ${st.shell}` : ''
  const path = st.shellPath ? ` @ ${st.shellPath}` : ''
  const cu = st.coreutilsBin ? ` · coreutils ${st.coreutilsBin}` : ''
  const proxy = st.allowlistActive && st.allowlistProxy ? ` · allowlist ${st.allowlistProxy}` : ''
  return `${st.backend}${caps}${shell}${path}${cu}${proxy}`
})

const environmentStatusText = computed(() => {
  const st = runtimeConfig.environmentStatus
  if (!st) return ''
  const eng = st.engine ? ` · ${st.engine}` : ''
  const tar = st.tarPresent ? ' · tar ok' : ' · tar missing'
  const img = st.imageLoaded ? ' · image loaded' : ''
  const res = [st.resources?.cpus && `cpus=${st.resources.cpus}`, st.resources?.memory && `mem=${st.resources.memory}`]
    .filter(Boolean)
    .join(' ')
  const resPart = res ? ` · ${res}` : ' · unlimited'
  return `${st.backend}${eng}${tar}${img}${resPart}`
})

const showGitBashHint = computed(() => {
  const st = runtimeConfig.sandboxStatus
  if (!st || st.platform !== 'windows') return false
  if (st.backend === 'wsl2') return false
  if (st.coreutilsBin) return false
  if (st.shell && st.shell !== 'cmd') return false
  return true
})

const ALLOWLIST_PRESET_GROUPS: {
  id: string
  presets: { id: string; domains: string[] }[]
}[] = [
  {
    id: 'combo',
    presets: [
      {
        // Debian apt + common language registries + GitHub (container / agent install path)
        id: 'devDeps',
        domains: [
          'deb.debian.org',
          'security.debian.org',
          '*.debian.org',
          'registry.npmjs.org',
          '*.npmjs.org',
          'registry.yarnpkg.com',
          'proxy.golang.org',
          'sum.golang.org',
          'pypi.org',
          '*.pythonhosted.org',
          'files.pythonhosted.org',
          'github.com',
          'api.github.com',
          '*.githubusercontent.com',
          'codeload.github.com',
        ],
      },
      {
        // Default + common web_search / web_fetch providers
        id: 'webSearch',
        domains: [
          'duckduckgo.com',
          'html.duckduckgo.com',
          '*.duckduckgo.com',
          'www.bing.com',
          'bing.com',
          'api.search.brave.com',
          'search.brave.com',
          'www.google.com',
          'google.com',
          '*.google.com',
          'www.baidu.com',
          'baidu.com',
          'api.tavily.com',
        ],
      },
    ],
  },
  {
    id: 'dev',
    presets: [
      {
        id: 'debian',
        domains: ['deb.debian.org', 'security.debian.org', '*.debian.org'],
      },
      {
        id: 'npm',
        domains: ['registry.npmjs.org', '*.npmjs.org', 'registry.yarnpkg.com'],
      },
      {
        id: 'go',
        domains: ['proxy.golang.org', 'sum.golang.org', 'github.com', '*.githubusercontent.com'],
      },
      {
        id: 'pypi',
        domains: ['pypi.org', '*.pythonhosted.org', 'files.pythonhosted.org'],
      },
      {
        id: 'crates',
        domains: ['static.crates.io', 'index.crates.io', 'crates.io'],
      },
      {
        id: 'github',
        domains: ['github.com', 'api.github.com', '*.githubusercontent.com', 'codeload.github.com'],
      },
    ],
  },
]

const ALLOWLIST_PRESETS: Record<string, string[]> = Object.fromEntries(
  ALLOWLIST_PRESET_GROUPS.flatMap((g) => g.presets.map((p) => [p.id, p.domains])),
)

function applyAllowlistPreset(name: string) {
  const extra = ALLOWLIST_PRESETS[name]
  if (!extra) return
  const cur = runtimeForm.value.sandboxAllowlistDomains
    .split(/[\n,]+/)
    .map((s) => s.trim())
    .filter(Boolean)
  const seen = new Set(cur.map((s) => s.toLowerCase()))
  for (const d of extra) {
    if (!seen.has(d.toLowerCase())) {
      cur.push(d)
      seen.add(d.toLowerCase())
    }
  }
  runtimeForm.value.sandboxAllowlistDomains = cur.join('\n')
  runtimeForm.value.sandboxNetwork = 'allowlist'
}

function formatTarBytes(n: number) {
  if (!n || n < 0) return '0 B'
  if (n < 1024) return `${n} B`
  if (n < 1024 * 1024) return `${(n / 1024).toFixed(1)} KB`
  return `${(n / (1024 * 1024)).toFixed(1)} MB`
}

async function downloadEnvTar(arch?: string) {
  await runtimeConfig.downloadEnvTar(arch)
}

const envTarVariants = computed(() => {
  const list = runtimeConfig.environmentStatus?.tarVariants
  if (list?.length) return list
  // Fallback until status loads
  return [
    { arch: 'amd64', present: false, downloadUrl: '', assetName: 'danmo-work-env-linux-amd64.tar' },
    { arch: 'arm64', present: false, downloadUrl: '', assetName: 'danmo-work-env-linux-arm64.tar' },
  ]
})

const browserStatusText = computed(() => {
  const st = runtimeConfig.browserStatus
  if (!st) return ''
  if (!st.available) {
    return st.degradedReason || 'unavailable'
  }
  const path = st.path ? ` @ ${st.path}` : ''
  return `${st.engine} (${st.mode})${path}`
})

const modelConfigForm = ref<ModelConfig[]>([])
const selectedModel = ref<string>('__default__')
const editingModelIdx = ref<number | null>(null)
const defaultForm = reactive({ context_window: 128000, max_output: 8192, temperature: 0.7 })
const showAddModelDialog = ref(false)
const newModelName = ref('')
const newContextWindow = ref(0)
const newMaxOutput = ref(0)
const newTemperature = ref(0)
const newAvailableEfforts = ref('')
const newThinkingMode = ref('')
const newEffortBudgetTokens = ref('')

function parseEffortBudgetTokens(raw: string): Record<string, number> | undefined {
  if (!raw.trim()) return undefined
  const map: Record<string, number> = {}
  for (const line of raw.split('\n')) {
    const trimmed = line.trim()
    if (!trimmed) continue
    const [k, v] = trimmed.split(':')
    const n = Number(v)
    if (k && !isNaN(n)) map[k.trim()] = n
  }
  return Object.keys(map).length > 0 ? map : undefined
}

function formatEffortBudgetTokens(map: Record<string, number> | undefined): string {
  if (!map) return ''
  return Object.entries(map).map(([k, v]) => `${k}:${v}`).join('\n')
}

onMounted(async () => {
  void initAppVersion()
  // Soft-fail each loader so one missing/optional API (e.g. channels on an old
  // sidecar) cannot take down the whole Settings view via unhandledrejection.
  await Promise.allSettled([
    llm.loadConfigs(),
    llm.loadModels(),
    llm.loadPresets(),
    searchConfig.loadConfig(),
    runtimeConfig.loadConfig(),
    marketConfig.loadConfig(),
    modelConfig.load(),
    sessions.loadCatalog(),
    projects.loadProjects(),
    weixin.refreshStatus(),
    feishu.refreshStatus(),
    wecom.refreshStatus(),
    qq.refreshStatus(),
  ])
  if (!weixinLoginProjectId.value && projects.sortedProjects.length) {
    weixinLoginProjectId.value = projects.sortedProjects[0].id
  }
  if (searchConfig.config) {
    searchForm.value = {
      provider: searchConfig.config.provider,
      baseUrl: searchConfig.config.baseUrl ?? '',
      apiKey: searchConfig.config.apiKey ?? '',
      timeoutMs: searchConfig.config.timeoutMs ?? 15000,
      maxResults: searchConfig.config.maxResults ?? 5,
      proxy: searchConfig.config.proxy ?? '',
      userAgent: searchConfig.config.userAgent ?? '',
      htmlFallback: searchConfig.config.htmlFallback ?? true,
    }
  }
  if (runtimeConfig.config) {
    runtimeForm.value = { ...runtimeConfig.config }
  }
  if (marketConfig.config) {
    marketForm.value = {
      cacheTtlHours: marketConfig.config.cacheTtlHours,
      sources: marketConfig.config.sources.map((s) => ({ ...s })),
    }
  }
  modelConfigForm.value = [...modelConfig.models]
  if (weixin.status) {
    weixinForm.value = {
      enabled: weixin.status.enabled,
      defaultAgentId: weixin.status.defaultAgentId || sessions.agents[0]?.id || '',
      defaultModelId: weixin.status.defaultModelId || '',
      autoApprove: !!weixin.status.autoApprove,
    }
  } else if (sessions.agents.length) {
    weixinForm.value.defaultAgentId = sessions.agents[0].id
  }
  if (feishu.status) {
    feishuForm.value = {
      enabled: feishu.status.enabled,
      defaultAgentId: feishu.status.defaultAgentId || sessions.agents[0]?.id || '',
      defaultModelId: feishu.status.defaultModelId || '',
      autoApprove: !!feishu.status.autoApprove,
      domain: (feishu.status.domain === 'lark' ? 'lark' : 'feishu'),
      appId: feishu.status.appId || '',
      appSecret: '',
      projectId: feishu.status.projectId || projects.sortedProjects[0]?.id || '',
      richProgress: feishu.status.richProgress !== false,
    }
  } else if (sessions.agents.length) {
    feishuForm.value.defaultAgentId = sessions.agents[0].id
    if (!feishuForm.value.projectId && projects.sortedProjects.length) {
      feishuForm.value.projectId = projects.sortedProjects[0].id
    }
  }
  if (wecom.status) {
    wecomForm.value = {
      enabled: wecom.status.enabled,
      defaultAgentId: wecom.status.defaultAgentId || sessions.agents[0]?.id || '',
      defaultModelId: wecom.status.defaultModelId || '',
      autoApprove: wecom.status.autoApprove !== false,
      botId: wecom.status.botId || '',
      secret: '',
      wsUrl: wecom.status.wsUrl || '',
      projectId: wecom.status.projectId || projects.sortedProjects[0]?.id || '',
    }
  } else if (sessions.agents.length) {
    wecomForm.value.defaultAgentId = sessions.agents[0].id
    if (!wecomForm.value.projectId && projects.sortedProjects.length) {
      wecomForm.value.projectId = projects.sortedProjects[0].id
    }
  }
  if (qq.status) {
    qqForm.value = {
      enabled: qq.status.enabled,
      defaultAgentId: qq.status.defaultAgentId || sessions.agents[0]?.id || '',
      defaultModelId: qq.status.defaultModelId || '',
      autoApprove: !!qq.status.autoApprove,
      appId: qq.status.appId || '',
      clientSecret: '',
      projectId: qq.status.projectId || projects.sortedProjects[0]?.id || '',
      groupDenyTools: (qq.status.groupDenyTools || []).join(', '),
      requireMention: qq.status.requireMention !== false,
      nativeC2cStream: qq.status.nativeC2cStream !== false,
    }
  } else if (sessions.agents.length) {
    qqForm.value.defaultAgentId = sessions.agents[0].id
    if (!qqForm.value.projectId && projects.sortedProjects.length) {
      qqForm.value.projectId = projects.sortedProjects[0].id
    }
  }
})

async function handleSaveWeixin() {
  if (weixinForm.value.enabled && !weixinForm.value.defaultAgentId) {
    toast.warning(t('settings.weixinAgentRequired'))
    return
  }
  if (weixinForm.value.enabled && !weixinForm.value.defaultModelId) {
    toast.warning(t('settings.weixinModelRequired'))
    return
  }
  try {
    await weixin.configure({
      enabled: weixinForm.value.enabled,
      defaultAgentId: weixinForm.value.defaultAgentId,
      defaultModelId: weixinForm.value.defaultModelId,
      autoApprove: weixinForm.value.autoApprove,
    })
    toast.success(t('settings.weixinSaved'))
  } catch (e) {
    toast.error(e instanceof Error ? e.message : t('settings.weixinSaveFailed'))
  }
}

async function handleSaveFeishu() {
  if (feishuForm.value.enabled && !feishuForm.value.defaultAgentId) {
    toast.warning(t('settings.feishuAgentRequired'))
    return
  }
  if (feishuForm.value.enabled && !feishuForm.value.defaultModelId) {
    toast.warning(t('settings.feishuModelRequired'))
    return
  }
  if (feishuForm.value.enabled && !feishuForm.value.projectId) {
    toast.warning(t('settings.feishuProjectRequired'))
    return
  }
  if (feishuForm.value.enabled && !feishuForm.value.appId) {
    toast.warning(t('settings.feishuAppRequired'))
    return
  }
  if (feishuForm.value.enabled && !feishuForm.value.appSecret && !feishu.status?.hasAppSecret) {
    toast.warning(t('settings.feishuSecretRequired'))
    return
  }
  try {
    await feishu.configure({
      enabled: feishuForm.value.enabled,
      defaultAgentId: feishuForm.value.defaultAgentId,
      defaultModelId: feishuForm.value.defaultModelId,
      autoApprove: feishuForm.value.autoApprove,
      domain: feishuForm.value.domain,
      appId: feishuForm.value.appId || undefined,
      appSecret: feishuForm.value.appSecret || undefined,
      projectId: feishuForm.value.projectId || undefined,
      richProgress: feishuForm.value.richProgress,
    })
    feishuForm.value.appSecret = ''
    toast.success(t('settings.feishuSaved'))
  } catch (e) {
    toast.error(e instanceof Error ? e.message : t('settings.feishuSaveFailed'))
  }
}

async function handleSaveWecom() {
  if (wecomForm.value.enabled && !wecomForm.value.defaultAgentId) {
    toast.warning(t('settings.wecomAgentRequired'))
    return
  }
  if (wecomForm.value.enabled && !wecomForm.value.defaultModelId) {
    toast.warning(t('settings.wecomModelRequired'))
    return
  }
  if (wecomForm.value.enabled && !wecomForm.value.projectId) {
    toast.warning(t('settings.wecomProjectRequired'))
    return
  }
  if (wecomForm.value.enabled && !wecomForm.value.botId) {
    toast.warning(t('settings.wecomBotRequired'))
    return
  }
  if (wecomForm.value.enabled && !wecomForm.value.secret && !wecom.status?.hasSecret) {
    toast.warning(t('settings.wecomSecretRequired'))
    return
  }
  try {
    await wecom.configure({
      enabled: wecomForm.value.enabled,
      defaultAgentId: wecomForm.value.defaultAgentId,
      defaultModelId: wecomForm.value.defaultModelId,
      autoApprove: wecomForm.value.autoApprove,
      botId: wecomForm.value.botId || undefined,
      secret: wecomForm.value.secret || undefined,
      wsUrl: wecomForm.value.wsUrl || undefined,
      projectId: wecomForm.value.projectId || undefined,
    })
    wecomForm.value.secret = ''
    toast.success(t('settings.wecomSaved'))
  } catch (e) {
    toast.error(e instanceof Error ? e.message : t('settings.wecomSaveFailed'))
  }
}

async function handleSaveQQ() {
  if (qqForm.value.enabled && !qqForm.value.defaultAgentId) {
    toast.warning(t('settings.qqAgentRequired'))
    return
  }
  if (qqForm.value.enabled && !qqForm.value.defaultModelId) {
    toast.warning(t('settings.qqModelRequired'))
    return
  }
  if (qqForm.value.enabled && !qqForm.value.projectId) {
    toast.warning(t('settings.qqProjectRequired'))
    return
  }
  if (qqForm.value.enabled && !qqForm.value.appId) {
    toast.warning(t('settings.qqAppRequired'))
    return
  }
  if (qqForm.value.enabled && !qqForm.value.clientSecret && !qq.status?.hasClientSecret) {
    toast.warning(t('settings.qqSecretRequired'))
    return
  }
  try {
    const denyTools = qqForm.value.groupDenyTools
      .split(/[,，\s]+/)
      .map((s) => s.trim())
      .filter(Boolean)
    await qq.configure({
      enabled: qqForm.value.enabled,
      defaultAgentId: qqForm.value.defaultAgentId,
      defaultModelId: qqForm.value.defaultModelId,
      autoApprove: qqForm.value.autoApprove,
      appId: qqForm.value.appId || undefined,
      clientSecret: qqForm.value.clientSecret || undefined,
      projectId: qqForm.value.projectId || undefined,
      groupDenyTools: denyTools,
      requireMention: qqForm.value.requireMention,
      nativeC2cStream: qqForm.value.nativeC2cStream,
    })
    qqForm.value.clientSecret = ''
    toast.success(t('settings.qqSaved'))
  } catch (e) {
    toast.error(e instanceof Error ? e.message : t('settings.qqSaveFailed'))
  }
}

function weixinProjectName(projectId?: string) {
  if (!projectId) return t('settings.weixinUnbound')
  return projects.projects.find((p) => p.id === projectId)?.name || projectId
}

function weixinAccountLabel(accountId: string) {
  const short = accountId.replace(/_im\.bot$/i, '').replace(/@im\.bot$/i, '')
  return short || accountId
}

function openWeixinAddDialog() {
  weixinAddCancelled.value = false
  weixin.qr = null
  weixin.loginMessage = ''
  weixinPolling.value = false
  if (!weixinLoginProjectId.value && projects.sortedProjects.length) {
    weixinLoginProjectId.value = projects.sortedProjects[0].id
  }
  showWeixinAddDialog.value = true
}

function closeWeixinAddDialog() {
  weixinAddCancelled.value = true
  weixinPolling.value = false
  weixin.qr = null
  showWeixinAddDialog.value = false
}

async function handleWeixinQr() {
  if (!weixinLoginProjectId.value) {
    toast.warning(t('settings.weixinSelectProjectFirst'))
    return
  }
  weixinAddCancelled.value = false
  try {
    await weixin.startLogin(weixinLoginProjectId.value)
    weixinPolling.value = true
    try {
      const res = await weixin.waitLogin(180000)
      if (weixinAddCancelled.value) return
      if (res.needsVerifyCode) {
        toast.warning(res.message || t('settings.weixinNeedVerify'))
        return
      }
      if (res.connected) {
        toast.success(res.message || t('settings.weixinConnected'))
        showWeixinAddDialog.value = false
        weixin.qr = null
      } else {
        toast.warning(res.message || t('settings.weixinLoginFailed'))
      }
    } finally {
      weixinPolling.value = false
    }
  } catch (e) {
    if (weixinAddCancelled.value) return
    weixinPolling.value = false
    weixin.qr = null
    const msg = e instanceof Error ? e.message : t('settings.weixinLoginFailed')
    const status = e && typeof e === 'object' && 'status' in e ? `HTTP ${(e as { status: number }).status}` : ''
    toast.error(status ? `${msg} (${status})` : msg)
  }
}

async function handleWeixinRebind(accountId: string, projectId: string) {
  const current = weixin.status?.accounts.find((a) => a.accountId === accountId)?.projectId || ''
  if ((projectId || '') === current) return
  try {
    await weixin.updateAccountProject(accountId, projectId)
    toast.success(projectId ? t('settings.weixinRebound') : t('settings.weixinUnboundOk'))
  } catch (e) {
    toast.error(e instanceof Error ? e.message : t('settings.weixinRebindFailed'))
  }
}

async function handleWeixinDeleteAccount(accountId: string) {
  try {
    await confirm(t('settings.weixinDeleteConfirm'), t('settings.weixinDeleteAccount'), {
      confirmButtonText: t('common.delete'),
      type: 'warning',
    })
  } catch {
    return
  }
  try {
    await weixin.logout(accountId)
    toast.success(t('settings.weixinAccountDeleted'))
  } catch (e) {
    toast.error(e instanceof Error ? e.message : t('settings.weixinLogoutFailed'))
  }
}

function addMarketSource() {
  marketForm.value.sources.push(marketConfig.emptySource())
}

function isGitMarketKind(kind?: string) {
  return !kind || kind === 'git'
}

function marketRepoLabel(kind?: string) {
  if (kind === 'clawhub') return 'settings.marketClawhubRepo'
  if (kind === 'techleads' || kind === 'tlc' || kind === 'tech-leads-club') return 'settings.marketTechleadsRepo'
  return 'settings.marketRepo'
}

function marketRepoPlaceholder(kind?: string) {
  if (kind === 'clawhub') return 'https://clawhub.ai'
  if (kind === 'techleads' || kind === 'tlc' || kind === 'tech-leads-club') return '@tech-leads-club/skills-catalog'
  return undefined
}

function removeMarketSource(idx: number) {
  marketForm.value.sources.splice(idx, 1)
}

async function handleSaveMarket() {
  const ids = marketForm.value.sources.map((s) => s.id.trim()).filter(Boolean)
  if (new Set(ids).size !== ids.length) {
    toast.warning(t('settings.marketSourceId') + ' must be unique')
    return
  }
  try {
    await marketConfig.saveConfig({
      cacheTtlHours: marketForm.value.cacheTtlHours,
      sources: marketForm.value.sources.map((s: MarketSourceConfig) => ({ ...s })),
    })
    if (marketConfig.config) {
      marketForm.value = {
        cacheTtlHours: marketConfig.config.cacheTtlHours,
        sources: marketConfig.config.sources.map((s) => ({ ...s })),
      }
    }
  } catch {
    /* toast in store */
  }
}

const displayedModels = computed<LLMModelRef[]>(() => form.value.models)

function providerLabel(p: LLMProviderType) {
  return providerOptions.value.find((o) => o.value === p)?.label ?? p
}

function openNewForm() {
  editingId.value = null
  dialogStep.value = 'choose'
  manualModelName.value = ''
  form.value = {
    provider: 'openai',
    name: '',
    apiKey: '',
    baseUrl: '',
    models: [],
  }
  showForm.value = true
}

function selectPreset(preset: LLMProviderPreset) {
  form.value = {
    provider: preset.provider,
    name: preset.name,
    apiKey: '',
    baseUrl: preset.baseUrl,
    models: [],
  }
  dialogStep.value = 'configure'
}

function selectCustom() {
  form.value = {
    provider: 'openai',
    name: '',
    apiKey: '',
    baseUrl: '',
    models: [],
  }
  dialogStep.value = 'configure'
}

function backToChoose() {
  dialogStep.value = 'choose'
}

const presetColors: Record<string, string> = {
  openai: 'var(--dq-success)',
  anthropic: 'var(--dq-warning)',
  deepseek: 'var(--dq-accent)',
  google: 'var(--dq-info)',
  zhipu: 'var(--dq-danger)',
  qwen: 'var(--dq-system-orange)',
  moonshot: 'var(--dq-system-blue)',
  ollama: 'var(--dq-label-secondary)',
}

function presetColor(id: string) {
  return presetColors[id] ?? 'var(--dq-label-secondary)'
}

function presetAbbr(id: string) {
  const map: Record<string, string> = {
    openai: 'GPT',
    anthropic: 'C',
    deepseek: 'DS',
    google: 'G',
    zhipu: 'GLM',
    qwen: 'Q',
    moonshot: 'K',
    ollama: '🦙',
  }
  return map[id] ?? id[0]?.toUpperCase() ?? '?'
}

function openEditForm(cfg: LLMProviderConfig) {
  editingId.value = cfg.id
  dialogStep.value = 'configure'
  manualModelName.value = ''
  form.value = {
    provider: cfg.provider,
    name: cfg.name,
    apiKey: cfg.apiKey ?? '',
    baseUrl: cfg.baseUrl ?? '',
    models: [...(cfg.models ?? [])],
  }
  showForm.value = true
}

function cancelForm() {
  showForm.value = false
  editingId.value = null
  manualModelName.value = ''
}

function addManualModel() {
  const name = manualModelName.value.trim()
  if (!name) {
    toast.warning(t('settings.manualModelRequired'))
    return
  }
  if (form.value.models.some((m) => m.name === name)) {
    toast.warning(t('settings.manualModelDuplicate'))
    return
  }
  form.value.models = [...form.value.models, { name, enabled: true }]
  manualModelName.value = ''
}

function removeManualModel(modelName: string) {
  form.value.models = form.value.models.filter((m) => m.name !== modelName)
}

function onManualModelKeydown(e: KeyboardEvent) {
  if (e.key === 'Enter') {
    e.preventDefault()
    addManualModel()
  }
}

async function handleSave() {
  if (!form.value.name.trim()) {
    toast.warning(t('settings.namePlaceholder'))
    return
  }
  const models = form.value.models
  if (!models.some((m) => m.enabled)) {
    toast.warning(t('settings.enabledModelsRequired'))
    return
  }
  const payload = {
    provider: form.value.provider,
    name: form.value.name.trim(),
    apiKey: form.value.apiKey.trim() || undefined,
    baseUrl: form.value.baseUrl.trim() || undefined,
    models,
  }
  try {
    if (editingId.value) {
      await llm.updateConfig(editingId.value, payload)
    } else {
      await llm.saveConfig(payload)
    }
    cancelForm()
  } catch {
    /* toast already shown in store */
  }
}

async function handleDelete(id: string) {
  try {
    await llm.deleteConfig(id)
  } catch {
    /* toast already shown in store */
  }
}

async function handleRefreshModels() {
  refreshingModels.value = true
  try {
    let models: LLMModelRef[]
    if (editingId.value) {
      models = await llm.refreshModels(editingId.value)
    } else {
      models = await llm.fetchModels({
        provider: form.value.provider,
        name: form.value.name.trim(),
        apiKey: form.value.apiKey.trim(),
        baseUrl: form.value.baseUrl.trim(),
        models: [],
      })
    }
    // Keep any manually added models that the remote list does not return.
    const remoteNames = new Set(models.map((m) => m.name))
    const kept = form.value.models.filter((m) => !remoteNames.has(m.name))
    form.value.models = [...models, ...kept]
  } catch {
    toast.warning(t('settings.refreshFailedManualHint'))
  } finally {
    refreshingModels.value = false
  }
}

async function handleToggleModel(modelName: string, enabled: boolean) {
  const model = form.value.models.find((m) => m.name === modelName)
  if (model) {
    model.enabled = enabled
  }
}

async function handleSaveSearch() {
  const payload = {
    provider: searchForm.value.provider,
    baseUrl: searchForm.value.baseUrl.trim() || undefined,
    apiKey: searchForm.value.apiKey.trim() || undefined,
    timeoutMs: searchForm.value.timeoutMs,
    maxResults: searchForm.value.maxResults,
    proxy: searchForm.value.proxy.trim() || undefined,
    userAgent: searchForm.value.userAgent.trim() || undefined,
    htmlFallback: searchForm.value.htmlFallback,
  }
  try {
    await searchConfig.saveConfig(payload)
  } catch {
    /* toast already shown in store */
  }
}

async function handleSaveRuntime() {
  try {
    await runtimeConfig.saveConfig(runtimeForm.value)
  } catch {
    /* toast already shown in store */
  }
}

function onSelectModel(model: string) {
  const idx = modelConfigForm.value.findIndex(m => m.model === model)
  editingModelIdx.value = idx >= 0 ? idx : null
}

function addModelEntry() {
  newModelName.value = ''
  newContextWindow.value = 0
  newMaxOutput.value = 0
  newTemperature.value = 0
  newAvailableEfforts.value = ''
  newThinkingMode.value = ''
  newEffortBudgetTokens.value = ''
  showAddModelDialog.value = true
}

function confirmAddModel() {
  const name = newModelName.value.trim()
  if (!name) return
  if (modelConfigForm.value.find(m => m.model === name)) return
  showAddModelDialog.value = false
  const efforts = newAvailableEfforts.value.trim()
  modelConfigForm.value.push({
    model: name,
    context_window: newContextWindow.value || undefined,
    max_output: newMaxOutput.value || undefined,
    temperature: newTemperature.value || undefined,
    available_efforts: efforts ? efforts.split(',').map(s => s.trim()).filter(Boolean) : undefined,
    thinking_mode: newThinkingMode.value.trim() || undefined,
    effort_budget_tokens: parseEffortBudgetTokens(newEffortBudgetTokens.value),
  })
  editingModelIdx.value = modelConfigForm.value.length - 1
  selectedModel.value = name
}

function removeSelectedModel() {
  if (editingModelIdx.value === null) return
  modelConfigForm.value.splice(editingModelIdx.value, 1)
  editingModelIdx.value = null
  selectedModel.value = '__default__'
}

async function handleSaveModelConfig() {
  modelConfigForm.value = modelConfigForm.value.filter(m => m.model.trim() !== '')
  try {
    await modelConfig.save(modelConfigForm.value)
    // Composer effort selector reads /llm/models — refresh after config save.
    await llm.loadModels()
    sessions.syncModelSelection(llm.models)
  } catch {
    /* toast already shown in store */
  }
}


const menuItems = computed(() => [
  { id: 'models' as SettingsTab, label: t('settings.models'), icon: Cpu },
  { id: 'modelConfig' as SettingsTab, label: t('settings.modelConfig'), icon: Setting },
  { id: 'search' as SettingsTab, label: t('settings.search'), icon: Search },
  { id: 'runtime' as SettingsTab, label: t('settings.runtime'), icon: Setting },
  { id: 'weixin' as SettingsTab, label: t('settings.weixin'), icon: Monitor },
  { id: 'feishu' as SettingsTab, label: t('settings.feishu'), icon: Monitor },
  { id: 'wecom' as SettingsTab, label: t('settings.wecom'), icon: Monitor },
  { id: 'qq' as SettingsTab, label: t('settings.qq'), icon: Monitor },
  { id: 'market' as SettingsTab, label: t('settings.market'), icon: Search },
  { id: 'appearance' as SettingsTab, label: t('settings.appearance'), icon: Brush },
  { id: 'about' as SettingsTab, label: t('settings.about'), icon: Monitor },
])

const footerHint = computed(() => {
  switch (activeTab.value) {
    case 'runtime': return t('common.saveShortcut')
    case 'search': return t('common.saveShortcut')
    case 'market': return t('settings.marketRestartHint')
    case 'models': return t('settings.modelsHint')
    case 'modelConfig': return t('settings.modelConfigHint')
    case 'about': return t('settings.aboutHint')
    default: return ''
  }
})

const updaterStatusText = computed(() => {
  switch (updaterStatus.value) {
    case 'checking':
      return t('updater.checking')
    case 'upToDate':
      return t('updater.upToDate')
    case 'available':
      return t('updater.availableToast', { version: availableVersion.value })
    case 'downloading':
      return downloadPercent.value != null
        ? t('updater.downloadingProgress', { percent: downloadPercent.value })
        : t('updater.downloading')
    case 'installing':
      return t('updater.installing')
    case 'error':
      return updaterError.value || t('updater.failed')
    default:
      return isDesktop ? t('updater.checkHint') : t('updater.desktopOnly')
  }
})

const hasFooterActions = computed(() => {
  return ['runtime', 'search', 'market', 'models', 'modelConfig', 'weixin', 'feishu', 'wecom', 'qq'].includes(activeTab.value)
})
</script>

<template>
  <div class="settings-view">
    <aside class="settings-sidebar">
      <div class="settings-sidebar__head">
        <span class="settings-sidebar__title">{{ $t('settings.title') }}</span>
      </div>
      <nav class="settings-sidebar__menu" :aria-label="$t('settings.category')">
        <button
          v-for="item in menuItems"
          :key="item.id"
          type="button"
          class="settings-sidebar__item"
          :class="{ 'is-active': activeTab === item.id }"
          @click="activeTab = item.id"
        >
          <DqIcon :size="18">
            <component :is="item.icon" />
          </DqIcon>
          <span>{{ item.label }}</span>
        </button>
      </nav>
    </aside>

    <main class="settings-panel">
      <div class="settings-panel__content">
      <div v-if="activeTab === 'appearance'" class="settings-section">
        <header class="settings-section__head">
          <h2>{{ $t('settings.appearance') }}</h2>
          <p>{{ $t('settings.appearanceDesc') }}</p>
        </header>

        <div class="theme-grid">
          <button
            v-for="theme in THEME_OPTIONS"
            :key="theme.id"
            type="button"
            class="theme-card"
            :class="{ 'is-active': themeStore.currentTheme === theme.id }"
            @click="themeStore.setTheme(theme.id as ThemeId)"
          >
            <div class="theme-card__preview" :style="{ '--preview-accent': theme.accent }">
              <div class="theme-card__preview-accent"></div>
              <div class="theme-card__preview-surface"></div>
              <div class="theme-card__preview-text"></div>
            </div>
            <div class="theme-card__info">
              <span class="theme-card__name">{{ theme.label }}</span>
              <span class="theme-card__desc">{{ theme.description }}</span>
            </div>
            <div v-if="themeStore.currentTheme === theme.id" class="theme-card__check">
              <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="3" stroke-linecap="round" stroke-linejoin="round">
                <polyline points="20 6 9 17 4 12"></polyline>
              </svg>
            </div>
          </button>
        </div>
      </div>

      <div v-else-if="activeTab === 'runtime'" class="settings-section">
        <header class="settings-section__head">
          <h2>{{ $t('settings.runtime') }}</h2>
          <p>{{ $t('settings.runtimeDesc') }}</p>
        </header>

        <div v-if="runtimeConfig.loading" class="settings-empty settings-empty--skeleton">
          <Skeleton variant="title" width="30%" />
          <Skeleton variant="card" width="100%" />
          <Skeleton variant="card" width="100%" />
        </div>

        <div v-else class="settings-form">
          <div class="settings-form-group">
            <h3 class="settings-form-group__title">{{ $t('settings.runtimeTurn') }}</h3>
            <p class="settings-form-group__desc">{{ $t('settings.runtimeTurnDesc') }}</p>
            <div class="settings-form-row">
              <div class="settings-field settings-field--half">
                <span class="settings-field__label">{{ $t('settings.doomLoopThreshold') }}</span>
                <div class="slider-row">
                  <DqSlider v-model="runtimeForm.doomLoopThreshold" :min="1" :max="50" :step="1" />
                  <span class="slider-row__value">{{ runtimeForm.doomLoopThreshold }}</span>
                </div>
              </div>
              <div class="settings-field settings-field--half">
                <span class="settings-field__label">{{ $t('settings.maxStepsDefault') }}</span>
                <div class="slider-row">
                  <DqSlider v-model="runtimeForm.maxStepsDefault" :min="5" :max="500" :step="1" />
                  <span class="slider-row__value">{{ runtimeForm.maxStepsDefault }}</span>
                </div>
              </div>
            </div>
            <div class="settings-form-row">
              <div class="settings-field settings-field--half">
                <span class="settings-field__label">{{ $t('settings.maxLLMFailures') }}</span>
                <div class="slider-row">
                  <DqSlider v-model="runtimeForm.maxLLMFailures" :min="1" :max="20" :step="1" />
                  <span class="slider-row__value">{{ runtimeForm.maxLLMFailures }}</span>
                </div>
              </div>
            </div>
          </div>

          <div class="settings-form-group">
            <h3 class="settings-form-group__title">{{ $t('settings.runtimeTeam') }}</h3>
            <p class="settings-form-group__desc">{{ $t('settings.runtimeTeamDesc') }}</p>
            <div class="settings-field">
              <span class="settings-field__label">{{ $t('settings.maxDelegationDepth') }}</span>
              <div class="slider-row">
                <DqSlider v-model="runtimeForm.maxDelegationDepth" :min="1" :max="10" :step="1" />
                <span class="slider-row__value">{{ runtimeForm.maxDelegationDepth }}</span>
              </div>
            </div>
          </div>

          <div class="settings-form-group">
            <h3 class="settings-form-group__title">{{ $t('settings.runtimeMemoryKnowledge') }}</h3>
            <p class="settings-form-group__desc">{{ $t('settings.runtimeMemoryKnowledgeDesc') }}</p>
            <div class="settings-form-row">
              <div class="settings-field settings-field--half">
                <span class="settings-field__label">{{ $t('settings.readTopK') }}</span>
                <div class="slider-row">
                  <DqSlider v-model="runtimeForm.readTopK" :min="1" :max="20" :step="1" />
                  <span class="slider-row__value">{{ runtimeForm.readTopK }}</span>
                </div>
              </div>
              <div class="settings-field settings-field--half">
                <span class="settings-field__label">{{ $t('settings.searchTopK') }}</span>
                <div class="slider-row">
                  <DqSlider v-model="runtimeForm.searchTopK" :min="1" :max="20" :step="1" />
                  <span class="slider-row__value">{{ runtimeForm.searchTopK }}</span>
                </div>
              </div>
            </div>
            <div class="settings-form-row">
              <div class="settings-field settings-field--half">
                <span class="settings-field__label">{{ $t('settings.chapterMaxTokens') }}</span>
                <div class="slider-row">
                  <DqSlider v-model="runtimeForm.chapterMaxTokens" :min="128" :max="2048" :step="64" />
                  <span class="slider-row__value">{{ runtimeForm.chapterMaxTokens }}</span>
                </div>
              </div>
              <div class="settings-field settings-field--half">
                <span class="settings-field__label">{{ $t('settings.vectorHybrid') }}</span>
                <DqSwitch v-model="runtimeForm.vectorHybrid" />
              </div>
            </div>
          </div>

          <div class="settings-form-group">
            <h3 class="settings-form-group__title">{{ $t('settings.runtimeTools') }}</h3>
            <p class="settings-form-group__desc">{{ $t('settings.runtimeToolsDesc') }}</p>
            <div class="settings-field">
              <span class="settings-field__label">{{ $t('settings.maxToolOutputChars') }}</span>
              <div class="slider-row">
                <DqSlider v-model="runtimeForm.maxToolOutputChars" :min="5000" :max="200000" :step="5000" />
                <span class="slider-row__value">{{ runtimeForm.maxToolOutputChars }}</span>
              </div>
            </div>
          </div>

          <div class="settings-form-group">
            <h3 class="settings-form-group__title">{{ $t('settings.runtimeAutoApprove') }}</h3>
            <p class="settings-form-group__desc">{{ $t('settings.runtimeAutoApproveDesc') }}</p>
            <label class="settings-field settings-field--switch">
              <span class="settings-field__label">{{ $t('settings.sandboxEnabled') }}</span>
              <DqSwitch
                :model-value="runtimeForm.sandboxEnabled"
                size="sm"
                @update:model-value="(v: boolean) => runtimeForm.sandboxEnabled = v"
              />
            </label>
            <template v-if="runtimeForm.sandboxEnabled">
              <div class="settings-form-row">
                <div class="settings-field settings-field--half">
                  <span class="settings-field__label">{{ $t('settings.sandboxMode') }}</span>
                  <DqSelect v-model="runtimeForm.sandboxMode">
                    <DqOption value="read-only" :label="$t('settings.sandboxModeReadOnly')" />
                    <DqOption value="workspace-write" :label="$t('settings.sandboxModeWorkspaceWrite')" />
                    <DqOption value="danger-full-access" :label="$t('settings.sandboxModeDanger')" />
                  </DqSelect>
                </div>
                <div class="settings-field settings-field--half">
                  <span class="settings-field__label">{{ $t('settings.sandboxNetwork') }}</span>
                  <DqSelect v-model="runtimeForm.sandboxNetwork">
                    <DqOption value="deny" :label="$t('settings.sandboxNetworkDeny')" />
                    <DqOption value="allowlist" :label="$t('settings.sandboxNetworkAllowlistRecommended')" />
                    <DqOption value="allow" :label="$t('settings.sandboxNetworkAllow')" />
                  </DqSelect>
                </div>
              </div>
              <p class="settings-form-group__desc">{{ $t('settings.sandboxNetworkDesc') }}</p>
              <template v-if="runtimeForm.sandboxNetwork === 'allowlist'">
                <div class="settings-field">
                  <span class="settings-field__label">{{ $t('settings.sandboxAllowlistDomains') }}</span>
                  <DqInput
                    v-model="runtimeForm.sandboxAllowlistDomains"
                    type="textarea"
                    :rows="5"
                    :placeholder="$t('settings.sandboxAllowlistDomainsPlaceholder')"
                  />
                </div>
                <p class="settings-form-group__desc">{{ $t('settings.sandboxAllowlistPresetsDesc') }}</p>
                <div
                  v-for="group in ALLOWLIST_PRESET_GROUPS"
                  :key="group.id"
                  class="settings-allowlist-presets"
                  style="margin-top: 10px"
                >
                  <span class="settings-field__label">{{ $t(`settings.sandboxPresetGroup_${group.id}`) }}</span>
                  <div class="settings-form-row" style="margin-top: 6px; gap: 8px; flex-wrap: wrap">
                    <DqButton
                      v-for="p in group.presets"
                      :key="p.id"
                      size="sm"
                      @click="applyAllowlistPreset(p.id)"
                    >
                      {{ $t(`settings.sandboxPreset_${p.id}`) }}
                    </DqButton>
                  </div>
                </div>
                <p class="settings-form-group__desc">{{ $t('settings.sandboxAllowlistDomainsDesc') }}</p>
              </template>
              <div class="settings-form-row">
                <div class="settings-field settings-field--half">
                  <span class="settings-field__label">{{ $t('settings.sandboxShell') }}</span>
                  <DqSelect v-model="runtimeForm.sandboxShell">
                    <DqOption value="auto" :label="$t('settings.sandboxShellAuto')" />
                    <DqOption value="bash" :label="$t('settings.sandboxShellBash')" />
                    <DqOption value="cmd" :label="$t('settings.sandboxShellCmd')" />
                  </DqSelect>
                </div>
              </div>
              <p class="settings-form-group__desc">{{ $t('settings.sandboxShellDesc') }}</p>
              <div v-if="sandboxStatusText" class="settings-sandbox-status">
                <span class="settings-field__label">{{ $t('settings.sandboxStatus') }}</span>
                <code class="settings-sandbox-status__value">{{ sandboxStatusText }}</code>
                <p
                  v-if="runtimeConfig.sandboxStatus?.degraded && runtimeConfig.sandboxStatus.degradedReason"
                  class="settings-sandbox-status__degraded"
                >
                  {{ $t('settings.sandboxDegraded') }}: {{ runtimeConfig.sandboxStatus.degradedReason }}
                </p>
                <p v-if="showGitBashHint" class="settings-sandbox-status__degraded">
                  {{ $t('settings.sandboxShellHint') }}
                </p>
              </div>
            </template>
            <p class="settings-form-group__desc">{{ $t('settings.envBackendDesc') }}</p>
            <div class="settings-form-row">
              <div class="settings-field settings-field--half">
                <span class="settings-field__label">{{ $t('settings.envBackend') }}</span>
                <DqSelect v-model="runtimeForm.envBackend">
                  <DqOption value="local" :label="$t('settings.envBackendLocal')" />
                  <DqOption value="container" :label="$t('settings.envBackendContainer')" />
                </DqSelect>
              </div>
              <div v-if="runtimeForm.envBackend === 'container'" class="settings-field settings-field--half">
                <span class="settings-field__label">{{ $t('settings.envEngine') }}</span>
                <DqSelect v-model="runtimeForm.envEngine">
                  <DqOption value="auto" :label="$t('settings.envEngineAuto')" />
                  <DqOption value="podman" label="Podman" />
                  <DqOption value="docker" label="Docker" />
                  <DqOption value="apple-container" :label="$t('settings.envEngineApple')" />
                </DqSelect>
              </div>
            </div>
            <template v-if="runtimeForm.envBackend === 'container'">
              <p class="settings-form-group__desc">{{ $t('settings.envTarVariantsDesc') }}</p>
              <div
                v-for="v in envTarVariants"
                :key="v.arch"
                class="settings-form-row"
                style="align-items: center; gap: 12px; flex-wrap: wrap; margin-bottom: 8px"
              >
                <DqButton
                  size="sm"
                  :loading="runtimeConfig.downloadingTar === v.arch"
                  :disabled="!!runtimeConfig.downloadingTar"
                  @click="downloadEnvTar(v.arch)"
                >
                  {{
                    (v.present
                      ? $t('settings.envTarRedownloadArch', { arch: v.arch })
                      : $t('settings.envTarDownloadArch', { arch: v.arch }))
                      + (v.recommended ? ` (${$t('settings.envTarRecommended')})` : '')
                  }}
                </DqButton>
                <a
                  v-if="v.downloadUrl"
                  class="settings-link"
                  :href="v.downloadUrl"
                  target="_blank"
                  rel="noopener noreferrer"
                >
                  {{ v.assetName }}
                </a>
                <span v-if="v.present" class="settings-form-group__desc" style="margin: 0">
                  {{ $t('settings.envTarInstalled', {
                    path: v.path || '',
                    size: formatTarBytes(v.bytes || 0),
                  }) }}
                </span>
                <span v-else class="settings-form-group__desc" style="margin: 0">
                  {{ $t('settings.envTarMissingArch', { arch: v.arch }) }}
                </span>
              </div>
              <p class="settings-form-group__desc">{{ $t('settings.envResourcesDesc') }}</p>
              <div class="settings-form-row">
                <div class="settings-field settings-field--half">
                  <span class="settings-field__label">{{ $t('settings.envCpus') }}</span>
                  <DqInput v-model="runtimeForm.envCpus" :placeholder="$t('settings.envCpusPlaceholder')" />
                </div>
                <div class="settings-field settings-field--half">
                  <span class="settings-field__label">{{ $t('settings.envMemory') }}</span>
                  <DqInput v-model="runtimeForm.envMemory" :placeholder="$t('settings.envMemoryPlaceholder')" />
                </div>
              </div>
              <div v-if="environmentStatusText" class="settings-sandbox-status">
                <span class="settings-field__label">{{ $t('settings.envStatus') }}</span>
                <code class="settings-sandbox-status__value">{{ environmentStatusText }}</code>
                <p
                  v-if="runtimeConfig.environmentStatus?.degraded && runtimeConfig.environmentStatus.degradedReason"
                  class="settings-sandbox-status__degraded"
                >
                  {{ $t('settings.sandboxDegraded') }}: {{ runtimeConfig.environmentStatus.degradedReason }}
                </p>
              </div>
            </template>
            <p class="settings-form-group__desc">{{ $t('settings.browserEnabledDesc') }}</p>
            <label class="settings-field settings-field--switch">
              <span class="settings-field__label">{{ $t('settings.browserEnabled') }}</span>
              <DqSwitch
                :model-value="runtimeForm.browserEnabled"
                size="sm"
                @update:model-value="(v: boolean) => runtimeForm.browserEnabled = v"
              />
            </label>
            <template v-if="runtimeForm.browserEnabled">
              <div class="settings-form-row">
                <div class="settings-field settings-field--half">
                  <span class="settings-field__label">{{ $t('settings.browserExecutablePath') }}</span>
                  <DqInput
                    v-model="runtimeForm.browserExecutablePath"
                    :placeholder="$t('settings.browserExecutablePathPlaceholder')"
                  />
                </div>
                <div class="settings-field settings-field--half">
                  <span class="settings-field__label">{{ $t('settings.browserCdpUrl') }}</span>
                  <DqInput
                    v-model="runtimeForm.browserCdpUrl"
                    :placeholder="$t('settings.browserCdpUrlPlaceholder')"
                  />
                </div>
              </div>
              <div v-if="browserStatusText" class="settings-sandbox-status">
                <span class="settings-field__label">{{ $t('settings.browserStatus') }}</span>
                <code class="settings-sandbox-status__value">{{ browserStatusText }}</code>
                <p
                  v-if="!runtimeConfig.browserStatus?.available && runtimeConfig.browserStatus?.degradedReason"
                  class="settings-sandbox-status__degraded"
                >
                  {{ $t('settings.browserUnavailable') }}: {{ runtimeConfig.browserStatus.degradedReason }}
                </p>
              </div>
            </template>
            <label class="settings-field settings-field--switch">
              <span class="settings-field__label">{{ $t('settings.autoApprove') }}</span>
              <DqSwitch
                :model-value="runtimeForm.autoApprove"
                size="sm"
                @update:model-value="(v: boolean) => runtimeForm.autoApprove = v"
              />
            </label>
            <label class="settings-field">
              <span class="settings-field__label">{{ $t('settings.permissionMode') }}</span>
              <DqSelect v-model="runtimeForm.permissionMode" :placeholder="$t('settings.permissionMode')">
                <DqOption value="discuss" :label="$t('settings.permDiscuss')" />
                <DqOption value="plan" :label="$t('settings.permPlan')" />
                <DqOption value="interactive" :label="$t('settings.permInteractive')" />
                <DqOption value="auto" :label="$t('settings.permAuto')" />
              </DqSelect>
              <span class="settings-field__hint">{{ $t('settings.permissionModeHint') }}</span>
            </label>
          </div>

          <div class="settings-form-group">
            <h3 class="settings-form-group__title">{{ $t('settings.runtimeCompaction') }}</h3>
            <p class="settings-form-group__desc">{{ $t('settings.runtimeCompactionDesc') }}</p>
            <label class="settings-field settings-field--switch">
              <span class="settings-field__label">{{ $t('settings.compactionEnabled') }}</span>
              <DqSwitch
                :model-value="runtimeForm.compactionEnabled"
                size="sm"
                @update:model-value="(v: boolean) => runtimeForm.compactionEnabled = v"
              />
            </label>
            <template v-if="runtimeForm.compactionEnabled">
              <div class="settings-form-row">
                <div class="settings-field settings-field--half">
                  <span class="settings-field__label">{{ $t('settings.compactionMaxTokens') }}</span>
                  <div class="slider-row">
                    <DqSlider v-model="runtimeForm.compactionMaxTokens" :min="16000" :max="256000" :step="1000" />
                    <span class="slider-row__value">{{ runtimeForm.compactionMaxTokens }}</span>
                  </div>
                </div>
                <div class="settings-field settings-field--half">
                  <span class="settings-field__label">{{ $t('settings.compactionTriggerRatio') }}</span>
                  <div class="slider-row">
                    <DqSlider v-model="runtimeForm.compactionTriggerRatio" :min="0.1" :max="1.0" :step="0.05" />
                    <span class="slider-row__value">{{ runtimeForm.compactionTriggerRatio }}</span>
                  </div>
                </div>
              </div>
              <div class="settings-form-row">
                <div class="settings-field settings-field--half">
                  <span class="settings-field__label">{{ $t('settings.compactionCutTokens') }}</span>
                  <div class="slider-row">
                    <DqSlider v-model="runtimeForm.compactionCutTokens" :min="1000" :max="64000" :step="1000" />
                    <span class="slider-row__value">{{ runtimeForm.compactionCutTokens }}</span>
                  </div>
                </div>
                <div class="settings-field settings-field--half">
                  <span class="settings-field__label">{{ $t('settings.compactionToolTruncate') }}</span>
                  <div class="slider-row">
                    <DqSlider v-model="runtimeForm.compactionToolTruncate" :min="500" :max="8000" :step="500" />
                    <span class="slider-row__value">{{ runtimeForm.compactionToolTruncate }}</span>
                  </div>
                </div>
              </div>
              <div class="settings-form-row">
                <div class="settings-field settings-field--half">
                  <span class="settings-field__label">{{ $t('settings.compactionTurnInterval') }}</span>
                  <div class="slider-row">
                    <DqSlider v-model="runtimeForm.compactionTurnInterval" :min="1" :max="50" :step="1" />
                    <span class="slider-row__value">{{ runtimeForm.compactionTurnInterval }}</span>
                  </div>
                </div>
                <div class="settings-field settings-field--half">
                  <span class="settings-field__label">{{ $t('settings.compactionSubInterval') }}</span>
                  <div class="slider-row">
                    <DqSlider v-model="runtimeForm.compactionSubInterval" :min="1" :max="50" :step="1" />
                    <span class="slider-row__value">{{ runtimeForm.compactionSubInterval }}</span>
                  </div>
                </div>
              </div>
            </template>
          </div>

        </div>
      </div>

      <div v-else-if="activeTab === 'weixin'" class="settings-section">
        <header class="settings-section__head">
          <h2>{{ $t('settings.weixin') }}</h2>
          <p>{{ $t('settings.weixinDesc') }}</p>
        </header>

        <div v-if="weixin.loading && !weixin.status" class="settings-empty settings-empty--skeleton">
          <Skeleton variant="title" width="30%" />
          <Skeleton variant="card" width="100%" />
        </div>

        <div v-else class="settings-form">
          <div class="settings-form-group">
            <h3 class="settings-form-group__title">{{ $t('settings.weixinChannel') }}</h3>
            <p class="settings-form-group__desc">{{ $t('settings.weixinChannelDesc') }}</p>
            <label class="settings-field settings-field--switch">
              <span class="settings-field__label">{{ $t('settings.weixinEnable') }}</span>
              <DqSwitch
                :model-value="weixinForm.enabled"
                size="sm"
                @update:model-value="(v: boolean) => weixinForm.enabled = v"
              />
            </label>
            <div class="settings-field">
              <span class="settings-field__label">{{ $t('settings.weixinDefaultAgent') }}</span>
              <DqSelect v-model="weixinForm.defaultAgentId" :placeholder="$t('settings.weixinSelectAgent')">
                <DqOption
                  v-for="a in sessions.agents"
                  :key="a.id"
                  :value="a.id"
                  :label="a.name || a.id"
                />
              </DqSelect>
            </div>
            <div class="settings-field">
              <span class="settings-field__label">{{ $t('settings.weixinDefaultModel') }}</span>
              <DqSelect v-model="weixinForm.defaultModelId" :placeholder="$t('settings.weixinSelectModel')">
                <DqOption
                  v-for="m in llm.models"
                  :key="m.id"
                  :value="m.id"
                  :label="m.id"
                />
              </DqSelect>
            </div>
            <label class="settings-field settings-field--switch">
              <span class="settings-field__label">{{ $t('settings.channelAutoApprove') }}</span>
              <DqSwitch
                :model-value="weixinForm.autoApprove"
                size="sm"
                @update:model-value="(v: boolean) => weixinForm.autoApprove = v"
              />
            </label>
            <p class="settings-form-group__desc">{{ $t('settings.channelAutoApproveHint') }}</p>
            <p class="settings-form-group__desc">{{ $t('settings.weixinProjectHint') }}</p>
            <p v-if="weixin.status" class="weixin-meta">
              <span :class="['weixin-meta__dot', weixin.status.running ? 'is-on' : 'is-off']" />
              {{ weixin.status.running ? $t('settings.weixinRunningOn') : $t('settings.weixinRunningOff') }}
              · {{ weixin.status.accounts?.length || 0 }} {{ $t('settings.weixinAccounts') }}
              · {{ weixin.status.bindingCount }} {{ $t('settings.weixinBindings') }}
            </p>
          </div>

          <div class="settings-form-group">
            <div class="settings-form-group__head">
              <span class="settings-form-group__title">{{ $t('settings.weixinAccounts') }}</span>
              <DqButton type="primary" size="sm" @click="openWeixinAddDialog">
                {{ $t('settings.weixinLogin') }}
              </DqButton>
            </div>
            <p class="settings-form-group__desc">{{ $t('settings.weixinAccountsDesc') }}</p>

            <div v-if="!weixin.status?.accounts?.length" class="settings-empty">
              {{ $t('settings.weixinNoAccounts') }}
            </div>

            <div v-else class="provider-list weixin-account-list">
              <div
                v-for="acc in weixin.status.accounts"
                :key="acc.accountId"
                class="provider-card weixin-account-card"
              >
                <div class="provider-card__head">
                  <div class="provider-card__info weixin-account-card__info">
                    <span class="provider-card__name">{{ weixinAccountLabel(acc.accountId) }}</span>
                    <span
                      class="weixin-account-badge"
                      :class="acc.projectId ? 'is-bound' : 'is-unbound'"
                    >
                      {{ acc.projectId ? weixinProjectName(acc.projectId) : $t('settings.weixinUnbound') }}
                    </span>
                  </div>
                  <div class="provider-card__actions">
                    <DqButton size="sm" type="danger" @click="handleWeixinDeleteAccount(acc.accountId)">
                      {{ $t('settings.weixinDeleteAccount') }}
                    </DqButton>
                  </div>
                </div>
                <div class="weixin-account-card__body">
                  <label class="settings-field weixin-account-card__project">
                    <span class="settings-field__label">{{ $t('settings.weixinBindProject') }}</span>
                    <DqSelect
                      :model-value="acc.projectId || ''"
                      :placeholder="$t('settings.weixinSelectProject')"
                      @update:model-value="(v: string) => handleWeixinRebind(acc.accountId, v)"
                    >
                      <DqOption value="" :label="$t('settings.weixinUnbound')" />
                      <DqOption
                        v-for="p in projects.sortedProjects"
                        :key="p.id"
                        :value="p.id"
                        :label="p.name"
                      />
                    </DqSelect>
                  </label>
                  <p v-if="acc.userId" class="weixin-account-card__hint">{{ acc.userId }}</p>
                </div>
              </div>
            </div>
          </div>
        </div>
      </div>

      <div v-else-if="activeTab === 'feishu'" class="settings-section">
        <header class="settings-section__head">
          <h2>{{ $t('settings.feishu') }}</h2>
          <p>{{ $t('settings.feishuDesc') }}</p>
        </header>

        <div v-if="feishu.loading && !feishu.status" class="settings-empty settings-empty--skeleton">
          <Skeleton variant="title" width="30%" />
          <Skeleton variant="card" width="100%" />
        </div>

        <div v-else class="settings-form">
          <div class="settings-form-group">
            <h3 class="settings-form-group__title">{{ $t('settings.feishuChannel') }}</h3>
            <p class="settings-form-group__desc">{{ $t('settings.feishuChannelDesc') }}</p>
            <label class="settings-field settings-field--switch">
              <span class="settings-field__label">{{ $t('settings.feishuEnable') }}</span>
              <DqSwitch
                :model-value="feishuForm.enabled"
                size="small"
                @update:model-value="(v: boolean) => feishuForm.enabled = v"
              />
            </label>
            <div v-if="feishu.status" class="settings-sandbox-status">
              <span class="settings-field__label">{{ $t('settings.feishuRunning') }}</span>
              <span class="settings-sandbox-status__value">{{ feishu.status.running ? $t('common.yes') : $t('common.no') }}</span>
            </div>
            <div class="settings-field">
              <span class="settings-field__label">{{ $t('settings.feishuDomain') }}</span>
              <DqSelect v-model="feishuForm.domain">
                <DqOption value="feishu" :label="$t('settings.feishuDomainFeishu')" />
                <DqOption value="lark" :label="$t('settings.feishuDomainLark')" />
              </DqSelect>
            </div>
            <div class="settings-field">
              <span class="settings-field__label">{{ $t('settings.weixinDefaultAgent') }}</span>
              <DqSelect v-model="feishuForm.defaultAgentId" :placeholder="$t('settings.weixinSelectAgent')">
                <DqOption
                  v-for="a in sessions.agents"
                  :key="a.id"
                  :value="a.id"
                  :label="a.name || a.id"
                />
              </DqSelect>
            </div>
            <div class="settings-field">
              <span class="settings-field__label">{{ $t('settings.weixinDefaultModel') }}</span>
              <DqSelect v-model="feishuForm.defaultModelId" :placeholder="$t('settings.weixinSelectModel')">
                <DqOption
                  v-for="m in llm.models"
                  :key="m.id"
                  :value="m.id"
                  :label="m.id"
                />
              </DqSelect>
            </div>
            <div class="settings-field">
              <span class="settings-field__label">{{ $t('settings.feishuProject') }}</span>
              <DqSelect v-model="feishuForm.projectId" :placeholder="$t('settings.weixinSelectProject')">
                <DqOption
                  v-for="p in projects.sortedProjects"
                  :key="p.id"
                  :value="p.id"
                  :label="p.name"
                />
              </DqSelect>
            </div>
            <label class="settings-field settings-field--switch">
              <span class="settings-field__label">{{ $t('settings.channelAutoApprove') }}</span>
              <DqSwitch
                :model-value="feishuForm.autoApprove"
                size="small"
                @update:model-value="(v: boolean) => feishuForm.autoApprove = v"
              />
            </label>
            <p class="settings-form-group__desc">{{ $t('settings.channelAutoApproveHint') }}</p>
            <label class="settings-field settings-field--switch">
              <span class="settings-field__label">{{ $t('settings.feishuRichProgress') }}</span>
              <DqSwitch
                :model-value="feishuForm.richProgress"
                size="small"
                @update:model-value="(v: boolean) => feishuForm.richProgress = v"
              />
            </label>
            <p class="settings-form-group__desc">{{ $t('settings.feishuRichProgressHint') }}</p>
            <p class="settings-form-group__desc">{{ $t('settings.channelProjectHint') }}</p>
          </div>

          <div class="settings-form-group">
            <h3 class="settings-form-group__title">{{ $t('settings.feishuCredentials') }}</h3>
            <p class="settings-form-group__desc">{{ $t('settings.feishuCredentialsDesc') }}</p>
            <label class="settings-field">
              <span class="settings-field__label">App ID</span>
              <DqInput v-model="feishuForm.appId" placeholder="cli_xxx" />
            </label>
            <label class="settings-field">
              <span class="settings-field__label">App Secret</span>
              <DqInput
                v-model="feishuForm.appSecret"
                type="password"
                :placeholder="feishu.status?.hasAppSecret ? $t('settings.feishuSecretKept') : ''"
              />
            </label>
            <p class="settings-form-group__desc">{{ $t('settings.feishuWebsocketHint') }}</p>
          </div>
        </div>
      </div>

      <div v-else-if="activeTab === 'qq'" class="settings-section">
        <header class="settings-section__head">
          <h2>{{ $t('settings.qq') }}</h2>
          <p>{{ $t('settings.qqDesc') }}</p>
        </header>

        <div v-if="qq.loading && !qq.status" class="settings-empty settings-empty--skeleton">
          <Skeleton variant="title" width="30%" />
          <Skeleton variant="card" width="100%" />
        </div>

        <div v-else class="settings-form">
          <div class="settings-form-group">
            <h3 class="settings-form-group__title">{{ $t('settings.qqChannel') }}</h3>
            <p class="settings-form-group__desc">{{ $t('settings.qqChannelDesc') }}</p>
            <label class="settings-field settings-field--switch">
              <span class="settings-field__label">{{ $t('settings.qqEnable') }}</span>
              <DqSwitch
                :model-value="qqForm.enabled"
                size="small"
                @update:model-value="(v: boolean) => qqForm.enabled = v"
              />
            </label>
            <div v-if="qq.status" class="settings-sandbox-status">
              <span class="settings-field__label">{{ $t('settings.qqRunning') }}</span>
              <span class="settings-sandbox-status__value">{{ qq.status.running ? $t('common.yes') : $t('common.no') }}</span>
            </div>
            <div class="settings-field">
              <span class="settings-field__label">{{ $t('settings.weixinDefaultAgent') }}</span>
              <DqSelect v-model="qqForm.defaultAgentId" :placeholder="$t('settings.weixinSelectAgent')">
                <DqOption
                  v-for="a in sessions.agents"
                  :key="a.id"
                  :value="a.id"
                  :label="a.name || a.id"
                />
              </DqSelect>
            </div>
            <div class="settings-field">
              <span class="settings-field__label">{{ $t('settings.weixinDefaultModel') }}</span>
              <DqSelect v-model="qqForm.defaultModelId" :placeholder="$t('settings.weixinSelectModel')">
                <DqOption
                  v-for="m in llm.models"
                  :key="m.id"
                  :value="m.id"
                  :label="m.id"
                />
              </DqSelect>
            </div>
            <div class="settings-field">
              <span class="settings-field__label">{{ $t('settings.qqProject') }}</span>
              <DqSelect v-model="qqForm.projectId" :placeholder="$t('settings.weixinSelectProject')">
                <DqOption
                  v-for="p in projects.sortedProjects"
                  :key="p.id"
                  :value="p.id"
                  :label="p.name"
                />
              </DqSelect>
            </div>
            <label class="settings-field settings-field--switch">
              <span class="settings-field__label">{{ $t('settings.channelAutoApprove') }}</span>
              <DqSwitch
                :model-value="qqForm.autoApprove"
                size="small"
                @update:model-value="(v: boolean) => qqForm.autoApprove = v"
              />
            </label>
            <p class="settings-form-group__desc">{{ $t('settings.channelAutoApproveHint') }}</p>
            <label class="settings-field settings-field--switch">
              <span class="settings-field__label">{{ $t('settings.qqNativeC2cStream') }}</span>
              <DqSwitch
                :model-value="qqForm.nativeC2cStream"
                size="small"
                @update:model-value="(v: boolean) => qqForm.nativeC2cStream = v"
              />
            </label>
            <p class="settings-form-group__desc">{{ $t('settings.qqNativeC2cStreamHint') }}</p>
            <label class="settings-field settings-field--switch">
              <span class="settings-field__label">{{ $t('settings.qqRequireMention') }}</span>
              <DqSwitch
                :model-value="qqForm.requireMention"
                size="small"
                @update:model-value="(v: boolean) => qqForm.requireMention = v"
              />
            </label>
            <p class="settings-form-group__desc">{{ $t('settings.qqRequireMentionHint') }}</p>
            <label class="settings-field">
              <span class="settings-field__label">{{ $t('settings.qqGroupDenyTools') }}</span>
              <DqInput
                v-model="qqForm.groupDenyTools"
                :placeholder="$t('settings.qqGroupDenyToolsPlaceholder')"
              />
            </label>
            <p class="settings-form-group__desc">{{ $t('settings.qqGroupDenyToolsHint') }}</p>
            <p class="settings-form-group__desc">{{ $t('settings.channelProjectHint') }}</p>
          </div>

          <div class="settings-form-group">
            <h3 class="settings-form-group__title">{{ $t('settings.qqCredentials') }}</h3>
            <p class="settings-form-group__desc">{{ $t('settings.qqCredentialsDesc') }}</p>
            <label class="settings-field">
              <span class="settings-field__label">App ID</span>
              <DqInput v-model="qqForm.appId" placeholder="102xxxxx" />
            </label>
            <label class="settings-field">
              <span class="settings-field__label">Client Secret</span>
              <DqInput
                v-model="qqForm.clientSecret"
                type="password"
                :placeholder="qq.status?.hasClientSecret ? $t('settings.qqSecretKept') : ''"
              />
            </label>
            <p class="settings-form-group__desc">{{ $t('settings.qqWebsocketHint') }}</p>
          </div>
        </div>
      </div>

      <div v-else-if="activeTab === 'wecom'" class="settings-section">
        <header class="settings-section__head">
          <h2>{{ $t('settings.wecom') }}</h2>
          <p>{{ $t('settings.wecomDesc') }}</p>
        </header>

        <div v-if="wecom.loading && !wecom.status" class="settings-empty settings-empty--skeleton">
          <Skeleton variant="title" width="30%" />
          <Skeleton variant="card" width="100%" />
        </div>

        <div v-else class="settings-form">
          <div class="settings-form-group">
            <h3 class="settings-form-group__title">{{ $t('settings.wecomChannel') }}</h3>
            <p class="settings-form-group__desc">{{ $t('settings.wecomChannelDesc') }}</p>
            <label class="settings-field settings-field--switch">
              <span class="settings-field__label">{{ $t('settings.wecomEnable') }}</span>
              <DqSwitch
                :model-value="wecomForm.enabled"
                size="small"
                @update:model-value="(v: boolean) => wecomForm.enabled = v"
              />
            </label>
            <div v-if="wecom.status" class="settings-sandbox-status">
              <span class="settings-field__label">{{ $t('settings.wecomRunning') }}</span>
              <span class="settings-sandbox-status__value">{{ wecom.status.running ? $t('common.yes') : $t('common.no') }}</span>
            </div>
            <div class="settings-field">
              <span class="settings-field__label">{{ $t('settings.weixinDefaultAgent') }}</span>
              <DqSelect v-model="wecomForm.defaultAgentId" :placeholder="$t('settings.weixinSelectAgent')">
                <DqOption
                  v-for="a in sessions.agents"
                  :key="a.id"
                  :value="a.id"
                  :label="a.name || a.id"
                />
              </DqSelect>
            </div>
            <div class="settings-field">
              <span class="settings-field__label">{{ $t('settings.weixinDefaultModel') }}</span>
              <DqSelect v-model="wecomForm.defaultModelId" :placeholder="$t('settings.weixinSelectModel')">
                <DqOption
                  v-for="m in llm.models"
                  :key="m.id"
                  :value="m.id"
                  :label="m.id"
                />
              </DqSelect>
            </div>
            <div class="settings-field">
              <span class="settings-field__label">{{ $t('settings.wecomProject') }}</span>
              <DqSelect v-model="wecomForm.projectId" :placeholder="$t('settings.weixinSelectProject')">
                <DqOption
                  v-for="p in projects.sortedProjects"
                  :key="p.id"
                  :value="p.id"
                  :label="p.name"
                />
              </DqSelect>
            </div>
            <label class="settings-field settings-field--switch">
              <span class="settings-field__label">{{ $t('settings.weixinAutoApprove') }}</span>
              <DqSwitch
                :model-value="wecomForm.autoApprove"
                size="small"
                @update:model-value="(v: boolean) => wecomForm.autoApprove = v"
              />
            </label>
          </div>

          <div class="settings-form-group">
            <h3 class="settings-form-group__title">{{ $t('settings.wecomCredentials') }}</h3>
            <p class="settings-form-group__desc">{{ $t('settings.wecomCredentialsDesc') }}</p>
            <label class="settings-field">
              <span class="settings-field__label">Bot ID</span>
              <DqInput v-model="wecomForm.botId" placeholder="aibot_xxx" />
            </label>
            <label class="settings-field">
              <span class="settings-field__label">Secret</span>
              <DqInput
                v-model="wecomForm.secret"
                type="password"
                :placeholder="wecom.status?.hasSecret ? $t('settings.wecomSecretKept') : ''"
              />
            </label>
            <label class="settings-field">
              <span class="settings-field__label">{{ $t('settings.wecomWsUrl') }}</span>
              <DqInput v-model="wecomForm.wsUrl" :placeholder="$t('settings.wecomWsUrlPlaceholder')" />
            </label>
            <p class="settings-form-group__desc">{{ $t('settings.wecomWebsocketHint') }}</p>
          </div>
        </div>
      </div>

      <div v-else-if="activeTab === 'models'" class="settings-section settings-section--wide">
        <header class="settings-section__head">
          <h2>{{ $t('settings.models') }}</h2>
          <p>{{ $t('settings.modelsDesc') }}</p>
        </header>

        <div v-if="llm.loading" class="settings-empty settings-empty--skeleton">
          <Skeleton variant="title" width="30%" />
          <Skeleton variant="card" width="100%" />
          <Skeleton variant="card" width="100%" />
        </div>

        <div v-else>
          <div v-if="llm.configs.length" class="provider-list">
            <div v-for="cfg in llm.configs" :key="cfg.id" class="provider-card">
              <div class="provider-card__head">
                <div class="provider-card__info">
                  <span class="provider-card__name">{{ cfg.name }}</span>
                  <span class="provider-card__type">{{ providerLabel(cfg.provider) }}</span>
                </div>
                <div class="provider-card__actions">
                  <DqButton size="sm" @click="openEditForm(cfg)">{{ $t('settings.edit') }}</DqButton>
                  <DqButton size="sm" type="danger" @click="handleDelete(cfg.id)">{{ $t('common.delete') }}</DqButton>
                </div>
              </div>
              <div class="provider-card__models">
                <div
                  v-for="model in cfg.models ?? []"
                  :key="model.name"
                  class="provider-card__model"
                  :class="{ 'is-disabled': !model.enabled }"
                >
                  <span class="provider-card__model-name">{{ model.name }}</span>
                  <span v-if="!model.enabled" class="provider-card__model-status">{{ $t('settings.disabled') }}</span>
                </div>
                <div v-if="!(cfg.models && cfg.models.length)" class="provider-card__models-empty">
                  {{ $t('settings.noModels') }}
                </div>
              </div>
            </div>
          </div>

          <div v-if="!llm.configs.length" class="settings-empty">
            {{ $t('settings.noProvider') }}
          </div>
        </div>
      </div>

      <div v-else-if="activeTab === 'search'" class="settings-section">
        <header class="settings-section__head">
          <h2>{{ $t('settings.webSearch') }}</h2>
          <p>{{ $t('settings.searchDesc') }}</p>
        </header>

        <div v-if="searchConfig.loading" class="settings-empty settings-empty--skeleton">
          <Skeleton variant="title" width="30%" />
          <Skeleton variant="card" width="100%" />
        </div>

        <div v-else class="settings-form">
          <label class="settings-field">
            <span class="settings-field__label">{{ $t('settings.searchProvider') }}</span>
            <DqSelect v-model="searchForm.provider">
              <DqOption
                v-for="opt in searchConfig.providerOptions"
                :key="opt.value"
                :value="opt.value"
                :label="opt.label"
              />
            </DqSelect>
          </label>

          <label class="settings-field">
            <span class="settings-field__label">{{ $t('settings.baseUrl') }}</span>
            <DqInput v-model="searchForm.baseUrl" :placeholder="$t('settings.searchPlaceholder')" />
          </label>

          <label class="settings-field">
            <span class="settings-field__label">{{ $t('settings.apiKey') }}</span>
            <DqInput v-model="searchForm.apiKey" type="password" :placeholder="$t('settings.apiKeyOptional')" />
          </label>

          <label class="settings-field">
            <span class="settings-field__label">{{ $t('settings.searchProxy') }}</span>
            <DqInput v-model="searchForm.proxy" :placeholder="$t('settings.searchProxyPlaceholder')" />
          </label>

          <label class="settings-field">
            <span class="settings-field__label">{{ $t('settings.searchUserAgent') }}</span>
            <DqInput v-model="searchForm.userAgent" :placeholder="$t('settings.searchUserAgentPlaceholder')" />
          </label>

          <div class="settings-form-row">
            <div class="settings-field settings-field--half">
              <span class="settings-field__label">{{ $t('settings.timeout') }}</span>
              <div class="slider-row">
                <DqSlider v-model="searchForm.timeoutMs" :min="1000" :max="60000" :step="1000" />
                <span class="slider-row__value">{{ searchForm.timeoutMs }}ms</span>
              </div>
            </div>
            <div class="settings-field settings-field--half">
              <span class="settings-field__label">{{ $t('settings.maxResults') }}</span>
              <div class="slider-row">
                <DqSlider v-model="searchForm.maxResults" :min="1" :max="10" :step="1" />
                <span class="slider-row__value">{{ searchForm.maxResults }}</span>
              </div>
            </div>
          </div>

          <label class="settings-field settings-field--switch">
            <span class="settings-field__label">{{ $t('settings.htmlFallback') }}</span>
            <DqSwitch
              :model-value="searchForm.htmlFallback"
              size="sm"
              @update:model-value="(v: boolean) => searchForm.htmlFallback = v"
            />
          </label>
          <p class="settings-form-group__desc">{{ $t('settings.htmlFallbackDesc') }}</p>

        </div>
      </div>

      <div v-else-if="activeTab === 'market'" class="settings-section settings-section--wide">
        <header class="settings-section__head">
          <h2>{{ $t('settings.market') }}</h2>
          <p>{{ $t('settings.marketDesc') }}</p>
        </header>

        <div v-if="marketConfig.loading" class="settings-empty settings-empty--skeleton">
          <Skeleton variant="title" width="30%" />
          <Skeleton variant="card" width="100%" />
        </div>

        <div v-else class="settings-form">
          <label class="settings-field settings-field--half">
            <span class="settings-field__label">{{ $t('settings.marketCacheTtl') }}</span>
            <DqInput v-model.number="marketForm.cacheTtlHours" type="number" min="1" max="168" />
          </label>

          <div class="settings-form-group">
            <div class="settings-form-group__head">
              <span class="settings-form-group__title">{{ $t('settings.marketSources') }}</span>
              <DqButton size="sm" @click="addMarketSource">{{ $t('settings.marketAddSource') }}</DqButton>
            </div>

            <div v-if="!marketForm.sources.length" class="settings-empty">
              {{ $t('settings.marketAddSource') }}
            </div>

            <article
              v-for="(src, idx) in marketForm.sources"
              :key="idx"
              class="market-source-card"
            >
              <div class="market-source-card__toolbar">
                <label class="settings-field settings-field--switch">
                  <span class="settings-field__label">{{ $t('settings.marketEnabled') }}</span>
                  <DqSwitch
                    :model-value="src.enabled"
                    size="sm"
                    @update:model-value="(v: boolean) => src.enabled = v"
                  />
                </label>
                <DqButton size="sm" type="danger" @click="removeMarketSource(idx)">
                  {{ $t('settings.marketRemoveSource') }}
                </DqButton>
              </div>

              <div class="settings-form-row">
                <label class="settings-field settings-field--half">
                  <span class="settings-field__label">{{ $t('settings.marketSourceId') }}</span>
                  <DqInput v-model="src.id" class="resource-input-mono" placeholder="official-github" />
                </label>
                <label class="settings-field settings-field--half">
                  <span class="settings-field__label">{{ $t('settings.marketSourceName') }}</span>
                  <DqInput v-model="src.name" placeholder="Official (GitHub)" />
                </label>
              </div>

              <div class="settings-form-row">
                <label class="settings-field settings-field--half">
                  <span class="settings-field__label">{{ $t('settings.marketKind') }}</span>
                  <DqSelect v-model="src.kind">
                    <DqOption
                      v-for="opt in marketConfig.kindOptions"
                      :key="opt.value"
                      :value="opt.value"
                      :label="opt.label"
                    />
                  </DqSelect>
                </label>
                <label class="settings-field settings-field--half">
                  <span class="settings-field__label">{{ $t('settings.marketPriority') }}</span>
                  <DqInput v-model.number="src.priority" type="number" />
                </label>
              </div>

              <div v-if="isGitMarketKind(src.kind)" class="settings-form-row">
                <label class="settings-field settings-field--half">
                  <span class="settings-field__label">{{ $t('settings.marketPlatform') }}</span>
                  <DqSelect v-model="src.platform">
                    <DqOption
                      v-for="opt in marketConfig.platformOptions"
                      :key="opt.value"
                      :value="opt.value"
                      :label="opt.label"
                    />
                  </DqSelect>
                </label>
              </div>

              <label class="settings-field">
                <span class="settings-field__label">{{ $t(marketRepoLabel(src.kind)) }}</span>
                <DqInput
                  v-model="src.repo"
                  :placeholder="marketRepoPlaceholder(src.kind) || $t('settings.marketRepoPlaceholder')"
                />
              </label>

              <div v-if="isGitMarketKind(src.kind)" class="settings-form-row">
                <label class="settings-field settings-field--half">
                  <span class="settings-field__label">{{ $t('settings.marketRef') }}</span>
                  <DqInput v-model="src.ref" placeholder="main" />
                </label>
                <label class="settings-field settings-field--half">
                  <span class="settings-field__label">{{ $t('settings.marketCatalogPath') }}</span>
                  <DqInput v-model="src.catalogPath" placeholder="catalog/index.json" />
                </label>
              </div>

              <div
                v-else-if="src.kind === 'techleads' || src.kind === 'tlc' || src.kind === 'tech-leads-club'"
                class="settings-form-row"
              >
                <label class="settings-field settings-field--half">
                  <span class="settings-field__label">{{ $t('settings.marketNpmRef') }}</span>
                  <DqInput v-model="src.ref" placeholder="latest" />
                </label>
              </div>

              <label class="settings-field">
                <span class="settings-field__label">{{ $t('settings.marketToken') }}</span>
                <DqInput
                  v-model="src.token"
                  type="password"
                  :placeholder="src.kind === 'clawhub' ? 'clh_…' : 'ghp_…'"
                />
              </label>
            </article>
          </div>
        </div>
      </div>

      <div v-else-if="activeTab === 'modelConfig'" class="settings-section settings-section--wide">
        <header class="settings-section__head">
          <h2>{{ $t('settings.modelConfig') }}</h2>
          <p>{{ $t('settings.modelConfigDesc') }}</p>
        </header>

        <div v-if="modelConfig.loading" class="settings-empty settings-empty--skeleton">
          <Skeleton variant="title" width="30%" />
          <Skeleton variant="card" width="100%" />
          <Skeleton variant="card" width="100%" />
        </div>

        <div v-else class="settings-form">
          <label class="settings-field">
            <span class="settings-field__label">{{ $t('settings.modelName') }}</span>
            <DqSelect v-model="selectedModel" @update:model-value="onSelectModel">
              <DqOption value="__default__" :label="$t('settings.defaultModelTitle') + ' (128K / 8K / 0.7)'" />
              <DqOption
                v-for="item in modelConfigForm"
                :key="item.model"
                :value="item.model"
                :label="item.model"
              />
            </DqSelect>
          </label>

          <template v-if="selectedModel === '__default__'">
            <label class="settings-field">
              <span class="settings-field__label">Context Window</span>
              <DqInput v-model.number="defaultForm.context_window" type="number" placeholder="0" />
            </label>
            <label class="settings-field">
              <span class="settings-field__label">Max Output</span>
              <DqInput v-model.number="defaultForm.max_output" type="number" placeholder="0" />
            </label>
            <label class="settings-field">
              <span class="settings-field__label">Temperature</span>
              <DqInput v-model.number="defaultForm.temperature" type="number" placeholder="0" step="0.1" />
            </label>
          </template>

          <template v-else-if="editingModelIdx !== null">
            <label class="settings-field">
              <span class="settings-field__label">Context Window</span>
              <DqInput v-model.number="modelConfigForm[editingModelIdx].context_window" type="number" placeholder="0" />
            </label>
            <label class="settings-field">
              <span class="settings-field__label">Max Output</span>
              <DqInput v-model.number="modelConfigForm[editingModelIdx].max_output" type="number" placeholder="0" />
            </label>
            <label class="settings-field">
              <span class="settings-field__label">Temperature</span>
              <DqInput v-model.number="modelConfigForm[editingModelIdx].temperature" type="number" placeholder="0" step="0.1" />
            </label>
            <label class="settings-field">
              <span class="settings-field__label">Available Efforts</span>
              <DqInput :model-value="(modelConfigForm[editingModelIdx].available_efforts ?? []).join(', ')" @update:model-value="modelConfigForm[editingModelIdx].available_efforts = ($event as string).split(',').map(s => s.trim()).filter(Boolean)" placeholder="off, low, medium, high, xhigh" />
            </label>
            <label class="settings-field">
              <span class="settings-field__label">Thinking Mode</span>
              <DqInput v-model="modelConfigForm[editingModelIdx].thinking_mode" placeholder="adaptive / enabled" />
            </label>
            <label class="settings-field settings-field--switch">
              <span class="settings-field__label">{{ $t('settings.modelVision') }}</span>
              <DqSwitch
                :model-value="!!modelConfigForm[editingModelIdx].vision"
                @update:model-value="(v: boolean) => { if (editingModelIdx !== null) modelConfigForm[editingModelIdx].vision = v }"
              />
              <span class="settings-field__hint">{{ $t('settings.modelVisionDesc') }}</span>
            </label>
            <label class="settings-field">
              <span class="settings-field__label">Effort Budget Tokens</span>
              <DqInput :model-value="formatEffortBudgetTokens(modelConfigForm[editingModelIdx].effort_budget_tokens)" type="textarea" :rows="3" @update:model-value="modelConfigForm[editingModelIdx].effort_budget_tokens = parseEffortBudgetTokens($event as string)" placeholder="high:16000&#10;max:32000" />
            </label>
          </template>
        </div>

      </div>

      <div v-else-if="activeTab === 'about'" class="settings-section">
        <header class="settings-section__head">
          <h2>{{ $t('settings.about') }}</h2>
          <p>{{ $t('settings.aboutDesc') }}</p>
        </header>

        <div class="settings-form">
          <div class="settings-form-group">
            <h3 class="settings-form-group__title">{{ $t('settings.version') }}</h3>
            <p class="settings-form-group__desc">{{ $t('settings.versionDesc') }}</p>
            <div class="about-version-row">
              <span class="about-version">v{{ appVersion || '…' }}</span>
              <span
                v-if="hasUpdate"
                class="about-update-badge"
              >{{ $t('updater.updateAvailableBadge') }}</span>
            </div>
            <p class="settings-form-group__desc">{{ updaterStatusText }}</p>
            <p v-if="updateNotes && hasUpdate" class="about-notes">
              {{ updateNotes }}
            </p>
            <div class="settings-actions about-actions">
              <DqButton
                :disabled="!isDesktop || updaterBusy"
                @click="handleCheckUpdate"
              >
                {{ updaterStatus === 'checking' ? $t('updater.checking') : $t('updater.check') }}
              </DqButton>
              <DqButton
                v-if="updaterStatus === 'available'"
                type="primary"
                :disabled="updaterBusy"
                @click="handleInstallUpdate"
              >
                {{ $t('updater.install') }}
              </DqButton>
            </div>
          </div>
        </div>
      </div>

      </div>

      <footer v-if="hasFooterActions" class="settings-panel__footer">
        <span class="settings-panel__footer-hint">{{ footerHint }}</span>
        <div class="settings-panel__footer-actions">
          <DqButton v-if="activeTab === 'modelConfig' && editingModelIdx !== null" type="danger" size="sm" @click="removeSelectedModel">{{ $t('common.delete') }}</DqButton>
          <DqButton v-if="activeTab === 'runtime'" type="primary" :disabled="runtimeConfig.saving" @click="handleSaveRuntime">
            {{ runtimeConfig.saving ? $t('common.saving') : $t('common.save_') }}
          </DqButton>
          <DqButton v-else-if="activeTab === 'weixin'" type="primary" :disabled="weixin.saving" @click="handleSaveWeixin">
            {{ weixin.saving ? $t('common.saving') : $t('common.save_') }}
          </DqButton>
          <DqButton v-else-if="activeTab === 'feishu'" type="primary" :disabled="feishu.saving" @click="handleSaveFeishu">
            {{ feishu.saving ? $t('common.saving') : $t('common.save_') }}
          </DqButton>
          <DqButton v-else-if="activeTab === 'wecom'" type="primary" :disabled="wecom.saving" @click="handleSaveWecom">
            {{ wecom.saving ? $t('common.saving') : $t('common.save_') }}
          </DqButton>
          <DqButton v-else-if="activeTab === 'qq'" type="primary" :disabled="qq.saving" @click="handleSaveQQ">
            {{ qq.saving ? $t('common.saving') : $t('common.save_') }}
          </DqButton>
          <DqButton v-else-if="activeTab === 'search'" type="primary" :disabled="searchConfig.saving" @click="handleSaveSearch">
            {{ searchConfig.saving ? $t('common.saving') : $t('common.save_') }}
          </DqButton>
          <DqButton v-else-if="activeTab === 'market'" type="primary" :disabled="marketConfig.saving" @click="handleSaveMarket">
            {{ marketConfig.saving ? $t('common.saving') : $t('common.save_') }}
          </DqButton>
          <DqButton v-else-if="activeTab === 'models'" type="primary" @click="openNewForm">{{ $t('settings.addProvider') }}</DqButton>
          <DqButton v-else-if="activeTab === 'modelConfig'" @click="addModelEntry">{{ $t('settings.addModelConfig') }}</DqButton>
          <DqButton v-if="activeTab === 'modelConfig'" type="primary" :disabled="modelConfig.saving" @click="handleSaveModelConfig">
            {{ modelConfig.saving ? $t('common.saving') : $t('common.save_') }}
          </DqButton>
        </div>
      </footer>

      <DqDialog
        v-model:open="showWeixinAddDialog"
        :title="$t('settings.weixinLogin')"
        variant="glass"
        width="420px"
        :closable="true"
        @update:open="(v: boolean) => { if (!v) closeWeixinAddDialog() }"
      >
        <div class="weixin-add-dialog">
          <p class="settings-form-group__desc">{{ $t('settings.weixinLoginDesc') }}</p>
          <label class="settings-field">
            <span class="settings-field__label">{{ $t('settings.weixinBindProject') }}</span>
            <DqSelect
              v-model="weixinLoginProjectId"
              :placeholder="$t('settings.weixinSelectProject')"
              :disabled="weixinPolling"
            >
              <DqOption
                v-for="p in projects.sortedProjects"
                :key="p.id"
                :value="p.id"
                :label="p.name"
              />
            </DqSelect>
          </label>

          <div v-if="weixin.qr?.url" class="weixin-add-dialog__qr">
            <div class="weixin-qr">
              <img :src="weixin.qr.url" alt="WeChat QR" width="200" height="200" />
            </div>
            <p class="weixin-add-dialog__hint">
              {{ weixinPolling ? $t('settings.weixinWaiting') : $t('settings.weixinScanHint') }}
            </p>
            <p v-if="weixin.loginMessage" class="weixin-add-dialog__msg">{{ weixin.loginMessage }}</p>
          </div>

          <div class="settings-actions weixin-add-dialog__actions">
            <DqButton size="sm" @click="closeWeixinAddDialog">
              {{ $t('common.cancel') }}
            </DqButton>
            <DqButton
              type="primary"
              size="sm"
              :disabled="weixinPolling || !weixinLoginProjectId"
              @click="handleWeixinQr"
            >
              {{ weixin.qr?.url ? $t('settings.weixinRescan') : $t('settings.weixinScan') }}
            </DqButton>
          </div>
        </div>
      </DqDialog>

      <DqDialog
        v-model:open="showAddModelDialog"
        :title="$t('settings.addModelConfig')"
        variant="glass"
        width="400px"
        :closable="true"
      >
        <div class="settings-form">
          <label class="settings-field">
            <span class="settings-field__label">{{ $t('settings.modelName') }}</span>
            <DqInput v-model="newModelName" :placeholder="$t('settings.modelNamePlaceholder')" />
          </label>
          <label class="settings-field">
            <span class="settings-field__label">Context Window</span>
            <DqInput v-model.number="newContextWindow" type="number" placeholder="0" />
          </label>
          <label class="settings-field">
            <span class="settings-field__label">Max Output</span>
            <DqInput v-model.number="newMaxOutput" type="number" placeholder="0" />
          </label>
          <label class="settings-field">
            <span class="settings-field__label">Temperature</span>
            <DqInput v-model.number="newTemperature" type="number" placeholder="0" step="0.1" />
          </label>
          <label class="settings-field">
            <span class="settings-field__label">Available Efforts</span>
            <DqInput v-model="newAvailableEfforts" placeholder="off, low, medium, high, xhigh" />
          </label>
          <label class="settings-field">
            <span class="settings-field__label">Thinking Mode</span>
            <DqInput v-model="newThinkingMode" placeholder="adaptive / enabled" />
          </label>
          <label class="settings-field">
            <span class="settings-field__label">Effort Budget Tokens</span>
            <DqInput v-model="newEffortBudgetTokens" type="textarea" :rows="3" placeholder="high:16000&#10;max:32000" />
          </label>
          <div class="settings-actions">
            <DqButton type="primary" @click="confirmAddModel">{{ $t('common.save_') }}</DqButton>
          </div>
        </div>
      </DqDialog>

      <DqDialog
        v-model:open="showForm"
        :title="editingId ? $t('settings.editProvider') : $t('settings.addProviderTitle')"
        variant="glass"
        :width="dialogStep === 'choose' && !editingId ? '640px' : '600px'"
        :closable="true"
      >
        <!-- Step 1: Choose preset -->
        <div v-if="dialogStep === 'choose' && !editingId" class="preset-grid">
          <p class="preset-grid__hint">{{ $t('settings.chooseProviderHint') }}</p>
          <div class="preset-cards">
            <button
              v-for="preset in llm.presets"
              :key="preset.id"
              type="button"
              class="preset-card"
              @click="selectPreset(preset)"
            >
              <span class="preset-card__badge" :style="{ background: presetColor(preset.id) }">{{ presetAbbr(preset.id) }}</span>
              <span class="preset-card__name">{{ preset.name }}</span>
              <span class="preset-card__desc">{{ preset.description }}</span>
            </button>
            <button type="button" class="preset-card preset-card--custom" @click="selectCustom">
              <span class="preset-card__badge preset-card__badge--custom">{{ $t('settings.customProvider')[0] }}</span>
              <span class="preset-card__name">{{ $t('settings.customProvider') }}</span>
              <span class="preset-card__desc">{{ $t('settings.customProviderDesc') }}</span>
            </button>
          </div>
        </div>

        <!-- Step 2: Configure -->
        <div v-else class="settings-form">
          <div v-if="!editingId" class="settings-form__back">
            <button type="button" class="settings-form__back-btn" @click="backToChoose">
              ← {{ $t('settings.chooseProvider') }}
            </button>
          </div>

          <label v-if="!editingId" class="settings-field">
            <span class="settings-field__label">{{ $t('settings.protocolType') }}</span>
            <DqSelect v-model="form.provider">
              <DqOption
                v-for="opt in providerOptions"
                :key="opt.value"
                :value="opt.value"
                :label="opt.label"
              />
            </DqSelect>
          </label>

          <label class="settings-field">
            <span class="settings-field__label">{{ $t('common.name') }}</span>
            <DqInput v-model="form.name" :placeholder="$t('settings.nameExample')" :disabled="!!editingId" />
          </label>

          <label class="settings-field">
            <span class="settings-field__label">{{ $t('settings.apiKey') }}</span>
            <DqInput v-model="form.apiKey" type="password" :placeholder="$t('settings.apiKeyPlaceholder')" />
          </label>

          <label class="settings-field">
            <span class="settings-field__label">{{ $t('settings.baseUrl') }}</span>
            <DqInput v-model="form.baseUrl" :placeholder="$t('settings.baseUrlPlaceholder')" />
          </label>

          <div class="settings-field settings-field--refresh">
            <div class="settings-field__refresh">
              <div class="settings-field__refresh-copy">
                <span class="settings-field__refresh-title">{{ $t('settings.refreshModels') }}</span>
                <span class="settings-field__hint">{{ $t('settings.refreshHint') }}</span>
              </div>
              <DqButton type="primary" :disabled="refreshingModels" @click="handleRefreshModels">
                {{ refreshingModels ? $t('common.refreshing') : $t('settings.refreshModels') }}
              </DqButton>
            </div>
          </div>

          <div class="settings-field settings-field--manual-model">
            <span class="settings-field__label">{{ $t('settings.manualModel') }}</span>
            <div class="settings-field__manual-row">
              <DqInput
                v-model="manualModelName"
                :placeholder="$t('settings.manualModelPlaceholder')"
                @keydown="onManualModelKeydown"
              />
              <DqButton size="sm" @click="addManualModel">{{ $t('settings.addManualModel') }}</DqButton>
            </div>
            <span class="settings-field__hint">{{ $t('settings.manualModelHint') }}</span>
          </div>

          <div class="model-list">
            <div class="model-list__head">
              <span class="model-list__title">{{ $t('settings.modelList') }}</span>
              <span class="model-list__hint">{{
                displayedModels.length ? $t('settings.modelToggleHint') : $t('settings.modelListEmpty')
              }}</span>
            </div>
            <div v-if="displayedModels.length" class="model-list__items">
              <div
                v-for="m in displayedModels"
                :key="m.name"
                class="model-list__item"
                :class="{ 'is-disabled': !m.enabled }"
              >
                <span class="model-list__name">{{ m.name }}</span>
                <div class="model-list__actions">
                  <DqSwitch
                    :model-value="m.enabled"
                    size="sm"
                    @update:model-value="(v: boolean) => handleToggleModel(m.name, v)"
                  />
                  <button
                    type="button"
                    class="model-list__remove"
                    :title="$t('common.delete')"
                    @click="removeManualModel(m.name)"
                  >
                    ×
                  </button>
                </div>
              </div>
            </div>
          </div>
        </div>

        <template #footer>
          <div v-if="dialogStep === 'choose' && !editingId" class="settings-actions">
            <DqButton @click="cancelForm">{{ $t('common.cancel') }}</DqButton>
          </div>
          <div v-else class="settings-actions">
            <DqButton @click="cancelForm">{{ $t('common.cancel') }}</DqButton>
            <DqButton type="primary" :disabled="llm.saving" @click="handleSave">
              {{ llm.saving ? $t('common.saving') : (editingId ? $t('common.update') : $t('common.save_')) }}
            </DqButton>
          </div>
        </template>
      </DqDialog>
    </main>
  </div>
</template>

<style scoped>
.settings-view {
  display: flex;
  height: 100%;
  min-width: 0;
  min-height: 0;
  overflow: hidden;
  background: var(--work-glass-bg);
}

/* ── Form control consistency ── */

/* Make DqInput match shared control density */
.settings-view :deep(.dq-input) {
  background: var(--dq-glass-control-bg-solid);
  border-color: var(--work-glass-border);
  min-height: 28px;
  font-size: var(--dq-font-size-body);
}

.settings-view :deep(.dq-input:hover:not(:disabled):not(:focus):not(:focus-visible)) {
  border-color: color-mix(in srgb, var(--dq-label-primary) 22%, transparent);
}

.settings-view :deep(.dq-input:focus),
.settings-view :deep(.dq-input:focus-visible) {
  background: var(--dq-bg-elevated);
  border-color: var(--dq-accent);
  box-shadow: var(--dq-focus-ring);
}

.settings-view :deep(.dq-input:disabled) {
  opacity: 0.5;
}

.settings-sidebar {
  flex-shrink: 0;
  width: 200px;
  border-right: 1px solid var(--work-glass-border);
  background: var(--work-glass-bg);
  display: flex;
  flex-direction: column;
  padding: 16px 12px;
}

.settings-sidebar__head {
  padding: 0 8px 16px;
}

.settings-sidebar__title {
  font-size: var(--dq-font-size-body);
  font-weight: 600;
  color: var(--dq-label-tertiary);
}

.settings-sidebar__menu {
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.settings-sidebar__item {
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 8px 10px;
  border: none;
  border-radius: var(--dq-radius-button);
  background: transparent;
  color: var(--dq-label-primary);
  font-size: var(--dq-font-size-body);
  font-weight: 500;
  cursor: pointer;
  text-align: left;
  transition: background var(--dq-transition-hover), color var(--dq-transition-hover);
  box-shadow: inset 0 0 0 0 transparent;
}

.settings-sidebar__item:hover {
  background: color-mix(in srgb, var(--dq-label-primary) 5%, transparent);
}

.settings-sidebar__item.is-active {
  background: color-mix(in srgb, var(--dq-accent) 12%, var(--dq-fill-tertiary));
  color: var(--dq-accent);
  box-shadow: inset 2px 0 0 var(--dq-accent);
}

.settings-panel {
  flex: 1;
  min-width: 0;
  overflow: hidden;
  display: flex;
  flex-direction: column;
}

.settings-panel__content {
  flex: 1;
  min-height: 0;
  overflow: auto;
  padding: 28px 36px 20px;
  background: color-mix(in srgb, var(--dq-bg-elevated) 30%, transparent);
}

.settings-panel__footer {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  padding: 12px 24px;
  border-top: 1px solid var(--dq-separator-light);
  background: color-mix(in srgb, var(--dq-bg-elevated) 50%, transparent);
  flex-shrink: 0;
}

.settings-panel__footer-hint {
  font-size: var(--dq-font-size-caption);
  color: var(--dq-label-quaternary);
}

.settings-panel__footer-actions {
  display: flex;
  align-items: center;
  gap: 8px;
}

.settings-section {
  max-width: 640px;
}

.settings-section--wide {
  max-width: 720px;
}

.about-version-row {
  display: flex;
  align-items: center;
  gap: 10px;
  margin: 8px 0 4px;
}

.about-version {
  font-size: var(--dq-font-size-title);
  font-weight: 600;
  color: var(--dq-label-primary);
  font-variant-numeric: tabular-nums;
}

.about-update-badge {
  display: inline-flex;
  align-items: center;
  padding: 2px 8px;
  border-radius: 999px;
  font-size: var(--dq-font-size-caption);
  font-weight: 600;
  color: var(--dq-accent);
  background: color-mix(in srgb, var(--dq-accent) 14%, transparent);
}

.about-notes {
  margin: 8px 0 0;
  white-space: pre-wrap;
  color: var(--dq-label-secondary);
  font-size: var(--dq-font-size-body);
}

.about-actions {
  margin-top: 16px;
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
}

.settings-section__head {
  margin-bottom: 24px;
}

.settings-section__head h2 {
  margin: 0 0 6px;
  font-size: var(--dq-font-size-heading);
  font-weight: 600;
  color: var(--dq-label-primary);
}

.settings-section__head p {
  margin: 0;
  font-size: var(--dq-font-size-body);
  color: var(--dq-label-tertiary);
}

.settings-form {
  display: flex;
  flex-direction: column;
  gap: 16px;
}

.settings-form-group {
  padding: 0;
  border-radius: 0;
  border: none;
  background: transparent;
}

.settings-form-group + .settings-form-group {
  margin-top: 24px;
}

.settings-form-group__head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  margin-bottom: 10px;
}

.settings-form-group__title {
  margin: 0 0 4px;
  font-size: var(--dq-font-size-secondary);
  font-weight: 600;
  color: var(--dq-label-primary);
}

.settings-form-group__desc {
  margin: 0 0 16px;
  font-size: var(--dq-font-size-footnote);
  color: var(--dq-label-tertiary);
  line-height: 1.5;
}

.market-source-card {
  border: 1px solid var(--work-glass-border);
  border-radius: 10px;
  padding: 14px;
  margin-bottom: 12px;
  display: flex;
  flex-direction: column;
  gap: 10px;
}

.market-source-card__toolbar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
}

.settings-sandbox-status {
  margin: 12px 0 16px;
  padding: 10px 12px;
  border-radius: 8px;
  background: var(--dq-fill-quaternary, rgba(127, 127, 127, 0.08));
}

.settings-sandbox-status__value {
  display: inline-block;
  margin-top: 4px;
  font-size: var(--dq-font-size-footnote);
  color: var(--dq-label-secondary);
}

.settings-sandbox-status__degraded {
  margin: 8px 0 0;
  font-size: var(--dq-font-size-footnote);
  color: var(--dq-label-tertiary);
  line-height: 1.4;
}

.settings-form-row {
  display: flex;
  gap: 16px;
}

.settings-form-row + .settings-form-row {
  margin-top: 12px;
}

.settings-field--switch + .settings-form-row {
  margin-top: 12px;
}

.settings-field--switch + .settings-field--switch {
  margin-top: 12px;
}

.settings-form-row + .settings-field--switch {
  margin-top: 12px;
}

.settings-form-group > .settings-field + .settings-field {
  margin-top: 12px;
}

.settings-form-group > .settings-field + .settings-sandbox-status {
  margin-top: 12px;
}

.settings-form-group > .settings-field--switch + .weixin-meta {
  margin-top: 12px;
}

.settings-form-group > .settings-form-row + .provider-list,
.settings-form-group > .provider-list + .settings-form-group__desc {
  margin-top: 12px;
}

.settings-field--half {
  flex: 1;
  min-width: 0;
}

.settings-field {
  display: flex;
  flex-direction: column;
  gap: 5px;
}

.settings-field--switch {
  flex-direction: row;
  align-items: center;
  justify-content: space-between;
}

.settings-field--block {
  width: 100%;
}

.settings-field--inline {
  flex-direction: row;
  align-items: center;
  gap: 12px;
}

.settings-field--refresh {
  padding: 14px 16px;
  border-radius: 12px;
  background: color-mix(in srgb, var(--dq-accent) 10%, var(--dq-bg-elevated));
  border: 1px solid color-mix(in srgb, var(--dq-accent) 35%, transparent);
  box-shadow: 0 0 0 1px color-mix(in srgb, var(--dq-accent) 8%, transparent);
}

.settings-field__refresh {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 16px;
  flex-wrap: wrap;
}

.settings-field__refresh-copy {
  display: flex;
  flex-direction: column;
  gap: 4px;
  min-width: 0;
  flex: 1;
}

.settings-field__refresh-title {
  font-size: var(--dq-font-size-body);
  font-weight: 600;
  color: var(--dq-accent);
  line-height: 1.3;
}

.settings-field--refresh .settings-field__hint {
  color: color-mix(in srgb, var(--dq-accent) 55%, var(--dq-label-secondary));
}

.settings-field__hint {
  font-size: var(--dq-font-size-footnote);
  color: var(--dq-label-tertiary);
  line-height: 1.4;
}

.settings-field--manual-model {
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.settings-field__manual-row {
  display: flex;
  align-items: center;
  gap: 8px;
}

.settings-field__manual-row :deep(.dq-input) {
  flex: 1;
  min-width: 0;
}

.settings-field__label {
  font-size: var(--dq-font-size-body);
  font-weight: 500;
  color: var(--dq-label-primary);
}

.model-list {
  display: flex;
  flex-direction: column;
  gap: 10px;
  padding: 14px;
  border-radius: 12px;
  background: var(--dq-bg-elevated);
  border: 1px solid var(--work-glass-border);
}

.model-list__head {
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.model-list__title {
  font-size: var(--dq-font-size-secondary);
  font-weight: 600;
  color: var(--dq-label-primary);
}

.model-list__hint {
  font-size: var(--dq-font-size-footnote);
  color: var(--dq-label-tertiary);
  line-height: 1.4;
}

.model-list__items {
  display: flex;
  flex-direction: column;
  gap: 6px;
}

.model-list__item {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  padding: 8px 10px;
  border-radius: 8px;
  border: 1px solid transparent;
  background: color-mix(in srgb, var(--dq-label-primary) 3%, transparent);
  transition: background 0.12s ease, border-color 0.12s ease, opacity 0.12s ease;
}

.model-list__item:hover {
  background: color-mix(in srgb, var(--dq-label-primary) 6%, transparent);
  border-color: var(--work-glass-border);
}

.model-list__item.is-disabled {
  opacity: 0.55;
}

.model-list__name {
  flex: 1;
  min-width: 0;
  color: var(--dq-label-primary);
  font-size: var(--dq-font-size-body);
  font-weight: 500;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.model-list__actions {
  display: flex;
  align-items: center;
  gap: 8px;
  flex-shrink: 0;
}

.model-list__remove {
  width: 22px;
  height: 22px;
  border: none;
  border-radius: 6px;
  background: transparent;
  color: var(--dq-label-tertiary);
  font-size: 16px;
  line-height: 1;
  cursor: pointer;
}

.model-list__remove:hover {
  color: var(--dq-danger);
  background: color-mix(in srgb, var(--dq-danger) 12%, transparent);
}

.settings-actions {
  display: flex;
  justify-content: flex-end;
  gap: 10px;
  margin-top: 8px;
}

.settings-empty {
  padding: 20px;
  font-size: var(--dq-font-size-body);
  color: var(--dq-label-tertiary);
  text-align: center;
  background: color-mix(in srgb, var(--dq-label-primary) 4%, transparent);
  border-radius: var(--dq-radius-control);
}

.settings-empty--skeleton {
  display: flex;
  flex-direction: column;
  gap: var(--dq-space-sm);
  text-align: left;
}

.provider-list-actions {
  margin-bottom: 16px;
}

.provider-list {
  display: flex;
  flex-direction: column;
  gap: 12px;
}

.provider-card {
  border-radius: 0;
  border: none;
  background: transparent;
  overflow: visible;
  padding: 16px 0;
  border-bottom: 1px solid var(--work-glass-border);
}

.provider-card:last-child {
  border-bottom: none;
}

.provider-card__head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 0;
  border-bottom: none;
}

.provider-card__info {
  display: flex;
  align-items: center;
  gap: 10px;
}

.provider-card__name {
  font-size: var(--dq-font-size-secondary);
  font-weight: 600;
  color: var(--dq-label-primary);
}

.provider-card__type {
  font-size: var(--dq-font-size-footnote);
  color: var(--dq-label-tertiary);
}

.provider-card__actions {
  display: flex;
  gap: 6px;
}

.provider-card__models {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
  padding: 12px 0 0;
}

.provider-card__model {
  display: flex;
  align-items: center;
  gap: 6px;
  padding: 4px 10px;
  border-radius: 6px;
  background: color-mix(in srgb, var(--dq-label-primary) 6%, transparent);
  border: 1px solid var(--work-glass-border);
  font-size: var(--dq-font-size-footnote);
}

.provider-card__model.is-disabled {
  opacity: 0.5;
}

.provider-card__model-name {
  color: var(--dq-label-primary);
  font-weight: 500;
}

.provider-card__model-status {
  font-size: var(--dq-font-size-caption);
  color: var(--dq-label-quaternary);
}

.provider-card__models-empty {
  padding: 8px 0;
  font-size: var(--dq-font-size-footnote);
  color: var(--dq-label-tertiary);
  width: 100%;
}

.preset-grid {
  display: flex;
  flex-direction: column;
  gap: 14px;
}

.preset-grid__hint {
  margin: 0;
  font-size: var(--dq-font-size-body);
  color: var(--dq-label-tertiary);
}

.preset-cards {
  display: grid;
  grid-template-columns: repeat(3, 1fr);
  gap: 10px;
}

.preset-card {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 6px;
  padding: 16px 10px;
  border-radius: 12px;
  border: 1px solid var(--work-glass-border);
  background: var(--dq-bg-elevated);
  cursor: pointer;
  transition: border-color 0.12s ease, background 0.12s ease, box-shadow 0.12s ease;
  text-align: center;
}

.preset-card:hover {
  border-color: var(--dq-accent);
  background: color-mix(in srgb, var(--dq-accent) 4%, var(--dq-bg-elevated));
  box-shadow: 0 0 0 3px color-mix(in srgb, var(--dq-accent) 10%, transparent);
}

.preset-card__badge {
  display: flex;
  align-items: center;
  justify-content: center;
  width: 32px;
  height: 32px;
  border-radius: 8px;
  font-size: var(--dq-font-size-footnote);
  font-weight: 700;
  color: var(--dq-color-white);
  letter-spacing: -0.5px;
}

.preset-card__badge--custom {
  background: var(--dq-label-tertiary);
}

.preset-card__name {
  font-size: var(--dq-font-size-body);
  font-weight: 600;
  color: var(--dq-label-primary);
}

.preset-card__desc {
  font-size: var(--dq-font-size-caption);
  color: var(--dq-label-tertiary);
  line-height: 1.3;
}

.preset-card--custom {
  border-style: dashed;
}

.preset-card--custom:hover {
  border-style: solid;
}

.settings-form__back {
  margin-bottom: 4px;
}

.settings-form__back-btn {
  border: none;
  background: none;
  padding: 4px 0;
  font-size: var(--dq-font-size-body);
  color: var(--dq-accent);
  cursor: pointer;
}

.settings-form__back-btn:hover {
  opacity: 0.8;
}

.slider-row {
  display: flex;
  align-items: center;
  gap: 10px;
  height: 36px;
}

.slider-row :deep(.dq-slider) {
  flex: 1;
  min-width: 0;
}

.slider-row :deep(.dq-slider__track) {
  display: flex;
  align-items: center;
}

.slider-row__value {
  flex-shrink: 0;
  min-width: 36px;
  text-align: right;
  font-size: var(--dq-font-size-footnote);
  font-weight: 500;
  font-family: ui-monospace, SFMono-Regular, 'SF Mono', Menlo, monospace;
  color: var(--dq-label-secondary);
}

/* Subsection divider within settings panel */
.settings-subsection {
  border-top: 1px solid var(--work-glass-border);
  padding-top: 16px;
}

.settings-subsection__title {
  margin: 0 0 4px;
  font-size: var(--dq-font-size-title);
  font-weight: 600;
  color: var(--dq-label-primary);
}

.settings-subsection__desc {
  margin: 0;
  font-size: var(--dq-font-size-body);
  color: var(--dq-label-secondary);
}

/* Model limits list */
.model-limit-list {
  margin-top: 12px;
  border: 1px solid var(--work-glass-border);
  border-radius: 10px;
  overflow: hidden;
}

.model-limit-list__head {
  display: flex;
  align-items: center;
  padding: 8px 12px;
  background: color-mix(in srgb, var(--dq-label-primary) 4%, transparent);
  border-bottom: 1px solid var(--work-glass-border);
  font-size: var(--dq-font-size-footnote);
  font-weight: 600;
  color: var(--dq-label-secondary);
}

.model-limit-list__row {
  display: flex;
  align-items: center;
  padding: 10px 12px;
  border-bottom: 1px solid color-mix(in srgb, var(--dq-label-primary) 5%, transparent);
  font-size: var(--dq-font-size-body);
}

.model-limit-list__row:last-child {
  border-bottom: none;
}

.model-limit-list__col {
  flex: 1;
  min-width: 0;
}

.model-limit-list__col--name {
  flex: 2;
  font-weight: 500;
  font-family: ui-monospace, SFMono-Regular, 'SF Mono', Menlo, monospace;
  font-size: var(--dq-font-size-footnote);
}

.model-limit-list__col--actions {
  flex: 0 0 auto;
  display: flex;
  gap: 6px;
  justify-content: flex-end;
}

/* Provider card border visibility */
.provider-card {
  border-color: color-mix(in srgb, var(--dq-label-primary) 10%, transparent);
}

/* Theme grid */
.theme-grid {
  display: grid;
  grid-template-columns: repeat(3, 1fr);
  gap: 12px;
}

@media (max-width: 1080px) {
  .theme-grid {
    grid-template-columns: repeat(2, 1fr);
  }
}

.theme-card {
  position: relative;
  display: flex;
  flex-direction: column;
  padding: 14px;
  border-radius: 14px;
  border: 1.5px solid var(--work-glass-border);
  background: var(--dq-bg-elevated);
  cursor: pointer;
  text-align: left;
  transition: border-color 0.15s ease, background 0.15s ease, box-shadow 0.15s ease, transform 0.12s ease;
}

.theme-card:hover {
  border-color: color-mix(in srgb, var(--dq-label-primary) 18%, transparent);
  background: color-mix(in srgb, var(--dq-label-primary) 4%, var(--dq-bg-elevated));
  transform: translateY(-1px);
}

.theme-card.is-active {
  border-color: var(--dq-accent);
  box-shadow: 0 0 0 3px color-mix(in srgb, var(--dq-accent) 18%, transparent);
}

.theme-card__preview {
  position: relative;
  display: flex;
  align-items: flex-end;
  gap: 6px;
  margin-bottom: 12px;
  height: 48px;
  padding: 0;
  border-radius: 10px;
  overflow: hidden;
  background: var(--dq-bg-page);
  border: 1px solid var(--dq-border-subtle);
}

.theme-card__preview-accent {
  flex: 1;
  height: 10px;
  border-radius: 3px 3px 0 0;
  background: linear-gradient(135deg, var(--preview-accent, var(--dq-accent)), color-mix(in srgb, var(--preview-accent, var(--dq-accent)) 70%, var(--dq-bg-page)));
}

.theme-card__preview-surface {
  flex: 2;
  height: 10px;
  border-radius: 3px 3px 0 0;
  background: var(--dq-bg-elevated);
  opacity: 0.6;
}

.theme-card__preview-text {
  flex: 3;
  height: 4px;
  border-radius: 2px;
  background: var(--dq-label-tertiary);
  opacity: 0.5;
}

.theme-card__info {
  display: flex;
  flex-direction: column;
  gap: 2px;
}

.theme-card__name {
  font-size: var(--dq-font-size-body);
  font-weight: 600;
  color: var(--dq-label-primary);
}

.theme-card__desc {
  font-size: var(--dq-font-size-caption);
  color: var(--dq-label-tertiary);
  line-height: 1.35;
}

.theme-card__check {
  position: absolute;
  top: 10px;
  right: 10px;
  width: 24px;
  height: 24px;
  border-radius: 50%;
  background: var(--dq-accent);
  color: var(--dq-color-white);
  display: flex;
  align-items: center;
  justify-content: center;
  box-shadow: 0 2px 8px color-mix(in srgb, var(--dq-accent) 40%, transparent);
}

.weixin-meta {
  display: flex;
  align-items: center;
  gap: 8px;
  margin: 4px 0 0;
  font-size: var(--dq-font-size-footnote);
  color: var(--dq-label-tertiary);
}

.weixin-meta__dot {
  width: 7px;
  height: 7px;
  border-radius: 50%;
  flex-shrink: 0;
}

.weixin-meta__dot.is-on {
  background: var(--dq-color-success, #34c759);
  box-shadow: 0 0 0 3px color-mix(in srgb, var(--dq-color-success, #34c759) 22%, transparent);
}

.weixin-meta__dot.is-off {
  background: var(--dq-label-tertiary);
}

.weixin-account-list {
  margin-top: 4px;
}

.weixin-account-card__info {
  flex-wrap: wrap;
  min-width: 0;
}

.weixin-account-badge {
  display: inline-flex;
  align-items: center;
  max-width: 160px;
  padding: 2px 8px;
  border-radius: 6px;
  font-size: var(--dq-font-size-footnote);
  font-weight: 500;
  line-height: 1.4;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.weixin-account-badge.is-bound {
  color: var(--dq-label-secondary);
  background: color-mix(in srgb, var(--dq-label-primary) 6%, transparent);
}

.weixin-account-badge.is-unbound {
  color: var(--dq-label-tertiary);
  background: color-mix(in srgb, var(--dq-label-primary) 4%, transparent);
}

.weixin-account-card__body {
  margin-top: 12px;
  display: flex;
  flex-direction: column;
  gap: 6px;
}

.weixin-account-card__project {
  margin: 0;
}

.weixin-account-card__hint {
  margin: 0;
  font-size: var(--dq-font-size-footnote);
  color: var(--dq-label-tertiary);
  word-break: break-all;
}

.weixin-add-dialog {
  display: flex;
  flex-direction: column;
  gap: 14px;
}

.weixin-add-dialog .settings-form-group__desc {
  margin: 0;
}

.weixin-add-dialog__qr {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 10px;
  padding: 8px 0 4px;
}

.weixin-add-dialog__hint {
  margin: 0;
  font-size: var(--dq-font-size-caption);
  color: var(--dq-label-secondary);
  text-align: center;
}

.weixin-add-dialog__msg {
  margin: 0;
  font-size: var(--dq-font-size-footnote);
  color: var(--dq-label-tertiary);
  text-align: center;
}

.weixin-add-dialog__actions {
  margin-top: 4px;
  justify-content: flex-end;
}

.weixin-qr {
  display: inline-flex;
  padding: 12px;
  border-radius: 12px;
  background: #fff;
  border: 1px solid var(--dq-separator);
}

.weixin-qr img {
  display: block;
  width: 200px;
  height: 200px;
  object-fit: contain;
}
</style>
