import { defineStore } from 'pinia'
import { ref } from 'vue'
import type { OfficeKind, OfficeMode } from '@/utils/office-route'
import { isWorkbenchId, type WorkbenchId } from '@/types/workbench'

export type RightWorkspaceTab = 'plan' | 'files' | 'memory' | 'tables' | 'changes' | 'terminal' | 'trajectory'
export type LayoutMode = 'chat' | 'stage' | 'immersive'

const ACTIVE_WORKBENCH_KEY = 'app-active-workbench'

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
const RIGHT_DRAWER_OPEN_KEY = 'app-right-drawer-open'
/** Legacy docked-panel key — migrated once into drawer-open semantics. */
const LEGACY_RIGHT_PANEL_COLLAPSED_KEY = 'app-right-collapsed'

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

function readPersistedRightDrawerOpen(): boolean {
  try {
    const raw = localStorage.getItem(RIGHT_DRAWER_OPEN_KEY)
    if (raw === '1' || raw === '0') return raw === '1'
    // Migrate: old collapsed=true → drawer closed; otherwise default closed (new UX).
    const legacy = localStorage.getItem(LEGACY_RIGHT_PANEL_COLLAPSED_KEY)
    if (legacy != null) {
      localStorage.removeItem(LEGACY_RIGHT_PANEL_COLLAPSED_KEY)
    }
    return false
  } catch {
    return false
  }
}

function writePersistedRightDrawerOpen(open: boolean) {
  try {
    localStorage.setItem(RIGHT_DRAWER_OPEN_KEY, open ? '1' : '0')
  } catch {
    /* ignore quota / private mode */
  }
}

function readPersistedWorkbenchId(): WorkbenchId {
  try {
    const raw = localStorage.getItem(ACTIVE_WORKBENCH_KEY) ?? ''
    if (isWorkbenchId(raw)) return raw
  } catch {
    /* ignore */
  }
  return 'novel'
}

