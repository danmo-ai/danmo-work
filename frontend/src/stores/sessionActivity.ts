import { defineStore } from 'pinia'
import { computed, ref } from 'vue'
import { asArray, fetchJSON } from '@/api/client'
import {
  activityRank,
  type SessionActivityItem,
  type SessionActivityState,
} from '@/types/session-activity'

const POLL_MS = 4000

export const useSessionActivityStore = defineStore('sessionActivity', () => {
  const items = ref<SessionActivityItem[]>([])
  const loading = ref(false)
  /** Local overlay: current session waiting on ask_user (not in approvals table). */
  const localAskSessionId = ref<string | null>(null)
  let timer: ReturnType<typeof setInterval> | null = null
  let started = false

  const byId = computed(() => {
    const map = new Map<string, SessionActivityItem>()
    for (const item of items.value) {
      map.set(item.sessionId, { ...item })
    }
    if (localAskSessionId.value) {
      const id = localAskSessionId.value
      const prev = map.get(id)
      if (prev?.state === 'awaiting_approval') {
        // Permission ask already covers "needs human".
        return map
      }
      map.set(id, {
        sessionId: id,
        state: 'awaiting_ask',
        runningTurnId: prev?.runningTurnId,
        pendingApprovalCount: Math.max(1, prev?.pendingApprovalCount ?? 0),
      })
    }
    return map
  })

  function stateFor(sessionId: string): SessionActivityState {
    const item = byId.value.get(sessionId)
    if (!item) return 'idle'
    if (item.state === 'awaiting_approval' || item.state === 'awaiting_ask' || item.state === 'running') {
      return item.state
    }
    return 'idle'
  }

  const activeItems = computed(() =>
    [...byId.value.values()]
      .filter((i) => i.state === 'running' || i.state === 'awaiting_approval' || i.state === 'awaiting_ask')
      .sort((a, b) => activityRank(a.state) - activityRank(b.state)),
  )

  const activeCount = computed(() => activeItems.value.length)

  async function refresh() {
    loading.value = true
    try {
      items.value = asArray(await fetchJSON<SessionActivityItem[]>('/sessions/activity'))
    } catch {
      /* keep last snapshot */
    } finally {
      loading.value = false
    }
  }

  function setLocalAsk(sessionId: string | null) {
    localAskSessionId.value = sessionId
  }

  function startPolling() {
    if (started) return
    started = true
    void refresh()
    timer = setInterval(() => {
      void refresh()
    }, POLL_MS)
  }

  function stopPolling() {
    started = false
    if (timer) {
      clearInterval(timer)
      timer = null
    }
  }

  return {
    items,
    loading,
    byId,
    activeItems,
    activeCount,
    stateFor,
    refresh,
    setLocalAsk,
    startPolling,
    stopPolling,
  }
})
