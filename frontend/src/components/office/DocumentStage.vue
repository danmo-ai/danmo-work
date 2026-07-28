<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { storeToRefs } from 'pinia'
import { useI18n } from 'vue-i18n'
import { useWorkspaceUiStore } from '@/stores/workspaceUi'
import { useSessionsStore } from '@/stores/sessions'
import DocSurface from '@/components/office/DocSurface.vue'
import SlidesSurface from '@/components/office/SlidesSurface.vue'
import SheetSurface from '@/components/office/SheetSurface.vue'
import PreviewSurface from '@/components/office/PreviewSurface.vue'
import CodeSurface from '@/components/office/CodeSurface.vue'
import DiffSurface from '@/components/office/DiffSurface.vue'
import OfficeAiToolbar from '@/components/office/OfficeAiToolbar.vue'
import { confirm, toast } from '@/utils/feedback'
import type { OfficeEditScope } from '@/utils/office-route'
import { siblingSlidesMarkdownPath } from '@/utils/slides-render'
import type { ElementAttachment } from '@/types/element-attachment'
import type { CodeSelectionAttachment } from '@/types/code-attachment'

const props = defineProps<{
  projectId: string
}>()

const emit = defineEmits<{
  attachElement: [att: ElementAttachment]
  attachCodeSelection: [att: CodeSelectionAttachment]
}>()

const { t } = useI18n()
const workspaceUi = useWorkspaceUiStore()
const sessions = useSessionsStore()
const { stage, stageReloadToken, layoutMode } = storeToRefs(workspaceUi)

const docRef = ref<InstanceType<typeof DocSurface> | null>(null)
const slidesRef = ref<InstanceType<typeof SlidesSurface> | null>(null)
const sheetRef = ref<InstanceType<typeof SheetSurface> | null>(null)
const codeRef = ref<InstanceType<typeof CodeSurface> | null>(null)
const dirty = ref(false)
const pageIndex = ref(0)
const editScope = ref<OfficeEditScope>('document')

const turnRunning = computed(() => {
  const id = sessions.runningTurnId
  if (!id) return false
  const turn = sessions.turns.find((x) => x.id === id)
  return turn?.status === 'running'
})

const isPreview = computed(() => stage.value?.kind === 'preview')
const isDiff = computed(() => stage.value?.kind === 'diff')
const isCodeKind = computed(() => stage.value?.kind === 'code')
/** Office AI toolbar kinds (not code/diff). */
const editableKind = computed(() => {
  const k = stage.value?.kind
  if (k === 'doc' || k === 'slides' || k === 'sheet') return k
  return null
})
const isEditableKind = computed(() => editableKind.value != null)
/** Kinds that support view/edit chrome + save. */
const canEditSave = computed(() => isEditableKind.value || isCodeKind.value)

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
  if (stage.value.kind === 'preview') {
    return stage.value.path || stage.value.url || ''
  }
  return stage.value.path
})

function getSelectionMarkdown(): string {
  const surface = activeSurface.value?.value
  return surface?.getSelectionMarkdown?.() || ''
}

function getEditScope(): OfficeEditScope {
  return editScope.value
}

async function save(opts?: { quiet?: boolean }) {
  const surface = activeSurface.value?.value
  await surface?.save?.(opts)
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
    if (kind === 'preview' || kind === 'diff') dirty.value = false
  },
  { immediate: true },
)

function close() {
  workspaceUi.closeStage()
}

async function setMode(mode: 'view' | 'edit' | 'present') {
  if (!stage.value) return

  // Slides Present: programmatically sync md → sibling html (no AI turn).
  if (mode === 'present' && stage.value.kind === 'slides') {
    if (/\.md$|\.markdown$/i.test(stage.value.path)) {
      const saved = await ensureSaved()
      if (!saved) return
      await slidesRef.value?.presentFromMarkdown?.()
      return
    }
    workspaceUi.setStageMode('present')
    return
  }

  // From playable HTML back to Markdown source for edit/view.
  if (
    (mode === 'edit' || mode === 'view') &&
    stage.value.kind === 'slides' &&
    /\.html?$/i.test(stage.value.path)
  ) {
    const mdPath = siblingSlidesMarkdownPath(stage.value.path)
    workspaceUi.setStagePath(mdPath, mode)
    return
  }

  workspaceUi.setStageMode(mode)
}

function setKind(kind: 'doc' | 'slides' | 'sheet') {
  workspaceUi.setStageKind(kind)
}

function onPreviewUrlChange(_display: string, loadUrl: string) {
  workspaceUi.setStageUrl(loadUrl || undefined)
}

watch(turnRunning, (running, was) => {
  if (was && !running) {
    workspaceUi.requestStageReload()
  }
})

defineExpose({ save })
</script>

