<script setup lang="ts">
import { onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { fetchJSON } from '@/api/client'
import { toast } from '@/utils/feedback'
import { parseUniverFile, stringifyUniverFile } from '@/utils/univer-ir'
import { emptyWorkbookData, migrateDanmoSheetJson } from '@/utils/univer-workbook'
import { isLegacyDanmoSheetPath } from '@/utils/file-route'
import { siblingUniverIrPath } from '@/utils/univer-ir'
import { useWorkspaceUiStore } from '@/stores/workspaceUi'
import { routeProjectFile } from '@/utils/file-route'
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
}>()

const { t } = useI18n()
const workspaceUi = useWorkspaceUiStore()
const containerRef = ref<HTMLElement | null>(null)
const loading = ref(false)
const saving = ref(false)
const dirty = ref(false)
const migrating = ref(false)

let univerInstance: { dispose: () => void } | null = null
let univerAPI: {
  createWorkbook: (data: unknown) => unknown
  getActiveWorkbook: () => { save: () => unknown; getSnapshot?: () => unknown } | null
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
  const { UniverSheetsCorePreset } = await import('@univerjs/preset-sheets-core')
  const UniverPresetSheetsCoreEnUS = (await import('@univerjs/preset-sheets-core/locales/en-US')).default
  await import('@univerjs/preset-sheets-core/lib/index.css')

  const created = createUniver({
    locale: LocaleType.EN_US,
    locales: {
      [LocaleType.EN_US]: mergeLocales(UniverPresetSheetsCoreEnUS),
    },
    presets: [
      UniverSheetsCorePreset({
        container: containerRef.value,
        toolbar: !readonly(),
        formulaBar: true,
        footer: {
          sheetBar: true,
          statisticBar: true,
          menus: true,
          zoomSlider: true,
        },
      }),
    ],
  })
  univerInstance = created.univer
  univerAPI = created.univerAPI as typeof univerAPI
  univerAPI!.createWorkbook(snapshot || emptyWorkbookData())
  dirty.value = false
  emit('dirty', false)
}

async function migrateLegacyIfNeeded(): Promise<string | null> {
  if (!isLegacyDanmoSheetPath(props.path)) return null
  migrating.value = true
  try {
    const fc = await fetchJSON<{ content: string }>(
      `/projects/${props.projectId}/files/content?path=${encodeURIComponent(props.path)}`,
    )
    const snapshot = migrateDanmoSheetJson(fc.content || '{}')
    let dest = siblingUniverIrPath(props.path, 'univer-sheet')
    // Avoid overwrite: add suffix if exists.
    try {
      await fetchJSON(`/projects/${props.projectId}/files/content?path=${encodeURIComponent(dest)}`)
      dest = dest.replace(/\.usheet\.json$/i, `.migrated-${Date.now()}.usheet.json`)
    } catch {
      /* dest free */
    }
    await fetchJSON(`/projects/${props.projectId}/files/content`, {
      method: 'PUT',
      body: JSON.stringify({ path: dest, content: stringifyUniverFile('univer-sheet', snapshot) }),
    })
    toast.success(t('office.migratedToUniverIr'))
    const routed = routeProjectFile(dest)
    workspaceUi.openStage({ ...routed })
    return dest
  } catch (e) {
    toast.error(e instanceof Error ? e.message : t('office.loadFailed'))
    return null
  } finally {
    migrating.value = false
  }
}

async function load() {
  if (!props.projectId || !props.path) return
  if (isLegacyDanmoSheetPath(props.path)) {
    await migrateLegacyIfNeeded()
    return
  }
  loading.value = true
  try {
    const fc = await fetchJSON<{ content: string }>(
      `/projects/${props.projectId}/files/content?path=${encodeURIComponent(props.path)}`,
    )
    const { snapshot } = parseUniverFile(fc.content || '{}', 'univer-sheet')
    await bootWithSnapshot(snapshot)
  } catch (e) {
    toast.error(e instanceof Error ? e.message : t('office.loadFailed'))
  } finally {
    loading.value = false
  }
}

async function save(opts?: { quiet?: boolean }) {
  if (!props.projectId || readonly()) return
  const wb = univerAPI?.getActiveWorkbook?.()
  if (!wb) return
  saving.value = true
  try {
    const snapshot = typeof wb.save === 'function' ? wb.save() : wb.getSnapshot?.()
    await fetchJSON(`/projects/${props.projectId}/files/content`, {
      method: 'PUT',
      body: JSON.stringify({
        path: props.path,
        content: stringifyUniverFile('univer-sheet', snapshot),
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
  return 'sheet'
}

function getSelectionMarkdown(): string {
  return `sheet: ${props.path}\nrange: active\n`
}

onMounted(() => {
  void load()
  // Univer mutates in-place; poll dirty lightly via click.
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

defineExpose({ save, getSelectionMarkdown, getEditScope, dirty, saving, loading })
</script>

<template>
  <div class="univer-sheet-surface">
    <div v-if="loading || migrating" class="univer-sheet-surface__status">
      {{ migrating ? t('office.migratingSheet') : t('office.loading') }}
    </div>
    <div ref="containerRef" class="univer-sheet-surface__host" />
  </div>
</template>

<style scoped>
.univer-sheet-surface {
  display: flex;
  flex-direction: column;
  min-height: 0;
  flex: 1;
  height: 100%;
}
.univer-sheet-surface__status {
  padding: 12px;
  color: var(--dq-label-secondary);
  font-size: var(--dq-font-size-caption);
}
.univer-sheet-surface__host {
  flex: 1;
  min-height: 0;
  height: 100%;
}
</style>
