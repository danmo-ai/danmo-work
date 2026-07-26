<script setup lang="ts">
import { computed, nextTick, ref, watch, onMounted, onUnmounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { useSessionsStore } from '@/stores/sessions'
import { useProjectsStore } from '@/stores/projects'
import { useLLMStore } from '@/stores/llm'
import { useWorkspaceUiStore } from '@/stores/workspaceUi'
import ComposerAttachmentTray from '@/components/composer/ComposerAttachmentTray.vue'
import ComposerSkillChips from '@/components/composer/ComposerSkillChips.vue'
import ComposerSkillPicker from '@/components/composer/ComposerSkillPicker.vue'
import ContextUsageBar from '@/components/center/ContextUsageBar.vue'
import { asArray, fetchJSON } from '@/api/client'
import { toast } from '@/utils/feedback'
import type { AvailableSkill } from '@/types'
import type { LLMModel } from '@/types/mission'
import type { ElementAttachment } from '@/types/element-attachment'
import {
  buildComposerUserInput,
  createComposerAttachmentId,
  MAX_IMAGE_ATTACHMENT_BYTES,
  toApiImageAttachments,
  type ComposerAttachment,
  type ElementComposerAttachment,
} from '@/types/composer-attachment'
import {
  detectAtSkillQuery,
  prependSkillSummon,
  removeAtSkillQuery,
} from '@/types/composer-skills'

const { t } = useI18n()
const content = ref('')
const attachments = ref<ComposerAttachment[]>([])
const editingId = ref<string | null>(null)
const editingAnnotation = ref('')
const inputWrap = ref<HTMLElement | null>(null)
const fileInputRef = ref<HTMLInputElement | null>(null)
const dragOver = ref(false)
const gitBranch = ref('')
const gitError = ref('')
const availableSkills = ref<AvailableSkill[]>([])
const availableSkillsLoading = ref(false)
const selectedSkillIds = ref<string[]>([])
const buttonPickerOpen = ref(false)
const atMenuOpen = ref(false)
const atQuery = ref('')
const atStart = ref(-1)
const skillPickerRef = ref<{ onKeydown: (e: KeyboardEvent) => void } | null>(null)
const sessions = useSessionsStore()
const projects = useProjectsStore()
const llm = useLLMStore()
const workspaceUi = useWorkspaceUiStore()

const isMac =
  typeof navigator !== 'undefined' &&
  /(Mac|iPhone|iPad|iPod)/i.test(navigator.platform || navigator.userAgent || '')

const sendShortcut = computed(() => (isMac ? '⌘↵' : 'Ctrl+↵'))

const availableModels = computed(() => {
  return llm.models.map((m) => ({ id: m.id, label: m.id, model: m }))
})

const selectedModel = computed<LLMModel | undefined>(() => {
  const parts = sessions.selectedModelId.split('/')
  const baseId = parts.length >= 2 ? `${parts[0]}/${parts[1]}` : sessions.selectedModelId
  return llm.models.find((m) => m.id === baseId)
})

const selectedBaseModelId = computed({
  get: () => {
    const parts = sessions.selectedModelId.split('/')
    return parts.length >= 2 ? `${parts[0]}/${parts[1]}` : sessions.selectedModelId
  },
  set: (v: string) => {
    const newModel = llm.models.find((m) => m.id === v)
    const efforts = newModel?.availableEfforts ?? []
    let effort = sessions.selectedEffort
    if (effort && efforts.length > 0 && !efforts.includes(effort)) {
      effort = efforts[0]
      sessions.selectedEffort = effort
    }
    sessions.selectedModelId = effort && effort !== 'off' ? `${v}/${effort}` : v
  },
})

const availableEfforts = computed<string[]>(() => {
  return selectedModel.value?.availableEfforts ?? []
})

watch(() => sessions.selectedEffort, (effort) => {
  const parts = sessions.selectedModelId.split('/')
  if (parts.length >= 2) {
    const base = `${parts[0]}/${parts[1]}`
    sessions.selectedModelId = effort && effort !== 'off' ? `${base}/${effort}` : base
  }
})

const primaryAgents = computed(() => {
  const list = sessions.agents.filter((a) => a.mode !== 'subagent')
  const order = ['default', 'team', 'planner']
  return [...list].sort((a, b) => {
    const ai = order.indexOf(a.id)
    const bi = order.indexOf(b.id)
    const ao = ai === -1 ? order.length : ai
    const bo = bi === -1 ? order.length : bi
    if (ao !== bo) return ao - bo
    return a.name.localeCompare(b.name, 'zh-CN')
  })
})

const placeholder = computed(() =>
  sessions.composingNew ? t('composer.placeholderNew') : t('composer.placeholderContinue'),
)

const selectedSkills = computed(() => {
  const map = new Map(availableSkills.value.map((s) => [s.id, s]))
  return selectedSkillIds.value
    .map((id) => map.get(id))
    .filter((s): s is AvailableSkill => Boolean(s))
})

const skillPickerOpen = computed(() => buttonPickerOpen.value || atMenuOpen.value)

const hasPendingApproval = computed(() => workspaceUi.pendingApprovals > 0)

const canSend = computed(
  () =>
    Boolean(content.value.trim() || attachments.value.length || selectedSkillIds.value.length) &&
    !sessions.loading &&
    !hasPendingApproval.value,
)

const isTurnRunning = computed(
  () => !sessions.composingNew && sessions.runningTurnId !== null,
)

const showAgentSelect = computed(
  () => (sessions.composingNew || sessions.currentSessionId) && primaryAgents.value.length > 0,
)

const gitDisplay = computed(() => gitBranch.value || gitError.value)

const showTray = computed(
  () =>
    sessions.composingNew ||
    showAgentSelect.value ||
    Boolean(sessions.currentSession) ||
    Boolean(gitDisplay.value),
)

/** Few primary agents → segmented toggle; many → dropdown. */
const useAgentSegmented = computed(() => primaryAgents.value.length > 0 && primaryAgents.value.length <= 4)

const agentOptions = computed(() =>
  primaryAgents.value.map((a) => ({ label: a.name, value: a.id })),
)

function clearGitStatus() {
  gitBranch.value = ''
  gitError.value = ''
}

function applyGitError(code?: string, fallback?: string) {
  gitBranch.value = ''
  if (code === 'git_missing') {
    gitError.value = t('composer.gitMissing')
  } else if (code === 'init_failed') {
    gitError.value = fallback?.trim() || t('composer.gitInitFailed')
  } else {
    gitError.value = fallback?.trim() || t('composer.gitUnavailable')
  }
}

async function loadGitBranch() {
  const projectId = sessions.selectedProjectId
  if (!projectId) {
    clearGitStatus()
    return
  }
  try {
    const res = await fetchJSON<{ current?: string; error?: string; code?: string }>(
      `/projects/${projectId}/git-branches`,
    )
    if (res.code || res.error) {
      applyGitError(res.code, res.error)
      return
    }
    gitBranch.value = res.current?.trim() || ''
    gitError.value = gitBranch.value ? '' : t('composer.gitNoBranch')
  } catch (e) {
    applyGitError(undefined, e instanceof Error ? e.message : undefined)
  }
}

onMounted(async () => {
  const oldIds = new Set(llm.models.map((m) => m.id))
  await llm.loadModels()
  sessions.syncModelSelection(llm.models, oldIds)
  void loadGitBranch()
  void loadAvailableSkills()
  document.addEventListener('visibilitychange', onVisibilityRefresh)
  document.addEventListener('mousedown', onDocPointerDown)
})

onUnmounted(() => {
  document.removeEventListener('visibilitychange', onVisibilityRefresh)
  document.removeEventListener('mousedown', onDocPointerDown)
})

function onVisibilityRefresh() {
  if (document.visibilityState === 'visible') void loadGitBranch()
}

watch(() => sessions.selectedProjectId, () => {
  void loadGitBranch()
  void loadAvailableSkills()
})

watch(
  () => sessions.selectedAgentId,
  () => {
    void loadAvailableSkills()
  },
)

watch(() => workspaceUi.rightTab, (tab, prev) => {
  if (prev === 'changes' || tab === 'changes') void loadGitBranch()
})

watch(
  () => llm.models,
  (newModels, oldModels) => {
    const oldIds = new Set((oldModels ?? []).map((m) => m.id))
    sessions.syncModelSelection(newModels, oldIds)
  },
)

watch(
  () => sessions.composingNew,
  (v) => {
    if (v) {
      clearComposer()
      if (!sessions.selectedProjectId && projects.sortedProjects.length) {
        sessions.selectedProjectId = projects.sortedProjects[0].id
      }
      focusInput()
    }
  },
)

function focusInput() {
  void nextTick(() => {
    inputWrap.value?.querySelector('textarea')?.focus()
  })
}

function appendContent(text: string) {
  if (!text) return
  content.value = (content.value ? content.value + '\n' : '') + text
  focusInput()
}

function addElementAttachment(att: ElementAttachment) {
  if (att.screenshotDataUrl) {
    const comma = att.screenshotDataUrl.indexOf(',')
    const b64 = comma >= 0 ? att.screenshotDataUrl.slice(comma + 1) : ''
    const size = Math.floor((b64.length * 3) / 4)
    if (size > MAX_IMAGE_ATTACHMENT_BYTES) {
      toast.warning(t('composer.attachImageTooLarge', { max: '10 MB' }))
      att = { ...att, screenshotDataUrl: undefined, screenshotName: undefined }
    }
  }
  const wrapped: ElementComposerAttachment = {
    id: att.id,
    kind: 'element',
    data: att,
  }
  attachments.value = [...attachments.value, wrapped]
  focusInput()
}

function removeAttachment(id: string) {
  attachments.value = attachments.value.filter((a) => a.id !== id)
  if (editingId.value === id) {
    editingId.value = null
    editingAnnotation.value = ''
  }
}

function startEditAnnotation(att: ElementComposerAttachment) {
  editingId.value = att.id
  editingAnnotation.value = att.data.annotation
}

function saveEditAnnotation() {
  const id = editingId.value
  if (!id) return
  attachments.value = attachments.value.map((a) => {
    if (a.id !== id || a.kind !== 'element') return a
    return { ...a, data: { ...a.data, annotation: editingAnnotation.value.trim() } }
  })
  editingId.value = null
  editingAnnotation.value = ''
}

function cancelEditAnnotation() {
  editingId.value = null
  editingAnnotation.value = ''
}

function clearComposer() {
  content.value = ''
  attachments.value = []
  editingId.value = null
  editingAnnotation.value = ''
  selectedSkillIds.value = []
  closeSkillPickers()
}

function getTextarea(): HTMLTextAreaElement | null {
  return inputWrap.value?.querySelector('textarea') ?? null
}

function closeSkillPickers() {
  buttonPickerOpen.value = false
  atMenuOpen.value = false
  atQuery.value = ''
  atStart.value = -1
}

function onDocPointerDown(e: MouseEvent) {
  if (!skillPickerOpen.value) return
  const root = (e.target as HTMLElement | null)?.closest?.('.composer-skill-pop, .composer-tool-btn--skill')
  if (!root) closeSkillPickers()
}

async function loadAvailableSkills() {
  const agentId = sessions.selectedAgentId
  if (!agentId) {
    availableSkills.value = []
    selectedSkillIds.value = []
    return
  }
  availableSkillsLoading.value = true
  try {
    const projectId = sessions.selectedProjectId || ''
    const q = projectId ? `?projectId=${encodeURIComponent(projectId)}` : ''
    const list = asArray(
      await fetchJSON<AvailableSkill[]>(`/agents/${encodeURIComponent(agentId)}/available-skills${q}`),
    )
    availableSkills.value = list
    const allowed = new Set(list.map((s) => s.id))
    selectedSkillIds.value = selectedSkillIds.value.filter((id) => allowed.has(id))
  } catch {
    availableSkills.value = []
  } finally {
    availableSkillsLoading.value = false
  }
}

function syncAtMenuFromCaret() {
  if (hasPendingApproval.value || isTurnRunning.value) {
    atMenuOpen.value = false
    return
  }
  const ta = getTextarea()
  if (!ta) {
    atMenuOpen.value = false
    return
  }
  const detected = detectAtSkillQuery(content.value, ta.selectionStart ?? 0)
  if (!detected) {
    atMenuOpen.value = false
    atQuery.value = ''
    atStart.value = -1
    return
  }
  buttonPickerOpen.value = false
  atMenuOpen.value = true
  atQuery.value = detected.query
  atStart.value = detected.start
}

watch(content, () => {
  void nextTick(() => syncAtMenuFromCaret())
})

function toggleButtonSkillPicker() {
  if (hasPendingApproval.value || isTurnRunning.value) return
  atMenuOpen.value = false
  buttonPickerOpen.value = !buttonPickerOpen.value
}

function onPickSkill(sk: AvailableSkill) {
  if (atMenuOpen.value && atStart.value >= 0) {
    const ta = getTextarea()
    const caret = ta?.selectionStart ?? content.value.length
    content.value = removeAtSkillQuery(content.value, atStart.value, caret)
    atMenuOpen.value = false
    atQuery.value = ''
    atStart.value = -1
    if (!selectedSkillIds.value.includes(sk.id)) {
      selectedSkillIds.value = [...selectedSkillIds.value, sk.id]
    }
    void nextTick(() => focusInput())
    return
  }
  if (selectedSkillIds.value.includes(sk.id)) {
    selectedSkillIds.value = selectedSkillIds.value.filter((id) => id !== sk.id)
  } else {
    selectedSkillIds.value = [...selectedSkillIds.value, sk.id]
  }
}

function removeSelectedSkill(id: string) {
  selectedSkillIds.value = selectedSkillIds.value.filter((x) => x !== id)
}

function openFilePicker() {
  fileInputRef.value?.click()
}

function onFilePicked(e: Event) {
  const input = e.target as HTMLInputElement
  const files = Array.from(input.files ?? [])
  input.value = ''
  for (const f of files) addLocalFile(f)
}

function addLocalFile(file: File) {
  if (file.type.startsWith('image/')) {
    addImageFile(file)
    return
  }
  // Non-image: placeholder chip until upload pipeline exists
  attachments.value = [
    ...attachments.value,
    {
      id: createComposerAttachmentId('file'),
      kind: 'file',
      name: file.name,
      mime: file.type || 'application/octet-stream',
      size: file.size,
      placeholder: true,
    },
  ]
  toast.info(t('composer.attachFilePlaceholder'))
  focusInput()
}

async function send() {
  if (hasPendingApproval.value) {
    toast.warning(t('sessions.pendingApprovalHint'))
    return
  }
  if (skillPickerOpen.value && atMenuOpen.value) {
    closeSkillPickers()
  }
  let text = buildComposerUserInput(content.value, attachments.value)
  text = prependSkillSummon(
    text,
    selectedSkills.value,
    (name) => t('composer.useSkillLine', { name }),
    t('composer.readSkillHint'),
  )
  let imageAtts = toApiImageAttachments(attachments.value)
  if (imageAtts.length && !selectedModel.value?.vision) {
    toast.warning(t('composer.modelNoVision'))
    imageAtts = []
  }
  if ((!text.trim() && !imageAtts.length) || sessions.loading) return

  if (sessions.composingNew) {
    if (!sessions.selectedProjectId) {
      toast.warning(t('composer.needProject'))
      return
    }
    try {
      await sessions.createSession(text, sessions.selectedProjectId, imageAtts)
      clearComposer()
      focusInput()
    } catch (e) {
      toast.error(e instanceof Error ? e.message : t('composer.sendFailed'))
    }
    return
  }

  try {
    await sessions.sendTurn(text, imageAtts)
    clearComposer()
    focusInput()
  } catch (e) {
    toast.error(e instanceof Error ? e.message : t('composer.sendFailed'))
  }
}

async function stop() {
  if (!sessions.runningTurnId) return
  try {
    await sessions.cancelTurn(sessions.runningTurnId)
  } catch (e) {
    toast.error(e instanceof Error ? e.message : t('composer.cancelFailed'))
  }
}

function onKeydown(e: KeyboardEvent) {
  if (skillPickerOpen.value && (e.key === 'ArrowDown' || e.key === 'ArrowUp' || e.key === 'Escape')) {
    skillPickerRef.value?.onKeydown(e)
    return
  }
  if (skillPickerOpen.value && e.key === 'Enter' && !e.shiftKey && !e.metaKey && !e.ctrlKey) {
    skillPickerRef.value?.onKeydown(e)
    return
  }
  if ((e.metaKey || e.ctrlKey) && e.key === 'Enter') {
    e.preventDefault()
    if (isTurnRunning.value) void stop()
    else void send()
    return
  }
  if (e.key === 'Enter' && !e.shiftKey) {
    e.preventDefault()
    if (isTurnRunning.value) void stop()
    else void send()
  }
}

function addImageFile(file: File) {
  if (!file.type.startsWith('image/')) return
  if (file.size > MAX_IMAGE_ATTACHMENT_BYTES) {
    toast.warning(t('composer.attachImageTooLarge', { max: '10 MB' }))
    return
  }
  const reader = new FileReader()
  reader.onload = () => {
    const dataUrl = String(reader.result ?? '')
    attachments.value = [
      ...attachments.value,
      {
        id: createComposerAttachmentId('img'),
        kind: 'image',
        name: file.name || `paste-${Date.now()}.png`,
        mime: file.type || 'image/png',
        size: file.size,
        dataUrl,
      },
    ]
    focusInput()
  }
  reader.readAsDataURL(file)
}

function onPaste(e: ClipboardEvent) {
  const items = e.clipboardData?.items
  if (!items) return
  const images = Array.from(items).filter((i) => i.type.startsWith('image/'))
  if (!images.length) return
  e.preventDefault()
  for (const item of images) {
    const file = item.getAsFile()
    if (file) addImageFile(file)
  }
  toast.success(t('composer.pasteImageAdded'))
}

function onDragOver() {
  if (hasPendingApproval.value) return
  dragOver.value = true
}

function onDrop(e: DragEvent) {
  e.preventDefault()
  dragOver.value = false
  if (hasPendingApproval.value) return
  const files = Array.from(e.dataTransfer?.files ?? [])
  if (!files.length) return
  for (const f of files) addLocalFile(f)
}

const emit = defineEmits<{
  'jump-pending': []
}>()

defineExpose({ focusInput, appendContent, addElementAttachment })
</script>

<template>
  <div
    class="composer-float"
    :class="{
      'is-dragover': dragOver,
      'is-blocked': hasPendingApproval,
      'is-compose': sessions.composingNew,
      'is-skill-picker-open': skillPickerOpen,
    }"
    role="form"
    aria-label="Session composer"
    @dragover.prevent="onDragOver"
    @dragleave.prevent="dragOver = false"
    @drop="onDrop"
  >
    <!-- Outside the glass card: card has overflow:hidden and would clip this popover. -->
    <div
      v-if="skillPickerOpen && !hasPendingApproval"
      class="composer-skill-pop"
      :class="{ 'composer-skill-pop--button': buttonPickerOpen }"
    >
      <ComposerSkillPicker
        ref="skillPickerRef"
        :skills="availableSkills"
        :selected-ids="selectedSkillIds"
        :query="atMenuOpen ? atQuery : ''"
        :show-search="buttonPickerOpen"
        :loading="availableSkillsLoading"
        @select="onPickSkill"
        @close="closeSkillPickers"
      />
    </div>

    <!-- Compact bar while ask_user / permission cards need attention -->
    <div v-if="hasPendingApproval" class="composer-float__card composer-float__card--blocked">
      <div class="composer-float__banner composer-float__banner--warn">
        <span class="composer-float__banner-text">{{ t('sessions.pendingApprovalHint') }}</span>
        <button
          type="button"
          class="composer-float__jump"
          @click="emit('jump-pending')"
        >
          {{ t('sessions.jumpToPending') }}
        </button>
      </div>
    </div>

    <template v-else>
    <!-- Upper card: input + model/effort/send -->
    <div class="composer-float__card">
      <div v-if="dragOver" class="composer-float__drop">{{ t('composer.dropHint') }}</div>

      <div v-if="isTurnRunning" class="composer-float__banner composer-float__banner--run">
        <span class="composer-float__run-dot" />
        {{ t('composer.running') }}
      </div>

      <ComposerAttachmentTray
        :attachments="attachments"
        :editing-id="editingId"
        :editing-annotation="editingAnnotation"
        @remove="removeAttachment"
        @edit-start="startEditAnnotation"
        @edit-save="saveEditAnnotation"
        @edit-cancel="cancelEditAnnotation"
        @update:editing-annotation="editingAnnotation = $event"
      />

      <ComposerSkillChips
        :skills="selectedSkills"
        @remove="removeSelectedSkill"
      />

      <div class="composer-float__input-stack">
        <div ref="inputWrap" class="composer-float__body">
          <DqInput
            v-model="content"
            type="textarea"
            :rows="2"
            class="composer-float__input"
            :placeholder="placeholder"
            @keydown="onKeydown"
            @paste="onPaste"
            @click="syncAtMenuFromCaret"
            @keyup="syncAtMenuFromCaret"
          />
        </div>
      </div>

      <input
        ref="fileInputRef"
        type="file"
        class="composer-float__file-input"
        multiple
        accept="image/*,.pdf,.txt,.md,.json,.csv,.png,.jpg,.jpeg,.webp,.gif"
        @change="onFilePicked"
      />

      <div class="composer-float__footer">
        <div class="composer-float__footer-leading">
          <button
            type="button"
            class="composer-tool-btn"
            :title="t('composer.attachFile')"
            :aria-label="t('composer.attachFile')"
            @click="openFilePicker"
          >
            <svg viewBox="0 0 24 24" width="16" height="16" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true">
              <path d="M12 5v14" />
              <path d="M5 12h14" />
            </svg>
          </button>
          <button
            type="button"
            class="composer-tool-btn composer-tool-btn--skill"
            :class="{ 'is-active': buttonPickerOpen || selectedSkillIds.length }"
            :title="t('composer.selectSkill')"
            :aria-label="t('composer.selectSkill')"
            :aria-expanded="buttonPickerOpen"
            :disabled="isTurnRunning"
            @click="toggleButtonSkillPicker"
          >
            <svg viewBox="0 0 24 24" width="16" height="16" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true">
              <path d="M12 3l1.8 4.8L19 9.5l-4 3.2L16.2 18 12 15.2 7.8 18l1.2-5.3-4-3.2 5.2-1.7L12 3z" />
            </svg>
          </button>
          <span
            v-if="llm.modelsLoaded && !availableModels.length"
            class="composer-meta-chip composer-meta-chip--warn"
          >
            {{ t('composer.needLlm') }}
          </span>
        </div>

        <div class="composer-float__footer-trailing">
          <div
            v-if="llm.modelsLoaded && availableModels.length"
            class="composer-select composer-select--model"
          >
            <DqSelect
              v-model="selectedBaseModelId"
              size="sm"
              variant="ghost"
              :aria-label="t('composer.selectModel')"
            >
              <DqOption
                v-for="model in availableModels"
                :key="model.id"
                :value="model.id"
                :label="model.label"
              />
            </DqSelect>
          </div>

          <div
            v-if="llm.modelsLoaded && availableEfforts.length > 1"
            class="composer-select composer-select--effort"
          >
            <DqSelect
              v-model="sessions.selectedEffort"
              size="sm"
              variant="ghost"
              :aria-label="t('composer.selectEffort')"
              :placeholder="t('composer.selectEffort')"
            >
              <DqOption v-for="e in availableEfforts" :key="e" :value="e" :label="e" />
            </DqSelect>
          </div>

          <button
            v-if="isTurnRunning"
            type="button"
            class="composer-send composer-send--stop"
            :aria-label="t('composer.stop')"
            @click="stop"
          >
            <svg viewBox="0 0 24 24" width="14" height="14" fill="currentColor" aria-hidden="true">
              <rect x="5" y="5" width="14" height="14" rx="2" />
            </svg>
            <span>{{ t('composer.stop') }}</span>
          </button>
          <button
            v-else
            type="button"
            class="composer-send"
            :disabled="!canSend"
            :aria-label="t('composer.send')"
            @click="send"
          >
            <span>{{ t('composer.send') }}</span>
            <kbd class="composer-send__kbd">{{ sendShortcut }}</kbd>
          </button>
        </div>
      </div>
    </div>

    <!-- Lower tray: project + agent + git branch + context usage -->
    <div
      v-if="showTray"
      class="composer-float__tray"
    >
      <div class="composer-float__tray-leading">
        <div
          v-if="sessions.composingNew"
          class="composer-select composer-select--project"
        >
          <DqSelect
            v-model="sessions.selectedProjectId"
            size="sm"
            variant="ghost"
            :aria-label="t('composer.selectProject')"
            :placeholder="t('composer.selectProject')"
          >
            <DqOption
              v-for="p in projects.sortedProjects"
              :key="p.id"
              :value="p.id"
              :label="p.name"
            />
          </DqSelect>
        </div>

        <DqSegmented
          v-if="showAgentSelect && useAgentSegmented"
          v-model="sessions.selectedAgentId"
          size="sm"
          class="composer-agent-seg composer-agent-seg--compact"
          :options="agentOptions"
          :aria-label="t('composer.selectAgent')"
        />
        <div
          v-else-if="showAgentSelect"
          class="composer-select composer-select--agent"
        >
          <DqSelect
            v-model="sessions.selectedAgentId"
            size="sm"
            variant="ghost"
            :aria-label="t('composer.selectAgent')"
          >
            <DqOption
              v-for="a in primaryAgents"
              :key="a.id"
              :value="a.id"
              :label="a.name"
            />
          </DqSelect>
        </div>

        <span
          v-if="gitDisplay"
          class="composer-git-branch"
          :class="{ 'is-error': Boolean(gitError) }"
          :title="gitDisplay"
          :aria-label="gitError ? gitError : `${t('composer.gitBranch')}: ${gitBranch}`"
        >
          <span class="composer-git-branch__icon" aria-hidden="true">⎇</span>
          <span class="composer-git-branch__name">{{ gitDisplay }}</span>
        </span>
      </div>

      <div class="composer-float__tray-trailing">
        <ContextUsageBar />
      </div>
    </div>
    </template>
  </div>
