<script setup lang="ts">
import { computed, nextTick, ref, watch, onMounted, onUnmounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { useSessionsStore } from '@/stores/sessions'
import { useProjectsStore } from '@/stores/projects'
import { useLLMStore } from '@/stores/llm'
import { useWorkspaceUiStore } from '@/stores/workspaceUi'
import ComposerAttachmentTray from '@/components/composer/ComposerAttachmentTray.vue'
import ComposerExpertChips from '@/components/composer/ComposerExpertChips.vue'
import ComposerExpertPicker from '@/components/composer/ComposerExpertPicker.vue'
import ComposerSkillChips from '@/components/composer/ComposerSkillChips.vue'
import ComposerSkillPicker from '@/components/composer/ComposerSkillPicker.vue'
import ComposerSlashPicker from '@/components/composer/ComposerSlashPicker.vue'
import ContextUsageBar from '@/components/center/ContextUsageBar.vue'
import { asArray, fetchJSON, uploadProjectFile } from '@/api/client'
import { toast } from '@/utils/feedback'
import type { Agent, AvailableSkill } from '@/types'
import type { LLMModel } from '@/types/mission'
import type { ElementAttachment } from '@/types/element-attachment'
import {
  buildComposerUserInput,
  createComposerAttachmentId,
  MAX_FILE_ATTACHMENT_BYTES,
  MAX_IMAGE_ATTACHMENT_BYTES,
  toApiImageAttachments,
  type ComposerAttachment,
  type CodeComposerAttachment,
  type ElementComposerAttachment,
  type FileComposerAttachment,
  type OfficeComposerAttachment,
} from '@/types/composer-attachment'
import type { CodeSelectionAttachment } from '@/types/code-attachment'
import {
  officeEditSnapshotPaths,
  type OfficeEditAttachment,
} from '@/types/office-edit-attachment'
import {
  detectAtSkillQuery,
  prependSkillSummon,
  removeAtSkillQuery,
} from '@/types/composer-skills'
import {
  filterSummonableExperts,
  listSummonableExperts,
  prependExpertSummon,
} from '@/types/composer-experts'
import {
  COMPOSER_SLASH_COMMANDS,
  detectSlashQuery,
  removeSlashQuery,
  type ComposerSlashCommand,
} from '@/types/composer-slash'

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
const selectedExpertIds = ref<string[]>([])
const buttonPickerOpen = ref(false)
const expertButtonPickerOpen = ref(false)
const atMenuOpen = ref(false)
const atQuery = ref('')
const atStart = ref(-1)
const slashMenuOpen = ref(false)
const slashQuery = ref('')
const slashStart = ref(-1)
const skillPickerRef = ref<{ onKeydown: (e: KeyboardEvent) => void } | null>(null)
const expertPickerRef = ref<{
  onKeydown: (e: KeyboardEvent) => void
  filtered?: { value: Agent[] }
} | null>(null)
const slashPickerRef = ref<{ onKeydown: (e: KeyboardEvent) => void } | null>(null)
const sessions = useSessionsStore()
const projects = useProjectsStore()
const llm = useLLMStore()
const workspaceUi = useWorkspaceUiStore()

const emit = defineEmits<{
  'jump-pending': []
  queued: []
}>()

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

const primaryAgents = computed(() =>
  [...sessions.agents.filter((a) => a.mode !== 'subagent')].sort((a, b) =>
    a.name.localeCompare(b.name, undefined, { sensitivity: 'base' }),
  ),
)

const placeholder = computed(() =>
  sessions.composingNew ? t('composer.placeholderNew') : t('composer.placeholderContinue'),
)

const selectedSkills = computed(() => {
  const map = new Map(availableSkills.value.map((s) => [s.id, s]))
  return selectedSkillIds.value
    .map((id) => map.get(id))
    .filter((s): s is AvailableSkill => Boolean(s))
})

const selectedLead = computed(() =>
  sessions.agents.find((a) => a.id === sessions.selectedAgentId) ?? null,
)

/** Lead must have canDelegate to summon sub-experts. */
const canDelegateExperts = computed(() => Boolean(selectedLead.value?.canDelegate))

const summonableExperts = computed(() =>
  listSummonableExperts(sessions.agents, sessions.selectedAgentId),
)

const selectedExperts = computed(() => {
  const map = new Map(summonableExperts.value.map((a) => [a.id, a]))
  return selectedExpertIds.value
    .map((id) => map.get(id))
    .filter((a): a is Agent => Boolean(a))
})

const atExpertMatches = computed(() =>
  canDelegateExperts.value
    ? filterSummonableExperts(sessions.agents, atQuery.value, sessions.selectedAgentId)
    : [],
)

const skillPickerOpen = computed(() => buttonPickerOpen.value || atMenuOpen.value)
const expertPickerOpen = computed(
  () => expertButtonPickerOpen.value || (atMenuOpen.value && canDelegateExperts.value),
)
const anyPickerOpen = computed(
  () => skillPickerOpen.value || expertPickerOpen.value || slashMenuOpen.value,
)

const hasPendingApproval = computed(() => workspaceUi.pendingApprovals > 0)

const hasDraft = computed(
  () =>
    Boolean(
      content.value.trim() ||
        attachments.value.length ||
        selectedSkillIds.value.length ||
        selectedExpertIds.value.length,
    ),
)

const isTurnRunning = computed(
  () => !sessions.composingNew && sessions.runningTurnId !== null,
)

/** Running turn → enqueue; idle → send. Pending approvals never start a new turn. */
const canSubmit = computed(
  () => hasDraft.value && !sessions.loading && !sessions.composingNew,
)

const canSend = computed(
  () => hasDraft.value && !sessions.loading && !hasPendingApproval.value && !isTurnRunning.value,
)

const canQueue = computed(
  () =>
    canSubmit.value &&
    Boolean(sessions.currentSessionId) &&
    (isTurnRunning.value || hasPendingApproval.value),
)

const primaryAction = computed<'send' | 'queue' | 'stop'>(() => {
  if (isTurnRunning.value && !hasDraft.value) return 'stop'
  if (canQueue.value) return 'queue'
  return 'send'
})

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
  primaryAgents.value.map((a) => ({
    label: a.name,
    value: a.id,
  })),
)

