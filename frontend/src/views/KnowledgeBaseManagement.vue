<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import WorkspaceShell from '@/components/common/WorkspaceShell.vue'
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
  editingDocId.value = null
  const base = knowledge.bases.find((b) => b.id === id)
  if (base) form.value = { ...base }
  await knowledge.loadDocs(id)
}

function openCreate() {
  isCreating.value = true
  selectedId.value = null
  activeTab.value = 'info'
  editingDocId.value = null
  form.value = { id: '', name: '', description: '', documentCount: 0, updatedAt: '' }
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
    toast.success(t('knowledge.deleted'))
  } catch (e) {
    toast.error(e instanceof Error ? e.message : t('common.saveFailed'))
  }
}

async function addDocument() {
  if (!selected.value || !pendingDocTitle.value.trim() || !pendingDocContent.value.trim()) return
  try {
    if (editingDocId.value) {
      await knowledge.updateDocument(editingDocId.value, pendingDocTitle.value.trim(), pendingDocContent.value.trim())
      editingDocId.value = null
    } else {
      await knowledge.addDocument(selected.value.id, pendingDocTitle.value.trim(), pendingDocContent.value.trim())
    }
    pendingDocTitle.value = ''
    pendingDocContent.value = ''
    await knowledge.loadBases()
    await knowledge.loadDocs(selected.value.id)
    toast.success(t('knowledge.docAdded'))
  } catch (e) {
    toast.error(e instanceof Error ? e.message : t('common.saveFailed'))
  }
}

async function editDocument(docId: string) {
  try {
    const doc = await knowledge.getDocument(docId)
    editingDocId.value = doc.id
    pendingDocTitle.value = doc.title
    pendingDocContent.value = doc.content ?? ''
  } catch (e) {
    toast.error(e instanceof Error ? e.message : t('common.saveFailed'))
  }
}

async function removeDocument(docId: string) {
  if (!selected.value) return
  try {
    await knowledge.removeDocument(docId)
    if (editingDocId.value === docId) {
      editingDocId.value = null
      pendingDocTitle.value = ''
      pendingDocContent.value = ''
    }
    await knowledge.loadBases()
    toast.success(t('knowledge.docDeleted'))
  } catch (e) {
    toast.error(e instanceof Error ? e.message : t('common.saveFailed'))
  }
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
    save()
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

      <section v-show="activeTab === 'documents'" class="resource-section">
        <div class="resource-form-grid resource-form-grid--2">
          <label class="resource-field">
            <span class="resource-field__label">{{ $t('knowledge.docTitle') }}</span>
            <DqInput v-model="pendingDocTitle" :placeholder="$t('knowledge.docTitlePlaceholder')" />
          </label>
          <div class="resource-field resource-field--action">
            <DqButton @click="addDocument">{{ editingDocId ? $t('common.save') : $t('knowledge.addDoc') }}</DqButton>
          </div>
        </div>
        <label class="resource-field resource-field--block">
          <span class="resource-field__label">{{ $t('knowledge.content') }}</span>
          <DqInput v-model="pendingDocContent" type="textarea" :rows="10" :placeholder="$t('knowledge.contentPlaceholder')" />
        </label>

        <div class="resource-list-card">
          <div v-for="doc in selectedDocs" :key="doc.id" class="resource-list-card__item">
            <div class="resource-list-card__meta">
              <span class="resource-list-card__name">{{ doc.title }}</span>
              <span class="resource-list-card__desc">{{ formatDate(doc.updatedAt) }}</span>
            </div>
            <div class="resource-list-card__actions">
              <button type="button" class="resource-list-card__action" @click="editDocument(doc.id)">{{ $t('common.edit') }}</button>
              <button type="button" class="resource-list-card__action resource-list-card__action--danger" @click="removeDocument(doc.id)">{{ $t('common.delete') }}</button>
            </div>
          </div>
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