</template>

<style scoped>
.composer-float {
  position: relative;
  display: flex;
  flex-direction: column;
  gap: 0;
  /* Allow skill popover to paint above the glass card / tray. */
  overflow: visible;
}

.composer-float.is-skill-picker-open {
  z-index: 30;
}

.composer-float.is-dragover .composer-float__card {
  border-color: var(--dq-accent);
  background: color-mix(in srgb, var(--dq-accent) 6%, var(--dq-glass-popover-bg));
}

.composer-float.is-blocked {
  pointer-events: none;
}

.composer-float__card {
  position: relative;
  z-index: 2;
}

.composer-float__card--blocked {
  overflow: hidden;
  pointer-events: auto;
}

.composer-float__banner-text {
  flex: 1;
  min-width: 0;
}

.composer-float__jump {
  flex-shrink: 0;
  margin-left: auto;
  padding: 4px 10px;
  border: 1px solid color-mix(in srgb, var(--dq-system-orange) 35%, transparent);
  border-radius: 6px;
  background: color-mix(in srgb, var(--dq-system-orange) 12%, transparent);
  color: inherit;
  font: inherit;
  font-weight: 600;
  cursor: pointer;
  transition: background 0.12s ease, border-color 0.12s ease;
}

.composer-float__jump:hover {
  background: color-mix(in srgb, var(--dq-system-orange) 20%, transparent);
  border-color: color-mix(in srgb, var(--dq-system-orange) 55%, transparent);
}

