<script setup lang="ts">
import { computed, ref, watch } from 'vue'
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
import CodeSurface from '@/components/office/CodeSurface.vue'
import DiffSurface from '@/components/office/DiffSurface.vue'
import OfficeAiToolbar from '@/components/office/OfficeAiToolbar.vue'
import { confirm, toast } from '@/utils/feedback'
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

const isDiff = computed(() => stage.value?.kind === 'diff')
const isCodeKind = computed(() => stage.value?.kind === 'code')
const stageEngine = computed(() => stage.value?.engine)
const isMsOffice = computed(() => stageEngine.value === 'ms-office')
/** Office AI toolbar kinds (not code/diff/ms-office). */
const editableKind = computed(() => {
  if (isMsOffice.value) return null
  const k = stage.value?.kind
  if (k === 'doc' || k === 'slides' || k === 'sheet') return k
  return null
})
const isEditableKind = computed(() => editableKind.value != null)
/** Kinds that support view/edit chrome + save. */
const canEditSave = computed(() => {
  if (isMsOffice.value) return false
  return isEditableKind.value || isCodeKind.value
})

const activeSurface = computed(() => {
  if (!stage.value) return null
  if (stage.value.kind === 'doc') return docRef
  if (stage.value.kind === 'slides') return slidesRef
  if (stage.value.kind === 'sheet') return sheetRef
  if (stage.value.kind === 'code') return codeRef
  return null
})

const chromeLabel = computed(() => {
  if (!stage.value) return ''
  if (stage.value.kind === 'web' || stage.value.kind === 'media') {
    return stage.value.path || stage.value.url || ''
  }
  return stage.value.path
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

async function setMode(mode: 'view' | 'edit' | 'present') {
  if (!stage.value) return
  if (isMsOffice.value && mode === 'edit') {
    toast.warning(t('office.msConvertBeforeEdit'))
    return
  }
  workspaceUi.setStageMode(mode)
}

function onPreviewUrlChange(_display: string, loadUrl: string) {
  workspaceUi.setStageUrl(loadUrl || undefined)
}

watch(turnRunning, async (running, was) => {
  if (!(was && !running)) return
  workspaceUi.requestStageReload()
  if (aiReviewAfterTurn.value === 'off') return
  const path = stage.value?.path
  const sid = sessions.currentSessionId
  if (!path || !sid || stage.value?.kind === 'diff' || stage.value?.kind === 'web' || stage.value?.kind === 'media') return
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
      <div class="file-stage__meta">
        <span class="file-stage__badge">{{ stage.kind }}</span>
        <span class="file-stage__path" :title="chromeLabel">{{ chromeLabel }}</span>
        <span v-if="dirty && canEditSave" class="file-stage__dirty">●</span>
        <span v-if="turnRunning && isEditableKind" class="file-stage__running">{{ t('office.aiRunning') }}</span>
      </div>
      <div class="file-stage__actions">
        <div v-if="canEditSave" class="file-stage__modes">
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
            v-if="stage.kind === 'slides' && stageEngine === 'univer-slides'"
            class="file-stage__chip"
            :class="{ 'is-active': stage.mode === 'present' }"
            @click="setMode('present')"
          >
            {{ t('office.present') }}
          </button>
        </div>
        <button
          v-if="canEditSave"
          class="file-stage__btn"
          :disabled="!dirty || turnRunning"
          @click="() => save()"
        >
          {{ t('office.save') }}
        </button>
        <button
          v-if="stage.kind === 'doc'"
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

    <DocSurface
      v-if="stage.kind === 'doc' && stageEngine === 'md'"
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
      v-else-if="stage.kind === 'doc' && stageEngine === 'univer-doc'"
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
      v-else-if="stage.kind === 'slides' && stageEngine === 'univer-slides'"
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
      v-else-if="stage.kind === 'sheet' && stageEngine === 'csv'"
      ref="sheetRef"
      :project-id="projectId"
      :path="stage.path"
      :mode="stage.mode"
      :reload-token="stageReloadToken"
      :turn-running="turnRunning"
      @dirty="dirty = $event"
    />
    <UniverSheetSurface
      v-else-if="stage.kind === 'sheet' && stageEngine === 'univer-sheet'"
      ref="sheetRef"
      :project-id="projectId"
      :path="stage.path"
      :mode="stage.mode"
      :reload-token="stageReloadToken"
      :turn-running="turnRunning"
      @dirty="dirty = $event"
    />
    <MsOfficePreviewSurface
      v-else-if="stageEngine === 'ms-office'"
      :project-id="projectId"
      :path="stage.path"
      :mode="stage.mode"
      :reload-token="stageReloadToken"
    />
    <CodeSurface
      v-else-if="stage.kind === 'code'"
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
      v-else-if="stage.kind === 'diff'"
      :project-id="projectId"
      :path="stage.path"
      :staged="stage.staged"
      :reload-token="stageReloadToken"
      :source="stage.diffSource || 'git'"
      :session-id="stage.sessionId"
      :turn-id="stage.turnId"
    />
    <WebSurface
      v-else-if="stage.kind === 'web' || stage.kind === 'media'"
      :project-id="projectId"
      :path="stage.path"
      :url="stage.url"
      :reload-token="stageReloadToken"
      @url-change="onPreviewUrlChange"
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
  padding: 8px 10px;
  border-bottom: 1px solid var(--dq-separator-light);
  background: color-mix(in srgb, var(--dq-bg-elevated) 55%, transparent);
  flex-wrap: wrap;
}
.file-stage__meta {
  display: flex;
  align-items: center;
  gap: 8px;
  min-width: 0;
}
.file-stage__badge {
  font-size: var(--dq-font-size-caption);
  font-weight: 600;
  text-transform: uppercase;
  letter-spacing: 0.04em;
  padding: 2px 6px;
  border-radius: 4px;
  background: var(--dq-accent-tint);
  color: var(--dq-accent);
  flex-shrink: 0;
}
.file-stage__path {
  font-size: var(--dq-font-size-caption);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  color: var(--dq-label-tertiary);
}
.file-stage__dirty {
  color: var(--dq-system-orange);
  font-size: var(--dq-font-size-caption);
}
.file-stage__running {
  font-size: var(--dq-font-size-caption);
  color: var(--dq-accent);
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
