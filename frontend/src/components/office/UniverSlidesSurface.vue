<script setup lang="ts">
import { onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { fetchJSON } from '@/api/client'
import { toast } from '@/utils/feedback'
import { parseUniverFile, stringifyUniverFile } from '@/utils/univer-ir'
import { emptySlideData } from '@/utils/univer-snapshots'
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
  updatePageIndex: [index: number]
}>()

const { t } = useI18n()
const containerRef = ref<HTMLElement | null>(null)
const loading = ref(false)
const saving = ref(false)
const dirty = ref(false)

type SlideUnit = {
  getSnapshot: () => Record<string, unknown>
  getActivePage: () => { id: string } | null | undefined
  getPageOrder: () => string[] | undefined
  activePage$?: { subscribe: (fn: (page: { id: string } | null | undefined) => void) => { unsubscribe: () => void } }
}

let univerInstance: { dispose: () => void } | null = null
let slideUnit: SlideUnit | null = null
let activePageSub: { unsubscribe: () => void } | null = null

const readonly = () => props.mode === 'view' || props.mode === 'present' || !!props.turnRunning

function syncPageIndex() {
  if (!slideUnit) {
    emit('updatePageIndex', 0)
    return
  }
  const order = slideUnit.getPageOrder?.() || []
  const active = slideUnit.getActivePage?.()
  const idx = active?.id ? Math.max(0, order.indexOf(active.id)) : 0
  emit('updatePageIndex', idx >= 0 ? idx : 0)
}

async function destroyUniver() {
  try {
    activePageSub?.unsubscribe()
  } catch {
    /* ignore */
  }
  activePageSub = null
  try {
    univerInstance?.dispose()
  } catch {
    /* ignore */
  }
  univerInstance = null
  slideUnit = null
  if (containerRef.value) containerRef.value.innerHTML = ''
}

async function bootWithSnapshot(snapshot: unknown) {
  await destroyUniver()
  if (!containerRef.value) return

  const { LocaleType, Univer, UniverInstanceType } = await import('@univerjs/core')
  const { mergeLocales } = await import('@univerjs/presets')
  const { UniverRenderEnginePlugin } = await import('@univerjs/engine-render')
  const { UniverFormulaEnginePlugin } = await import('@univerjs/engine-formula')
  const { UniverUIPlugin } = await import('@univerjs/ui')
  const { UniverDocsPlugin } = await import('@univerjs/docs')
  const { UniverDocsUIPlugin } = await import('@univerjs/docs-ui')
  const { UniverDrawingPlugin } = await import('@univerjs/drawing')
  const { UniverSlidesPlugin } = await import('@univerjs/slides')
  const { UniverSlidesUIPlugin } = await import('@univerjs/slides-ui')

  const DesignEnUS = (await import('@univerjs/design/locale/en-US')).default
  const UIEnUS = (await import('@univerjs/ui/locale/en-US')).default
  const DocsUIEnUS = (await import('@univerjs/docs-ui/locale/en-US')).default
  const SlidesUIEnUS = (await import('@univerjs/slides-ui/locale/en-US')).default

  await import('@univerjs/design/lib/index.css')
  await import('@univerjs/ui/lib/index.css')
  await import('@univerjs/docs-ui/lib/index.css')
  await import('@univerjs/slides-ui/lib/index.css')

  const univer = new Univer({
    locale: LocaleType.EN_US,
    locales: {
      [LocaleType.EN_US]: mergeLocales(DesignEnUS, UIEnUS, DocsUIEnUS, SlidesUIEnUS),
    },
  })

  univer.registerPlugin(UniverRenderEnginePlugin)
  univer.registerPlugin(UniverUIPlugin, {
    container: containerRef.value,
    toolbar: !readonly(),
  })
  univer.registerPlugin(UniverDocsPlugin)
  univer.registerPlugin(UniverDocsUIPlugin)
  univer.registerPlugin(UniverFormulaEnginePlugin)
  univer.registerPlugin(UniverDrawingPlugin)
  univer.registerPlugin(UniverSlidesPlugin)
  univer.registerPlugin(UniverSlidesUIPlugin)

  const unit = univer.createUnit(
    UniverInstanceType.UNIVER_SLIDE,
    (snapshot && typeof snapshot === 'object' ? snapshot : emptySlideData()) as Record<string, unknown>,
  ) as unknown as SlideUnit

  univerInstance = univer
  slideUnit = unit
  try {
    activePageSub = unit.activePage$?.subscribe(() => syncPageIndex()) ?? null
  } catch {
    activePageSub = null
  }
  syncPageIndex()
  dirty.value = false
  emit('dirty', false)
}

async function load() {
  if (!props.projectId || !props.path) return
  loading.value = true
  try {
    const fc = await fetchJSON<{ content: string }>(
      `/projects/${props.projectId}/files/content?path=${encodeURIComponent(props.path)}`,
    )
    const { snapshot } = parseUniverFile<Record<string, unknown>>(fc.content || '{}', 'univer-slides')
    await bootWithSnapshot(snapshot && Object.keys(snapshot).length ? snapshot : emptySlideData())
  } catch (e) {
    toast.error(e instanceof Error ? e.message : t('office.loadFailed'))
  } finally {
    loading.value = false
  }
}

async function save(opts?: { quiet?: boolean }) {
  if (!props.projectId || readonly() || !slideUnit) return
  saving.value = true
  try {
    const snapshot = slideUnit.getSnapshot()
    await fetchJSON(`/projects/${props.projectId}/files/content`, {
      method: 'PUT',
      body: JSON.stringify({
        path: props.path,
        content: stringifyUniverFile('univer-slides', snapshot),
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
  return 'slide'
}

function getSelectionMarkdown(): string {
  if (!slideUnit) return ''
  const snap = slideUnit.getSnapshot() as {
    title?: string
    body?: { pageOrder?: string[]; pages?: Record<string, { title?: string; pageElements?: Record<string, { richText?: { text?: string } }> }> }
  }
  const order = snap.body?.pageOrder || []
  const active = slideUnit.getActivePage?.()
  const pageId = active?.id || order[0]
  const page = pageId ? snap.body?.pages?.[pageId] : null
  if (!page) return ''
  const idx = pageId ? Math.max(0, order.indexOf(pageId)) : 0
  const texts: string[] = []
  for (const el of Object.values(page.pageElements || {})) {
    const text = el.richText?.text
    if (text) texts.push(text)
  }
  return `slide: ${idx + 1}\ntitle: ${page.title || snap.title || ''}\n\n${texts.join('\n')}\n`
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

defineExpose({ save, getSelectionMarkdown, getEditScope, dirty, saving, loading })
</script>

<template>
  <div class="univer-slides-surface">
    <div v-if="loading" class="univer-slides-surface__status">{{ t('office.loading') }}</div>
    <div ref="containerRef" class="univer-slides-surface__host" />
  </div>
</template>

<style scoped>
.univer-slides-surface {
  display: flex;
  flex-direction: column;
  min-height: 0;
  flex: 1;
  height: 100%;
}
.univer-slides-surface__status {
  padding: 12px;
  color: var(--dq-label-secondary);
  font-size: var(--dq-font-size-caption);
}
.univer-slides-surface__host {
  flex: 1;
  min-height: 0;
  height: 100%;
}
</style>
