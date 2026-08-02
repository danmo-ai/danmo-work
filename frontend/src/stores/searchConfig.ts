import { defineStore } from 'pinia'
import { computed, ref } from 'vue'
import { fetchJSON } from '@/api/client'
import { toast } from '@/utils/feedback'
import { i18n } from '@/i18n'
import type { SearchConfig, UpsertSearchConfigRequest, SearchProvider, ConfigFile, UpdateConfigFileRequest } from '@/types/mission'

export type SearchProviderMeta = {
  value: SearchProvider
  label: string
  /** API providers that require an API key */
  needsApiKey: boolean
  /** Providers that require a base URL (SearXNG) */
  needsBaseUrl: boolean
  /** Free HTML providers that optionally accept a custom endpoint */
  optionalBaseUrl: boolean
  signupUrl?: string
  /** i18n key under settings.* for short provider hint */
  hintKey: 'searchHintFree' | 'searchHintApiKey' | 'searchHintSearxng'
}

export const SEARCH_PROVIDER_META: SearchProviderMeta[] = [
  {
    value: 'duckduckgo',
    label: 'DuckDuckGo (HTML)',
    needsApiKey: false,
    needsBaseUrl: false,
    optionalBaseUrl: true,
    hintKey: 'searchHintFree',
  },
  {
    value: 'bing',
    label: 'Bing (HTML)',
    needsApiKey: false,
    needsBaseUrl: false,
    optionalBaseUrl: false,
    hintKey: 'searchHintFree',
  },
  {
    value: 'brave',
    label: 'Brave Search',
    needsApiKey: true,
    needsBaseUrl: false,
    optionalBaseUrl: false,
    signupUrl: 'https://brave.com/search/api/',
    hintKey: 'searchHintApiKey',
  },
  {
    value: 'tavily',
    label: 'Tavily',
    needsApiKey: true,
    needsBaseUrl: false,
    optionalBaseUrl: false,
    signupUrl: 'https://tavily.com/',
    hintKey: 'searchHintApiKey',
  },
  {
    value: 'bocha',
    label: 'Bocha',
    needsApiKey: true,
    needsBaseUrl: false,
    optionalBaseUrl: false,
    signupUrl: 'https://open.bochaai.com/',
    hintKey: 'searchHintApiKey',
  },
  {
    value: 'metaso',
    label: 'Metaso',
    needsApiKey: true,
    needsBaseUrl: false,
    optionalBaseUrl: false,
    signupUrl: 'https://metaso.cn/',
    hintKey: 'searchHintApiKey',
  },
  {
    value: 'searxng',
    label: 'SearXNG',
    needsApiKey: false,
    needsBaseUrl: true,
    optionalBaseUrl: false,
    signupUrl: 'https://docs.searxng.org/',
    hintKey: 'searchHintSearxng',
  },
  {
    value: 'baidu',
    label: 'Baidu AI Search',
    needsApiKey: true,
    needsBaseUrl: false,
    optionalBaseUrl: false,
    signupUrl: 'https://cloud.baidu.com/product/qianfan',
    hintKey: 'searchHintApiKey',
  },
  {
    value: 'volcengine',
    label: 'Volcengine Ark',
    needsApiKey: true,
    needsBaseUrl: false,
    optionalBaseUrl: false,
    signupUrl: 'https://console.volcengine.com/ark',
    hintKey: 'searchHintApiKey',
  },
  {
    value: 'sofya',
    label: 'Sofya',
    needsApiKey: true,
    needsBaseUrl: false,
    optionalBaseUrl: false,
    signupUrl: 'https://sofya.co/',
    hintKey: 'searchHintApiKey',
  },
]

export function getSearchProviderMeta(provider: SearchProvider): SearchProviderMeta {
  return SEARCH_PROVIDER_META.find((m) => m.value === provider) ?? SEARCH_PROVIDER_META[0]
}

export const useSearchConfigStore = defineStore('searchConfig', () => {
  const config = ref<SearchConfig | null>(null)
  const loading = ref(false)
  const saving = ref(false)

  const providerOptions = computed(() =>
    SEARCH_PROVIDER_META.map((m) => ({ value: m.value, label: m.label })),
  )

  async function loadConfig() {
    loading.value = true
    try {
      const cfg = await fetchJSON<ConfigFile>('/config')
      config.value = cfg.search
    } catch {
      config.value = null
    } finally {
      loading.value = false
    }
  }

  async function saveConfig(payload: UpsertSearchConfigRequest) {
    saving.value = true
    try {
      const req: UpdateConfigFileRequest = { search: payload }
      const cfg = await fetchJSON<ConfigFile>('/config', {
        method: 'PUT',
        body: JSON.stringify(req),
      })
      config.value = cfg.search
      toast.success(i18n.global.t('settings.searchSaved'))
    } catch (e) {
      toast.error(e instanceof Error ? e.message : i18n.global.t('common.saveFailed'))
      throw e
    } finally {
      saving.value = false
    }
  }

  function providerLabel(p: SearchProvider) {
    return getSearchProviderMeta(p).label
  }

  return {
    config,
    loading,
    saving,
    providerOptions,
    loadConfig,
    saveConfig,
    providerLabel,
  }
})
