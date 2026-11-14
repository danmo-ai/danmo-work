<script setup lang="ts">
import { computed, ref, watch, nextTick } from 'vue'
import { storeToRefs } from 'pinia'
import { useI18n } from 'vue-i18n'
import { useWorkspaceUiStore } from '@/stores/workspaceUi'
import { useSessionsStore } from '@/stores/sessions'
import { useStageAiReviewStore } from '@/stores/stageAiReview'
import { fetchAiReviewStatus, revertAiReviewFile } from '@/api/aiReview'
import DocSurface from '@/components/office/DocSurface.vue'
import UniverSlidesSurface from '@/components/office/UniverSlidesSurface.vue'
import SheetSurface from '@/components/office/SheetSurface.vue'
import UniverSheetSurface from '@/components/office/UniverSheetSurface.vue'
import UniverDocSurface from '@/components/office/UniverDocSurface.vue'
import MsOfficePreviewSurface from '@/components/office/MsOfficePreviewSurface.vue'
import WebSurface from '@/components/office/WebSurface.vue'
import BrowserLiveSurface from '@/components/office/BrowserLiveSurface.vue'
import DesktopPanel from '@/components/center/DesktopPanel.vue'
import CodeSurface from '@/components/office/CodeSurface.vue'
import DiffSurface from '@/components/office/DiffSurface.vue'
import VueSfcSurface from '@/components/office/VueSfcSurface.vue'
import OfficeAiToolbar from '@/components/office/OfficeAiToolbar.vue'
import { confirm, toast } from '@/utils/feedback'
import { stageFromUri } from '@/utils/stage-uri'
import type { FileEditScope } from '@/utils/file-route'
import { exportMarkdownPdf } from '@/utils/md-export-pdf'
import type { ElementAttachment } from '@/types/element-attachment'
import type { CodeSelectionAttachment } from '@/types/code-attachment'
import {
  createOfficeEditAttachment,
  type OfficeEditAttachment,
} from '@/types/office-edit-attachment'

const props = defineProps<{
  projectId: string
}>()

const emit = defineEmits<{
  attachElement: [att: ElementAttachment]
  attachCodeSelection: [att: CodeSelectionAttachment]
  attachOfficeEdit: [att: OfficeEditAttachment]
  attachConsole: [text: string]
}>()

const { t } = useI18n()
const workspaceUi = useWorkspaceUiStore()
const sessions = useSessionsStore()
const aiReview = useStageAiReviewStore()
const { stage, stageReloadToken, layoutMode } = storeToRefs(workspaceUi)
const { pending: pendingAiReview, afterTurn: aiReviewAfterTurn } = storeToRefs(aiReview)
const lastFinishedTurnId = ref<string | null>(null)
const bannerBusy = ref(false)

const docRef = ref<InstanceType<typeof DocSurface> | InstanceType<typeof UniverDocSurface> | null>(null)
const slidesRef = ref<InstanceType<typeof UniverSlidesSurface> | null>(null)
const sheetRef = ref<InstanceType<typeof SheetSurface> | InstanceType<typeof UniverSheetSurface> | null>(null)
const codeRef = ref<InstanceType<typeof CodeSurface> | null>(null)
const aiToolbarRef = ref<InstanceType<typeof OfficeAiToolbar> | null>(null)
const dirty = ref(false)
const pageIndex = ref(0)
const editScope = ref<FileEditScope>('document')

const turnRunning = computed(() => {
  const id = sessions.runningTurnId
  if (!id) return false
  const turn = sessions.turns.find((x) => x.id === id)
  return turn?.status === 'running'
})

