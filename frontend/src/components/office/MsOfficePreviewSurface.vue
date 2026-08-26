<script setup lang="ts">
import { computed, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { fetchJSON } from '@/api/client'
import { apiBaseUrl } from '@/utils/desktop'
import { toast } from '@/utils/feedback'
import { routeOfficeFile } from '@/utils/office-route'
import { siblingUniverIrPath, stringifyUniverFile } from '@/utils/univer-ir'
import {
  docxArrayBufferToDocumentData,
  pptxArrayBufferToSlideData,
  xlsxArrayBufferToWorkbookData,
} from '@/utils/ms-office-convert'
import { useWorkspaceUiStore } from '@/stores/workspaceUi'

const props = defineProps<{
  projectId: string
  path: string
  mode: 'view' | 'edit' | 'present'
  reloadToken: number
}>()

const { t } = useI18n()
const workspaceUi = useWorkspaceUiStore()
const converting = ref(false)

const kindLabel = computed(() => {
  const lower = props.path.toLowerCase()
  if (lower.endsWith('.xlsx')) return 'Excel'
  if (lower.endsWith('.docx')) return 'Word'
  if (lower.endsWith('.pptx')) return 'PowerPoint'
  return 'Office'
})

const rawUrl = computed(() => {
  if (!props.projectId || !props.path) return ''
  return `${apiBaseUrl()}/api/v1/projects/${props.projectId}/raw/${encodeURIComponent(props.path)}`
})

async function convertToIr() {
  if (!props.projectId || !props.path) return
  converting.value = true
  try {
    const res = await fetch(rawUrl.value)
    if (!res.ok) throw new Error(`HTTP ${res.status}`)
    const buf = await res.arrayBuffer()
    const lower = props.path.toLowerCase()
    let dest = ''
    let content = ''
    if (lower.endsWith('.xlsx')) {
      const snapshot = await xlsxArrayBufferToWorkbookData(buf)
      dest = siblingUniverIrPath(props.path, 'univer-sheet')
      content = stringifyUniverFile('univer-sheet', snapshot)
    } else if (lower.endsWith('.docx')) {
      const snapshot = await docxArrayBufferToDocumentData(buf)
      dest = siblingUniverIrPath(props.path, 'univer-doc')
      content = stringifyUniverFile('univer-doc', snapshot)
    } else if (lower.endsWith('.pptx')) {
      const snapshot = await pptxArrayBufferToSlideData(buf)
      dest = siblingUniverIrPath(props.path, 'univer-slides')
      content = stringifyUniverFile('univer-slides', snapshot)
    } else {
      throw new Error('unsupported format')
    }
    try {
      await fetchJSON(`/projects/${props.projectId}/files/content?path=${encodeURIComponent(dest)}`)
      const stamp = Date.now()
      dest = dest.replace(/\.(usheet|udoc|uslides)\.json$/i, `.${stamp}$&`)
    } catch {
      /* free */
    }
    await fetchJSON(`/projects/${props.projectId}/files/content`, {
      method: 'PUT',
      body: JSON.stringify({ path: dest, content }),
    })
    toast.success(t('office.convertedToUniverIr'))
    workspaceUi.openStage({ ...routeOfficeFile(dest) })
    workspaceUi.requestFilesReload()
  } catch (e) {
    toast.error(e instanceof Error ? e.message : t('office.convertFailed'))
  } finally {
    converting.value = false
  }
}
</script>

<template>
  <div class="ms-office-preview">
    <div class="ms-office-preview__card">
      <h2 class="ms-office-preview__title">{{ t('office.msViewOnlyTitle', { kind: kindLabel }) }}</h2>
      <p class="ms-office-preview__path">{{ path }}</p>
      <p class="ms-office-preview__hint">{{ t('office.msViewOnlyHint') }}</p>
      <div class="ms-office-preview__actions">
        <a class="ms-office-preview__link" :href="rawUrl" target="_blank" rel="noopener">
          {{ t('office.downloadOriginal') }}
        </a>
        <button type="button" class="ms-office-preview__btn" :disabled="converting" @click="convertToIr">
          {{ converting ? t('office.converting') : t('office.convertToUniverIr') }}
        </button>
      </div>
    </div>
  </div>
</template>

<style scoped>
.ms-office-preview {
  display: flex;
  align-items: center;
  justify-content: center;
  min-height: 0;
  flex: 1;
  padding: 24px;
}
.ms-office-preview__card {
  max-width: 520px;
  width: 100%;
  padding: 24px;
  border: 1px solid var(--dq-separator-light);
  border-radius: 12px;
  background: var(--dq-bg-elevated);
}
.ms-office-preview__title {
  margin: 0 0 8px;
  font-size: 18px;
}
.ms-office-preview__path {
  margin: 0 0 12px;
  color: var(--dq-label-tertiary);
  font-size: var(--dq-font-size-caption);
  word-break: break-all;
}
.ms-office-preview__hint {
  margin: 0 0 16px;
  color: var(--dq-label-secondary);
  line-height: 1.5;
}
.ms-office-preview__actions {
  display: flex;
  gap: 10px;
  flex-wrap: wrap;
}
.ms-office-preview__link {
  height: 32px;
  display: inline-flex;
  align-items: center;
  padding: 0 12px;
  border-radius: 6px;
  border: 1px solid var(--dq-border);
  color: var(--dq-label-primary);
  text-decoration: none;
  font-size: var(--dq-font-size-caption);
}
.ms-office-preview__btn {
  height: 32px;
  padding: 0 12px;
  border-radius: 6px;
  border: 0;
  background: var(--dq-accent);
  color: var(--dq-label-on-accent, #fff);
  cursor: pointer;
  font-size: var(--dq-font-size-caption);
}
.ms-office-preview__btn:disabled {
  opacity: 0.6;
  cursor: default;
}
</style>
