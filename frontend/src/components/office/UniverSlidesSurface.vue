<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { fetchJSON } from '@/api/client'
import { toast } from '@/utils/feedback'
import { parseUniverFile, stringifyUniverFile } from '@/utils/univer-ir'
import { emptySlideData, pagesToSlideData } from '@/utils/univer-snapshots'
import type { FileEditScope } from '@/utils/file-route'

interface SlidePageView {
  id: string
  title: string
  body: string
}

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
const loading = ref(false)
const saving = ref(false)
const dirty = ref(false)
const deckTitle = ref('Presentation')
const pages = ref<SlidePageView[]>([])
const pageIndex = ref(0)
const snapshotMeta = ref<Record<string, unknown>>({})

const readonly = computed(() => props.mode === 'view' || props.mode === 'present' || !!props.turnRunning)
const active = computed(() => pages.value[pageIndex.value] || null)

function extractPages(snapshot: Record<string, unknown>): SlidePageView[] {
  const body = (snapshot.body || {}) as {
    pageOrder?: string[]
    pages?: Record<string, Record<string, unknown>>
  }
  const order = body.pageOrder || Object.keys(body.pages || {})
  return order.map((id, i) => {
    const page = body.pages?.[id] || {}
    const elements = (page.pageElements || {}) as Record<string, Record<string, unknown>>
    const texts: string[] = []
    for (const el of Object.values(elements)) {
      const rich = el.richText as { text?: string } | undefined
      if (rich?.text) texts.push(rich.text)
    }
    return {
      id,
      title: texts[0] || String(page.title || `Slide ${i + 1}`),
      body: texts.slice(1).join('\n') || '',
    }
  })
}

function buildSnapshot(): Record<string, unknown> {
  const built = pagesToSlideData(
    pages.value.map((p) => ({ title: p.title, body: p.body })),
    deckTitle.value,
  ) as Record<string, unknown>
  return { ...snapshotMeta.value, ...built, title: deckTitle.value }
}

async function load() {
  if (!props.projectId || !props.path) return
  loading.value = true
  try {
    const fc = await fetchJSON<{ content: string }>(
      `/projects/${props.projectId}/files/content?path=${encodeURIComponent(props.path)}`,
    )
    const { snapshot } = parseUniverFile<Record<string, unknown>>(fc.content || '{}', 'univer-slides')
    const snap = snapshot && Object.keys(snapshot).length ? snapshot : (emptySlideData() as Record<string, unknown>)
    snapshotMeta.value = { id: snap.id, pageSize: snap.pageSize, locale: snap.locale }
    deckTitle.value = String(snap.title || 'Presentation')
    pages.value = extractPages(snap)
    if (!pages.value.length) {
      pages.value = extractPages(emptySlideData() as Record<string, unknown>)
    }
    pageIndex.value = Math.min(pageIndex.value, pages.value.length - 1)
    emit('updatePageIndex', pageIndex.value)
    dirty.value = false
    emit('dirty', false)
  } catch (e) {
    toast.error(e instanceof Error ? e.message : t('office.loadFailed'))
  } finally {
    loading.value = false
  }
}