.composer-float__drop {
  position: absolute;
  inset: 0;
  z-index: 2;
  display: flex;
  align-items: center;
  justify-content: center;
  border-radius: inherit;
  background: color-mix(in srgb, var(--dq-accent) 12%, var(--dq-glass-popover-bg));
  color: var(--dq-accent);
  font-weight: 600;
  pointer-events: none;
}

.composer-float__banner {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 8px 14px;
  font-size: var(--dq-font-size-caption);
  font-weight: 600;
  border-bottom: 1px solid var(--dq-separator-light);
}

.composer-float__card--blocked .composer-float__banner {
  border-bottom: none;
}

.composer-float__banner--warn {
  color: var(--dq-system-orange);
  background: color-mix(in srgb, var(--dq-system-orange) 8%, transparent);
}

.composer-float__banner--run {
  color: var(--dq-accent);
  background: color-mix(in srgb, var(--dq-accent) 8%, transparent);
}

.composer-float__run-dot {
  width: 6px;
  height: 6px;
  border-radius: 50%;
  background: currentColor;
  animation: pulse 1.2s ease-in-out infinite;
}

.composer-float__body {
  padding: 6px 14px 0;
}

.composer-float__body :deep(.dq-input),
.composer-float__body :deep(textarea) {
  border: none !important;
  background: transparent !important;
  box-shadow: none !important;
  resize: none;
  min-height: 0 !important;
  height: calc(2 * 1.45em) !important;
  max-height: calc(2 * 1.45em) !important;
  line-height: 1.45;
  overflow-y: auto;
}

