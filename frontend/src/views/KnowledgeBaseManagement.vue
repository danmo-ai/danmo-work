<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import WorkspaceShell from '@/components/common/WorkspaceShell.vue'
import MdEditor from '@/components/common/MdEditor.vue'
import { useKnowledgeStore } from '@/stores/knowledge'
import { confirm, toast } from '@/utils/feedback'
import { handleResourceRailArrowKeys } from '@/composables/useResourceRailKeyboard'

const SELECTED_KB_KEY = 'app-selected-kb-id'

const { t, locale } = useI18n()
const knowledge = useKnowledgeStore()

const selectedBaseId = ref('')
const selectedDocId = ref<string | null>(null)
const isCreatingDoc = ref(false)
const saving = ref(false)
const dirty = ref(false)
const contentSnapshot = ref('')
const titleSnapshot = ref('')

const docTitle = ref('')
const docContent = ref('')

const showBaseCreate = ref(false)
const showBaseSettings = ref(false)
const baseFormName = ref('')
const baseFormDescription = ref('')
const baseSaving = ref(false)

const sortedBases = computed(() =>
  [...knowledge.bases].sort((a, b) => a.name.localeCompare(b.name, locale.value)),
)

const selectedBase = computed(() => knowledge.bases.find((b) => b.id === selectedBaseId.value))

const selectedDocs = computed(() =>
  selectedBaseId.value ? knowledge.documentsFor(selectedBaseId.value) : [],
)

const sortedDocs = computed(() =>
  [...selectedDocs.value].sort((a, b) => a.title.localeCompare(b.title, locale.value)),
)

const hasSelection = computed(() => isCreatingDoc.value || !!selectedDocId.value)

const headerTitle = computed(() => {
  if (isCreatingDoc.value) return docTitle.value.trim() || t('knowledge.addDoc')
  return docTitle.value.trim() || t('knowledge.untitledDoc')
})

const canDeleteBase = computed(() => sortedBases.value.length > 1)

onMounted(async () => {
  window.addEventListener('keydown', onKeydown)
  await knowledge.loadBases()
  if (!sortedBases.value.length) {
    const base = await knowledge.createBase({ name: t('knowledge.defaultBaseName') })
    await selectBase(base.id)
    return
  }
  const saved = localStorage.getItem(SELECTED_KB_KEY)
  const initial =
    (saved && sortedBases.value.some((b) => b.id === saved) && saved) ||
    sortedBases.value.find((b) => b.id === 'kb-default')?.id ||
    sortedBases.value[0].id
  await selectBase(initial)
})

onUnmounted(() => {
  window.removeEventListener('keydown', onKeydown)
})

watch(selectedBaseId, (id) => {
  if (id) localStorage.setItem(SELECTED_KB_KEY, id)
})

watch([docTitle, docContent], () => {
  if (!hasSelection.value) return
  dirty.value = docTitle.value !== titleSnapshot.value || docContent.value !== contentSnapshot.value
})

async function guardDirty(): Promise<boolean> {
  if (!dirty.value) return true
  try {
    await confirm(t('knowledge.unsavedLeave'), t('knowledge.unsavedTitle'), { type: 'warning' })
    return true
  } catch {
    return false
  }
}

async function onBaseSwitchCommand(cmd: string) {
  if (!cmd.startsWith('base:')) return
  const next = cmd.slice('base:'.length)
  if (!next || next === selectedBaseId.value) return
  if (!(await guardDirty())) return
  await selectBase(next)
}

async function selectBase(id: string) {
  selectedBaseId.value = id
  clearDocEditor()
  await knowledge.loadDocs(id)
  if (selectedDocs.value.length) {
    await openDocument(selectedDocs.value[0].id, true)
  }
}

function clearDocEditor() {
  selectedDocId.value = null
  isCreatingDoc.value = false
  docTitle.value = ''
  docContent.value = ''
  titleSnapshot.value = ''
  contentSnapshot.value = ''
  dirty.value = false
}