const isDiff = computed(() => surfaceKey.value === 'diff')
const isVisualStage = computed(() => surfaceKey.value === 'browser-live' || surfaceKey.value === 'desktop')
const isCodeKind = computed(() => surfaceKey.value === 'code')
const stageEngine = computed(() => stage.value?.engine)
const isMsOffice = computed(() => surfaceKey.value === 'ms-office')
/** Office AI toolbar kinds (not code/diff/ms-office). */
const editableKind = computed(() => {
  switch (surfaceKey.value) {
    case 'doc-md':
    case 'doc-univer':
      return 'doc' as const
    case 'slides-univer':
      return 'slides' as const
    case 'sheet-csv':
    case 'sheet-univer':
      return 'sheet' as const
    default:
      return null
  }
})
const isEditableKind = computed(() => editableKind.value != null)
/** Kinds that support view/edit chrome + save. */
const canEditSave = computed(() => {
  if (isMsOffice.value) return false
  return isEditableKind.value || isCodeKind.value
})

const activeSurface = computed(() => {
  switch (surfaceKey.value) {
    case 'doc-md':
    case 'doc-univer':
      return docRef
    case 'slides-univer':
      return slidesRef
    case 'sheet-csv':
    case 'sheet-univer':
      return sheetRef
    case 'code':
      return codeRef
    default:
      return null
  }
})

const chromeLabel = computed(() => {
  const target = resolved.value
  if (!target) return ''
  if (target.kind === 'browser-live') return t('sessions.browserLive.title')
  if (target.kind === 'desktop') return t('sessions.desktop.title')
  if (target.kind === 'web' || target.kind === 'media') return target.path || target.url || ''
  return target.path
})

function getSelectionMarkdown(): string {
  if (stage.value?.kind === 'code' || stage.value?.kind === 'diff') return ''
  const surface = activeSurface.value?.value as { getSelectionMarkdown?: () => string } | null | undefined
  return surface?.getSelectionMarkdown?.() || ''
}

function getSelectionLines(): { startLine: number; endLine: number } | null {
  if (stage.value?.kind !== 'doc') return null
  const surface = docRef.value as { getSelectionLines?: () => { startLine: number; endLine: number } | null } | null
  return surface?.getSelectionLines?.() ?? null
}

function getEditScope(): FileEditScope {
  return editScope.value
}

async function onDocAiEdit(payload: {
  action: 'polish' | 'continue' | 'modify'
  instruction: string
  selection: string
  startLine?: number
  endLine?: number
}) {
  if (!stage.value || stage.value.kind !== 'doc') return
  editScope.value = 'selection'
  const saved = await ensureSaved()
  if (!saved) return
  const att = createOfficeEditAttachment({
    action: payload.action,
    path: stage.value.path,
    officeKind: 'doc',
    scope: 'selection',
    selection: payload.selection,
    instruction: payload.instruction,
    startLine: payload.startLine,
    endLine: payload.endLine,
    engine: stage.value.engine,
  })
  emit('attachOfficeEdit', att)
  toast.success(t('office.officeAttached'))
}

function onAttachOfficeEdit(att: OfficeEditAttachment) {
  emit('attachOfficeEdit', att)
}

async function save(opts?: { quiet?: boolean }) {
  const surface = activeSurface.value?.value
  await surface?.save?.(opts)
}

const exportingPdf = ref(false)

async function exportPdf() {
  if (!stage.value || stage.value.kind !== 'doc' || exportingPdf.value) return
  exportingPdf.value = true
  try {
    if (dirty.value) {
      await save({ quiet: true })
    }
    const md =
      (docRef.value as { getMarkdown?: () => string } | null)?.getMarkdown?.() || ''
    const result = await exportMarkdownPdf(md, stage.value.path)
    if (!result.ok) return // user cancelled save dialog
    if (result.method === 'download') {
      toast.success(t('office.exportPdfDownloaded'))
    } else if (result.path) {
      toast.success(t('office.exportPdfSaved', { path: result.path }))
    } else {
      toast.success(t('office.exportPdfOk'))
    }
  } catch (e) {
    toast.error(e instanceof Error ? e.message : t('office.exportPdfFailed'))
  } finally {
    exportingPdf.value = false
  }
}