.composer-float__file-input {
  display: none;
}

.composer-float__footer {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 10px;
  padding: 4px 10px 8px;
  min-width: 0;
}

.composer-float__footer-leading,
.composer-float__footer-trailing {
  display: flex;
  align-items: center;
  gap: 4px;
  min-width: 0;
}

.composer-float__footer-trailing {
  flex: 1 1 auto;
  justify-content: flex-end;
  flex-shrink: 1;
  gap: 8px;
  min-width: 0;
  max-width: 100%;
}

.composer-float__tray {
  position: relative;
  z-index: 1;
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
  margin-top: -10px;
  padding: 14px 10px 6px;
  min-width: 0;
  border-radius: 0 0 var(--dq-radius-menu) var(--dq-radius-menu);
  background: color-mix(in srgb, var(--dq-glass-popover-bg) 92%, var(--dq-bg-base));
  border: 1px solid var(--dq-glass-border-strong);
  border-top: none;
  box-shadow:
    0 8px 24px color-mix(in srgb, var(--dq-mask) 18%, transparent);
  backdrop-filter: var(--dq-glass-blur-heavy);
  -webkit-backdrop-filter: var(--dq-glass-blur-heavy);
}

.composer-float__tray-leading {
  display: flex;
  flex-wrap: nowrap;
  align-items: center;
  gap: 4px;
  min-width: 0;
  overflow-x: auto;
  scrollbar-width: none;
}

