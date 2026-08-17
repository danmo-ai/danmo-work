import { defineStore } from 'pinia'
import { ref } from 'vue'
import { fetchJSON } from '@/api/client'
import { toast } from '@/utils/feedback'
import { i18n } from '@/i18n'
import type { ConfigFile, ConfigMarketSection, MarketSourceConfig, UpdateConfigFileRequest } from '@/types/mission'

function emptySource(): MarketSourceConfig {
  return {
    id: '',
    name: '',
    kind: 'git',
    platform: 'github',
    repo: '',
    ref: 'main',
    catalogPath: 'catalog/index.json',
    token: '',
    enabled: true,
    priority: 50,
  }
}

function normalizeMarket(m?: ConfigMarketSection | null): ConfigMarketSection {
  return {
    cacheTtlHours: m?.cacheTtlHours && m.cacheTtlHours > 0 ? m.cacheTtlHours : 6,
    sources: (m?.sources ?? []).map((s) => {
      const kind = s.kind || 'git'
      const isRegistry = kind === 'clawhub' || kind === 'techleads' || kind === 'tlc' || kind === 'tech-leads-club'
      return {
        id: s.id ?? '',
        name: s.name ?? '',
        kind,
        platform: s.platform || (isRegistry ? '' : 'github'),
        repo: s.repo ?? '',
        ref: s.ref || (kind === 'techleads' || kind === 'tlc' || kind === 'tech-leads-club' ? 'latest' : isRegistry ? '' : 'main'),
        catalogPath: s.catalogPath || (isRegistry ? '' : 'catalog/index.json'),
        token: s.token ?? '',
        enabled: !!s.enabled,
        priority: typeof s.priority === 'number' ? s.priority : 50,
      }
    }),
  }
}

export const useMarketConfigStore = defineStore('marketConfig', () => {
  const config = ref<ConfigMarketSection | null>(null)
  const loading = ref(false)
  const saving = ref(false)

  const kindOptions = [
    { value: 'git', label: 'Git catalog' },
    { value: 'techleads', label: 'Tech Leads Club' },
    { value: 'clawhub', label: 'ClawHub' },
  ]

  const platformOptions = [
    { value: 'github', label: 'GitHub' },
    { value: 'gitee', label: 'Gitee' },
    { value: 'generic', label: 'Generic Git' },
    { value: 'local', label: 'Local path' },
  ]

  async function loadConfig() {
    loading.value = true
    try {
      const cfg = await fetchJSON<ConfigFile>('/config')
      config.value = normalizeMarket(cfg.market)
    } catch {
      config.value = normalizeMarket(null)
    } finally {
      loading.value = false
    }
  }

  async function saveConfig(payload: ConfigMarketSection) {
    saving.value = true
    try {
      const cleaned: ConfigMarketSection = {
        cacheTtlHours: payload.cacheTtlHours > 0 ? payload.cacheTtlHours : 6,
        sources: payload.sources
          .filter((s) => s.id.trim() && (s.repo ?? '').trim())
          .map((s) => {
            const kind = s.kind || 'git'
            if (kind === 'clawhub') {
              return {
                id: s.id.trim(),
                name: s.name.trim() || s.id.trim(),
                kind: 'clawhub',
                repo: (s.repo ?? '').trim() || 'https://clawhub.ai',
                token: s.token?.trim() || undefined,
                enabled: !!s.enabled,
                priority: typeof s.priority === 'number' ? s.priority : 50,
              }
            }
            if (kind === 'techleads' || kind === 'tlc' || kind === 'tech-leads-club') {
              return {
                id: s.id.trim(),
                name: s.name.trim() || s.id.trim(),
                kind: 'techleads',
                repo: (s.repo ?? '').trim() || '@tech-leads-club/skills-catalog',
                ref: s.ref?.trim() || 'latest',
                token: s.token?.trim() || undefined,
                enabled: !!s.enabled,
                priority: typeof s.priority === 'number' ? s.priority : 50,
              }
            }
            return {
              ...s,
              id: s.id.trim(),
              name: s.name.trim() || s.id.trim(),
              kind: 'git',
              platform: s.platform || 'github',
              repo: (s.repo ?? '').trim(),
              ref: s.ref?.trim() || 'main',
              catalogPath: s.catalogPath?.trim() || 'catalog/index.json',
              token: s.token?.trim() || undefined,
            }
          }),
      }
      const req: UpdateConfigFileRequest = { market: cleaned }
      const cfg = await fetchJSON<ConfigFile>('/config', {
        method: 'PUT',
        body: JSON.stringify(req),
      })
      config.value = normalizeMarket(cfg.market)
      toast.success(i18n.global.t('settings.marketSaved'))
    } catch (e) {
      toast.error(e instanceof Error ? e.message : i18n.global.t('common.saveFailed'))
      throw e
    } finally {
      saving.value = false
    }
  }

  return {
    config,
    loading,
    saving,
    kindOptions,
    platformOptions,
    emptySource,
    loadConfig,
    saveConfig,
  }
})
