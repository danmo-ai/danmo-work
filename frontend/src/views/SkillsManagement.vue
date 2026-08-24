<script setup lang="ts">
import { computed, nextTick, onMounted, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { useSkillsStore } from '@/stores/skills'
import { useProjectsStore } from '@/stores/projects'
import { confirm, toast } from '@/utils/feedback'
import type { Skill, SkillFile } from '@/types'
import MdEditor from '@/components/common/MdEditor.vue'
import WorkspaceShell from '@/components/common/WorkspaceShell.vue'
import MarketBrowser from '@/components/market/MarketBrowser.vue'
import MarketCatalogRail from '@/components/market/MarketCatalogRail.vue'
import { useMarketStore } from '@/stores/market'

type SkillTab = 'info' | 'body' | 'files' | 'tools'
type PageView = 'library' | 'market'

const { t, locale } = useI18n()
const store = useSkillsStore()
const projectsStore = useProjectsStore()
const marketStore = useMarketStore()

const pageView = ref<PageView>('library')
const pageViewOptions = computed(() => [
  { label: t('market.library'), value: 'library' as const },
  { label: t('market.tab'), value: 'market' as const },
])
const selectedId = ref<string | null>(null)
const marketSelectedKey = ref<string | null>(null)
const isCreating = ref(false)
const saving = ref(false)
const activeTab = ref<SkillTab>('info')
const pendingKeyword = ref('')
const pendingToolId = ref('')
const showImportDialog = ref(false)
const importPath = ref('')
const skillFiles = ref<SkillFile[]>([])
const viewingFile = ref<SkillFile | null>(null)
const showViewerDialog = ref(false)
const showFileEditor = ref(false)
const fileEditorMode = ref<'create' | 'edit'>('edit')
const fileEditorPath = ref('')
const fileEditorContent = ref('')
const fileEditorSaving = ref(false)
const fileEditorOriginalPath = ref('')

const skillForm = ref<Skill>(emptySkill())

function emptySkill(): Skill {
  return {
    id: '',
    name: '',
    description: '',
    license: '',
    compatibility: '',
    metadata: {},
    allowedTools: '',
    keywords: [],
    toolIds: [],
    systemHint: '',
    body: '',
    sourcePath: '',
  }
}

const sortedSkills = computed(() =>
  [...store.items].sort((a, b) => a.name.localeCompare(b.name, locale.value)),
)
// Builtin = has embedded template (server marks builtin at read time). Keep
// those in the Built-in group even if a market overwrite set marketSource.
const builtinSkills = computed(() =>
  sortedSkills.value.filter((s) => s.builtin),
)
const marketSkills = computed(() =>
  sortedSkills.value.filter((s) => !!s.marketSource && !s.builtin),
)
const customSkills = computed(() =>
  sortedSkills.value.filter((s) => !s.builtin && !s.marketSource && !s.projectId),
)
const projectSkills = computed(() =>
  sortedSkills.value.filter((s) => !!s.projectId),
)
const projectSkillGroups = computed(() => {
  const groups = new Map<string, Skill[]>()
  for (const s of projectSkills.value) {
    const pid = s.projectId!
    if (!groups.has(pid)) groups.set(pid, [])
    groups.get(pid)!.push(s)
  }
  return groups
})

function projectName(id: string) {
  return projectsStore.projects.find((p) => p.id === id)?.name ?? id
}

async function loadSkills() {
  const projectIds = projectsStore.sortedProjects.map((p) => p.id)
  await store.load(projectIds)
}

async function onMarketUninstalled() {
  await loadSkills()
  if (selectedId.value && !store.items.some((s) => s.id === selectedId.value)) {
    selectedId.value = null
  }
}
const selectedSkill = computed(() =>
  selectedId.value ? store.items.find((s) => s.id === selectedId.value) : null,
)
const marketSelected = computed(() => {
  if (!marketSelectedKey.value) return null
  return (
    marketStore.catalog.find(
      (item) => item.kind === 'skill' && `${item.sourceId}:${item.id}` === marketSelectedKey.value,
    ) ?? null
  )
})
const hasSelection = computed(
  () =>
    (pageView.value === 'market' && !!marketSelectedKey.value) ||
    isCreating.value ||
    !!selectedId.value,
)
const headerTitle = computed(() => {
  if (pageView.value === 'market') {
    return marketSelected.value?.name || t('market.tab')
  }
  if (isCreating.value) return t('skills.newSkill')
  if (selectedSkill.value) return selectedSkill.value.name || t('skills.untitled')
  return ''
})

async function onMarketInstalled() {
  await loadSkills()
}

const skillTabs = computed(() => [
  { label: t('common.basicInfo'), value: 'info' as const },
  { label: t('skills.instructions'), value: 'body' as const },
  {
    label: skillFiles.value.length
      ? `${t('skills.files')} (${skillFiles.value.length})`
      : t('skills.files'),
    value: 'files' as const,
  },
  {
    label: (skillForm.value.toolIds ?? []).length
      ? `${t('common.tools')} (${(skillForm.value.toolIds ?? []).length})`
      : t('common.tools'),
    value: 'tools' as const,
  },
])

const metadataEntries = computed(() => {
  return Object.entries(skillForm.value.metadata ?? {}).map(([k, v]) => ({ key: k, value: v }))
})

onMounted(() => {
  loadSkills()
  if (!selectedId.value && sortedSkills.value.length) {
    selectSkill(sortedSkills.value[0].id)
  }
})

watch(selectedId, async (id) => {
  if (id) {
    skillFiles.value = await store.getFiles(id)
  } else {
    skillFiles.value = []
  }
})

function selectSkill(id: string) {
  isCreating.value = false
  selectedId.value = id
  activeTab.value = 'info'
  const skill = store.items.find((s) => s.id === id)
  if (skill) {
    skillForm.value = { ...skill, keywords: skill.keywords ? [...skill.keywords] : [], toolIds: skill.toolIds ? [...skill.toolIds] : [] }
  }
}

function startCreate() {
  isCreating.value = true
  selectedId.value = null
  skillForm.value = emptySkill()
  activeTab.value = 'info'
  skillFiles.value = []
}

function initial(name: string) {
  return name.trim().charAt(0).toUpperCase() || '?'
}

async function save() {
  const name = skillForm.value.name.trim()
  if (!name) {
    toast.warning(t('skills.namePlaceholder'))
    return
  }
  saving.value = true
  try {
    if (isCreating.value) {
      const created = await store.create({ ...skillForm.value, id: name, name })
      toast.success(t('skills.created'))
      isCreating.value = false
      selectedId.value = created.id
    } else if (selectedId.value) {
      const updated = await store.update(selectedId.value, { ...skillForm.value })
      skillForm.value = {
        ...updated,
        keywords: updated.keywords ? [...updated.keywords] : [],
        toolIds: updated.toolIds ? [...updated.toolIds] : [],
      }
      toast.success(t('skills.saved'))
    }
  } catch (e) {
    toast.error(e instanceof Error ? e.message : t('common.saveFailed'))
  } finally {
    saving.value = false
  }
}

async function removeSelected() {
  if (!selectedSkill.value) return
  try {
    await confirm(t('skills.deleteConfirm', { name: selectedSkill.value.name }), t('skills.deleteTitle'), { type: 'warning' })
    await store.remove(selectedSkill.value.id)
    selectedId.value = null
    toast.success(t('skills.deleted'))
  } catch (e) {
    if (e instanceof Error) toast.error(e.message)
  }
}

async function resetSelected() {
  if (!selectedSkill.value) return
  try {
    await confirm(t('skills.resetConfirm', { name: selectedSkill.value.name }), t('skills.resetTitle'), { type: 'warning' })
  } catch {
    return
  }
  try {
    const s = await store.reset(selectedSkill.value.id)
    selectSkill(s.id)
    skillFiles.value = await store.getFiles(s.id)
    toast.success(t('skills.reset'))
  } catch (e) {
    toast.error(e instanceof Error ? e.message : t('skills.resetFailed'))
  }
}

async function refreshSelectedStatus() {
  if (!selectedId.value) return
  const updated = await store.refresh(selectedId.value)
  if (updated && selectedId.value === updated.id && !isCreating.value) {
    skillForm.value = {
      ...updated,
      keywords: updated.keywords ? [...updated.keywords] : [],
      toolIds: updated.toolIds ? [...updated.toolIds] : [],
    }
  }
}

async function doImport() {
  if (!importPath.value.trim()) {
    toast.warning(t('skills.importPathRequired'))
    return
  }
  try {
    const result = await store.importDir(importPath.value.trim())
    showImportDialog.value = false
    importPath.value = ''
    toast.success(t('skills.importSuccess', { name: result.skill.name, count: result.fileCount }))
    selectSkill(result.skill.id)
  } catch (e) {
    toast.error(e instanceof Error ? e.message : t('skills.importFailed'))
  }
}

function addKeyword() {
  if (!pendingKeyword.value.trim()) return
  skillForm.value.keywords = [...(skillForm.value.keywords ?? []), pendingKeyword.value.trim()]
  pendingKeyword.value = ''
}

function removeKeyword(idx: number) {
  skillForm.value.keywords = (skillForm.value.keywords ?? []).filter((_, i) => i !== idx)
}

function addSkillTool() {
  if (!pendingToolId.value.trim()) return
  const ids = skillForm.value.toolIds ?? []
  if (!ids.includes(pendingToolId.value.trim())) {
    skillForm.value.toolIds = [...ids, pendingToolId.value.trim()]
  }
  pendingToolId.value = ''
}

function removeSkillTool(id: string) {
  skillForm.value.toolIds = (skillForm.value.toolIds ?? []).filter((x) => x !== id)
}

function addMetadata() {
  skillForm.value.metadata = { ...(skillForm.value.metadata ?? {}), '': '' }
}

function updateMetadataKey(oldKey: string, newKey: string, idx: number) {
  const entries = Object.entries(skillForm.value.metadata ?? {})
  entries[idx] = [newKey, entries[idx][1]]
  skillForm.value.metadata = Object.fromEntries(entries)
}

function updateMetadataValue(key: string, value: string, idx: number) {
  const entries = Object.entries(skillForm.value.metadata ?? {})
  entries[idx] = [entries[idx][0], value]
  skillForm.value.metadata = Object.fromEntries(entries)
}

function removeMetadata(key: string) {
  const m = { ...(skillForm.value.metadata ?? {}) }
  delete m[key]
  skillForm.value.metadata = m
}

async function viewFileContent(file: SkillFile) {
  try {
    const content = await store.getFileContent(file.skillId, file.path)
    viewingFile.value = { ...file, content }
    showViewerDialog.value = true
  } catch {
    toast.error(t('skills.loadFileFailed'))
  }
}

function onViewerClose(open: boolean) {
  if (!open) {
    viewingFile.value = null
    showViewerDialog.value = false
  }
}

function startCreateFile() {
  if (!selectedId.value || isCreating.value) {
    toast.warning(t('skills.emptySelection'))
    return
  }
  fileEditorMode.value = 'create'
  fileEditorPath.value = 'references/'
  fileEditorOriginalPath.value = ''
  fileEditorContent.value = ''
  showFileEditor.value = true
}

async function startEditFile(file: SkillFile) {
  try {
    const content = await store.getFileContent(file.skillId, file.path)
    fileEditorMode.value = 'edit'
    fileEditorPath.value = file.path
    fileEditorOriginalPath.value = file.path
    fileEditorContent.value = content
    showFileEditor.value = true
  } catch {
    toast.error(t('skills.loadFileFailed'))
  }
}

function closeFileEditor() {
  showFileEditor.value = false
  fileEditorPath.value = ''
  fileEditorContent.value = ''
  fileEditorOriginalPath.value = ''
}

async function saveFileEditor() {
  if (!selectedId.value) return
  const path = fileEditorPath.value.trim()
  if (!path) {
    toast.warning(t('skills.filePathPlaceholder'))
    return
  }
  fileEditorSaving.value = true
  try {
    if (
      fileEditorMode.value === 'edit' &&
      fileEditorOriginalPath.value &&
      fileEditorOriginalPath.value !== path
    ) {
      await store.deleteFile(selectedId.value, fileEditorOriginalPath.value)
    }
    await store.upsertFile(selectedId.value, path, fileEditorContent.value)
    skillFiles.value = await store.getFiles(selectedId.value)
    await refreshSelectedStatus()
    toast.success(
      fileEditorMode.value === 'create' ? t('skills.fileCreated') : t('skills.fileSaved'),
    )
    closeFileEditor()
  } catch (e) {
    toast.error(e instanceof Error ? e.message : t('skills.saveFileFailed'))
  } finally {
    fileEditorSaving.value = false
  }
}

async function removeFile(file: SkillFile) {
  if (!selectedId.value) return
  try {
    await confirm(t('skills.deleteFileConfirm', { path: file.path }), t('skills.deleteFileTitle'), {
      type: 'warning',
    })
  } catch {
    return
  }
  try {
    await store.deleteFile(selectedId.value, file.path)
    skillFiles.value = await store.getFiles(selectedId.value)
    await refreshSelectedStatus()
    toast.success(t('skills.fileDeleted'))
  } catch (e) {
    toast.error(e instanceof Error ? e.message : t('skills.saveFileFailed'))
  }
}

async function exportSelected() {
  if (!selectedSkill.value) return
  try {
    const md = await store.getExportMD(selectedSkill.value.id)
    const blob = new Blob([md], { type: 'text/markdown;charset=utf-8' })
    const url = URL.createObjectURL(blob)
    const a = document.createElement('a')
    a.href = url
    a.download = 'SKILL.md'
    a.click()
    URL.revokeObjectURL(url)
    toast.success(t('skills.exportSuccess'))
  } catch (e) {
    toast.error(e instanceof Error ? e.message : t('skills.exportFailed'))
  }
}

function onFileEditorClose(open: boolean) {
  if (!open) closeFileEditor()
}

function isEditableTarget(target: EventTarget | null) {
  return target instanceof HTMLElement && !!target.closest('input, textarea, select, [contenteditable="true"]')
}

function scrollActiveRailRowIntoView() {
  nextTick(() => {
    document.querySelector('.resource-rail__row.is-active')?.scrollIntoView({ block: 'nearest' })
  })
}

function onKeydown(e: KeyboardEvent) {
  if ((e.metaKey || e.ctrlKey) && e.key === 's') {
    e.preventDefault()
    if (showFileEditor.value) {
      saveFileEditor()
      return
    }
    save()
    return
  }

  if (pageView.value !== 'library' || isCreating.value || isEditableTarget(e.target)) return
  if (e.key !== 'ArrowDown' && e.key !== 'ArrowUp') return

  const items = sortedSkills.value
  if (!items.length) return

  e.preventDefault()
  const idx = items.findIndex((s) => s.id === selectedId.value)
  const nextIdx =
    e.key === 'ArrowDown'
      ? idx < 0
        ? 0
        : Math.min(idx + 1, items.length - 1)
      : idx < 0
        ? items.length - 1
        : Math.max(idx - 1, 0)
  selectSkill(items[nextIdx].id)
  scrollActiveRailRowIntoView()
}

function formatSize(bytes: number): string {
  if (!bytes) return '0 B'
  const units = ['B', 'KB', 'MB', 'GB']
  let i = 0
  let size = bytes
  while (size >= 1024 && i < units.length - 1) { size /= 1024; i++ }
  return `${size.toFixed(i === 0 ? 0 : 1)} ${units[i]}`
}
</script>

<template>
  <WorkspaceShell
    custom-rail
    :has-selection="hasSelection"
    @keydown="onKeydown"
    @create="startCreate"
  >
    <template #rail>
      <div class="resource-rail__section">
        <div class="resource-rail__section-head">
          <DqSegmented v-model="pageView" block class="resource-rail__page-view" :options="pageViewOptions" />
        </div>
        <template v-if="pageView === 'library'">
        <div class="resource-rail__section-head">
          <span class="resource-rail__section-title">{{ $t('skills.title') }}</span>
          <div class="resource-rail__section-actions">
            <DqIconButton :aria-label="$t('skills.import')" @click="showImportDialog = true">
              <svg viewBox="0 0 24 24" width="14" height="14" fill="none" stroke="currentColor" stroke-width="2">
                <path d="M21 15v4a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2v-4M7 10l5 5 5-5M12 15V3" stroke-linecap="round" stroke-linejoin="round" />
              </svg>
            </DqIconButton>
            <DqIconButton :aria-label="$t('skills.newSkill')" @click="startCreate">
              <svg viewBox="0 0 24 24" width="14" height="14" fill="none" stroke="currentColor" stroke-width="2">
                <path d="M12 5v14M5 12h14" stroke-linecap="round" />
              </svg>
            </DqIconButton>
          </div>
        </div>
        <DqEmpty v-if="!sortedSkills.length" class="resource-rail__empty" :description="$t('skills.noSkills')" />
        <div v-else class="resource-rail__scroll">
          <div v-if="builtinSkills.length" class="resource-rail__group">
            <div class="resource-rail__group-title">{{ $t('skills.builtinSkills') }}</div>
            <nav class="resource-rail__list" :aria-label="$t('skills.builtinSkills')">
              <button
                v-for="skill in builtinSkills"
                :key="skill.id"
                type="button"
                class="resource-rail__row"
                :class="{ 'is-active': selectedSkill?.id === skill.id && !isCreating }"
                @click="selectSkill(skill.id)"
              >
                <span class="resource-rail__avatar">{{ initial(skill.name) }}</span>
                <span class="resource-rail__meta">
                  <span class="resource-rail__name-row">
                    <span class="resource-rail__name">{{ skill.name }}</span>
                    <span
                      v-if="skill.templateDiverged"
                      class="resource-rail__dot"
                      :title="$t('skills.templateDiverged')"
                      aria-hidden="true"
                    />
                  </span>
                </span>
              </button>
            </nav>
          </div>
          <div v-if="customSkills.length" class="resource-rail__group">
            <div class="resource-rail__group-title">{{ $t('skills.customSkills') }}</div>
            <nav class="resource-rail__list" :aria-label="$t('skills.customSkills')">
              <button
                v-for="skill in customSkills"
                :key="skill.id"
                type="button"
                class="resource-rail__row"
                :class="{ 'is-active': selectedSkill?.id === skill.id && !isCreating }"
                @click="selectSkill(skill.id)"
              >
                <span class="resource-rail__avatar">{{ initial(skill.name) }}</span>
                <span class="resource-rail__meta">
                  <span class="resource-rail__name">{{ skill.name }}</span>
                  <span class="resource-rail__desc">{{ skill.sourcePath ? $t('skills.imported') : '' }}</span>
                </span>
              </button>
            </nav>
          </div>
          <div v-if="marketSkills.length" class="resource-rail__group">
            <div class="resource-rail__group-title">{{ $t('skills.marketSkills') }}</div>
            <nav class="resource-rail__list" :aria-label="$t('skills.marketSkills')">
              <button
                v-for="skill in marketSkills"
                :key="skill.id"
                type="button"
                class="resource-rail__row"
                :class="{ 'is-active': selectedSkill?.id === skill.id && !isCreating }"
                @click="selectSkill(skill.id)"
              >
                <span class="resource-rail__avatar">{{ initial(skill.name) }}</span>
                <span class="resource-rail__meta">
                  <span class="resource-rail__name">{{ skill.name }}</span>
                </span>
              </button>
            </nav>
          </div>
          <div v-if="projectSkills.length" class="resource-rail__group">
            <div class="resource-rail__group-title">{{ $t('skills.projectSkills') }}</div>
            <template v-for="[projectId, skills] in [...projectSkillGroups]" :key="projectId as string">
              <div class="resource-rail__group-title resource-rail__group-subtitle">{{ projectName(projectId as string) }}</div>
              <nav class="resource-rail__list">
                <button
                  v-for="skill in (skills as Skill[])"
                  :key="skill.id"
                  type="button"
                  class="resource-rail__row"
                  :class="{ 'is-active': selectedSkill?.id === skill.id && !isCreating }"
                  @click="selectSkill(skill.id)"
                >
                  <span class="resource-rail__avatar">{{ initial(skill.name) }}</span>
                  <span class="resource-rail__meta">
                    <span class="resource-rail__name">{{ skill.name }}</span>
                    <span class="resource-rail__desc">{{ skill.dir }}</span>
                  </span>
                </button>
              </nav>
            </template>
          </div>
        </div>
        </template>
        <MarketCatalogRail v-else v-model:selected-key="marketSelectedKey" kind="skill" />
      </div>
    </template>

    <template #empty>
      <DqEmpty :description="$t('skills.emptySelection')">
        <div class="resource-workspace__empty-actions">
          <DqButton @click="startCreate">{{ $t('skills.newSkill') }}</DqButton>
          <DqButton @click="showImportDialog = true">{{ $t('skills.import') }}</DqButton>
          <DqButton @click="pageView = 'market'">{{ $t('market.tab') }}</DqButton>
        </div>
      </DqEmpty>
    </template>

    <template #header>
      <div class="resource-workspace__identity">
        <h1 class="resource-workspace__title">{{ headerTitle }}</h1>
      </div>
      <DqSegmented
        v-if="pageView === 'library'"
        v-model="activeTab"
        class="resource-workspace__segmented"
        :options="skillTabs"
      />
    </template>

    <template #body>
        <MarketBrowser
          v-if="pageView === 'market'"
          kind="skill"
          :selected-key="marketSelectedKey"
          @installed="onMarketInstalled"
          @uninstalled="onMarketUninstalled"
        />
        <template v-else>
        <div
          v-if="!isCreating && selectedSkill?.builtin && selectedSkill.templateDiverged"
          class="skill-template-banner"
          role="status"
        >
          <div class="skill-template-banner__text">
            <strong>{{ $t('skills.templateDiverged') }}</strong>
            <span>{{ $t('skills.templateDivergedHint') }}</span>
          </div>
          <DqButton size="sm" type="primary" @click="resetSelected">
            {{ $t('skills.resetToTemplate') }}
          </DqButton>
        </div>

        <!-- Info Tab -->
        <section v-show="activeTab === 'info'" class="resource-section">
          <div class="resource-form-grid resource-form-grid--3">
            <label class="resource-field">
              <span class="resource-field__label">{{ $t('common.name') }}</span>
              <DqInput v-model="skillForm.name" :placeholder="$t('skills.namePlaceholder')" />
            </label>
            <label class="resource-field">
              <span class="resource-field__label">{{ $t('skills.path') }}</span>
              <DqInput v-model="skillForm.dir" class="resource-input-mono" readonly :placeholder="$t('skills.pathPlaceholder')" />
            </label>
            <label class="resource-field">
              <span class="resource-field__label">License</span>
              <DqInput v-model="skillForm.license" placeholder="MIT" />
            </label>
          </div>
          <div class="resource-form-grid resource-form-grid--2">
            <label class="resource-field">
              <span class="resource-field__label">Compatibility</span>
              <DqInput v-model="skillForm.compatibility" placeholder="Requires git, python3" />
            </label>
            <label class="resource-field">
              <span class="resource-field__label">Allowed Tools</span>
              <DqInput v-model="skillForm.allowedTools" placeholder="Bash Read" />
            </label>
          </div>
          <div class="resource-form-grid resource-form-grid--2">
            <label class="resource-field">
              <span class="resource-field__label">{{ $t('skills.keywords') }}</span>
              <div class="resource-chip-list">
                <span v-for="(kw, idx) in (skillForm.keywords ?? [])" :key="idx" class="resource-chip">
                  {{ kw }}
                  <button type="button" class="resource-chip__remove" @click="removeKeyword(idx)">×</button>
                </span>
                <input v-model="pendingKeyword" class="resource-chip-list__add" placeholder="+" @keydown.enter.prevent="addKeyword" />
              </div>
            </label>
            <label class="resource-field">
              <span class="resource-field__label">Metadata</span>
              <div class="resource-meta-list">
                <div v-for="(entry, idx) in metadataEntries" :key="idx" class="resource-meta-row">
                  <input v-model="entry.key" class="resource-meta-key" placeholder="key" @change="updateMetadataKey(metadataEntries[idx].key, entry.key, idx)" />
                  <input v-model="entry.value" class="resource-meta-value" placeholder="value" @change="updateMetadataValue(entry.key, entry.value, idx)" />
                  <button type="button" class="resource-meta-remove" @click="removeMetadata(entry.key)">×</button>
                </div>
                <button type="button" class="resource-meta-add" @click="addMetadata">+ {{ $t('skills.add') }}</button>
              </div>
            </label>
          </div>
          <label class="resource-field resource-field--block">
            <span class="resource-field__label">{{ $t('common.description') }}</span>
            <DqInput v-model="skillForm.description" type="textarea" :rows="4" :placeholder="$t('skills.descriptionPlaceholder')" />
          </label>
          <label class="resource-field resource-field--block">
            <span class="resource-field__label">{{ $t('skills.systemHint') }}</span>
            <DqInput v-model="skillForm.systemHint" type="textarea" :rows="4" :placeholder="$t('skills.systemHintPlaceholder')" />
          </label>
        </section>

        <!-- Body Tab (Markdown Editor) -->
        <section v-show="activeTab === 'body'" class="resource-section resource-section--body">
          <MdEditor v-model="skillForm.body" :label="$t('skills.bodyLabel')" :rows="20" :placeholder="$t('skills.bodyPlaceholder')" />
        </section>

        <!-- Files Tab -->
        <section v-show="activeTab === 'files'" class="resource-section">
          <div class="resource-section__toolbar">
            <DqButton
              size="sm"
              :disabled="isCreating || !selectedId"
              @click="startCreateFile"
            >
              {{ $t('skills.addFile') }}
            </DqButton>
          </div>
          <div v-if="!skillFiles.length" class="resource-section__empty">
            <DqEmpty :description="$t('skills.noFiles')" />
          </div>
          <div v-else class="resource-list-card">
            <div v-for="file in skillFiles" :key="file.id" class="resource-list-card__item">
              <div class="resource-list-card__meta">
                <span class="resource-list-card__name">{{ file.path }}</span>
                <span class="resource-list-card__desc">{{ formatSize(file.size) }}</span>
              </div>
              <div class="resource-list-card__actions">
                <DqButton size="sm" @click="viewFileContent(file)">{{ $t('skills.view') }}</DqButton>
                <DqButton size="sm" @click="startEditFile(file)">{{ $t('skills.edit') }}</DqButton>
                <button
                  type="button"
                  class="resource-list-card__action resource-list-card__action--danger"
                  @click="removeFile(file)"
                >
                  {{ $t('common.delete') }}
                </button>
              </div>
            </div>
          </div>
        </section>

        <!-- Tools Tab -->
        <section v-show="activeTab === 'tools'" class="resource-section">
          <div class="resource-form-grid resource-form-grid--2">
            <label class="resource-field">
              <span class="resource-field__label">{{ $t('toolsPage.toolId') }}</span>
              <DqInput v-model="pendingToolId" class="resource-input-mono" placeholder="search_kb" @keydown.enter.prevent="addSkillTool" />
            </label>
            <div class="resource-field resource-field--action">
              <DqButton @click="addSkillTool">{{ $t('common.addTool') }}</DqButton>
            </div>
          </div>
          <div class="resource-list-card">
            <div v-for="id in (skillForm.toolIds ?? [])" :key="id" class="resource-list-card__item">
              <div class="resource-list-card__meta">
                <span class="resource-list-card__name">{{ id }}</span>
              </div>
              <div class="resource-list-card__actions">
                <button type="button" class="resource-list-card__action resource-list-card__action--danger" @click="removeSkillTool(id)">{{ $t('common.delete') }}</button>
              </div>
            </div>
          </div>
        </section>
        </template>
    </template>

    <template #footer>
      <template v-if="pageView === 'library'">
        <span class="resource-workspace__hint">{{ $t('common.saveShortcut') }}</span>
        <div class="resource-workspace__footer-actions">
          <DqButton v-if="isCreating" @click="isCreating = false; selectedId = null">{{ $t('common.cancel') }}</DqButton>
          <DqButton v-if="!isCreating && selectedSkill" @click="exportSelected">{{ $t('skills.export') }}</DqButton>
          <DqButton v-if="!isCreating && selectedSkill?.builtin" @click="resetSelected">{{ $t('common.reset') }}</DqButton>
          <DqButton v-if="!isCreating && !selectedSkill?.builtin" @click="removeSelected">{{ $t('common.delete') }}</DqButton>
          <DqButton type="primary" :disabled="saving" @click="save">
            {{ isCreating ? $t('common.create') : $t('common.save') }}
          </DqButton>
        </div>
      </template>
    </template>
  </WorkspaceShell>

  <!-- File Viewer Dialog -->
  <DqDialog v-model:open="showViewerDialog" :title="viewingFile?.path ?? ''" variant="glass" width="700px" :closable="true" @update:open="onViewerClose">
    <pre class="file-content">{{ viewingFile?.content }}</pre>
  </DqDialog>

  <!-- File Editor Dialog -->
  <DqDialog
    v-model:open="showFileEditor"
    :title="fileEditorMode === 'create' ? $t('skills.addFileTitle') : $t('skills.editFileTitle')"
    variant="glass"
    width="720px"
    :closable="true"
    @update:open="onFileEditorClose"
  >
    <div class="import-form">
      <label class="import-field">
        <span class="import-field__label">{{ $t('skills.filePath') }}</span>
        <DqInput
          v-model="fileEditorPath"
          class="resource-input-mono"
          :placeholder="$t('skills.filePathPlaceholder')"
          :disabled="fileEditorMode === 'edit'"
          spellcheck="false"
        />
        <span class="import-field__hint">{{ $t('skills.filePathHint') }}</span>
      </label>
      <label class="import-field">
        <span class="import-field__label">{{ $t('skills.fileContent') }}</span>
        <DqInput
          v-model="fileEditorContent"
          type="textarea"
          :rows="16"
          :placeholder="$t('skills.fileContentPlaceholder')"
          spellcheck="false"
        />
      </label>
    </div>
    <template #footer>
      <div class="import-actions">
        <DqButton @click="closeFileEditor">{{ $t('common.cancel') }}</DqButton>
        <DqButton type="primary" :disabled="fileEditorSaving" @click="saveFileEditor">
          {{ $t('common.save') }}
        </DqButton>
      </div>
    </template>
  </DqDialog>

  <!-- Import Dialog -->
  <DqDialog v-model:open="showImportDialog" :title="$t('skills.importTitle')" variant="glass" width="460px" :closable="true">
    <div class="import-form">
      <label class="import-field">
        <span class="import-field__label">{{ $t('skills.importPath') }}</span>
        <DqInput v-model="importPath" placeholder="/path/to/skill-directory" spellcheck="false" @keydown.enter="doImport" />
        <span class="import-field__hint">{{ $t('skills.importHint') }}</span>
      </label>
    </div>
    <template #footer>
      <div class="import-actions">
        <DqButton @click="showImportDialog = false">{{ $t('common.cancel') }}</DqButton>
        <DqButton type="primary" :disabled="!importPath.trim()" @click="doImport">{{ $t('skills.import') }}</DqButton>
      </div>
    </template>
  </DqDialog>
</template>


<style scoped>
.resource-rail__section {
  display: flex;
  flex-direction: column;
  min-height: 0;
  flex: 1;
  overflow: hidden;
}

.resource-rail__section-actions {
  display: flex;
  align-items: center;
  gap: 4px;
}

.resource-rail__page-view {
  width: 100%;
}
.resource-rail__section > .resource-rail__section-head:first-child {
  padding-inline: 10px;
}
.resource-rail__section-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 10px 10px 6px 14px;
  flex-shrink: 0;
}

