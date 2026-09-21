<script setup lang="ts">
import { computed, nextTick, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { toast } from '@/utils/feedback'
import {
  formatChapterPlain,
  formatUnitProsePlain,
  type BookOutlineVolumeRow,
  type NovelUnitEntry,
  type NovelUnitProseSection,
  type VolumeUnitRow,
} from '@/types/novel-workbench'

const props = defineProps<{
  crumb: string
  readTitle: string
  readHtml: string
  readLoading: boolean
  readContent: string
  treeKind: 'book' | 'volume' | 'setup' | 'unit' | 'dossier'
  readingIsOutline: boolean
  readingIsProse: boolean
  readingEntry: NovelUnitEntry | null
  chapterChips: { chapter: number; title: string }[]
  chapterSections: NovelUnitProseSection[]
  highlightChapter: number | null
  hasBookOutline: boolean
  bookOutlineRows: BookOutlineVolumeRow[]
  volumeUnits: VolumeUnitRow[]
  prevUnit: NovelUnitEntry | null
  nextUnit: NovelUnitEntry | null
}>()

const emit = defineEmits<{
  'open-outline': []
  'open-prose': []
  'prev-unit': []
  'next-unit': []
  'highlight-chapter': [n: number]
}>()

const { t } = useI18n()
const bodyEl = ref<HTMLElement | null>(null)
const copyFlash = ref<'chapter' | 'unit' | number | null>(null)
let copyFlashTimer: ReturnType<typeof setTimeout> | null = null

const canCopyProse = computed(
  () => props.treeKind === 'unit' && props.readingIsProse && props.chapterSections.length > 0,
)

const activeChapter = computed(() => {
  if (!canCopyProse.value) return null
  if (props.highlightChapter != null) {
    const hit = props.chapterSections.find((s) => s.chapter === props.highlightChapter)
    if (hit) return hit
  }
  return props.chapterSections[0] ?? null
})

function emptyMessage(): string {
  if (props.treeKind === 'book' && !props.hasBookOutline) return t('novelWorkbench.noBookOutline')
  if (props.treeKind === 'volume' && !props.readContent) return t('novelWorkbench.volumeUnitsEmpty')
  if (props.readingIsProse && !props.readingEntry?.prose) return t('novelWorkbench.noProseYet')
  if (props.readingIsOutline && !props.readingEntry?.outline) return t('novelWorkbench.noContractYet')
  return ''
}

function unitNavLabel(entry: NovelUnitEntry | null, fallback: string): string {
  if (!entry) return fallback
  if (entry.chapterFrom > 0 && entry.chapterTo >= entry.chapterFrom) {
    return t('novelWorkbench.unitRow', {
      n: entry.index,
      from: entry.chapterFrom,
      to: entry.chapterTo,
    })
  }
  return entry.unitId
}

function chipLabel(chip: { chapter: number; title: string }): string {
  return chip.title
    ? t('novelWorkbench.chapterTitle', { n: chip.chapter, title: chip.title })
    : t('novelWorkbench.chapterN', { n: chip.chapter })
}

async function writeClipboard(text: string): Promise<boolean> {
  try {
    if (navigator.clipboard?.writeText) {
      await navigator.clipboard.writeText(text)
      return true
    }
  } catch {
    /* fall through */
  }
  try {
    const ta = document.createElement('textarea')
    ta.value = text
    ta.setAttribute('readonly', '')
    ta.style.position = 'fixed'
    ta.style.left = '-9999px'
    document.body.appendChild(ta)
    ta.select()
    const ok = document.execCommand('copy')
    document.body.removeChild(ta)
    return ok
  } catch {
    return false
  }
}

function flashCopy(kind: 'chapter' | 'unit' | number) {
  copyFlash.value = kind
  if (copyFlashTimer) clearTimeout(copyFlashTimer)
  copyFlashTimer = setTimeout(() => {
    copyFlash.value = null
  }, 1400)
}

async function copyText(text: string, kind: 'chapter' | 'unit' | number) {
  const ok = await writeClipboard(text)
  if (!ok) {
    toast.error(t('novelWorkbench.copyFailed'))
    return
  }
  flashCopy(kind)
  toast.success(
    kind === 'unit' ? t('novelWorkbench.copyUnitDone') : t('novelWorkbench.copyChapterDone'),
  )
}

async function copyActiveChapter() {
  const section = activeChapter.value
  if (!section) return
  if (props.highlightChapter !== section.chapter) {
    emit('highlight-chapter', section.chapter)
  }
  await copyText(formatChapterPlain(section), 'chapter')
}

async function copyUnit() {
  if (!props.chapterSections.length) return
  await copyText(formatUnitProsePlain(props.chapterSections), 'unit')
}

async function copyChapter(n: number) {
  const section = props.chapterSections.find((s) => s.chapter === n)
  if (!section) return
  emit('highlight-chapter', n)
  await copyText(formatChapterPlain(section), n)
}

async function onChip(n: number) {
  emit('highlight-chapter', n)
  await nextTick()
  bodyEl.value?.querySelector(`#unit-ch-${n}`)?.scrollIntoView({ block: 'start' })
}

function onBodyClick(ev: MouseEvent) {
  const target = ev.target as HTMLElement | null
  const btn = target?.closest?.('[data-copy-chapter]') as HTMLElement | null
  if (!btn) return
  ev.preventDefault()
  const n = Number(btn.getAttribute('data-copy-chapter'))
  if (!Number.isFinite(n)) return
  void copyChapter(n)
}
</script>

<template>
  <section class="novel-reader">
    <div class="novel-reader__bar">
      <div class="novel-reader__head">
        <div class="novel-reader__crumb">{{ crumb }}</div>
        <div class="novel-reader__file">{{ readTitle }}</div>
      </div>
      <div class="novel-reader__bar-actions">
        <div
          v-if="canCopyProse"
          class="novel-reader__copy"
        >
          <button
            type="button"
            class="novel-wb-link"
            :disabled="!activeChapter"
            :title="
              activeChapter
                ? t('novelWorkbench.copyChapterTip', { n: activeChapter.chapter })
                : undefined
            "
            @click="copyActiveChapter"
          >
            {{
              copyFlash === 'chapter' || (activeChapter && copyFlash === activeChapter.chapter)
                ? t('novelWorkbench.copied')
                : t('novelWorkbench.copyChapter')
            }}
          </button>
          <button
            type="button"
            class="novel-wb-link"
            @click="copyUnit"
          >
            {{ copyFlash === 'unit' ? t('novelWorkbench.copied') : t('novelWorkbench.copyUnit') }}
          </button>
        </div>
        <div
          v-if="treeKind === 'unit'"
          class="novel-reader__nav"
          :aria-label="t('novelWorkbench.unitNav')"
        >
          <button
            type="button"
            class="novel-wb-link"
            :disabled="!prevUnit"
            @click="emit('prev-unit')"
          >
            ← {{ unitNavLabel(prevUnit, t('novelWorkbench.prevUnitNone')) }}
          </button>
          <button
            type="button"
            class="novel-wb-link"
            :disabled="!nextUnit"
            @click="emit('next-unit')"
          >
            {{ unitNavLabel(nextUnit, t('novelWorkbench.nextUnitNone')) }} →
          </button>
        </div>
      </div>
    </div>

    <div v-if="treeKind === 'unit'" class="novel-reader__tabs" role="tablist">
      <button
        type="button"
        role="tab"
        class="novel-reader__tab"
        :class="{
          'novel-reader__tab--active': readingIsOutline,
          'novel-reader__tab--missing': !readingEntry?.outline,
        }"
        :aria-selected="readingIsOutline"
        @click="emit('open-outline')"
      >
        {{ t('novelWorkbench.badgeContract') }}
      </button>
      <button
        type="button"
        role="tab"
        class="novel-reader__tab"
        :class="{
          'novel-reader__tab--active': readingIsProse,
          'novel-reader__tab--missing': !readingEntry?.prose,
        }"
        :aria-selected="readingIsProse"
        @click="emit('open-prose')"
      >
        {{ t('novelWorkbench.badgeProse') }}
      </button>
    </div>

    <div v-if="treeKind === 'unit' && readingIsProse && chapterChips.length" class="novel-reader__chips">
      <div
        v-for="chip in chapterChips"
        :key="chip.chapter"
        class="novel-reader__chip-wrap"
      >
        <button
          type="button"
          class="novel-reader__chip"
          :class="{ 'novel-reader__chip--on': highlightChapter === chip.chapter }"
          @click="onChip(chip.chapter)"
        >
          {{ chipLabel(chip) }}
        </button>
        <button
          type="button"
          class="novel-reader__chip-copy"
          :class="{ 'novel-reader__chip-copy--on': copyFlash === chip.chapter }"
          :title="t('novelWorkbench.copyChapterTip', { n: chip.chapter })"
          @click="copyChapter(chip.chapter)"
        >
          {{ copyFlash === chip.chapter ? t('novelWorkbench.copiedShort') : t('novelWorkbench.copyShort') }}
        </button>
      </div>
    </div>

    <div v-if="treeKind === 'book' && bookOutlineRows.length" class="novel-reader__summary">
      <div class="novel-reader__summary-title">{{ t('novelWorkbench.volumeSummary') }}</div>
      <ul class="novel-reader__summary-list">
        <li v-for="(row, i) in bookOutlineRows" :key="i">
          <strong>{{ row.vol }}</strong>
          <span v-if="row.goal"> — {{ row.goal }}</span>
        </li>
      </ul>
    </div>

    <div v-if="treeKind === 'volume' && volumeUnits.length" class="novel-reader__summary">
      <div class="novel-reader__summary-title">{{ t('novelWorkbench.unitSummary') }}</div>
      <ul class="novel-reader__summary-list">
        <li v-for="(u, i) in volumeUnits" :key="i">
          <strong>{{ u.id || u.range }}</strong>
          <span v-if="u.range && u.id"> · {{ u.range }}</span>
          <span v-if="u.purpose"> — {{ u.purpose }}</span>
        </li>
      </ul>
    </div>

    <div v-if="readLoading" class="novel-reader__empty">{{ t('novelWorkbench.loading') }}</div>
    <div v-else-if="emptyMessage()" class="novel-reader__empty">{{ emptyMessage() }}</div>
    <div
      v-else
      ref="bodyEl"
      class="novel-reader__body"
      @click="onBodyClick"
      v-html="readHtml"
    />
  </section>