.composer-float__tray-leading::-webkit-scrollbar {
  display: none;
}

.composer-float__tray-trailing {
  flex-shrink: 0;
  margin-left: auto;
  display: flex;
  align-items: center;
  justify-content: flex-end;
  min-width: 0;
}

.composer-git-branch {
  display: inline-flex;
  align-items: center;
  gap: 4px;
  flex: 0 1 auto;
  max-width: min(100%, 420px);
  min-width: 0;
  height: 28px;
  padding: 0 8px;
  border-radius: 6px;
  color: var(--dq-label-secondary);
  font-size: var(--dq-font-size-caption);
  font-weight: 500;
  white-space: nowrap;
  overflow: hidden;
}

.composer-git-branch.is-error {
  color: var(--dq-danger, var(--dq-label-secondary));
}

.composer-git-branch__icon {
  flex-shrink: 0;
  opacity: 0.75;
}

.composer-git-branch__name {
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
}

.composer-agent-seg--compact {
  width: auto;
  flex-shrink: 0;
  border: none;
  background: transparent;
  padding: 0;
}

.composer-agent-seg--compact :deep(.dq-segmented__item) {
  padding: 4px 8px;
  font-size: var(--dq-font-size-caption);
  font-weight: 500;
  color: var(--dq-label-tertiary);
  border-radius: 6px;
}