/** DqSegmented/DqSelect require string|number; store may be null before catalog loads. */
const selectedAgentModel = computed({
  get: () => sessions.selectedAgentId ?? primaryAgents.value[0]?.id ?? '',
  set: (v: string | number) => {
    sessions.selectedAgentId = String(v)
  },
})

const selectedLeadAgent = computed(() =>
  primaryAgents.value.find((a) => a.id === sessions.selectedAgentId),
)

const selectedLeadHint = computed(() => {
  const agent = selectedLeadAgent.value
  if (!agent) return ''
  const base = (agent.description || agent.persona || '').trim()
  if (agent.canDelegate) {
    return base ? `${base} ${t('composer.canDelegateHint')}` : t('composer.canDelegateHint')
  }
  return base
})

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

function addCodeSelectionAttachment(att: CodeSelectionAttachment) {
  const wrapped: CodeComposerAttachment = {
    id: att.id,
    kind: 'code',
    data: att,
  }
  attachments.value = [...attachments.value, wrapped]
  focusInput()
}

function addOfficeEditAttachment(att: OfficeEditAttachment) {
  const wrapped: OfficeComposerAttachment = {
    id: att.id,
    kind: 'office',
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

function startEditAnnotation(
  att: ElementComposerAttachment | CodeComposerAttachment | OfficeComposerAttachment,
) {
  editingId.value = att.id
  if (att.kind === 'office') {
    editingAnnotation.value = att.data.instruction
    return
  }
  editingAnnotation.value = att.data.annotation
}

function saveEditAnnotation() {
  const id = editingId.value
  if (!id) return
  const note = editingAnnotation.value.trim()
  attachments.value = attachments.value.map((a): ComposerAttachment => {
    if (a.id !== id) return a
    if (a.kind === 'element') {
      return { ...a, data: { ...a.data, annotation: note } }
    }
    if (a.kind === 'code') {
      return { ...a, data: { ...a.data, annotation: note } }
    }
    if (a.kind === 'office') {
      return { ...a, data: { ...a.data, instruction: note } }
    }
    return a
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
  selectedExpertIds.value = []
  closeAllPickers()
}

function getTextarea(): HTMLTextAreaElement | null {
  return inputWrap.value?.querySelector('textarea') ?? null
}

function autosizeTextarea() {
  const ta = getTextarea()
  if (!ta) return
  ta.style.height = 'auto'
  const maxRaw = getComputedStyle(document.documentElement).getPropertyValue('--dq-composer-max-height').trim()
  const maxPx = Number.parseFloat(maxRaw) || 240
  const next = Math.min(Math.max(ta.scrollHeight, 40), maxPx)
  ta.style.height = `${next}px`
}

watch(content, () => {
  nextTick(autosizeTextarea)
})

function closeSkillPickers() {
  buttonPickerOpen.value = false
  atMenuOpen.value = false
  atQuery.value = ''
  atStart.value = -1
}

function closeExpertButtonPicker() {
  expertButtonPickerOpen.value = false
}

function closeSlashPicker() {
  slashMenuOpen.value = false
  slashQuery.value = ''
  slashStart.value = -1
}

function closeAllPickers() {
  closeSkillPickers()
  closeExpertButtonPicker()
  closeSlashPicker()
}

function onDocPointerDown(e: MouseEvent) {
  if (!anyPickerOpen.value) return
  const root = (e.target as HTMLElement | null)?.closest?.(
    '.composer-skill-pop, .composer-expert-pop, .composer-at-pop, .composer-tool-btn--skill, .composer-tool-btn--expert, .composer-slash-pop',
  )
  if (!root) closeAllPickers()
}

watch(
  () => [sessions.selectedAgentId, canDelegateExperts.value] as const,
  () => {
    if (!canDelegateExperts.value) {
      selectedExpertIds.value = []
      expertButtonPickerOpen.value = false
      return
    }
    const allowed = new Set(summonableExperts.value.map((a) => a.id))
    selectedExpertIds.value = selectedExpertIds.value.filter((id) => allowed.has(id))
  },
)

watch(
  () => workspaceUi.composerPrefillToken,
  () => {
    const text = workspaceUi.consumeComposerPrefill()
    if (text) appendContent(text)
  },
)

watch(
  () => workspaceUi.composerSelectExpertToken,
  () => {
    const ids = workspaceUi.consumeComposerSelectExperts()
    if (!ids.length) return
    if (!canDelegateExperts.value) return
    const allowed = new Set(summonableExperts.value.map((a) => a.id))
    for (const id of ids) {
      if (!allowed.has(id)) continue
      if (!selectedExpertIds.value.includes(id)) {
        selectedExpertIds.value = [...selectedExpertIds.value, id]
      }
    }
    void nextTick(() => focusInput())
  },
)

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
    slashMenuOpen.value = false
    return
  }
  const ta = getTextarea()
  if (!ta) {
    atMenuOpen.value = false
    slashMenuOpen.value = false
    return
  }
  const caret = ta.selectionStart ?? 0
  const at = detectAtSkillQuery(content.value, caret)
  if (at) {
    buttonPickerOpen.value = false
    closeExpertButtonPicker()
    closeSlashPicker()
    atMenuOpen.value = true
    atQuery.value = at.query
    atStart.value = at.start
    return
  }
  atMenuOpen.value = false
  atQuery.value = ''
  atStart.value = -1

  const slash = detectSlashQuery(content.value, caret)
  if (slash) {
    buttonPickerOpen.value = false
    closeExpertButtonPicker()
    slashMenuOpen.value = true
    slashQuery.value = slash.query
    slashStart.value = slash.start
    return
  }
  closeSlashPicker()
}

watch(content, () => {
  void nextTick(() => syncAtMenuFromCaret())
})

function toggleButtonSkillPicker() {
  if (hasPendingApproval.value || isTurnRunning.value) return
  closeSlashPicker()
  closeExpertButtonPicker()
  atMenuOpen.value = false
  buttonPickerOpen.value = !buttonPickerOpen.value
}

function toggleButtonExpertPicker() {
  if (hasPendingApproval.value || isTurnRunning.value) return
  if (!canDelegateExperts.value) {
    toast.warning(t('composer.expertNeedDelegate'))
    return
  }
  closeSlashPicker()
  buttonPickerOpen.value = false
  atMenuOpen.value = false
  expertButtonPickerOpen.value = !expertButtonPickerOpen.value
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

function onPickExpert(agent: Agent) {
  if (!canDelegateExperts.value) {
    toast.warning(t('composer.expertNeedDelegate'))
    return
  }
  if (atMenuOpen.value && atStart.value >= 0) {
    const ta = getTextarea()
    const caret = ta?.selectionStart ?? content.value.length
    content.value = removeAtSkillQuery(content.value, atStart.value, caret)
    atMenuOpen.value = false
    atQuery.value = ''
    atStart.value = -1
    if (!selectedExpertIds.value.includes(agent.id)) {
      selectedExpertIds.value = [...selectedExpertIds.value, agent.id]
    }
    void nextTick(() => focusInput())
    return
  }
  if (selectedExpertIds.value.includes(agent.id)) {
    selectedExpertIds.value = selectedExpertIds.value.filter((id) => id !== agent.id)
  } else {
    selectedExpertIds.value = [...selectedExpertIds.value, agent.id]
  }
}

async function onPickSlash(cmd: ComposerSlashCommand) {
  if (slashStart.value >= 0) {
    const ta = getTextarea()
    const caret = ta?.selectionStart ?? content.value.length
    content.value = removeSlashQuery(content.value, slashStart.value, caret)
  }
  closeSlashPicker()
  switch (cmd.id) {
    case 'changes':
      workspaceUi.setRightTab('changes')
      break
    case 'plan':
      workspaceUi.setRightTab('plan')
      break
    case 'files':
      workspaceUi.setRightTab('files')
      break
    case 'memory':
      workspaceUi.setRightTab('memory')
      break
    case 'tables':
      workspaceUi.setRightTab('tables')
      break
    case 'terminal':
      workspaceUi.setRightTab('terminal')
      break
    case 'approve':
      emit('jump-pending')
      break
    case 'stop':
      if (sessions.runningTurnId) await stop()
      break
    case 'new':
      sessions.startCompose(sessions.selectedProjectId)
      break
    case 'novel':
      workspaceUi.openWorkbench('novel')
      break
    default:
      break
  }
  void nextTick(() => focusInput())
}

function removeSelectedSkill(id: string) {
  selectedSkillIds.value = selectedSkillIds.value.filter((x) => x !== id)
}

function removeSelectedExpert(id: string) {
  selectedExpertIds.value = selectedExpertIds.value.filter((x) => x !== id)
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

function updateFileAttachment(id: string, patch: Partial<FileComposerAttachment>) {
  attachments.value = attachments.value.map((a): ComposerAttachment => {
    if (a.kind === 'file' && a.id === id) return { ...a, ...patch }
    return a
  })
}

async function addLocalFile(file: File) {
  if (file.type.startsWith('image/')) {
    addImageFile(file)
    return
  }
  const projectId = sessions.selectedProjectId
  if (!projectId) {
    toast.warning(t('composer.needProject'))
    return
  }
  if (file.size > MAX_FILE_ATTACHMENT_BYTES) {
    toast.warning(t('composer.attachFileTooLarge', { max: '50 MB' }))
    return
  }
  const id = createComposerAttachmentId('file')
  attachments.value = [
    ...attachments.value,
    {
      id,
      kind: 'file',
      name: file.name,
      mime: file.type || 'application/octet-stream',
      size: file.size,
      status: 'uploading',
    },
  ]
  focusInput()
  try {
    const res = await uploadProjectFile(projectId, file)
    updateFileAttachment(id, { status: 'ready', remotePath: res.path, error: undefined })
    workspaceUi.requestFilesReload()
    toast.success(t('composer.attachFileUploaded', { path: res.path }))
  } catch (e) {
    const message = e instanceof Error ? e.message : t('composer.attachFileUploadFailed')
    updateFileAttachment(id, { status: 'error', error: message })
    toast.error(message)
  }
}

function fileAttachmentsReady(): boolean {
  const files = attachments.value.filter((a): a is FileComposerAttachment => a.kind === 'file')
  if (!files.length) return true
  if (files.some((f) => f.status === 'uploading')) {
    toast.warning(t('composer.attachStillUploading'))
    return false
  }
  if (files.some((f) => f.status !== 'ready' || !f.remotePath)) {
    toast.warning(t('composer.attachUploadIncomplete'))
    return false
  }
  return true
}

async function buildOutgoing() {
  let text = buildComposerUserInput(content.value, attachments.value)
  // Experts first (collaboration), then skills, then user body.
  if (canDelegateExperts.value && selectedExperts.value.length) {
    text = prependExpertSummon(
      text,
      selectedExperts.value,
      (name, id) => t('composer.useExpertLine', { name, id }),
      t('composer.delegateExpertHint'),
    )
  }
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
  return { text, imageAtts }
}

async function send() {
  if (primaryAction.value === 'queue') {
    await queue()
    return
  }
  if (hasPendingApproval.value) {
    toast.warning(t('sessions.pendingApprovalHint'))
    return
  }
  if (isTurnRunning.value) {
    toast.warning(t('composer.queueWhileRunningHint'))
    return
  }
  if (anyPickerOpen.value) {
    closeAllPickers()
  }
  if (!fileAttachmentsReady()) return
  const { text, imageAtts } = await buildOutgoing()
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
    const stagePath = workspaceUi.stage?.path
    const snapKinds = new Set(['doc', 'slides', 'sheet', 'code'])
    const officePaths = officeEditSnapshotPaths(
      attachments.value.filter((a): a is OfficeComposerAttachment => a.kind === 'office').map((a) => a.data),
    )
    const paths = new Set<string>(officePaths)
    if (stagePath && workspaceUi.stage && snapKinds.has(workspaceUi.stage.kind)) {
      paths.add(stagePath)
    }
    const snapshotPaths = paths.size ? [...paths] : undefined
    await sessions.sendTurn(text, imageAtts, snapshotPaths ? { snapshotPaths } : undefined)
    clearComposer()
    focusInput()
  } catch (e) {
    toast.error(e instanceof Error ? e.message : t('composer.sendFailed'))
  }
}

async function queue() {
  if (!canQueue.value) {
    if (hasPendingApproval.value && !hasDraft.value) {
      toast.warning(t('sessions.pendingApprovalHint'))
    }
    return
  }
  if (anyPickerOpen.value) {
    closeAllPickers()
  }
  if (!fileAttachmentsReady()) return
  const { text, imageAtts } = await buildOutgoing()
  if (!text.trim() && !imageAtts.length) return
  try {
    await sessions.enqueuePending(text, imageAtts)
    clearComposer()
    focusInput()
    emit('queued')
    toast.success(t('composer.queued'))
  } catch (e) {
    toast.error(e instanceof Error ? e.message : t('composer.queueFailed'))
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
  if (slashMenuOpen.value && (e.key === 'ArrowDown' || e.key === 'ArrowUp' || e.key === 'Escape')) {
    slashPickerRef.value?.onKeydown(e)
    return
  }
  if (slashMenuOpen.value && e.key === 'Enter' && !e.shiftKey && !e.metaKey && !e.ctrlKey) {
    slashPickerRef.value?.onKeydown(e)
    return
  }
  // @ menu: prefer skills when they match; otherwise route keys to experts.
  if (atMenuOpen.value) {
    const preferExperts =
      canDelegateExperts.value &&
      atExpertMatches.value.length > 0 &&
      availableSkills.value.filter((s) => {
        const q = atQuery.value.trim().toLowerCase()
        if (!q) return true
        const hay = [s.id, s.name, s.description ?? ''].join('\n').toLowerCase()
        return hay.includes(q)
      }).length === 0
    const target = preferExperts ? expertPickerRef.value : skillPickerRef.value
    if (e.key === 'ArrowDown' || e.key === 'ArrowUp' || e.key === 'Escape') {
      target?.onKeydown(e)
      return
    }
    if (e.key === 'Enter' && !e.shiftKey && !e.metaKey && !e.ctrlKey) {
      target?.onKeydown(e)
      return
    }
  }
  if (expertButtonPickerOpen.value && (e.key === 'ArrowDown' || e.key === 'ArrowUp' || e.key === 'Escape')) {
    expertPickerRef.value?.onKeydown(e)
    return
  }
  if (expertButtonPickerOpen.value && e.key === 'Enter' && !e.shiftKey && !e.metaKey && !e.ctrlKey) {
    expertPickerRef.value?.onKeydown(e)
    return
  }
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
    if (primaryAction.value === 'queue') void queue()
    else if (isTurnRunning.value && !hasDraft.value) void stop()
    else void send()
    return
  }
  if (e.key === 'Enter' && !e.shiftKey) {
    e.preventDefault()
    if (primaryAction.value === 'queue') void queue()
    else if (isTurnRunning.value && !hasDraft.value) void stop()
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
  dragOver.value = true
}

function onDrop(e: DragEvent) {
  e.preventDefault()
  dragOver.value = false
  const files = Array.from(e.dataTransfer?.files ?? [])
  if (!files.length) return
  for (const f of files) addLocalFile(f)
}

defineExpose({
  focusInput,
  appendContent,
  addElementAttachment,
  addCodeSelectionAttachment,
  addOfficeEditAttachment,
})
</script>

<template>
  <div
    class="composer-float"
    :class="{
      'is-dragover': dragOver,
      'is-compose': sessions.composingNew,
      'is-skill-picker-open': skillPickerOpen || expertPickerOpen,
      'is-slash-picker-open': slashMenuOpen,
    }"
    role="form"
    aria-label="Session composer"
    @dragover.prevent="onDragOver"
    @dragleave.prevent="dragOver = false"
    @drop="onDrop"
  >
    <!-- Outside the glass card: card has overflow:hidden and would clip this popover. -->
    <div
      v-if="atMenuOpen"
      class="composer-at-pop"
    >
      <ComposerSkillPicker
        ref="skillPickerRef"
        :skills="availableSkills"
        :selected-ids="selectedSkillIds"
        :query="atQuery"
        :show-search="false"
        :loading="availableSkillsLoading"
        @select="onPickSkill"
        @close="closeAllPickers"
      />
      <ComposerExpertPicker
        v-if="canDelegateExperts"
        ref="expertPickerRef"
        :agents="sessions.agents"
        :selected-ids="selectedExpertIds"
        :exclude-agent-id="sessions.selectedAgentId"
        :query="atQuery"
        :show-search="false"
        hide-close
        @select="onPickExpert"
        @close="closeAllPickers"
      />
    </div>
    <div
      v-else-if="buttonPickerOpen"
      class="composer-skill-pop composer-skill-pop--button"
    >
      <ComposerSkillPicker
        ref="skillPickerRef"
        :skills="availableSkills"
        :selected-ids="selectedSkillIds"
        :query="''"
        :show-search="true"
        :loading="availableSkillsLoading"
        @select="onPickSkill"
        @close="closeSkillPickers"
      />
    </div>
    <div
      v-else-if="expertButtonPickerOpen && canDelegateExperts"
      class="composer-expert-pop composer-expert-pop--button"
    >
      <ComposerExpertPicker
        ref="expertPickerRef"
        :agents="sessions.agents"
        :selected-ids="selectedExpertIds"
        :exclude-agent-id="sessions.selectedAgentId"
        :show-search="true"
        @select="onPickExpert"
        @close="closeExpertButtonPicker"
      />
    </div>
    <div v-if="slashMenuOpen" class="composer-slash-pop">
      <ComposerSlashPicker
        ref="slashPickerRef"
        :commands="COMPOSER_SLASH_COMMANDS"
        :query="slashQuery"
        @select="onPickSlash"
        @close="closeSlashPicker"
      />
    </div>

    <!-- Upper card: input + model/effort/send -->
    <div class="composer-float__card dq-glass--composer">
      <div v-if="dragOver" class="composer-float__drop">{{ t('composer.dropHint') }}</div>

      <div v-if="isTurnRunning" class="composer-float__banner composer-float__banner--run">
        <span class="composer-float__run-dot" />
        {{ t('composer.running') }}
        <template v-if="hasPendingApproval">
          <span class="composer-float__banner-sep">·</span>
          <span class="composer-float__banner-text">{{
            t('sessions.pendingApprovalHintCount', { n: workspaceUi.pendingApprovals })
          }}</span>
        </template>
      </div>
      <!-- Pending cards sit in ComposerPendingDecisions above — no “go handle” jump here. -->
      <div
        v-else-if="hasPendingApproval"
        class="composer-float__banner composer-float__banner--warn"
      >
        <span class="composer-float__banner-text">{{
          t('sessions.pendingApprovalHintCount', { n: workspaceUi.pendingApprovals })
        }}</span>
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

      <ComposerExpertChips
        :experts="selectedExperts"
        @remove="removeSelectedExpert"
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
            :placeholder="hasPendingApproval || isTurnRunning ? t('composer.placeholderQueue') : placeholder"
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
          <button
            type="button"
            class="composer-tool-btn composer-tool-btn--expert"
            :class="{
              'is-active': expertButtonPickerOpen || selectedExpertIds.length,
              'is-disabled-hint': !canDelegateExperts,
            }"
            :title="canDelegateExperts ? t('composer.selectExpert') : t('composer.expertNeedDelegate')"
            :aria-label="t('composer.selectExpert')"
            :aria-expanded="expertButtonPickerOpen"
            :disabled="isTurnRunning"
            @click="toggleButtonExpertPicker"
          >
            <svg viewBox="0 0 24 24" width="16" height="16" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true">
              <path d="M16 21v-2a4 4 0 0 0-4-4H6a4 4 0 0 0-4 4v2" />
              <circle cx="9" cy="7" r="4" />
              <path d="M22 21v-2a4 4 0 0 0-3-3.87" />
              <path d="M16 3.13a4 4 0 0 1 0 7.75" />
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
            v-if="primaryAction === 'queue'"
            type="button"
            class="composer-send composer-send--queue"
            :disabled="!canQueue || sessions.loading"
            :aria-label="t('composer.queue')"
            @click="queue"
          >
            <span>{{ t('composer.queue') }}</span>
            <kbd class="composer-send__kbd">↵</kbd>
          </button>
          <button
            v-else-if="!isTurnRunning"
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

      <!-- Meta strip inside the same glass capsule (no second plate) -->
      <div
        v-if="showTray"
        class="composer-float__tray"
      >
        <div class="composer-float__tray-leading">
      <div
        v-if="sessions.composingNew"
        class="composer-select composer-select--project composer-tray-segment"
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

          <div
            v-if="showAgentSelect"
            class="composer-agent-picker composer-tray-segment"
          >
            <DqSegmented
              v-if="useAgentSegmented"
              v-model="selectedAgentModel"
              size="sm"
              class="composer-agent-seg composer-agent-seg--compact"
              :options="agentOptions"
              :aria-label="t('composer.selectAgent')"
              :title="selectedLeadHint"
            />
            <div
              v-else
              class="composer-select composer-select--agent"
            >
              <DqSelect
                v-model="selectedAgentModel"
                size="sm"
                variant="ghost"
                :aria-label="t('composer.selectAgent')"
                :title="selectedLeadHint"
              >
                <DqOption
                  v-for="a in primaryAgents"
                  :key="a.id"
                  :value="a.id"
                  :label="a.name"
                />
              </DqSelect>
            </div>
          </div>

          <div
            class="composer-plan-toggle composer-tray-segment"
            :class="{ 'is-active': sessions.selectedPlanMode }"
          >
            <DqSwitch
              v-model="sessions.selectedPlanMode"
              size="sm"
              :aria-label="t('composer.planModeToggle')"
            />
            <span class="composer-plan-toggle__label">{{ t('composer.planModeLabel') }}</span>
          </div>

          <div
            v-if="gitDisplay"
            class="composer-tray-segment"
          >
            <span
              class="composer-git-branch"
              :class="{ 'is-error': Boolean(gitError) }"
              :title="gitDisplay"
              :aria-label="gitError ? gitError : `${t('composer.gitBranch')}: ${gitBranch}`"
            >
              <span class="composer-git-branch__icon" aria-hidden="true">⎇</span>
              <span class="composer-git-branch__name">{{ gitDisplay }}</span>
            </span>
          </div>
        </div>

        <div class="composer-float__tray-trailing">
          <ContextUsageBar compact />
        </div>
      </div>
    </div>
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

.composer-float__card {
  /* Single glass capsule — tray is an inner strip, not a second plate */
  position: relative;
  z-index: 2;
  isolation: isolate;
  width: 100%;
  display: flex;
  flex-direction: column;
  overflow: hidden;
}

.composer-float__banner-text {
  flex: 1;
  min-width: 0;
}

.composer-float__banner-sep {
  opacity: 0.5;
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

.composer-float__jump--inline {
  margin-left: 0;
  padding: 2px 8px;
  font-size: var(--dq-font-size-caption);
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
  color: var(--dq-label-primary);
  background: var(--dq-accent-tint-strong, color-mix(in srgb, var(--dq-accent) 14%, transparent));
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
  min-height: calc(2.4 * 1.45em) !important;
  height: auto !important;
  max-height: var(--dq-composer-max-height, 240px) !important;
  line-height: 1.45;
  overflow-y: auto;
  field-sizing: content;
}

.composer-float__file-input {
  display: none;
}

.composer-float__footer {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 10px;
  padding: 8px 14px 10px;
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
  display: grid;
  grid-template-columns: minmax(0, 1fr) auto;
  align-items: center;
  column-gap: 12px;
  width: 100%;
  margin: 0;
  /* Symmetric inset — keep meta inside the capsule curve (overflow:hidden) */
  padding: 6px 18px;
  min-width: 0;
  min-height: 40px;
  box-sizing: border-box;
  /* Hairline only — inherits capsule glass, no fill/shadow/blur of its own */
  background: transparent;
  border: none;
  border-top: 1px solid color-mix(in srgb, var(--dq-label-primary) 8%, transparent);
  box-shadow: none;
  backdrop-filter: none;
  -webkit-backdrop-filter: none;
}

.composer-float__tray-leading {
  display: flex;
  flex-wrap: nowrap;
  align-items: center;
  gap: 8px;
  min-width: 0;
  height: 28px;
  overflow-x: auto;
  scrollbar-width: none;
}

.composer-float__tray-leading::-webkit-scrollbar {
  display: none;
}

.composer-float__tray-trailing {
  display: flex;
  align-items: center;
  justify-content: flex-end;
  height: 28px;
  flex: 0 0 auto;
}

.composer-float__tray :deep(.context-usage),
.composer-float__tray :deep(.context-usage__main) {
  height: 28px;
  display: inline-flex;
  align-items: center;
}

.composer-float__tray :deep(.context-usage__label),
.composer-float__tray :deep(.context-usage__icon) {
  color: var(--dq-label-secondary);
}

.composer-float__tray :deep(.context-usage__pct) {
  color: var(--dq-label-tertiary);
}

.composer-git-branch {
  display: inline-flex;
  align-items: center;
  gap: 4px;
  flex: 0 1 140px;
  max-width: 200px;
  min-width: 64px;
  height: 28px;
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

.composer-agent-picker {
  display: flex;
  align-items: center;
  flex-shrink: 0;
  overflow: visible;
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
  flex: 0 1 120px;
  min-width: 72px;
  max-width: 160px;
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

.composer-skill-pop,
.composer-expert-pop,
.composer-at-pop {
  position: absolute;
  left: 10px;
  right: 10px;
  /* Sit above the whole composer (card + tray), not inside the clipped card. */
  bottom: calc(100% + 8px);
  z-index: 60;
  display: flex;
  justify-content: flex-start;
  pointer-events: none;
}

.composer-at-pop {
  flex-direction: column;
  align-items: stretch;
  gap: 6px;
  max-height: min(520px, 70vh);
  overflow: auto;
}

.composer-slash-pop {
  position: absolute;
  left: 10px;
  right: 10px;
  bottom: calc(100% + 8px);
  z-index: 61;
  display: flex;
  justify-content: flex-start;
  pointer-events: none;
}

.composer-slash-pop > *,
.composer-skill-pop > *,
.composer-expert-pop > *,
.composer-at-pop > * {
  pointer-events: auto;
}

.composer-skill-pop--button,
.composer-expert-pop--button {
  left: 10px;
  right: auto;
  width: min(320px, calc(100% - 20px));
}

.composer-tool-btn--expert.is-disabled-hint {
  opacity: 0.45;
}

.composer-send {
  display: inline-flex;
  align-items: center;
  gap: 8px;
  height: 32px;
  margin-left: 4px;
  padding: 0 12px 0 14px;
  border: none;
  border-radius: 999px;
  background: var(--dq-accent);
  color: var(--dq-on-accent);
  font-size: var(--dq-font-size-footnote);
  font-weight: 650;
  cursor: pointer;
  box-shadow: 0 1px 6px var(--dq-accent-shadow-cta, transparent);
  transition: opacity 0.15s ease, transform 0.12s ease, filter 0.12s ease;
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

.composer-send--queue {
  background: color-mix(in srgb, var(--dq-accent) 82%, #000);
}

.composer-send__kbd {
  display: inline-flex;
  align-items: center;
  height: 18px;
  padding: 0 5px;
  border-radius: 5px;
  background: color-mix(in srgb, var(--dq-on-accent) 18%, transparent);
  font-family: var(--dq-font-mono, ui-monospace, monospace);
  font-size: var(--dq-font-size-caption);
  font-weight: 600;
  opacity: 0.9;
}

.composer-float__tray-leading {
  gap: 0;
}

.composer-float__tray-leading > .composer-tray-segment {
  display: flex;
  align-items: center;
  min-height: 28px;
  padding: 0 8px;
}

.composer-float__tray-leading > .composer-tray-segment:not(:first-child) {
  border-left: 1px solid color-mix(in srgb, var(--dq-label-primary) 8%, transparent);
}

.composer-plan-toggle {
  display: flex;
  align-items: center;
  gap: 6px;
  color: var(--dq-label-secondary);
  font-size: var(--dq-font-size-caption);
  font-weight: 500;
  cursor: pointer;
  transition: color 0.12s ease;
}

.composer-plan-toggle.is-active {
  color: var(--dq-accent);
}

.composer-plan-toggle__label {
  white-space: nowrap;
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
