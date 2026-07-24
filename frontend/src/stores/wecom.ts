import { defineStore } from 'pinia'
import { ref } from 'vue'
import { fetchJSON } from '@/api/client'

export interface WecomStatus {
  enabled: boolean
  running: boolean
  defaultAgentId?: string
  defaultModelId?: string
  autoApprove: boolean
  botId?: string
  projectId?: string
  wsUrl?: string
  hasSecret?: boolean
}

export const useWecomStore = defineStore('wecom', () => {
  const status = ref<WecomStatus | null>(null)
  const loading = ref(false)
  const saving = ref(false)

  async function refreshStatus() {
    loading.value = true
    try {
      status.value = await fetchJSON<WecomStatus>('/channels/wecom/status')
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
    botId?: string
    secret?: string
    wsUrl?: string
    projectId?: string
  }) {
    saving.value = true
    try {
      status.value = await fetchJSON<WecomStatus>('/channels/wecom', {
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