.composer-agent-seg--compact :deep(.dq-segmented__item.is-active) {
  color: var(--dq-label-primary);
  background: color-mix(in srgb, var(--dq-label-primary) 10%, transparent);
  box-shadow: none;
}

.composer-float__tray :deep(.context-usage__label),
.composer-float__tray :deep(.context-usage__icon) {
  color: var(--dq-label-secondary);
}

.composer-float__tray :deep(.context-usage__pct) {
  color: var(--dq-label-tertiary);
}

.composer-meta-chip {
  display: inline-flex;
  align-items: center;
  height: 28px;
  padding: 0 8px;
  border-radius: 6px;
  border: none;
  font-size: var(--dq-font-size-caption);
  white-space: nowrap;
}

.composer-meta-chip--warn {
  color: var(--dq-system-orange);
  background: color-mix(in srgb, var(--dq-system-orange) 10%, transparent);
}

.composer-select {
  flex: 0 1 auto;
  width: auto !important;
  min-width: 0;
  max-width: 100%;
  display: block;
  overflow: hidden;
}

.composer-select :deep(.dq-select) {
  display: block;
  width: 100%;
  min-width: 0;
  max-width: 100%;
}

.composer-select :deep(.dq-select__trigger) {
  width: 100%;
  max-width: 100%;
  min-width: 0;
  min-height: 28px;
  padding: 2px 6px;
  font-size: var(--dq-font-size-caption);
  overflow: hidden;
}