async function openNewDocument() {
  if (!selectedBaseId.value) return
  if (!(await guardDirty())) return
  isCreatingDoc.value = true
  selectedDocId.value = null
  docTitle.value = ''
  docContent.value = ''
  titleSnapshot.value = ''
  contentSnapshot.value = ''
  dirty.value = false
}

async function openDocument(docId: string, skipGuard = false) {
  if (!skipGuard && selectedDocId.value === docId && !isCreatingDoc.value) return
  if (!skipGuard && !(await guardDirty())) return
  try {
    const doc = await knowledge.getDocument(docId)
    isCreatingDoc.value = false
    selectedDocId.value = doc.id
    docTitle.value = doc.title
    docContent.value = doc.content ?? ''
    titleSnapshot.value = docTitle.value
    contentSnapshot.value = docContent.value
    dirty.value = false
  } catch (e) {
    toast.error(e instanceof Error ? e.message : t('common.saveFailed'))
  }
}

async function saveDocument() {
  if (!selectedBaseId.value || !docTitle.value.trim()) {
    toast.warning(t('knowledge.docTitlePlaceholder'))
    return
  }
  if (!docContent.value.trim()) {
    toast.warning(t('knowledge.contentPlaceholder'))
    return
  }
  saving.value = true
  try {
    if (selectedDocId.value) {
      await knowledge.updateDocument(selectedDocId.value, docTitle.value.trim(), docContent.value)
    } else {
      const doc = await knowledge.addDocument(
        selectedBaseId.value,
        docTitle.value.trim(),
        docContent.value,
      )
      selectedDocId.value = doc.id
      isCreatingDoc.value = false
    }
    titleSnapshot.value = docTitle.value.trim()
    contentSnapshot.value = docContent.value
    dirty.value = false
    await knowledge.loadBases()
    await knowledge.loadDocs(selectedBaseId.value)
    toast.success(t('knowledge.docAdded'))
  } catch (e) {
    toast.error(e instanceof Error ? e.message : t('common.saveFailed'))
  } finally {
    saving.value = false
  }
}

async function removeSelectedDoc() {
  if (!selectedDocId.value) {
    clearDocEditor()
    return
  }
  const title = docTitle.value.trim() || t('knowledge.untitledDoc')
  try {
    await confirm(t('knowledge.deleteDocConfirm', { name: title }), t('knowledge.deleteDocTitle'), {
      type: 'warning',
    })
  } catch {
    return
  }
  try {
    await knowledge.removeDocument(selectedDocId.value)
    toast.success(t('knowledge.docDeleted'))
    clearDocEditor()
    await knowledge.loadBases()
    if (selectedDocs.value.length) await openDocument(selectedDocs.value[0].id)
  } catch (e) {
    toast.error(e instanceof Error ? e.message : t('common.saveFailed'))
  }
}

function openCreateBase() {
  baseFormName.value = ''
  baseFormDescription.value = ''
  showBaseCreate.value = true
}

function openBaseSettings() {
  if (!selectedBase.value) return
  baseFormName.value = selectedBase.value.name
  baseFormDescription.value = selectedBase.value.description ?? ''
  showBaseSettings.value = true
}

async function submitCreateBase() {
  if (!baseFormName.value.trim()) {
    toast.warning(t('knowledge.namePlaceholder'))
    return
  }
  baseSaving.value = true
  try {
    const base = await knowledge.createBase({
      name: baseFormName.value.trim(),
      description: baseFormDescription.value.trim(),
    })
    showBaseCreate.value = false
    toast.success(t('knowledge.created'))
    await selectBase(base.id)
  } catch (e) {
    toast.error(e instanceof Error ? e.message : t('common.saveFailed'))
  } finally {
    baseSaving.value = false
  }
}

async function submitBaseSettings() {
  if (!selectedBase.value) return
  if (!baseFormName.value.trim()) {
    toast.warning(t('knowledge.namePlaceholder'))
    return
  }
  baseSaving.value = true
  try {
    await knowledge.updateBase(selectedBase.value.id, {
      name: baseFormName.value.trim(),
      description: baseFormDescription.value.trim(),
    })
    showBaseSettings.value = false
    toast.success(t('knowledge.saved'))
    await knowledge.loadBases()
  } catch (e) {
    toast.error(e instanceof Error ? e.message : t('common.saveFailed'))
  } finally {
    baseSaving.value = false
  }
}

