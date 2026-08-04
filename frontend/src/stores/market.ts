import { defineStore } from 'pinia'
import { ref } from 'vue'
import { fetchJSON, asArray } from '@/api/client'
import type { MarketListing, MarketSource, InstallMarketResult, UninstallMarketResult } from '@/types'

interface MarketCatalogResponse {
  items?: MarketListing[] | null
  warnings?: string[] | null
}

export const useMarketStore = defineStore('market', () => {
  const sources = ref<MarketSource[]>([])
  const catalog = ref<MarketListing[]>([])
  const warnings = ref<string[]>([])
  const loading = ref(false)
  const installing = ref<string | null>(null)
  const error = ref('')
  const lastInstallResult = ref<InstallMarketResult | null>(null)
  const lastUninstallResult = ref<UninstallMarketResult | null>(null)

  async function loadSources() {
    sources.value = asArray(await fetchJSON<MarketSource[]>('/market/sources').catch(() => [] as MarketSource[]))
  }

  async function loadCatalog(refresh = false) {
    loading.value = true
    error.value = ''
    warnings.value = []
    try {
      const q = refresh ? '?refresh=1' : ''
      const resp = await fetchJSON<MarketCatalogResponse | MarketListing[]>(`/market/catalog${q}`)
      if (Array.isArray(resp)) {
        catalog.value = resp
        warnings.value = []
      } else {
        catalog.value = asArray(resp.items)
        warnings.value = asArray(resp.warnings)
      }
      if (!catalog.value.length && warnings.value.length) {
        error.value = warnings.value.join('；')
      }
    } catch (e) {
      error.value = e instanceof Error ? e.message : String(e)
      catalog.value = []
      warnings.value = []
    } finally {
      loading.value = false
    }
  }

  function markInstalled(kind: string, id: string, installed: boolean) {
    const i = catalog.value.findIndex((item) => item.kind === kind && item.id === id)
    if (i >= 0) {
      catalog.value[i] = { ...catalog.value[i], installed }
    }
  }

  async function install(sourceId: string, kind: string, id: string, overwrite = false) {
    installing.value = `${kind}:${id}`
    try {
      const result = await fetchJSON<InstallMarketResult>('/market/install', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ sourceId, kind, id, overwrite }),
      })
      lastInstallResult.value = result
      // Optimistic update — don't block the install button on a full catalog refresh
      // (GitHub/Gitee refresh can hang and leave the spinner stuck).
      markInstalled(kind, id, true)
      if (result?.installed?.length) {
        for (const iid of result.installed) {
          if (iid !== id) markInstalled('connector', iid, true)
          if (iid !== id) markInstalled('skill', iid, true)
        }
      }
      void loadCatalog(true)
      return result
    } finally {
      installing.value = null
    }
  }

  async function uninstall(
    kind: string,
    id: string,
    opts?: { runCleanup?: boolean; sourceId?: string },
  ) {
    installing.value = `${kind}:${id}`
    try {
      const result = await fetchJSON<UninstallMarketResult>('/market/uninstall', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          kind,
          id,
          runCleanup: !!opts?.runCleanup,
          sourceId: opts?.sourceId || undefined,
        }),
      })
      lastUninstallResult.value = result
      markInstalled(kind, id, false)
      void loadCatalog(true)
      return result
    } finally {
      installing.value = null
    }
  }

  return {
    sources,
    catalog,
    warnings,
    loading,
    installing,
    error,
    lastInstallResult,
    lastUninstallResult,
    loadSources,
    loadCatalog,
    install,
    uninstall,
  }
})
