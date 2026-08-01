import { defineStore } from 'pinia'
import { ref } from 'vue'
import type { OfficeKind, OfficeMode } from '@/utils/office-route'

export type RightWorkspaceTab = 'plan' | 'files' | 'memory' | 'tables' | 'changes' | 'terminal'
export type LayoutMode = 'chat' | 'stage' | 'immersive'

export interface OfficeStageState {
  kind: OfficeKind
  path: string
  mode: OfficeMode
  /** Preview kind: project raw URL or proxied external URL. */
  url?: string
  /** Diff kind: staged (index) vs unstaged (working tree) patch. */
  staged?: boolean
  /** Diff source: git working tree (default) or AI pre-turn snapshot. */
  diffSource?: 'git' | 'ai'
  sessionId?: string
  turnId?: string
}

const LEFT_RAIL_COLLAPSED_KEY = 'app-left-collapsed'
const RIGHT_PANEL_COLLAPSED_KEY = 'app-right-collapsed'

function readPersistedLeftRailCollapsed(): boolean {
  try {
    return localStorage.getItem(LEFT_RAIL_COLLAPSED_KEY) === '1'
  } catch {
    return false
  }
}

function writePersistedLeftRailCollapsed(collapsed: boolean) {
  try {
    localStorage.setItem(LEFT_RAIL_COLLAPSED_KEY, collapsed ? '1' : '0')
  } catch {
    /* ignore quota / private mode */
  }
}

function readPersistedRightPanelCollapsed(): boolean {
  try {
    return localStorage.getItem(RIGHT_PANEL_COLLAPSED_KEY) === '1'
  } catch {
    return false
  }
}

function writePersistedRightPanelCollapsed(collapsed: boolean) {
  try {
    localStorage.setItem(RIGHT_PANEL_COLLAPSED_KEY, collapsed ? '1' : '0')
  } catch {
    /* ignore quota / private mode */
  }
}