</template>

<style scoped>
.novel-reader {
  flex: 1;
  min-width: 0;
  display: flex;
  flex-direction: column;
  overflow: hidden;
  background: color-mix(in srgb, var(--dq-glass-popover-bg, #fff) 88%, transparent);
}

.novel-reader__bar {
  flex-shrink: 0;
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 10px;
  padding: 10px 14px 6px;
}

.novel-reader__head {
  min-width: 0;
}

.novel-reader__crumb {
  font-size: var(--dq-font-size-caption);
  font-weight: 650;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.novel-reader__file {
  margin-top: 2px;
  font-size: 11px;
  opacity: 0.5;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.novel-reader__bar-actions {
  display: flex;
  flex-shrink: 0;
  flex-wrap: wrap;
  align-items: center;
  justify-content: flex-end;
  gap: 8px;
}

.novel-reader__copy,
.novel-reader__nav {
  display: flex;
  flex-shrink: 0;
  gap: 4px;
}

.novel-reader__tabs {
  flex-shrink: 0;
  display: flex;
  gap: 4px;
  padding: 0 14px 8px;
}

.novel-reader__tab {
  margin: 0;
  padding: 5px 12px;
  border: 1px solid color-mix(in srgb, var(--dq-border-subtle, #000) 45%, transparent);
  border-radius: 6px;
  background: transparent;
  color: inherit;
  font: inherit;
  font-size: var(--dq-font-size-caption);
  cursor: pointer;
}

.novel-reader__tab--active {
  border-color: color-mix(in srgb, var(--dq-accent) 50%, transparent);
  background: color-mix(in srgb, var(--dq-accent) 12%, transparent);
  font-weight: 650;
}

.novel-reader__tab--missing {
  opacity: 0.55;
}

.novel-reader__chips {
  flex-shrink: 0;
  display: flex;
  flex-wrap: wrap;
  gap: 4px;
  padding: 0 14px 8px;
}

.novel-reader__chip-wrap {
  display: inline-flex;
  align-items: stretch;
  max-width: 100%;
}

.novel-reader__chip {
  margin: 0;
  padding: 2px 8px;
  border: 1px solid color-mix(in srgb, var(--dq-border-subtle, #000) 40%, transparent);
  border-right: none;
  border-radius: 999px 0 0 999px;
  background: transparent;
  color: inherit;
  font: inherit;
  font-size: 11px;
  cursor: pointer;
  max-width: 14rem;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.novel-reader__chip--on {
  border-color: color-mix(in srgb, var(--dq-accent) 55%, transparent);
  background: color-mix(in srgb, var(--dq-accent) 14%, transparent);
  font-weight: 650;
}

.novel-reader__chip-copy {
  margin: 0;
  padding: 2px 7px;
  border: 1px solid color-mix(in srgb, var(--dq-border-subtle, #000) 40%, transparent);
  border-radius: 0 999px 999px 0;
  background: color-mix(in srgb, var(--dq-border-subtle, #000) 6%, transparent);
  color: inherit;
  font: inherit;
  font-size: 11px;
  cursor: pointer;
  opacity: 0.78;
}

.novel-reader__chip-wrap:hover .novel-reader__chip-copy,
.novel-reader__chip--on + .novel-reader__chip-copy,
.novel-reader__chip-copy--on {
  opacity: 1;
  border-color: color-mix(in srgb, var(--dq-accent) 45%, transparent);
  background: color-mix(in srgb, var(--dq-accent) 10%, transparent);
}

.novel-reader__summary {
  flex-shrink: 0;
  margin: 0 14px 8px;
  padding: 8px 10px;
  border-radius: 8px;
  background: color-mix(in srgb, var(--dq-accent) 6%, transparent);
  border: 1px solid color-mix(in srgb, var(--dq-accent) 12%, transparent);
}

.novel-reader__summary-title {
  font-size: 11px;
  font-weight: 650;
  opacity: 0.7;
  margin-bottom: 4px;
}

.novel-reader__summary-list {
  margin: 0;
  padding-left: 16px;
  font-size: var(--dq-font-size-caption);
  line-height: 1.45;
  opacity: 0.9;
}

.novel-reader__empty {
  padding: 24px 16px;
  font-size: var(--dq-font-size-body);
  opacity: 0.7;
  line-height: 1.45;
}

.novel-reader__body {
  flex: 1;
  min-height: 0;
  overflow: auto;
  padding: 4px 18px 28px;
  max-width: 42rem;
  width: 100%;
  margin: 0 auto;
  font-size: 15px;
  line-height: 1.7;
}

.novel-reader__body :deep(h1),
.novel-reader__body :deep(h2),
.novel-reader__body :deep(h3) {
  line-height: 1.3;
  margin: 1.1em 0 0.45em;
}

.novel-reader__body :deep(p) {
  margin: 0.55em 0;
}

.novel-reader__body :deep(.novel-unit-cut) {
  border: none;
  border-top: 1px solid color-mix(in srgb, var(--dq-border-subtle, #000) 55%, transparent);
  margin: 1.6rem 0 0.4rem;
}

.novel-reader__body :deep(.novel-unit-ch__head) {
  display: flex;
  align-items: baseline;
  justify-content: space-between;
  gap: 10px;
  margin: 1.1em 0 0.45em;
}

.novel-reader__body :deep(.novel-unit-ch__head h2) {
  margin: 0;
  flex: 1;
  min-width: 0;
}

.novel-reader__body :deep(.novel-unit-ch__copy) {
  flex-shrink: 0;
  margin: 0;
  padding: 2px 8px;
  border: 1px solid color-mix(in srgb, var(--dq-border-subtle, #000) 45%, transparent);
  border-radius: 6px;
  background: color-mix(in srgb, var(--dq-glass-popover-bg, #fff) 70%, transparent);
  color: inherit;
  font: inherit;
  font-size: 11px;
  cursor: pointer;
  opacity: 0.55;
}

.novel-reader__body :deep(.novel-unit-ch:hover .novel-unit-ch__copy),
.novel-reader__body :deep(.novel-unit-ch--on .novel-unit-ch__copy) {
  opacity: 1;
  border-color: color-mix(in srgb, var(--dq-accent) 45%, transparent);
}

.novel-reader__body :deep(.novel-unit-ch--on) {
  background: color-mix(in srgb, var(--dq-accent) 7%, transparent);
  border-radius: 8px;
  padding: 4px 8px;
}

.novel-reader__body :deep(pre),
.novel-reader__body :deep(.novel-wb__pre) {
  overflow: auto;
  padding: 10px 12px;
  border-radius: 8px;
  background: color-mix(in srgb, var(--dq-border-subtle, #000) 12%, transparent);
  font-size: 12.5px;
  line-height: 1.45;
}
</style>
