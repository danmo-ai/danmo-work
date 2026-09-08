import { defineStore } from 'pinia'
import { computed, ref } from 'vue'
import { useSessionsStore } from '@/stores/sessions'

const OPEN_TABS_KEY = 'app-open-session-tabs'

/** Sentinel id for the draft "New Session" compose tab (not a real session). */
export const COMPOSE_TAB_ID = '__compose__'

function readPersisted(): string[] {
  try {
    const raw = localStorage.getItem(OPEN_TABS_KEY)
    if (!raw) return []
    const parsed = JSON.parse(raw) as unknown
    if (!Array.isArray(parsed)) return []
    return parsed.filter((id): id is string => typeof id === 'string' && id.length > 0 && id !== COMPOSE_TAB_ID)
  } catch {
    return []
  }
}

function writePersisted(ids: string[]) {
  try {
    localStorage.setItem(OPEN_TABS_KEY, JSON.stringify(ids))
  } catch {
    /* ignore quota / private mode */
  }
}

/**
 * Browser-like open working set for sessions.
 *
 * Semantics:
 * - Opening a session (select / create) adds it to the open set.
 * - Closing a tab removes it from the working set only — does not archive or delete.
 * - Archive / delete / missing ids prune the tab from the set.
 * - Order is left-to-right open order; persisted across reloads.
 */
export const useOpenSessionTabsStore = defineStore('openSessionTabs', () => {
  const openIds = ref<string[]>(readPersisted())

  const openSessions = computed(() => {
    const sessions = useSessionsStore()
    const byId = new Map(
      sessions.sessions
        .filter((s) => s.status !== 'archived')
        .map((s) => [s.id, s]),
    )
    return openIds.value.map((id) => byId.get(id)).filter((s): s is NonNullable<typeof s> => !!s)
  })

  function persist() {
    writePersisted(openIds.value)
  }

  function open(id: string) {
    if (!id || id === COMPOSE_TAB_ID) return
    if (openIds.value.includes(id)) return
    openIds.value = [...openIds.value, id]
    persist()
  }

  /** Silent remove (archive / delete / prune). Does not decide navigation. */
  function remove(id: string) {
    if (!openIds.value.includes(id)) return
    openIds.value = openIds.value.filter((x) => x !== id)
    persist()
  }

  /**
   * Close a tab from the working set.
   * @returns next session id to activate, or null to enter compose.
   */
  function close(id: string): string | null {
    const idx = openIds.value.indexOf(id)
    if (idx < 0) return null
    const next = openIds.value[idx + 1] ?? openIds.value[idx - 1] ?? null
    openIds.value = openIds.value.filter((x) => x !== id)
    persist()
    return next
  }

  function prune(validIds: Set<string>) {
    const next = openIds.value.filter((id) => validIds.has(id))
    if (next.length === openIds.value.length) return
    openIds.value = next
    persist()
  }

  function reorder(fromIndex: number, toIndex: number) {
    if (fromIndex === toIndex) return
    if (fromIndex < 0 || toIndex < 0) return
    if (fromIndex >= openIds.value.length || toIndex >= openIds.value.length) return
    const next = [...openIds.value]
    const [item] = next.splice(fromIndex, 1)
    next.splice(toIndex, 0, item)
    openIds.value = next
    persist()
  }

  return {
    openIds,
    openSessions,
    open,
    remove,
    close,
    prune,
    reorder,
  }
})