async function save(opts?: { quiet?: boolean }) {
  if (!props.projectId || readonly.value) return
  saving.value = true
  try {
    await fetchJSON(`/projects/${props.projectId}/files/content`, {
      method: 'PUT',
      body: JSON.stringify({
        path: props.path,
        content: stringifyUniverFile('univer-slides', buildSnapshot()),
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
  if (readonly.value) return
  dirty.value = true
  emit('dirty', true)
}

function selectPage(i: number) {
  pageIndex.value = i
  emit('updatePageIndex', i)
}

function addPage() {
  if (readonly.value) return
  pages.value = [
    ...pages.value,
    { id: `p_${Date.now()}`, title: `Slide ${pages.value.length + 1}`, body: '' },
  ]
  pageIndex.value = pages.value.length - 1
  emit('updatePageIndex', pageIndex.value)
  markDirty()
}

function removePage() {
  if (readonly.value || pages.value.length <= 1) return
  pages.value = pages.value.filter((_, i) => i !== pageIndex.value)
  pageIndex.value = Math.min(pageIndex.value, pages.value.length - 1)
  emit('updatePageIndex', pageIndex.value)
  markDirty()
}

function getEditScope(): FileEditScope {
  return 'slide'
}

function getSelectionMarkdown(): string {
  const p = active.value
  if (!p) return ''
  return `slide: ${pageIndex.value + 1}\ntitle: ${p.title}\n\n${p.body}\n`
}

watch(
  () => [props.projectId, props.path, props.reloadToken] as const,
  () => {
    void load()
  },
  { immediate: true },
)

defineExpose({ save, getSelectionMarkdown, getEditScope, dirty, saving, loading })
</script>

<template>
  <div class="univer-slides-surface">
    <div v-if="loading" class="univer-slides-surface__status">{{ t('office.loading') }}</div>
    <template v-else>
      <div class="univer-slides-surface__toolbar">
        <input
          v-model="deckTitle"
          class="univer-slides-surface__deck-title"
          :disabled="readonly"
          @input="markDirty"
        />
        <button type="button" class="univer-slides-surface__btn" :disabled="readonly" @click="addPage">
          {{ t('office.addSlide') }}
        </button>
        <button
          type="button"
          class="univer-slides-surface__btn"
          :disabled="readonly || pages.length <= 1"
          @click="removePage"
        >
          {{ t('office.deleteSlide') }}
        </button>
      </div>
      <div class="univer-slides-surface__body">
        <aside class="univer-slides-surface__thumbs">
          <button
            v-for="(p, i) in pages"
            :key="p.id"
            type="button"
            class="univer-slides-surface__thumb"
            :class="{ 'is-active': i === pageIndex }"
            @click="selectPage(i)"
          >
            <span class="univer-slides-surface__thumb-num">{{ i + 1 }}</span>
            <span class="univer-slides-surface__thumb-title">{{ p.title || t('office.emptySlide') }}</span>
          </button>
        </aside>
        <div v-if="active" class="univer-slides-surface__canvas">
          <input
            v-model="active.title"
            class="univer-slides-surface__title"
            :disabled="readonly"
            :placeholder="t('office.slideTitle')"
            @input="markDirty"
          />
          <textarea
            v-model="active.body"
            class="univer-slides-surface__text"
            :disabled="readonly"
            :placeholder="t('office.slideBody')"
            @input="markDirty"
          />
        </div>
      </div>
    </template>
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
}
.univer-slides-surface__toolbar {
  display: flex;
  gap: 8px;
  padding: 8px 10px;
  border-bottom: 1px solid var(--dq-separator-light);
  align-items: center;
}
.univer-slides-surface__deck-title {
  flex: 1;
  min-width: 0;
  height: 28px;
  border: 1px solid var(--dq-border);
  border-radius: 6px;
  padding: 0 8px;
  background: var(--dq-bg-elevated);
  color: var(--dq-label-primary);
}
.univer-slides-surface__btn {
  height: 28px;
  padding: 0 10px;
  border: 1px solid var(--dq-border);
  border-radius: 6px;
  background: var(--dq-fill-tertiary);
  color: var(--dq-label-primary);
  cursor: pointer;
  font-size: var(--dq-font-size-caption);
}
.univer-slides-surface__body {
  display: flex;
  min-height: 0;
  flex: 1;
}
.univer-slides-surface__thumbs {
  width: 180px;
  overflow: auto;
  border-right: 1px solid var(--dq-separator-light);
  padding: 8px;
  display: flex;
  flex-direction: column;
  gap: 6px;
}
.univer-slides-surface__thumb {
  display: flex;
  gap: 8px;
  text-align: left;
  border: 1px solid transparent;
  border-radius: 6px;
  padding: 8px;
  background: var(--dq-fill-tertiary);
  color: var(--dq-label-primary);
  cursor: pointer;
}
.univer-slides-surface__thumb.is-active {
  border-color: var(--dq-accent);
  background: color-mix(in srgb, var(--dq-accent) 12%, transparent);
}
.univer-slides-surface__thumb-num {
  color: var(--dq-label-tertiary);
  font-size: var(--dq-font-size-caption);
}
.univer-slides-surface__thumb-title {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  font-size: var(--dq-font-size-caption);
}
.univer-slides-surface__canvas {
  flex: 1;
  min-width: 0;
  padding: 24px;
  display: flex;
  flex-direction: column;
  gap: 12px;
  background: color-mix(in srgb, var(--dq-bg-elevated) 80%, #1a1a1a 20%);
}
.univer-slides-surface__title {
  font-size: 28px;
  font-weight: 650;
  border: 0;
  background: transparent;
  color: var(--dq-label-primary);
  padding: 8px 0;
}
.univer-slides-surface__text {
  flex: 1;
  min-height: 200px;
  border: 0;
  resize: none;
  background: transparent;
  color: var(--dq-label-primary);
  font-size: 16px;
  line-height: 1.5;
  font-family: inherit;
}
</style>
