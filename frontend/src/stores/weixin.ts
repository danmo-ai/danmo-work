import { defineStore } from 'pinia'
import { ref } from 'vue'
import { fetchJSON, asArray } from '@/api/client'

export interface WeixinAccount {
  accountId: string
  baseUrl?: string
  userId?: string
  projectId?: string
  createdAt?: string
  updatedAt?: string
}

export interface WeixinStatus {
  enabled: boolean
  running: boolean
  defaultAgentId?: string
  defaultModelId?: string
  autoApprove: boolean
  accounts: WeixinAccount[]
  bindingCount: number
}

export interface WeixinBinding {
  id: string
  accountId: string
  peerUserId: string
  sessionId: string
  contextToken?: string
}

export interface WeixinLoginStart {
  sessionKey: string
  qrcodeUrl: string
}

export interface WeixinLoginWait {
  connected: boolean
  alreadyConnected?: boolean
  accountId?: string
  userId?: string
  projectId?: string
  message?: string
  needsVerifyCode?: boolean
}

export const useWeixinStore = defineStore('weixin', () => {
  const status = ref<WeixinStatus | null>(null)
  const bindings = ref<WeixinBinding[]>([])
  const loading = ref(false)
  const saving = ref(false)
  const qr = ref<{ sessionKey: string; url: string } | null>(null)
  const loginMessage = ref('')
  const verifyCode = ref('')

  async function refreshStatus() {
    loading.value = true
    try {
      status.value = await fetchJSON<WeixinStatus>('/channels/weixin/status')
    } catch {
      status.value = null
    } finally {
      loading.value = false
    }
  }

  async function refreshBindings() {
    try {
      bindings.value = asArray(await fetchJSON<WeixinBinding[]>('/channels/weixin/bindings'))
    } catch {
      bindings.value = []
    }
  }

  async function configure(payload: {
    enabled: boolean
    defaultAgentId: string
    defaultModelId?: string
    autoApprove?: boolean
  }) {
    saving.value = true
    try {
      status.value = await fetchJSON<WeixinStatus>('/channels/weixin', {
        method: 'PUT',
        body: JSON.stringify(payload),
      })
      await refreshBindings()
      return status.value
    } finally {
      saving.value = false
    }
  }

  async function startLogin(projectId: string) {
    loginMessage.value = ''
    const res = await fetchJSON<WeixinLoginStart>('/channels/weixin/login/start', {
      method: 'POST',
      body: JSON.stringify({ projectId }),
    })
    qr.value = { sessionKey: res.sessionKey, url: res.qrcodeUrl }
    return res
  }

  async function waitLogin(timeoutMs = 120000) {
    if (!qr.value) throw new Error('no login session')
    const res = await fetchJSON<WeixinLoginWait>('/channels/weixin/login/wait', {
      method: 'POST',
      body: JSON.stringify({
        sessionKey: qr.value.sessionKey,
        verifyCode: verifyCode.value || undefined,
        timeoutMs,
      }),
    })
    loginMessage.value = res.message || ''
    if (res.connected) {
      qr.value = null
      verifyCode.value = ''
      await refreshStatus()
      await refreshBindings()
    }
    return res
  }

  async function updateAccountProject(accountId: string, projectId: string) {
    const acc = await fetchJSON<WeixinAccount>(`/channels/weixin/accounts/${encodeURIComponent(accountId)}`, {
      method: 'PUT',
      body: JSON.stringify({ projectId }),
    })
    await refreshStatus()
    return acc
  }

  async function logout(accountId?: string) {
    await fetchJSON('/channels/weixin/logout', {
      method: 'POST',
      body: JSON.stringify({ accountId: accountId || '' }),
    })
    await refreshStatus()
    await refreshBindings()
  }

  function isWeixinSession(sessionId: string): boolean {
    return bindings.value.some((b) => b.sessionId === sessionId)
  }

  return {
    status,
    bindings,
    loading,
    saving,
    qr,
    loginMessage,
    verifyCode,
    refreshStatus,
    refreshBindings,
    configure,
    startLogin,
    waitLogin,
    updateAccountProject,
    logout,
    isWeixinSession,
  }
})