.resource-rail__section-title {
  font-size: var(--dq-font-size-caption);
  font-weight: 600;
  letter-spacing: 0.06em;
  text-transform: uppercase;
  color: var(--dq-label-tertiary);
}

.resource-rail__name-row {
  display: flex;
  align-items: center;
  gap: 6px;
  min-width: 0;
}

.resource-rail__name-row .resource-rail__name {
  min-width: 0;
}

.resource-rail__dot {
  width: 7px;
  height: 7px;
  border-radius: 50%;
  background: var(--dq-system-orange);
  flex-shrink: 0;
}

.skill-template-banner {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  margin: 0 0 16px;
  padding: 12px 14px;
  border-radius: 10px;
  background: color-mix(in srgb, var(--dq-system-orange) 12%, transparent);
  border: 1px solid color-mix(in srgb, var(--dq-system-orange) 28%, transparent);
}

.skill-template-banner__text {
  display: flex;
  flex-direction: column;
  gap: 2px;
  min-width: 0;
  font-size: var(--dq-font-size-footnote);
  color: var(--dq-label-primary);
  line-height: 1.45;
}

.skill-template-banner__text strong {
  font-weight: 600;
}

.skill-template-banner__text span {
  color: var(--dq-label-secondary);
}

.resource-workspace__empty-actions {
  display: flex;
  gap: 8px;
  margin-top: 12px;
  justify-content: center;
}