/** Flush dirty edits to disk before AI reads the file. Auto-saves; confirms if save fails. */
async function ensureSaved(): Promise<boolean> {
  const surface = activeSurface.value?.value
  if (!surface || !dirty.value) return true
  try {
    await surface.save?.({ quiet: true })
    if (dirty.value) throw new Error('still dirty')
    toast.success(t('office.autoSaved'))
    return true
  } catch {
    try {
      await confirm(t('office.dirtyConfirmBody'), t('office.dirtyConfirmTitle'), {
        type: 'warning',
        confirmButtonText: t('office.save'),
      })
      await surface.save?.()
      return !dirty.value
    } catch {
      return false
    }
  }
}

watch(
  () => stage.value?.kind,
  (kind) => {
    if (kind === 'slides') editScope.value = 'slide'
    else if (kind === 'sheet') editScope.value = 'sheet'
    else editScope.value = 'document'
    if (kind === 'web' || kind === 'media' || kind === 'diff') dirty.value = false
  },
  { immediate: true },
)

function close() {
  workspaceUi.closeStage()
}

const uriInput = ref('')
/** Keep the address bar readable; percent-encoding stays in the store's canonical URI. */
function prettyUri(uri: string): string {
  try { return decodeURI(uri) } catch { return uri }
}
watch(() => workspaceUi.stageResourceUri, (uri) => { uriInput.value = prettyUri(uri) }, { immediate: true })

/**
 * Routing happens here, from the URI — not from scattered per-kind flags.
 *
 *   vnc://…        desktop
 *   browser://…    agent browser
 *   https://…      web preview (iframe)
 *   file:///…?kind=&engine=…  doc / slides / sheet / code / diff / vue-sfc
 *
 * The committed store stage wins when the typed text is not a valid address, so
 * a half-typed URI never blanks the surface.
 */
type SurfaceKey =
  | 'none'
  | 'desktop'
  | 'browser-live'
  | 'web'
  | 'vue-sfc'
  | 'diff'
  | 'code'
  | 'doc-md'
  | 'doc-univer'
  | 'slides-univer'
  | 'sheet-csv'
  | 'sheet-univer'
  | 'ms-office'

const resolved = computed(() => stageFromUri(workspaceUi.stageResourceUri) ?? stage.value)

const surfaceKey = computed<SurfaceKey>(() => {
  const target = resolved.value
  if (!target) return 'none'
  const engine = target.engine ?? stage.value?.engine
  switch (target.kind) {
    case 'desktop': return 'desktop'
    case 'browser-live': return 'browser-live'
    case 'diff': return 'diff'
    case 'code': return 'code'
    case 'web':
    case 'media':
      return engine === 'vue-sfc' ? 'vue-sfc' : 'web'
    case 'doc':
      if (engine === 'ms-office') return 'ms-office'
      return engine === 'univer-doc' ? 'doc-univer' : 'doc-md'
    case 'slides':
      return engine === 'ms-office' ? 'ms-office' : 'slides-univer'
    case 'sheet':
      if (engine === 'ms-office') return 'ms-office'
      return engine === 'univer-sheet' ? 'sheet-univer' : 'sheet-csv'
    default: return 'none'
  }
})

/** View/edit + Save only apply to editable document surfaces. */
const showEditControls = computed(() =>
  surfaceKey.value === 'doc-md' ||
  surfaceKey.value === 'doc-univer' ||
  surfaceKey.value === 'slides-univer' ||
  surfaceKey.value === 'sheet-csv' ||
  surfaceKey.value === 'sheet-univer' ||
  surfaceKey.value === 'code',
)
const showExportPdf = computed(() => surfaceKey.value === 'doc-md' || surfaceKey.value === 'doc-univer')

/** Web preview controls live in this header, so this stage tracks their state. */
const webRef = ref<InstanceType<typeof WebSurface> | null>(null)
const webToolbar = ref({ selecting: false, drawing: false, consoleCount: 0 })

/** Agent-browser status is mirrored here for the same reason. */
const browserRef = ref<InstanceType<typeof BrowserLiveSurface> | null>(null)
const browserState = ref({ status: '', label: '', pageTitle: '', pageUrl: '' })