async function removeCurrentBase() {
  if (!selectedBase.value) return
  if (!canDeleteBase.value) {
    toast.warning(t('knowledge.cannotDeleteLastBase'))
    return
  }
  try {
    await confirm(
      t('knowledge.deleteConfirm', { name: selectedBase.value.name }),
      t('knowledge.deleteTitle'),
      { type: 'warning' },
    )
  } catch {
    return
  }
  try {
    const deletingId = selectedBase.value.id
    await knowledge.deleteBase(deletingId)
    showBaseSettings.value = false
    toast.success(t('knowledge.deleted'))
    await knowledge.loadBases()
    const next =
      sortedBases.value.find((b) => b.id === 'kb-default')?.id || sortedBases.value[0]?.id
    if (next) await selectBase(next)
  } catch (e) {
    toast.error(e instanceof Error ? e.message : t('common.saveFailed'))
  }
}

function onBaseMenu(cmd: string) {
  if (cmd === 'settings') openBaseSettings()
  else if (cmd === 'delete') void removeCurrentBase()
}

function docInitial(title: string) {
  return title.trim().charAt(0).toUpperCase() || 'D'
}

function onKeydown(e: KeyboardEvent) {
  if ((e.metaKey || e.ctrlKey) && e.key === 's') {
    e.preventDefault()
    if (hasSelection.value) void saveDocument()
    return
  }

  handleResourceRailArrowKeys(
    e,
    sortedDocs.value,
    selectedDocId,
    (id) => void openDocument(id),
    !isCreatingDoc.value,
  )
}
</script>