.composer-select :deep(.dq-select__value) {
  flex: 1 1 auto;
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.composer-select--project {
  flex: 0 1 160px;
  max-width: 200px;
}

.composer-select--model {
  flex: 1 1 100px;
  min-width: 64px;
  max-width: 180px;
}

.composer-select--effort {
  flex: 0 0 auto;
  width: max-content;
  min-width: 64px;
  max-width: 96px;
  overflow: visible;
}

.composer-select--effort :deep(.dq-select),
.composer-select--effort :deep(.dq-select__trigger) {
  width: auto;
}

.composer-select--agent {
  flex: 0 1 120px;
  max-width: 160px;
}

.composer-float__footer-trailing .composer-send {
  flex-shrink: 0;
}

.composer-tool-btn {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 32px;
  height: 32px;
  border: none;
  border-radius: 8px;
  background: transparent;
  color: var(--dq-label-tertiary);
  cursor: pointer;
  transition: background 0.12s ease, color 0.12s ease;
}

.composer-tool-btn:hover {
  background: color-mix(in srgb, var(--dq-label-primary) 8%, transparent);
  color: var(--dq-label-primary);
}

.composer-tool-btn.is-active {
  color: var(--dq-accent);
  background: color-mix(in srgb, var(--dq-accent) 12%, transparent);
}

.composer-tool-btn:disabled {
  opacity: 0.4;
  cursor: not-allowed;
}

.composer-float__input-stack {
  position: relative;
}

.composer-skill-pop {
  position: absolute;
  left: 10px;
  right: 10px;
  /* Sit above the whole composer (card + tray), not inside the clipped card. */
  bottom: calc(100% + 8px);
  z-index: 40;
  display: flex;
  justify-content: flex-start;
  pointer-events: none;
}

.composer-skill-pop > * {
  pointer-events: auto;
}

.composer-skill-pop--button {
  left: 10px;
  right: auto;
  width: min(320px, calc(100% - 20px));
}

.composer-send {
  display: inline-flex;
  align-items: center;
  gap: 8px;
  height: 32px;
  margin-left: 4px;
  padding: 0 12px 0 14px;
  border: none;
  border-radius: 10px;
  background: var(--dq-accent);
  color: var(--dq-color-white);
  font-size: var(--dq-font-size-footnote);
  font-weight: 650;
  cursor: pointer;
  transition: opacity 0.15s ease, transform 0.12s ease;
}

.composer-send:hover:not(:disabled) {
  filter: brightness(1.06);
}

.composer-send:active:not(:disabled) {
  transform: scale(0.98);
}

.composer-send:disabled {
  opacity: 0.4;
  cursor: not-allowed;
}

.composer-send--stop {
  background: color-mix(in srgb, var(--dq-system-orange) 88%, #000);
}

.composer-send__kbd {
  display: inline-flex;
  align-items: center;
  height: 18px;
  padding: 0 5px;
  border-radius: 5px;
  background: color-mix(in srgb, #000 18%, transparent);
  font-family: var(--dq-font-mono, ui-monospace, monospace);
  font-size: 10px;
  font-weight: 600;
  opacity: 0.9;
}

@keyframes pulse {
  0%,
  100% {
    opacity: 1;
  }
  50% {
    opacity: 0.4;
  }
}
</style>
