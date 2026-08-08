import { defineStore } from 'pinia'
import { ref } from 'vue'
import { fetchJSON } from '@/api/client'

export interface RemoteStatus {
  enabled: boolean
  connected: boolean
  hubUrl: string
  localBase?: string
  tlsInsecure?: boolean
  deviceId?: string
  lastError?: string
  connectedAt?: string
}

export interface RemoteConfigurePayload {
  enabled: boolean
  hubUrl: string
  localBase?: string
  tlsInsecure?: boolean
}

export const useRemoteStore = defineStore('remote', () => {
  const status = ref<RemoteStatus | null>(null)
  const loading = ref(false)
  const saving = ref(false)
  const pairing = ref(false)
  const pairCode = ref('')
  const pairExpiresIn = ref(0)
  const revoking = ref(false)

  async function refreshStatus() {
    loading.value = true
    try {
      status.value = await fetchJSON<RemoteStatus>('/remote/status')
    } catch {
      status.value = null
    } finally {
      loading.value = false
    }
  }

  async function configure(payload: RemoteConfigurePayload) {
    saving.value = true
    try {
      status.value = await fetchJSON<RemoteStatus>('/remote', {
        method: 'PUT',
        body: JSON.stringify({
          enabled: payload.enabled,
          hubUrl: payload.hubUrl,
          localBase: payload.localBase || 'http://127.0.0.1:7801',
          tlsInsecure: !!payload.tlsInsecure,
        }),
      })
      return status.value
    } finally {
      saving.value = false
    }
  }

  async function requestPairCode() {
    pairing.value = true
    pairCode.value = ''
    pairExpiresIn.value = 0
    try {
      const res = await fetchJSON<{ code: string; expiresIn: number }>('/remote/pair/code', {
        method: 'POST',
        body: '{}',
      })
      pairCode.value = res.code
      pairExpiresIn.value = res.expiresIn
      return res
    } finally {
      pairing.value = false
    }
  }

  async function revokeTokens() {
    revoking.value = true
    try {
      await fetchJSON('/remote/pair/revoke', { method: 'POST', body: '{}' })
    } finally {
      revoking.value = false
    }
  }

  return {
    status,
    loading,
    saving,
    pairing,
    pairCode,
    pairExpiresIn,
    revoking,
    refreshStatus,
    configure,
    requestPairCode,
    revokeTokens,
  }
})