<template>
  <WorkspaceShell
    custom-rail
    :has-selection="hasSelection"
    @keydown="onKeydown"
    @create="openNewDocument"
  >
    <template #rail>
      <div class="resource-rail__section">
        <div class="knowledge-rail__base">
          <DqDropdown class="knowledge-rail__base-switch" @command="onBaseSwitchCommand">
            <button
              type="button"
              class="knowledge-rail__base-trigger"
              :aria-label="$t('knowledge.title')"
            >
              <span
                class="resource-rail__name knowledge-rail__base-name"
                style="font-size: var(--dq-font-size-body); font-weight: 600;"
              >
                {{ selectedBase?.name || $t('knowledge.title') }}
              </span>
              <svg
                class="knowledge-rail__base-chevron"
                viewBox="0 0 24 24"
                width="16"
                height="16"
                fill="none"
                stroke="currentColor"
                stroke-width="2"
                aria-hidden="true"
              >
                <path d="M6 9l6 6 6-6" stroke-linecap="round" stroke-linejoin="round" />
              </svg>
            </button>
            <template #dropdown>
              <DqDropdownMenu>
                <DqDropdownItem
                  v-for="base in sortedBases"
                  :key="base.id"
                  :command="`base:${base.id}`"
                >
                  {{ base.name }}
                </DqDropdownItem>
              </DqDropdownMenu>
            </template>
          </DqDropdown>
          <DqIconButton :aria-label="$t('knowledge.newBase')" @click="openCreateBase">
            <svg viewBox="0 0 24 24" width="14" height="14" fill="none" stroke="currentColor" stroke-width="2">
              <path d="M12 5v14M5 12h14" stroke-linecap="round" />
            </svg>
          </DqIconButton>
          <DqDropdown @command="onBaseMenu">
            <DqIconButton :aria-label="$t('knowledge.baseSettings')">
              <svg viewBox="0 0 24 24" width="14" height="14" fill="currentColor" aria-hidden="true">
                <circle cx="12" cy="5" r="1.6" />
                <circle cx="12" cy="12" r="1.6" />
                <circle cx="12" cy="19" r="1.6" />
              </svg>
            </DqIconButton>
            <template #dropdown>
              <DqDropdownMenu>
                <DqDropdownItem command="settings">{{ $t('knowledge.baseSettings') }}</DqDropdownItem>
                <DqDropdownItem command="delete" :disabled="!canDeleteBase">
                  {{ $t('common.delete') }}
                </DqDropdownItem>
              </DqDropdownMenu>
            </template>
          </DqDropdown>
        </div>

        <div class="resource-rail__section-head">
          <span class="resource-rail__section-title">{{ $t('knowledge.documents') }}</span>
          <DqIconButton
            :aria-label="$t('knowledge.addDoc')"
            :disabled="!selectedBaseId"
            @click="openNewDocument"
          >
            <svg viewBox="0 0 24 24" width="14" height="14" fill="none" stroke="currentColor" stroke-width="2">
              <path d="M12 5v14M5 12h14" stroke-linecap="round" />
            </svg>
          </DqIconButton>
        </div>

        <DqEmpty
          v-if="!sortedDocs.length && !isCreatingDoc"
          class="resource-rail__empty"
          :description="$t('knowledge.noDocuments')"
        />
        <div v-else class="resource-rail__scroll">
        <nav class="resource-rail__list" :aria-label="$t('knowledge.documents')">
          <button
            v-if="isCreatingDoc"
            type="button"
            class="resource-rail__row is-active"
          >
            <span class="resource-rail__avatar resource-rail__avatar--new">+</span>
            <span class="resource-rail__meta">
              <span class="resource-rail__name">{{ docTitle.trim() || $t('knowledge.addDoc') }}</span>
            </span>
          </button>
          <button
            v-for="doc in sortedDocs"
            :key="doc.id"
            type="button"
            class="resource-rail__row"
            :class="{ 'is-active': selectedDocId === doc.id && !isCreatingDoc }"
            @click="openDocument(doc.id)"
          >
            <span class="resource-rail__avatar">{{ docInitial(doc.title) }}</span>
            <span class="resource-rail__meta">
              <span class="resource-rail__name">{{ doc.title }}</span>
            </span>
          </button>
        </nav>
        </div>
      </div>
    </template>

    <template #empty>
      <DqEmpty :description="$t('knowledge.emptyDocSelection')">
        <p class="resource-workspace__hint">{{ $t('knowledge.emptyDocSelectionHint') }}</p>
        <div class="resource-workspace__empty-actions">
          <DqButton type="primary" :disabled="!selectedBaseId" @click="openNewDocument">
            {{ $t('knowledge.addDoc') }}
          </DqButton>
        </div>
      </DqEmpty>
    </template>

    <template #header>
      <div class="resource-workspace__identity">
        <h1 class="resource-workspace__title">{{ headerTitle }}</h1>
      </div>
    </template>

    <template #body>
      <section class="resource-section resource-section--body knowledge-doc-editor">
        <label class="resource-field resource-field--inline knowledge-doc-editor__title">
          <span class="resource-field__label">{{ $t('knowledge.docTitle') }}</span>
          <DqInput
            v-model="docTitle"
            class="knowledge-doc-editor__title-input"
            :placeholder="$t('knowledge.docTitlePlaceholder')"
          />
        </label>
        <MdEditor
          v-model="docContent"
          :rows="20"
          :placeholder="$t('knowledge.contentPlaceholder')"
        />
      </section>
    </template>

    <template #footer>
      <span class="resource-workspace__hint">{{ $t('common.saveShortcut') }}</span>
      <div class="resource-workspace__footer-actions">
        <DqButton v-if="selectedDocId || isCreatingDoc" @click="removeSelectedDoc">
          {{ isCreatingDoc ? $t('common.cancel') : $t('common.delete') }}
        </DqButton>
        <DqButton type="primary" :disabled="saving || !dirty" @click="saveDocument">
          {{ $t('common.save') }}
        </DqButton>
      </div>
    </template>
  </WorkspaceShell>

  <DqDialog
    v-model:open="showBaseCreate"
    :title="$t('knowledge.newBase')"
    variant="glass"
    width="400px"
  >
    <div class="knowledge-base-form">
      <label class="resource-field resource-field--block">
        <span class="resource-field__label">{{ $t('common.name') }}</span>
        <DqInput v-model="baseFormName" :placeholder="$t('knowledge.dummyName')" />
      </label>
      <label class="resource-field resource-field--block">
        <span class="resource-field__label">{{ $t('common.description') }}</span>
        <DqInput
          v-model="baseFormDescription"
          type="textarea"
          :rows="3"
          :placeholder="$t('knowledge.descriptionPlaceholder')"
        />
      </label>
    </div>
    <template #footer>
      <DqButton @click="showBaseCreate = false">{{ $t('common.cancel') }}</DqButton>
      <DqButton type="primary" :disabled="baseSaving" @click="submitCreateBase">
        {{ $t('common.create') }}
      </DqButton>
    </template>
  </DqDialog>

  <DqDialog
    v-model:open="showBaseSettings"
    :title="$t('knowledge.baseSettings')"
    variant="glass"
    width="400px"
  >
    <div class="knowledge-base-form">
      <label class="resource-field resource-field--block">
        <span class="resource-field__label">{{ $t('common.name') }}</span>
        <DqInput v-model="baseFormName" :placeholder="$t('knowledge.dummyName')" />
      </label>
      <label class="resource-field resource-field--block">
        <span class="resource-field__label">{{ $t('common.description') }}</span>
        <DqInput
          v-model="baseFormDescription"
          type="textarea"
          :rows="3"
          :placeholder="$t('knowledge.descriptionPlaceholder')"
        />
      </label>
    </div>
    <template #footer>
      <DqButton
        class="knowledge-base-form__danger"
        :disabled="!canDeleteBase"
        @click="removeCurrentBase"
      >
        {{ $t('common.delete') }}
      </DqButton>
      <div class="knowledge-base-form__spacer" />
      <DqButton @click="showBaseSettings = false">{{ $t('common.cancel') }}</DqButton>
      <DqButton type="primary" :disabled="baseSaving" @click="submitBaseSettings">
        {{ $t('common.save') }}
      </DqButton>
    </template>
  </DqDialog>