export const useWorkspaceUiStore = defineStore('workspaceUi', () => {
  const rightTab = ref<RightWorkspaceTab>('plan')
  const changesCount = ref(0)
  const pendingApprovals = ref(0)
  const paletteOpen = ref(false)
  const layoutMode = ref<LayoutMode>('chat')
  const stage = ref<OfficeStageState | null>(null)
  /** Bumped when Stage should reload file from disk (e.g. after AI turn). */
  const stageReloadToken = ref(0)
  /** One-shot composer prefill (Changes / Diff “ask about this”). */
  const composerPrefill = ref<string | null>(null)
  const composerPrefillToken = ref(0)

  /** Left rail collapsed UI state (may be temporary while Stage is open). */
  const leftRailCollapsed = ref(readPersistedLeftRailCollapsed())
  /**
   * Snapshot of left-rail collapsed before Stage auto-collapse.
   * null = no pending restore (user toggled during Stage, or Stage never auto-collapsed).
   */
  const leftRailCollapsedBeforeStage = ref<boolean | null>(null)
  const rightPanelCollapsed = ref(readPersistedRightPanelCollapsed())
  /**
   * Snapshot of right-panel collapsed before Stage auto-collapse.
   * null = no pending restore (user toggled during Stage, or Stage never auto-collapsed).
   */
  const rightPanelCollapsedBeforeStage = ref<boolean | null>(null)

  function setRightTab(tab: RightWorkspaceTab) {
    rightTab.value = tab
  }

  /** User (or UI) toggles the left rail; persists preference and cancels Stage restore. */
  function setLeftRailCollapsed(collapsed: boolean) {
    leftRailCollapsed.value = collapsed
    writePersistedLeftRailCollapsed(collapsed)
    leftRailCollapsedBeforeStage.value = null
  }

  /** User (or UI) toggles the right panel; persists preference and cancels Stage restore. */
  function setRightPanelCollapsed(collapsed: boolean) {
    rightPanelCollapsed.value = collapsed
    writePersistedRightPanelCollapsed(collapsed)
    rightPanelCollapsedBeforeStage.value = null
  }

  function toggleRightPanelCollapsed() {
    setRightPanelCollapsed(!rightPanelCollapsed.value)
  }

  function collapseLeftRailForStage() {
    if (leftRailCollapsed.value) return
    // Only snapshot once per Stage session so reopen/replace keeps original preference.
    if (leftRailCollapsedBeforeStage.value == null) {
      leftRailCollapsedBeforeStage.value = false
    }
    leftRailCollapsed.value = true
    // Do not write localStorage — temporary for Stage focus.
  }

  function restoreLeftRailAfterStage() {
    if (leftRailCollapsedBeforeStage.value == null) return
    leftRailCollapsed.value = leftRailCollapsedBeforeStage.value
    leftRailCollapsedBeforeStage.value = null
    // Persisted preference was never overwritten by temp collapse.
  }

  function collapseRightPanelForStage() {
    if (rightPanelCollapsed.value) return
    if (rightPanelCollapsedBeforeStage.value == null) {
      rightPanelCollapsedBeforeStage.value = false
    }
    rightPanelCollapsed.value = true
    // Do not write localStorage — temporary for Stage focus.
  }

  function restoreRightPanelAfterStage() {
    if (rightPanelCollapsedBeforeStage.value == null) return
    rightPanelCollapsed.value = rightPanelCollapsedBeforeStage.value
    rightPanelCollapsedBeforeStage.value = null
  }

  function openStage(next: OfficeStageState) {
    stage.value = { ...next }
    layoutMode.value = next.mode === 'present' ? 'immersive' : 'stage'
    collapseLeftRailForStage()
    collapseRightPanelForStage()
  }

  function closeStage() {
    stage.value = null
    layoutMode.value = 'chat'
    restoreLeftRailAfterStage()
    restoreRightPanelAfterStage()
  }

  function setStageMode(mode: OfficeMode) {
    if (!stage.value) return
    stage.value = { ...stage.value, mode }
    layoutMode.value = mode === 'present' ? 'immersive' : 'stage'
  }

  function setStageKind(kind: OfficeKind) {
    if (!stage.value) return
    stage.value = { ...stage.value, kind }
  }

  /** Replace Stage path (and optional mode), e.g. md → sibling html for Present. */
  function setStagePath(path: string, mode?: OfficeMode) {
    if (!stage.value) return
    const nextMode = mode ?? stage.value.mode
    stage.value = { ...stage.value, path, mode: nextMode }
    layoutMode.value = nextMode === 'present' ? 'immersive' : 'stage'
  }

  function setStageUrl(url: string | undefined) {
    if (!stage.value) return
    if (stage.value.url === url) return
    stage.value = { ...stage.value, url }
  }

  function requestStageReload() {
    stageReloadToken.value++
  }

  function openPalette() {
    paletteOpen.value = true
  }

  function closePalette() {
    paletteOpen.value = false
  }

  function togglePalette() {
    paletteOpen.value = !paletteOpen.value
  }

  function prefillComposer(text: string) {
    const trimmed = text.trim()
    if (!trimmed) return
    composerPrefill.value = trimmed
    composerPrefillToken.value++
  }

  function consumeComposerPrefill(): string | null {
    const text = composerPrefill.value
    composerPrefill.value = null
    return text
  }

  return {
    rightTab,
    changesCount,
    pendingApprovals,
    paletteOpen,
    layoutMode,
    stage,
    stageReloadToken,
    composerPrefill,
    composerPrefillToken,
    leftRailCollapsed,
    leftRailCollapsedBeforeStage,
    rightPanelCollapsed,
    rightPanelCollapsedBeforeStage,
    setRightTab,
    setLeftRailCollapsed,
    setRightPanelCollapsed,
    toggleRightPanelCollapsed,
    openStage,
    closeStage,
    setStageMode,
    setStageKind,
    setStagePath,
    setStageUrl,
    requestStageReload,
    openPalette,
    closePalette,
    togglePalette,
    prefillComposer,
    consumeComposerPrefill,
  }
})