.resource-workspace__tab-badge {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  min-width: 18px;
  height: 18px;
  padding: 0 4px;
  border-radius: 9px;
  background: color-mix(in srgb, var(--dq-accent) 18%, transparent);
  color: var(--dq-accent);
  font-size: var(--dq-font-size-caption);
  font-weight: 600;
  margin-left: 4px;
}

.resource-section--body {
  display: flex;
  flex-direction: column;
  flex: 1;
  min-height: 0;
}

.resource-section--body .work-md {
  flex: 1;
  min-height: 360px;
}

.resource-section--body .work-md__body {
  flex: 1;
  min-height: 320px;
}

.resource-section__toolbar {
  display: flex;
  justify-content: flex-end;
  margin-bottom: 10px;
}

.resource-meta-list {
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.resource-meta-row {
  display: flex;
  gap: 4px;
  align-items: center;
}

.resource-meta-key,
.resource-meta-value {
  flex: 1;
  padding: 5px 8px;
  border: 1px solid var(--dq-border);
  border-radius: 6px;
  background: var(--dq-fill-on-glass-subtle);
  color: var(--dq-label-primary);
  font-size: var(--dq-font-size-footnote);
  font-family: inherit;
  outline: none;
}

.resource-meta-key:focus,
.resource-meta-value:focus {
  border-color: var(--dq-accent);
}

.resource-meta-remove {
  display: flex;
  align-items: center;
  justify-content: center;
  width: 22px;
  height: 22px;
  border: none;
  border-radius: 4px;
  background: transparent;
  color: var(--dq-label-tertiary);
  font-size: var(--dq-font-size-title);
  cursor: pointer;
}

.resource-meta-add {
  align-self: flex-start;
  padding: 4px 8px;
  border: none;
  border-radius: 6px;
  background: transparent;
  color: var(--dq-accent);
  font-size: var(--dq-font-size-footnote);
  font-weight: 500;
  cursor: pointer;
  transition: background 0.12s ease;
}

.resource-meta-add:hover {
  background: color-mix(in srgb, var(--dq-accent) 8%, transparent);
}

.import-form {
  display: flex;
  flex-direction: column;
  gap: 12px;
}

.import-field {
  display: flex;
  flex-direction: column;
  gap: 6px;
}

.import-field__label {
  font-size: var(--dq-font-size-footnote);
  font-weight: 500;
  color: var(--dq-label-secondary);
}

.import-field__input {
  width: 100%;
  padding: 8px 12px;
  border: 1px solid var(--dq-border);
  border-radius: 8px;
  background: var(--dq-fill-on-glass-subtle);
  color: var(--dq-label-primary);
  font-size: var(--dq-font-size-body);
  font-family: ui-monospace, monospace;
  outline: none;
}

.import-field__hint {
  font-size: var(--dq-font-size-caption);
  color: var(--dq-label-tertiary);
}

.import-actions {
  display: flex;
  gap: 8px;
  justify-content: flex-end;
}

.file-content {
  max-height: 400px;
  overflow: auto;
  padding: 14px;
  border-radius: 8px;
  background: var(--dq-bg-base);
  font-size: var(--dq-font-size-footnote);
  font-family: ui-monospace, monospace;
  line-height: 1.5;
  white-space: pre-wrap;
  word-break: break-all;
}
</style>
