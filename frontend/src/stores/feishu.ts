import { defineStore } from 'pinia'
import { ref } from 'vue'
import { fetchJSON } from '@/api/client'

export type FeishuDomain = 'feishu' | 'lark'

export interface FeishuStatus {
  enabled: boolean
  running: boolean
  domain?: FeishuDomain | string
  defaultAgentId?: string
  defaultModelId?: string
  autoApprove: boolean
  appId?: string
  projectId?: string
  hasAppSecret?: boolean
}

export const useFeishuStore = defineStore('feishu', () => {
  const status = ref<FeishuStatus | null>(null)
  const loading = ref(false)
  const saving = ref(false)

  async function refreshStatus() {
    loading.value = true
    try {
      status.value = await fetchJSON<FeishuStatus>('/channels/feishu/status')
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
    domain?: FeishuDomain
    appId?: string
    appSecret?: string
    projectId?: string
  }) {
    saving.value = true
    try {
      status.value = await fetchJSON<FeishuStatus>('/channels/feishu', {
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
