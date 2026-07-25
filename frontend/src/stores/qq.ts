import { defineStore } from 'pinia'
import { ref } from 'vue'
import { fetchJSON } from '@/api/client'

export interface QQStatus {
  enabled: boolean
  running: boolean
  defaultAgentId?: string
  defaultModelId?: string
  autoApprove: boolean
  appId?: string
  projectId?: string
  hasClientSecret?: boolean
  groupDenyTools?: string[]
}

export const useQQStore = defineStore('qq', () => {
  const status = ref<QQStatus | null>(null)
  const loading = ref(false)
  const saving = ref(false)

  async function refreshStatus() {
    loading.value = true
    try {
      status.value = await fetchJSON<QQStatus>('/channels/qq/status')
    } catch {
      status.value = null
    } finally {
      loading.value = false
    }
  }

  async function configure(payload: {
    enabled: boolean
    defaultAgentId: string
    defaultModelId?: string
    autoApprove?: boolean
    appId?: string
    clientSecret?: string
    projectId?: string
    groupDenyTools?: string[]
  }) {
    saving.value = true
    try {
      status.value = await fetchJSON<QQStatus>('/channels/qq', {
        method: 'PUT',
        body: JSON.stringify(payload),
      })
      return status.value
    } finally {
      saving.value = false
    }
  }

  return { status, loading, saving, refreshStatus, configure }
})