<template>
  <section v-if="stage" class="document-stage" :class="{ 'is-immersive': layoutMode === 'immersive' }">
    <header class="document-stage__chrome">
      <div class="document-stage__meta">
        <span class="document-stage__badge">{{ stage.kind }}</span>
        <span class="document-stage__path" :title="chromeLabel">{{ chromeLabel }}</span>
        <span v-if="dirty && canEditSave" class="document-stage__dirty">●</span>
        <span v-if="turnRunning && isEditableKind" class="document-stage__running">{{ t('office.aiRunning') }}</span>
      </div>
      <div class="document-stage__actions">
        <div v-if="canEditSave" class="document-stage__modes">
          <button
            class="document-stage__chip"
            :class="{ 'is-active': stage.mode === 'view' }"
            @click="setMode('view')"
          >
            {{ t('office.view') }}
          </button>
          <button
            class="document-stage__chip"
            :class="{ 'is-active': stage.mode === 'edit' }"
            @click="setMode('edit')"
          >
            {{ t('office.edit') }}
          </button>
          <button
            v-if="stage.kind === 'slides'"
            class="document-stage__chip"
            :class="{ 'is-active': stage.mode === 'present' }"
            @click="setMode('present')"
          >
            {{ t('office.present') }}
          </button>
        </div>
        <div v-if="stage.path.match(/\.md$/i) && !isPreview && !isDiff" class="document-stage__modes">
          <button
            class="document-stage__chip"
            :class="{ 'is-active': stage.kind === 'doc' }"
            @click="setKind('doc')"
          >
            Doc
          </button>
          <button
            class="document-stage__chip"
            :class="{ 'is-active': stage.kind === 'slides' }"
            @click="setKind('slides')"
          >
            Slides
          </button>
        </div>
        <button
          v-if="canEditSave"
          class="document-stage__btn"
          :disabled="!dirty || turnRunning"
          @click="() => save()"
        >
          {{ t('office.save') }}
        </button>
        <button
          class="document-stage__btn"
          @click="workspaceUi.layoutMode = layoutMode === 'immersive' ? 'stage' : 'immersive'"
        >
          {{ layoutMode === 'immersive' ? t('office.exitImmersive') : t('office.immersive') }}
        </button>
        <button class="document-stage__btn" @click="close">{{ t('office.close') }}</button>
      </div>
    </header>

    <div v-if="editableKind && stage.mode === 'edit'" class="document-stage__ai">
      <OfficeAiToolbar
        :path="stage.path"
        :kind="editableKind"
        :page-index="pageIndex"
        :disabled="turnRunning"
        :get-selection-markdown="getSelectionMarkdown"
        :get-edit-scope="getEditScope"
        :ensure-saved="ensureSaved"
        :scope="editScope"
      />
    </div>

    <DocSurface
      v-if="stage.kind === 'doc'"
      ref="docRef"
      :project-id="projectId"
      :path="stage.path"
      :mode="stage.mode"
      :reload-token="stageReloadToken"
      :turn-running="turnRunning"
      @dirty="dirty = $event"
      @scope="editScope = $event"
    />
    <SlidesSurface
      v-else-if="stage.kind === 'slides'"
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
      v-else-if="stage.kind === 'sheet'"
      ref="sheetRef"
      :project-id="projectId"
      :path="stage.path"
      :mode="stage.mode"
      :reload-token="stageReloadToken"
      :turn-running="turnRunning"
      @dirty="dirty = $event"
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
    />
    <PreviewSurface
      v-else-if="stage.kind === 'preview'"
      :project-id="projectId"
      :path="stage.path"
      :url="stage.url"
      :reload-token="stageReloadToken"
      @url-change="onPreviewUrlChange"
      @attach-element="emit('attachElement', $event)"
    />
  </section>
</template>

<style scoped>
.document-stage {
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
.document-stage.is-immersive {
  border: 0;
}
.document-stage__chrome {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
  padding: 8px 10px;
  border-bottom: 1px solid var(--dq-separator-light);
  background: color-mix(in srgb, var(--dq-bg-elevated) 55%, transparent);
  flex-wrap: wrap;
}
.document-stage__meta {
  display: flex;
  align-items: center;
  gap: 8px;
  min-width: 0;
}
.document-stage__badge {
  font-size: 11px;
  font-weight: 600;
  text-transform: uppercase;
  letter-spacing: 0.04em;
  padding: 2px 6px;
  border-radius: 4px;
  background: var(--dq-accent-tint);
  color: var(--dq-accent);
  flex-shrink: 0;
}
.document-stage__path {
  font-size: 12px;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  color: var(--dq-label-tertiary);
}
.document-stage__dirty {
  color: var(--dq-system-orange);
  font-size: 12px;
}
.document-stage__running {
  font-size: 11px;
  color: var(--dq-accent);
}
.document-stage__actions {
  display: flex;
  align-items: center;
  gap: 6px;
  flex-wrap: wrap;
}
.document-stage__modes {
  display: flex;
  gap: 2px;
  background: var(--dq-fill-tertiary);
  border-radius: 6px;
  padding: 2px;
}
.document-stage__chip {
  border: 0;
  background: transparent;
  height: 26px;
  padding: 0 8px;
  border-radius: 4px;
  font-size: 12px;
  color: var(--dq-label-secondary);
  cursor: pointer;
}
.document-stage__chip:hover {
  color: var(--dq-label-primary);
}
.document-stage__chip.is-active {
  background: var(--dq-bg-elevated);
  color: var(--dq-label-primary);
  box-shadow: 0 1px 2px color-mix(in srgb, var(--dq-mask) 12%, transparent);
}
.document-stage__btn {
  height: 28px;
  padding: 0 10px;
  border: 1px solid var(--dq-border);
  border-radius: 6px;
  background: var(--dq-fill-tertiary);
  color: var(--dq-label-primary);
  font-size: 12px;
  cursor: pointer;
}
.document-stage__btn:hover:not(:disabled) {
  background: color-mix(in srgb, var(--dq-label-primary) 8%, var(--dq-fill-tertiary));
}
.document-stage__btn:disabled {
  opacity: 0.5;
}
.document-stage__ai {
  padding: 6px 10px;
  border-bottom: 1px solid var(--dq-separator-light);
  background: color-mix(in srgb, var(--dq-bg-elevated) 35%, transparent);
}
</style>
