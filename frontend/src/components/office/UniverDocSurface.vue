<script setup lang="ts">
import { onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { fetchJSON } from '@/api/client'
import { toast } from '@/utils/feedback'
import { parseUniverFile, stringifyUniverFile } from '@/utils/univer-ir'
import { emptyDocumentData } from '@/utils/univer-snapshots'
import type { FileEditScope } from '@/utils/file-route'

const props = defineProps<{
  projectId: string
  path: string
  mode: 'view' | 'edit' | 'present'
  reloadToken: number
  turnRunning?: boolean
}>()

const emit = defineEmits<{
  dirty: [value: boolean]
  saved: []
  scope: [scope: FileEditScope]
}>()

const { t } = useI18n()
const containerRef = ref<HTMLElement | null>(null)
const loading = ref(false)
const saving = ref(false)
const dirty = ref(false)

let univerInstance: { dispose: () => void } | null = null
let univerAPI: {
  createUniverDoc: (data: unknown) => unknown
  getActiveDocument?: () => { getSnapshot: () => unknown } | null
  dispose?: () => void
} | null = null

const readonly = () => props.mode === 'view' || !!props.turnRunning

async function destroyUniver() {
  try {
    univerAPI?.dispose?.()
  } catch {
    /* ignore */
  }
  try {
    univerInstance?.dispose()
  } catch {
    /* ignore */
  }
  univerAPI = null
  univerInstance = null
  if (containerRef.value) containerRef.value.innerHTML = ''
}

async function bootWithSnapshot(snapshot: unknown) {
  await destroyUniver()
  if (!containerRef.value) return

  const { createUniver, LocaleType, mergeLocales } = await import('@univerjs/presets')
  const { UniverDocsCorePreset } = await import('@univerjs/preset-docs-core')
  const UniverPresetDocsCoreEnUS = (await import('@univerjs/preset-docs-core/locales/en-US')).default
  await import('@univerjs/preset-docs-core/lib/index.css')

  const created = createUniver({
    locale: LocaleType.EN_US,
    locales: {
      [LocaleType.EN_US]: mergeLocales(UniverPresetDocsCoreEnUS),
    },
    presets: [
      UniverDocsCorePreset({
        container: containerRef.value,
        toolbar: !readonly(),
      }),
    ],
  })
  univerInstance = created.univer
  univerAPI = created.univerAPI as typeof univerAPI
  univerAPI!.createUniverDoc(snapshot || emptyDocumentData())
  dirty.value = false
  emit('dirty', false)
  emit('scope', 'document')
}

async function load() {
  if (!props.projectId || !props.path) return
  loading.value = true
  try {
    const fc = await fetchJSON<{ content: string }>(
      `/projects/${props.projectId}/files/content?path=${encodeURIComponent(props.path)}`,
    )
    const { snapshot } = parseUniverFile(fc.content || '{}', 'univer-doc')
    await bootWithSnapshot(snapshot)
  } catch (e) {
    toast.error(e instanceof Error ? e.message : t('office.loadFailed'))
  } finally {
    loading.value = false
  }
}

async function save(opts?: { quiet?: boolean }) {
  if (!props.projectId || readonly()) return
  const doc = univerAPI?.getActiveDocument?.()
  if (!doc) return
  saving.value = true
  try {
    const snapshot = doc.getSnapshot()
    await fetchJSON(`/projects/${props.projectId}/files/content`, {
      method: 'PUT',
      body: JSON.stringify({
        path: props.path,
        content: stringifyUniverFile('univer-doc', snapshot),
      }),
    })
    dirty.value = false
    emit('dirty', false)
    emit('saved')
    if (!opts?.quiet) toast.success(t('office.saved'))
  } catch (e) {
    toast.error(e instanceof Error ? e.message : t('office.saveFailed'))
    throw e
  } finally {
    saving.value = false
  }
}

function markDirty() {
  if (readonly()) return
  dirty.value = true
  emit('dirty', true)
}

function getEditScope(): FileEditScope {
  return 'document'
}

function getSelectionMarkdown(): string {
  return ''
}

function getSelectionLines(): { startLine: number; endLine: number } | null {
  return null
}

onMounted(() => {
  void load()
  containerRef.value?.addEventListener('pointerup', markDirty)
})

onBeforeUnmount(() => {
  containerRef.value?.removeEventListener('pointerup', markDirty)
  void destroyUniver()
})

watch(
  () => [props.projectId, props.path, props.reloadToken, props.mode] as const,
  () => {
    void load()
  },
)

defineExpose({ save, getSelectionMarkdown, getSelectionLines, getEditScope, dirty, saving, loading })
</script>

<template>
  <div class="univer-doc-surface">
    <div v-if="loading" class="univer-doc-surface__status">{{ t('office.loading') }}</div>
    <div ref="containerRef" class="univer-doc-surface__host" />
  </div>
</template>

<style scoped>
.univer-doc-surface {
  display: flex;
  flex-direction: column;
  min-height: 0;
  flex: 1;
  height: 100%;
}
.univer-doc-surface__status {
  padding: 12px;
  color: var(--dq-label-secondary);
  font-size: var(--dq-font-size-caption);
}
.univer-doc-surface__host {
  flex: 1;
  min-height: 0;
  height: 100%;
}
</style>