</template>

<style scoped>
.knowledge-rail__base {
  display: flex;
  align-items: center;
  gap: 4px;
  padding: 10px 8px 6px 10px;
  flex-shrink: 0;
}

.knowledge-rail__base-switch {
  flex: 1 1 auto;
  min-width: 0;
}

.knowledge-rail__base-trigger {
  display: flex;
  align-items: center;
  gap: 6px;
  width: 100%;
  min-height: 34px;
  padding: 6px 8px;
  border: 1px solid var(--dq-glass-control-border, var(--dq-glass-border));
  border-radius: 8px;
  background: var(--dq-glass-control-bg, color-mix(in srgb, var(--dq-bg-elevated) 40%, transparent));
  color: var(--dq-label-primary);
  text-align: left;
  cursor: pointer;
}

.knowledge-rail__base-trigger:hover {
  background: var(--dq-glass-control-bg-hover, color-mix(in srgb, var(--dq-label-primary) 6%, transparent));
}

.knowledge-rail__base-name {
  flex: 1 1 auto;
  min-width: 0;
  font-size: var(--dq-font-size-body);
  font-weight: 600;
  line-height: 1.3;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.knowledge-rail__base-chevron {
  flex-shrink: 0;
  color: var(--dq-label-tertiary);
}

.resource-workspace__empty-actions {
  display: flex;
  gap: 8px;
  margin-top: 12px;
  justify-content: center;
}

.knowledge-doc-editor {
  display: flex;
  flex-direction: column;
  gap: 12px;
  min-height: 0;
  height: 100%;
}

.knowledge-doc-editor__title {
  margin: 0;
  flex-shrink: 0;
  justify-content: flex-start;
  gap: 12px;
}

.knowledge-doc-editor__title .resource-field__label {
  flex-shrink: 0;
  white-space: nowrap;
}

.knowledge-doc-editor__title-input {
  flex: 1 1 auto;
  min-width: 0;
}

.knowledge-doc-editor :deep(.work-md) {
  flex: 1 1 auto;
  min-height: 0;
}

.knowledge-doc-editor :deep(.work-md__body) {
  min-height: 320px;
}

.knowledge-base-form {
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.knowledge-base-form__spacer {
  flex: 1;
}

.knowledge-base-form__danger {
  margin-right: auto;
}
</style>
