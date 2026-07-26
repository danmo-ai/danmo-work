import { defineStore } from 'pinia'
import { ref } from 'vue'
import type { OfficeKind, OfficeMode } from '@/utils/office-route'

export type RightWorkspaceTab = 'plan' | 'files' | 'memory' | 'changes' | 'terminal'
export type LayoutMode = 'chat' | 'stage' | 'immersive'

export interface OfficeStageState {
  kind: OfficeKind
  path: string
  mode: OfficeMode
  /** Preview kind: project raw URL or proxied external URL. */
  url?: string
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

  function setRightTab(tab: RightWorkspaceTab) {
    rightTab.value = tab
  }

  function openStage(next: OfficeStageState) {
    stage.value = { ...next }
    layoutMode.value = next.mode === 'present' ? 'immersive' : 'stage'
  }

  function closeStage() {
    stage.value = null
    layoutMode.value = 'chat'
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

  return {
    rightTab,
    changesCount,
    pendingApprovals,
    paletteOpen,
    layoutMode,
    stage,
    stageReloadToken,
    setRightTab,
    openStage,
    closeStage,
    setStageMode,
    setStageKind,
    setStageUrl,
    requestStageReload,
    openPalette,
    closePalette,
    togglePalette,
  }
})