export const useWorkspaceUiStore = defineStore('workspaceUi', () => {
  const rightTab = ref<RightWorkspaceTab>('plan')
  const changesCount = ref(0)
  const memoryCount = ref(0)
  const pendingApprovals = ref(0)
  const paletteOpen = ref(false)
  const layoutMode = ref<LayoutMode>('chat')
  const stage = ref<OfficeStageState | null>(null)
  /** Bumped when Stage should reload file from disk (e.g. after AI turn). */
  const stageReloadToken = ref(0)
  /** Bumped when the Files tree should reload (e.g. after composer upload). */
  const filesReloadToken = ref(0)
  /** One-shot composer prefill (Changes / Diff “ask about this”). */
  const composerPrefill = ref<string | null>(null)
  const composerPrefillToken = ref(0)
  /** One-shot expert ids to select in Composer (e.g. novel workbench actions). */
  const composerSelectExpertIds = ref<string[] | null>(null)
  const composerSelectExpertToken = ref(0)
  /** One-shot skill ids to select in Composer (e.g. novel workbench actions). */
  const composerSelectSkillIds = ref<string[] | null>(null)
  const composerSelectSkillToken = ref(0)

  /** Session split-pane workbench host (Office-style left stream | right workbench). */
  const workbenchOpen = ref(false)
  const activeWorkbenchId = ref<WorkbenchId>(readPersistedWorkbenchId())

  /** Left rail collapsed UI state (may be temporary while Stage is open). */
  const leftRailCollapsed = ref(readPersistedLeftRailCollapsed())
  /**
   * Snapshot of left-rail collapsed before Stage auto-collapse.
   * null = no pending restore (user toggled during Stage, or Stage never auto-collapsed).
   */
  const leftRailCollapsedBeforeStage = ref<boolean | null>(null)

  /** Right workspace glass drawer (floats over stream). */
  const rightDrawerOpen = ref(readPersistedRightDrawerOpen())
  /** Snapshot before Stage auto-close; null = no pending restore. */
  const rightDrawerOpenBeforeStage = ref<boolean | null>(null)

  function setRightTab(tab: RightWorkspaceTab) {
    rightTab.value = tab
  }

  /** User (or UI) toggles the left rail; persists preference and cancels Stage restore. */
  function setLeftRailCollapsed(collapsed: boolean) {
    leftRailCollapsed.value = collapsed
    writePersistedLeftRailCollapsed(collapsed)
    leftRailCollapsedBeforeStage.value = null
  }

  function setRightDrawerOpen(open: boolean) {
    rightDrawerOpen.value = open
    writePersistedRightDrawerOpen(open)
    rightDrawerOpenBeforeStage.value = null
  }

  function openRightDrawer(tab?: RightWorkspaceTab) {
    if (tab) rightTab.value = tab
    setRightDrawerOpen(true)
  }

  function closeRightDrawer() {
    setRightDrawerOpen(false)
  }

  /** Same tab while open → close; otherwise open (and switch tab). */
  function toggleRightDrawer(tab?: RightWorkspaceTab) {
    if (tab && rightDrawerOpen.value && rightTab.value === tab) {
      closeRightDrawer()
      return
    }
    if (tab) rightTab.value = tab
    setRightDrawerOpen(true)
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

  function closeRightDrawerForStage() {
    if (!rightDrawerOpen.value) return
    if (rightDrawerOpenBeforeStage.value == null) {
      rightDrawerOpenBeforeStage.value = true
    }
    rightDrawerOpen.value = false
    // Do not write localStorage — temporary for Stage focus.
  }

  function restoreRightDrawerAfterStage() {
    if (rightDrawerOpenBeforeStage.value == null) return
    rightDrawerOpen.value = rightDrawerOpenBeforeStage.value
    rightDrawerOpenBeforeStage.value = null
  }

  function openStage(next: OfficeStageState) {
    workbenchOpen.value = false
    stage.value = { ...next }
    layoutMode.value = next.mode === 'present' ? 'immersive' : 'stage'
    collapseLeftRailForStage()
    closeRightDrawerForStage()
  }

  function closeStage() {
    stage.value = null
    if (!workbenchOpen.value) {
      layoutMode.value = 'chat'
      restoreLeftRailAfterStage()
      restoreRightDrawerAfterStage()
    }
  }

  function setActiveWorkbenchId(id: WorkbenchId) {
    activeWorkbenchId.value = id
    try {
      localStorage.setItem(ACTIVE_WORKBENCH_KEY, id)
    } catch {
      /* ignore */
    }
  }

  function openWorkbench(id?: WorkbenchId) {
    if (id) setActiveWorkbenchId(id)
    stage.value = null
    closeRightDrawer()
    workbenchOpen.value = true
    layoutMode.value = 'stage'
    collapseLeftRailForStage()
  }

  function closeWorkbench() {
    if (!workbenchOpen.value) return
    workbenchOpen.value = false
    if (!stage.value) {
      layoutMode.value = 'chat'
      restoreLeftRailAfterStage()
      restoreRightDrawerAfterStage()
    }
  }

  function toggleWorkbench(id?: WorkbenchId) {
    if (workbenchOpen.value && (!id || activeWorkbenchId.value === id)) {
      closeWorkbench()
      return
    }
    openWorkbench(id ?? activeWorkbenchId.value)
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

  function requestFilesReload() {
    filesReloadToken.value++
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

  function requestComposerSelectExperts(ids: string[]) {
    const cleaned = ids.map((id) => id.trim()).filter(Boolean)
    if (!cleaned.length) return
    composerSelectExpertIds.value = cleaned
    composerSelectExpertToken.value++
  }

  function consumeComposerSelectExperts(): string[] {
    const ids = composerSelectExpertIds.value ?? []
    composerSelectExpertIds.value = null
    return ids
  }

  function requestComposerSelectSkills(ids: string[]) {
    const cleaned = ids.map((id) => id.trim()).filter(Boolean)
    if (!cleaned.length) return
    composerSelectSkillIds.value = cleaned
    composerSelectSkillToken.value++
  }

  function consumeComposerSelectSkills(): string[] {
    const ids = composerSelectSkillIds.value ?? []
    composerSelectSkillIds.value = null
    return ids
  }

  return {
    rightTab,
    changesCount,
    memoryCount,
    pendingApprovals,
    paletteOpen,
    layoutMode,
    stage,
    stageReloadToken,
    filesReloadToken,
    composerPrefill,
    composerPrefillToken,
    composerSelectExpertIds,
    composerSelectExpertToken,
    composerSelectSkillIds,
    composerSelectSkillToken,
    workbenchOpen,
    activeWorkbenchId,
    leftRailCollapsed,
    leftRailCollapsedBeforeStage,
    rightDrawerOpen,
    rightDrawerOpenBeforeStage,
    setRightTab,
    setLeftRailCollapsed,
    setRightDrawerOpen,
    openRightDrawer,
    closeRightDrawer,
    toggleRightDrawer,
    openStage,
    closeStage,
    openWorkbench,
    closeWorkbench,
    toggleWorkbench,
    setActiveWorkbenchId,
    setStageMode,
    setStageKind,
    setStagePath,
    setStageUrl,
    requestStageReload,
    requestFilesReload,
    openPalette,
    closePalette,
    togglePalette,
    prefillComposer,
    consumeComposerPrefill,
    requestComposerSelectExperts,
    consumeComposerSelectExperts,
    requestComposerSelectSkills,
    consumeComposerSelectSkills,
  }
})
