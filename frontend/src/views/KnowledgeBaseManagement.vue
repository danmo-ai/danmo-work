<script setup lang="ts">
import { computed, nextTick, onMounted, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import WorkspaceShell from '@/components/common/WorkspaceShell.vue'
import MarkdownRichEditor from '@/components/office/MarkdownRichEditor.vue'
import { useKnowledgeStore } from '@/stores/knowledge'
import { confirm, toast } from '@/utils/feedback'
import type { KnowledgeBase } from '@/types'

const { t } = useI18n()
const knowledge = useKnowledgeStore()

const selectedId = ref<string | null>(null)
const isCreating = ref(false)
const saving = ref(false)
const activeTab = ref<'info' | 'documents'>('info')
const pendingDocTitle = ref('')
const pendingDocContent = ref('')
const editingDocId = ref<string | null>(null)
const docEditorOpen = ref(false)
const docMode = ref<'view' | 'edit'>('edit')
const docDirty = ref(false)
const docSaving = ref(false)
const docEditorRef = ref<InstanceType<typeof MarkdownRichEditor> | null>(null)

const form = ref<KnowledgeBase>({
  id: '',
  name: '',
  description: '',
  documentCount: 0,
  updatedAt: '',
})

const sortedBases = computed(() =>
  [...knowledge.bases].sort((a, b) => a.name.localeCompare(b.name, 'zh-CN')),
)

const selected = computed(() => knowledge.bases.find((b) => b.id === selectedId.value))
const hasSelection = computed(() => isCreating.value || !!selectedId.value)
const headerTitle = computed(() => {
  if (isCreating.value) return form.value.name.trim() || t('knowledge.newBase')
  return selected.value?.name.trim() || t('knowledge.untitled')
})

const selectedDocs = computed(() => (selectedId.value ? knowledge.documentsFor(selectedId.value) : []))

onMounted(async () => {
  await knowledge.loadBases()
  if (sortedBases.value.length && !selectedId.value) {
    await selectBase(sortedBases.value[0].id)
  }
})

watch(activeTab, async (tab) => {
  if (tab === 'documents' && selectedId.value) {
    await knowledge.loadDocs(selectedId.value)
  }
})

async function selectBase(id: string) {
  isCreating.value = false
  selectedId.value = id
  activeTab.value = 'info'
  closeDocEditor()
  const base = knowledge.bases.find((b) => b.id === id)
  if (base) form.value = { ...base }
  await knowledge.loadDocs(id)
}

function openCreate() {
  isCreating.value = true
  selectedId.value = null
  activeTab.value = 'info'
  closeDocEditor()
  form.value = { id: '', name: '', description: '', documentCount: 0, updatedAt: '' }
}

function closeDocEditor() {
  editingDocId.value = null
  pendingDocTitle.value = ''
  pendingDocContent.value = ''
  docEditorOpen.value = false
  docDirty.value = false
  docMode.value = 'edit'
}

async function save() {
  if (!form.value.name.trim()) {
    toast.warning(t('knowledge.namePlaceholder'))
    return
  }
  saving.value = true
  try {
    if (isCreating.value) {
      const base = await knowledge.createBase({
        name: form.value.name.trim(),
        description: form.value.description?.trim() ?? '',
      })
      toast.success(t('knowledge.created'))
      isCreating.value = false
      await selectBase(base.id)
    } else if (selected.value) {
      await knowledge.updateBase(selected.value.id, {
        name: form.value.name.trim(),
        description: form.value.description?.trim() ?? '',
      })
      toast.success(t('knowledge.saved'))
      await selectBase(selected.value.id)
    }
  } catch (e) {
    toast.error(e instanceof Error ? e.message : t('common.saveFailed'))
  } finally {
    saving.value = false
  }
}

async function removeSelected() {
  if (!selected.value) return
  try {
    await confirm(t('knowledge.deleteConfirm', { name: selected.value.name }), t('knowledge.deleteTitle'), { type: 'warning' })
  } catch {
    return
  }
  try {
    await knowledge.removeBase(selected.value.id)
    selectedId.value = null
    isCreating.value = false
    closeDocEditor()
    toast.success(t('knowledge.deleted'))
  } catch (e) {
    toast.error(e instanceof Error ? e.message : t('common.saveFailed'))
  }
}

function openNewDocument() {
  editingDocId.value = null
  pendingDocTitle.value = ''
  pendingDocContent.value = ''
  docMode.value = 'edit'
  docEditorOpen.value = true
  docDirty.value = false
  void nextTick(() => {
    docEditorRef.value?.setContent('', { emitUpdate: false })
  })
}

async function openDocument(docId: string, mode: 'view' | 'edit' = 'edit') {
  try {
    const doc = await knowledge.getDocument(docId)
    editingDocId.value = doc.id
    pendingDocTitle.value = doc.title
    pendingDocContent.value = doc.content ?? ''
    docMode.value = mode
    docEditorOpen.value = true
    docDirty.value = false
    await nextTick()
    docEditorRef.value?.setContent(pendingDocContent.value, { emitUpdate: false })
  } catch (e) {
    toast.error(e instanceof Error ? e.message : t('common.saveFailed'))
  }
}

async function saveDocument() {
  if (!selected.value || !pendingDocTitle.value.trim()) {
    toast.warning(t('knowledge.docTitlePlaceholder'))
    return
  }
  const content = docEditorRef.value?.getMarkdown() || pendingDocContent.value
  if (!content.trim()) {
    toast.warning(t('knowledge.contentPlaceholder'))
    return
  }
  docSaving.value = true
  try {
    if (editingDocId.value) {
      await knowledge.updateDocument(editingDocId.value, pendingDocTitle.value.trim(), content)
    } else {
      const doc = await knowledge.addDocument(selected.value.id, pendingDocTitle.value.trim(), content)
      editingDocId.value = doc.id
    }
    pendingDocContent.value = content
    docDirty.value = false
    await knowledge.loadBases()
    await knowledge.loadDocs(selected.value.id)
    toast.success(t('knowledge.docAdded'))
  } catch (e) {
    toast.error(e instanceof Error ? e.message : t('common.saveFailed'))
  } finally {
    docSaving.value = false
  }
}

async function removeDocument(docId: string) {
  if (!selected.value) return
  try {
    await knowledge.removeDocument(docId)
    if (editingDocId.value === docId) closeDocEditor()
    await knowledge.loadBases()
    toast.success(t('knowledge.docDeleted'))
  } catch (e) {
    toast.error(e instanceof Error ? e.message : t('common.saveFailed'))
  }
}

function onDocUpdate() {
  docDirty.value = true
  pendingDocContent.value = docEditorRef.value?.getMarkdown() || pendingDocContent.value
}

function baseInitial(name: string) {
  return name.trim().charAt(0).toUpperCase() || 'K'
}

function formatDate(value: string) {
  if (!value) return ''
  return new Date(value).toLocaleString('zh-CN')
}

function onKeydown(e: KeyboardEvent) {
  if ((e.metaKey || e.ctrlKey) && e.key === 's') {
    e.preventDefault()
    if (docEditorOpen.value && docMode.value === 'edit') void saveDocument()
    else void save()
  }
}
</script>

<template>
  <WorkspaceShell
    :title="$t('knowledge.title')"
    :count="sortedBases.length"
    :count-label="$t('knowledge.countLabel')"
    :create-label="$t('knowledge.newBase')"
    :has-selection="hasSelection"
    @create="openCreate"
    @keydown="onKeydown"
  >
    <template #rail>
      <DqEmpty v-if="!sortedBases.length" class="resource-rail__empty" :description="$t('knowledge.noBases')" />
      <nav v-else class="resource-rail__list" aria-label="知识库列表">
        <button
          v-for="base in sortedBases"
          :key="base.id"
          type="button"
          class="resource-rail__row"
          :class="{ 'is-active': selectedId === base.id && !isCreating }"
          @click="selectBase(base.id)"
        >
          <span class="resource-rail__avatar">{{ baseInitial(base.name) }}</span>
          <span class="resource-rail__meta">
            <span class="resource-rail__name">{{ base.name }}</span>
            <span class="resource-rail__desc">{{ base.documentCount }} {{ $t('knowledge.documents') }}</span>
          </span>
        </button>
      </nav>
    </template>

    <template #empty>
      <DqEmpty :description="$t('knowledge.emptySelection')">
        <p class="resource-workspace__hint">{{ $t('knowledge.emptySelectionHint') }}</p>
      </DqEmpty>
    </template>

    <template #header>
      <div class="resource-workspace__identity">
        <h1 class="resource-workspace__title">{{ headerTitle }}</h1>
        <div v-if="selected?.updatedAt && !isCreating" class="resource-workspace__badges">
          <span class="resource-workspace__hint">{{ $t('knowledge.updated') }}{{ formatDate(selected.updatedAt) }}</span>
        </div>
      </div>
      <DqSegmented
        v-if="!isCreating"
        v-model="activeTab"
        class="resource-workspace__segmented"
        :options="[
          { label: $t('knowledge.basicInfo'), value: 'info' },
          { label: $t('knowledge.documents'), value: 'documents' },
        ]"
      />
    </template>

    <template #body>
      <section v-show="activeTab === 'info'" class="resource-section">
        <label class="resource-field resource-field--block">
          <span class="resource-field__label">{{ $t('common.name') }}</span>
          <DqInput v-model="form.name" :placeholder="$t('knowledge.dummyName')" />
        </label>
        <label class="resource-field resource-field--block">
          <span class="resource-field__label">{{ $t('common.description') }}</span>
          <DqInput v-model="form.description" type="textarea" :rows="5" :placeholder="$t('knowledge.descriptionPlaceholder')" />
        </label>
      </section>

      <section v-show="activeTab === 'documents'" class="resource-section knowledge-docs">
        <div class="knowledge-docs__list-card resource-list-card">
          <div class="knowledge-docs__list-head">
            <DqButton type="primary" @click="openNewDocument">{{ $t('knowledge.addDoc') }}</DqButton>
          </div>
          <div v-for="doc in selectedDocs" :key="doc.id" class="resource-list-card__item">
            <div class="resource-list-card__meta">
              <span class="resource-list-card__name">{{ doc.title }}</span>
              <span class="resource-list-card__desc">{{ formatDate(doc.updatedAt) }}</span>
            </div>
            <div class="resource-list-card__actions">
              <button type="button" class="resource-list-card__action" @click="openDocument(doc.id, 'view')">
                {{ $t('knowledge.docViewMode') }}
              </button>
              <button type="button" class="resource-list-card__action" @click="openDocument(doc.id, 'edit')">
                {{ $t('knowledge.docOpen') }}
              </button>
              <button
                type="button"
                class="resource-list-card__action resource-list-card__action--danger"
                @click="removeDocument(doc.id)"
              >
                {{ $t('common.delete') }}
              </button>
            </div>
          </div>
        </div>

        <div v-if="docEditorOpen" class="knowledge-docs__editor">
          <div class="knowledge-docs__editor-bar">
            <label class="resource-field knowledge-docs__title-field">
              <span class="resource-field__label">{{ $t('knowledge.docTitle') }}</span>
              <DqInput
                v-model="pendingDocTitle"
                :disabled="docMode === 'view'"
                :placeholder="$t('knowledge.docTitlePlaceholder')"
              />
            </label>
            <div class="knowledge-docs__editor-actions">
              <DqSegmented
                v-model="docMode"
                size="sm"
                :options="[
                  { label: $t('knowledge.docViewMode'), value: 'view' },
                  { label: $t('knowledge.docEditMode'), value: 'edit' },
                ]"
              />
              <DqButton v-if="docMode === 'edit'" type="primary" :disabled="docSaving" @click="saveDocument">
                {{ $t('knowledge.docSave') }}
              </DqButton>
              <DqButton @click="closeDocEditor">{{ $t('knowledge.docCancel') }}</DqButton>
            </div>
          </div>
          <div class="knowledge-docs__editor-body">
            <MarkdownRichEditor
              ref="docEditorRef"
              :editable="docMode === 'edit'"
              :show-toolbar="docMode === 'edit'"
              :show-toc="true"
              :placeholder="$t('knowledge.contentPlaceholder')"
              @update="onDocUpdate"
            />
          </div>
          <p v-if="docDirty" class="resource-workspace__hint">{{ $t('common.saveShortcut') }}</p>
        </div>
      </section>
    </template>

    <template #footer>
      <span class="resource-workspace__hint">{{ $t('common.saveShortcut') }}</span>
      <div class="resource-workspace__footer-actions">
        <DqButton v-if="isCreating" @click="isCreating = false; selectedId = null">{{ $t('common.cancel') }}</DqButton>
        <DqButton v-if="!isCreating" @click="removeSelected">{{ $t('common.delete') }}</DqButton>
        <DqButton type="primary" :disabled="saving" @click="save">
          {{ isCreating ? $t('knowledge.createBase') : $t('common.save') }}
        </DqButton>
      </div>
    </template>
  </WorkspaceShell>
</template>

<style scoped>
.knowledge-docs {
  display: flex;
  flex-direction: column;
  gap: 12px;
  min-height: 0;
  height: 100%;
}
.knowledge-docs__list-head {
  margin-bottom: 8px;
}
.knowledge-docs__editor {
  flex: 1;
  min-height: 420px;
  display: flex;
  flex-direction: column;
  gap: 8px;
  border: 1px solid var(--dq-border);
  border-radius: 8px;
  background: var(--dq-bg-base);
  overflow: hidden;
}
.knowledge-docs__editor-bar {
  display: flex;
  flex-wrap: wrap;
  gap: 10px;
  align-items: flex-end;
  justify-content: space-between;
  padding: 10px 12px;
  border-bottom: 1px solid var(--dq-separator-light);
}
.knowledge-docs__title-field {
  flex: 1;
  min-width: 200px;
}
.knowledge-docs__editor-actions {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
  align-items: center;
}
.knowledge-docs__editor-body {
  flex: 1;
  min-height: 360px;
  display: flex;
  flex-direction: column;
}
</style>