function toggleElementSelect() {
  if (webToolbar.value.selecting && !webToolbar.value.drawing) webRef.value?.stopElementSelect()
  else webRef.value?.startElementSelect()
}
function toggleDrawRegion() {
  if (webToolbar.value.drawing) webRef.value?.stopElementSelect()
  else webRef.value?.startDrawRegion()
}

const uriPlaceholder = computed(() => {
  switch (resolved.value?.kind) {
    case 'browser-live': return 'browser://<sessionId>'
    case 'desktop': return 'vnc://<projectId>'
    case 'web': return resolved.value.engine === 'vue-sfc' ? 'file://<path>.vue?kind=web&engine=vue-sfc' : 'https://example.com/'
    default: return 'file://<path>?kind=doc&mode=view'
  }
})

function navigateUri() {
  if (!workspaceUi.openStageUri(uriInput.value)) return
  void nextTick(() => { uriInput.value = prettyUri(workspaceUi.stageResourceUri) })
}

async function setMode(mode: 'view' | 'edit' | 'present') {
  if (!stage.value) return
  if (isMsOffice.value && mode === 'edit') {
    toast.warning(t('office.msConvertBeforeEdit'))
    return
  }
  workspaceUi.setStageMode(mode)
}

/**
 * Only a real external address belongs in the address bar. `loadUrl` is the
 * transport URL WebSurface actually puts in the iframe (backend proxy or
 * `/projects/:id/raw/…`); storing it would leak an implementation detail and
 * make the bar read differently from every other surface.
 */
