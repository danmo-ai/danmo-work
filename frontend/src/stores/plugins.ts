import { defineStore } from 'pinia'
import { computed, ref } from 'vue'
import { fetchJSON, asArray } from '@/api/client'

export interface PluginAuthor {
  name?: string
  email?: string
  url?: string
}

export interface PluginComponents {
  skills?: string[]
  experts?: string[]
  mcp?: string[]
  knowledge?: string[]
}

export interface PluginInstalled {
  name: string
  version: string
  description?: string
  author?: PluginAuthor
  homepage?: string
  repository?: string
  license?: string
  keywords?: string[]
  rootPath: string
  marketSource?: string
  installedAt: string
  components: PluginComponents
}

export const usePluginsStore = defineStore('plugins', () => {
  const items = ref<PluginInstalled[]>([])
  const loading = ref(false)
  const uninstalling = ref<string | null>(null)
  const selectedName = ref<string | null>(null)

  const selected = computed(() => {
    if (!selectedName.value) return null
    return items.value.find((p) => p.name === selectedName.value) ?? null
  })

  async function load() {
    loading.value = true
    try {
      items.value = asArray(await fetchJSON<PluginInstalled[]>('/plugins'))
    } finally {
      loading.value = false
    }
  }

  async function uninstall(name: string) {
    uninstalling.value = name
    try {
      await fetchJSON('/market/uninstall', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ kind: 'plugin', id: name }),
      })
      items.value = items.value.filter((p) => p.name !== name)
      if (selectedName.value === name) {
        selectedName.value = null
      }
    } finally {
      uninstalling.value = null
    }
  }

  return {
    items,
    loading,
    uninstalling,
    selectedName,
    selected,
    load,
    uninstall,
  }
})