function onPreviewUrlChange(display: string, _loadUrl: string) {
  const next = display.trim()
  if (!next) {
    workspaceUi.setStageUrl(undefined)
    return
  }
  if (!/^https?:\/\//i.test(next)) return // project file: keep its file:// URI
  workspaceUi.setStageUrl(next)
}

watch(turnRunning, async (running, was) => {
  if (!(was && !running)) return
  if (!isVisualStage.value) workspaceUi.requestStageReload()
  if (aiReviewAfterTurn.value === 'off') return
  const path = stage.value?.path
  const sid = sessions.currentSessionId
  if (!path || !sid || isVisualStage.value || stage.value?.kind === 'diff' || stage.value?.kind === 'web' || stage.value?.kind === 'media') return
  // Prefer the turn that just finished (was running).
  const finished =
    sessions.turns.find((x) => x.status !== 'running' && x.id === lastFinishedTurnId.value) ||
    sessions.turns.find((x) => x.status === 'completed' || x.status === 'failed') ||
    sessions.turns[0]
  const turnId = finished?.id
  if (!turnId) return
  try {
    const st = await fetchAiReviewStatus(sid, turnId, path)
    if (st.changed && st.hasSnapshot) {
      aiReview.setPending({
        sessionId: sid,
        turnId,
        path,
        canRevert: st.canRevert,
      })
    } else {
      aiReview.clearPending()
    }
  } catch {
    /* snapshot may be absent for non-office turns */
  }
})

watch(
  () => sessions.runningTurnId,
  (id, prev) => {
    if (prev && !id) lastFinishedTurnId.value = prev
    if (id) lastFinishedTurnId.value = id
  },
)

function viewAiDiff() {
  const p = pendingAiReview.value
  if (!p) return
  workspaceUi.openStage({
    kind: 'diff',
    path: p.path,
    mode: 'view',
    engine: 'diff',
    diffSource: 'ai',
    sessionId: p.sessionId,
    turnId: p.turnId,
  })
}

async function keepAiChange() {
  aiReview.clearPending()
}

async function revertAiChange() {
  const p = pendingAiReview.value
  if (!p || bannerBusy.value) return
  bannerBusy.value = true
  try {
    await revertAiReviewFile(p.sessionId, p.turnId, p.path)
    aiReview.clearPending()
    toast.success(t('office.aiReviewReverted'))
    workspaceUi.requestStageReload()
  } catch (e) {
    toast.error(e instanceof Error ? e.message : t('office.aiReviewRevertFailed'))
  } finally {
    bannerBusy.value = false
  }
}

const showAiBanner = computed(() => {
  const p = pendingAiReview.value
  if (!p || aiReviewAfterTurn.value === 'off') return false
  if (stage.value?.kind === 'diff') return false
  return stage.value?.path === p.path
})

defineExpose({ save })
</script>

<template>
  <section v-if="stage" class="file-stage" :class="{ 'is-immersive': layoutMode === 'immersive' }">
    <header class="file-stage__chrome">
      <input
        v-model="uriInput"
        class="file-stage__uri"
        :title="chromeLabel"
        :placeholder="uriPlaceholder"
        :aria-label="t('sessions.visualStage.uriLabel')"
        spellcheck="false"
        @keydown.enter.prevent="navigateUri"
      >
      <span v-if="dirty && canEditSave" class="file-stage__dirty">●</span>
      <span v-if="turnRunning && isEditableKind" class="file-stage__running">{{ t('office.aiRunning') }}</span>
      <!-- Surface-specific controls sit in this same row for every renderer. -->
      <div v-if="surfaceKey === 'web'" class="file-stage__surface-tools">
        <button class="file-stage__icon-btn" :title="t('office.previewRefresh')" @click="webRef?.refresh()">
          <svg viewBox="0 0 24 24" width="14" height="14" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><polyline points="23 4 23 10 17 10"/><path d="M20.49 15a9 9 0 1 1-2.12-9.36L23 10"/></svg>
        </button>
        <button
          class="file-stage__icon-btn"
          :class="{ 'is-active': webToolbar.selecting && !webToolbar.drawing }"
          :title="webToolbar.selecting && !webToolbar.drawing ? t('sessions.designModeOn') : t('sessions.designModeOff')"
          @click="toggleElementSelect"
        >
          <svg viewBox="0 0 24 24" width="14" height="14" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><circle cx="12" cy="12" r="10"/><line x1="22" y1="12" x2="18" y2="12"/><line x1="6" y1="12" x2="2" y2="12"/><line x1="12" y1="6" x2="12" y2="2"/><line x1="12" y1="22" x2="12" y2="18"/></svg>
        </button>
        <button
          class="file-stage__icon-btn"
          :class="{ 'is-active': webToolbar.drawing }"
          :title="t('sessions.designModeDraw')"
          @click="toggleDrawRegion"
        >
          <svg viewBox="0 0 24 24" width="14" height="14" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M12 19l7-7 3 3-7 7-3-3z"/><path d="M18 13l-1.5-7.5L2 2l3.5 14.5L13 18l5-5z"/><path d="M2 2l7.586 7.586"/></svg>
        </button>
        <button
          class="file-stage__icon-btn file-stage__icon-btn--badged"
          :title="t('sessions.designModeConsole')"
          @click="webRef?.requestConsoleDump()"
        >
          <svg viewBox="0 0 24 24" width="14" height="14" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><polyline points="4 17 10 11 4 5"/><line x1="12" y1="19" x2="20" y2="19"/></svg>
          <span v-if="webToolbar.consoleCount" class="file-stage__icon-badge">{{ webToolbar.consoleCount > 9 ? '9+' : webToolbar.consoleCount }}</span>
        </button>
        <button class="file-stage__icon-btn" :title="t('office.previewClear')" @click="webRef?.clearPreview()">
          <svg viewBox="0 0 24 24" width="14" height="14" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><line x1="18" y1="6" x2="6" y2="18"/><line x1="6" y1="6" x2="18" y2="18"/></svg>
        </button>
      </div>
      <div v-else-if="surfaceKey === 'browser-live'" class="file-stage__surface-status">
        <span class="file-stage__status-dot" :class="`is-${browserState.status}`" />
        <span class="file-stage__readonly">{{ t('sessions.browserLive.readonly') }}</span>
        <span class="file-stage__page" :title="browserState.pageUrl">
          {{ browserState.pageTitle || browserState.pageUrl || t('sessions.browserLive.noPage') }}
        </span>
        <span class="file-stage__status-label">{{ browserState.label }}</span>
      </div>
      <div class="file-stage__actions">
        <div v-if="showEditControls" class="file-stage__modes">
          <button
            class="file-stage__chip"
            :class="{ 'is-active': stage.mode === 'view' }"
            @click="setMode('view')"
          >
            {{ t('office.view') }}
          </button>
          <button
            class="file-stage__chip"
            :class="{ 'is-active': stage.mode === 'edit' }"
            @click="setMode('edit')"
          >
            {{ t('office.edit') }}
          </button>
          <button
            v-if="surfaceKey === 'slides-univer'"
            class="file-stage__chip"
            :class="{ 'is-active': stage.mode === 'present' }"
            @click="setMode('present')"
          >
            {{ t('office.present') }}
          </button>
        </div>
        <button
          v-if="showEditControls"
          class="file-stage__btn"
          :disabled="!dirty || turnRunning"
          @click="() => save()"
        >
          {{ t('office.save') }}
        </button>
        <button
          v-if="showExportPdf"
          class="file-stage__btn"
          :disabled="exportingPdf || turnRunning"
          @click="exportPdf"
        >
          {{ t('office.exportPdf') }}
        </button>
        <button
          class="file-stage__btn"
          @click="workspaceUi.layoutMode = layoutMode === 'immersive' ? 'stage' : 'immersive'"
        >
          {{ layoutMode === 'immersive' ? t('office.exitImmersive') : t('office.immersive') }}
        </button>
        <button class="file-stage__btn" @click="close">{{ t('office.close') }}</button>
      </div>
    </header>

    <div v-if="showAiBanner" class="file-stage__ai-banner" role="status">
      <span class="file-stage__ai-banner-text">{{ t('office.aiReviewBanner') }}</span>
      <span class="file-stage__ai-banner-actions">
        <button type="button" class="file-stage__btn" @click="viewAiDiff">{{ t('office.aiReviewViewDiff') }}</button>
        <button type="button" class="file-stage__btn" :disabled="bannerBusy" @click="keepAiChange">
          {{ t('office.aiReviewKeep') }}
        </button>
        <button
          type="button"
          class="file-stage__btn"
          :disabled="bannerBusy || !pendingAiReview?.canRevert"
          @click="revertAiChange"
        >
          {{ t('office.aiReviewRevert') }}
        </button>
      </span>
    </div>

    <!-- Slides/Sheet: attach polish/modify chips to Composer (doc uses selection bubble). -->
    <div v-if="editableKind && editableKind !== 'doc' && stage.mode === 'edit'" class="file-stage__ai">
      <OfficeAiToolbar
        ref="aiToolbarRef"
        :path="stage.path"
        :kind="editableKind"
        :engine="stageEngine"
        :page-index="editableKind === 'slides' ? pageIndex : undefined"
        :get-selection-markdown="getSelectionMarkdown"
        :get-selection-lines="getSelectionLines"
        :get-edit-scope="getEditScope"
        :ensure-saved="ensureSaved"
        :scope="editScope"
        @attach-office-edit="onAttachOfficeEdit"
      />
    </div>

    <BrowserLiveSurface
      v-if="surfaceKey === 'browser-live' && stage.sessionId"
      ref="browserRef"
      :key="stage.sessionId"
      :session-id="stage.sessionId"
      :active="true"
      @status-state="browserState = $event"
    />
    <DesktopPanel
      v-else-if="surfaceKey === 'desktop'"
      :project-id="projectId"
      :active="true"
    />
    <VueSfcSurface
      v-else-if="surfaceKey === 'vue-sfc'"
      :project-id="projectId"
      :path="stage.path"
      :reload-token="stageReloadToken"
    />
    <DocSurface
      v-else-if="surfaceKey === 'doc-md'"
      ref="docRef"
      :project-id="projectId"
      :path="stage.path"
      :mode="stage.mode"
      :reload-token="stageReloadToken"
      :turn-running="turnRunning"
      @dirty="dirty = $event"
      @scope="editScope = $event"
      @ai-edit="onDocAiEdit"
    />
    <UniverDocSurface
      v-else-if="surfaceKey === 'doc-univer'"
      ref="docRef"
      :project-id="projectId"
      :path="stage.path"
      :mode="stage.mode"
      :reload-token="stageReloadToken"
      :turn-running="turnRunning"
      @dirty="dirty = $event"
      @scope="editScope = $event"
    />
    <UniverSlidesSurface
      v-else-if="surfaceKey === 'slides-univer'"
      ref="slidesRef"
      :project-id="projectId"
      :path="stage.path"
      :mode="stage.mode"
      :reload-token="stageReloadToken"
      :turn-running="turnRunning"
      @dirty="dirty = $event"
      @update-page-index="pageIndex = $event"
    />
    <SheetSurface
      v-else-if="surfaceKey === 'sheet-csv'"
      ref="sheetRef"
      :project-id="projectId"
      :path="stage.path"
      :mode="stage.mode"
      :reload-token="stageReloadToken"
      :turn-running="turnRunning"
      @dirty="dirty = $event"
    />
    <UniverSheetSurface
      v-else-if="surfaceKey === 'sheet-univer'"
      ref="sheetRef"
      :project-id="projectId"
      :path="stage.path"
      :mode="stage.mode"
      :reload-token="stageReloadToken"
      :turn-running="turnRunning"
      @dirty="dirty = $event"
    />
    <MsOfficePreviewSurface
      v-else-if="surfaceKey === 'ms-office'"
      :project-id="projectId"
      :path="stage.path"
      :mode="stage.mode"
      :reload-token="stageReloadToken"
    />
    <CodeSurface
      v-else-if="surfaceKey === 'code'"
      ref="codeRef"
      :project-id="projectId"
      :path="stage.path"
      :mode="stage.mode"
      :reload-token="stageReloadToken"
      :turn-running="turnRunning"
      @dirty="dirty = $event"
      @attach-code-selection="emit('attachCodeSelection', $event)"
    />
    <DiffSurface
      v-else-if="surfaceKey === 'diff'"
      :project-id="projectId"
      :path="stage.path"
      :staged="stage.staged"
      :reload-token="stageReloadToken"
      :source="stage.diffSource || 'git'"
      :session-id="stage.sessionId"
      :turn-id="stage.turnId"
    />
    <WebSurface
      v-else-if="surfaceKey === 'web'"
      ref="webRef"
      :project-id="projectId"
      :path="stage.path"
      :url="stage.url"
      :reload-token="stageReloadToken"
      @url-change="onPreviewUrlChange"
      @toolbar-state="webToolbar = $event"
      @attach-element="emit('attachElement', $event)"
      @attach-console="emit('attachConsole', $event)"
    />
  </section>
</template>

<style scoped>
.file-stage {
  display: flex;
  flex-direction: column;
  min-width: 0;
  min-height: 0;
  height: 100%;
  background: var(--dq-bg-base);
  color: var(--dq-label-primary);
  border-left: 1px solid var(--dq-separator-light);
  border-right: 1px solid var(--dq-separator-light);
}
.file-stage.is-immersive {
  border: 0;
}
.file-stage__ai-banner {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 10px;
  flex-wrap: wrap;
  padding: 8px 12px;
  border-bottom: 1px solid var(--dq-separator-light);
  background: color-mix(in srgb, var(--dq-accent) 12%, var(--dq-bg-elevated));
  font-size: var(--dq-font-size-caption);
}
.file-stage__ai-banner-text {
  color: var(--dq-label-primary);
  font-weight: 550;
}
.file-stage__ai-banner-actions {
  display: flex;
  gap: 6px;
  flex-wrap: wrap;
}
.file-stage__chrome {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
  padding: 6px 10px;
  border-bottom: 1px solid var(--dq-separator-light);
  background: color-mix(in srgb, var(--dq-bg-elevated) 55%, transparent);
  flex-shrink: 0;
}
.file-stage__uri {
  flex: 1;
  min-width: 0;
  height: 26px;
  padding: 0 8px;
  border: 1px solid var(--dq-border);
  border-radius: 6px;
  background: var(--dq-fill-tertiary);
  color: var(--dq-label-primary);
  font: var(--dq-font-size-caption) var(--dq-font-mono);
  outline: none;
}
.file-stage__uri::placeholder {
  color: var(--dq-label-quaternary);
}
.file-stage__uri:focus {
  border-color: var(--dq-accent);
}
.file-stage__dirty {
  color: var(--dq-system-orange);
  font-size: var(--dq-font-size-caption);
}
.file-stage__running {
  font-size: var(--dq-font-size-caption);
  color: var(--dq-accent);
}
/* Surface-specific controls rendered inline in the chrome row. */
.file-stage__surface-tools {
  display: inline-flex;
  align-items: center;
  gap: 2px;
  flex-shrink: 0;
}
.file-stage__icon-btn {
  position: relative;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 26px;
  height: 26px;
  border: none;
  border-radius: 6px;
  background: transparent;
  color: var(--dq-label-secondary);
  cursor: pointer;
}
.file-stage__icon-btn:hover {
  background: var(--dq-fill-tertiary);
  color: var(--dq-label-primary);
}
.file-stage__icon-btn.is-active {
  background: var(--dq-accent);
  color: var(--dq-on-accent);
}
.file-stage__icon-badge {
  position: absolute;
  top: -2px;
  right: -2px;
  min-width: 13px;
  height: 13px;
  padding: 0 3px;
  border-radius: 7px;
  background: var(--dq-system-orange, #f59e0b);
  color: #fff;
  font-size: 9px;
  line-height: 13px;
  text-align: center;
}
.file-stage__surface-status {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  min-width: 0;
  flex-shrink: 1;
  font-size: var(--dq-font-size-caption);
  color: var(--dq-label-tertiary);
}
.file-stage__status-dot {
  width: 7px;
  height: 7px;
  flex-shrink: 0;
  border-radius: 50%;
  background: var(--dq-label-quaternary);
}
.file-stage__status-dot.is-live { background: var(--dq-success); }
.file-stage__status-dot.is-error { background: var(--dq-danger); }
.file-stage__readonly {
  flex-shrink: 0;
  padding: 1px 6px;
  border-radius: 4px;
  color: var(--dq-accent);
  background: var(--dq-accent-tint);
}
.file-stage__page {
  min-width: 0;
  overflow: hidden;
  font-family: var(--dq-font-mono);
  text-overflow: ellipsis;
  white-space: nowrap;
}
.file-stage__status-label {
  flex-shrink: 0;
}
.file-stage__actions {
  display: flex;
  align-items: center;
  gap: 6px;
  flex-wrap: wrap;
}
.file-stage__modes {
  display: flex;
  gap: 2px;
  background: var(--dq-fill-tertiary);
  border-radius: 6px;
  padding: 2px;
}
.file-stage__chip {
  border: 0;
  background: transparent;
  height: 26px;
  padding: 0 8px;
  border-radius: 4px;
  font-size: var(--dq-font-size-caption);
  color: var(--dq-label-secondary);
  cursor: pointer;
}
.file-stage__chip:hover {
  color: var(--dq-label-primary);
}
.file-stage__chip.is-active {
  background: var(--dq-bg-elevated);
  color: var(--dq-label-primary);
  box-shadow: 0 1px 2px color-mix(in srgb, var(--dq-mask) 12%, transparent);
}
.file-stage__btn {
  height: 28px;
  padding: 0 10px;
  border: 1px solid var(--dq-border);
  border-radius: 6px;
  background: var(--dq-fill-tertiary);
  color: var(--dq-label-primary);
  font-size: var(--dq-font-size-caption);
  cursor: pointer;
}
.file-stage__btn:hover:not(:disabled) {
  background: color-mix(in srgb, var(--dq-label-primary) 8%, var(--dq-fill-tertiary));
}
.file-stage__btn:disabled {
  opacity: 0.5;
}
.file-stage__ai {
  padding: 6px 10px;
  border-bottom: 1px solid var(--dq-separator-light);
  background: color-mix(in srgb, var(--dq-bg-elevated) 35%, transparent);
}
</style>
