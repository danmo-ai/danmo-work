<script setup lang="ts">
import { computed, nextTick, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { toast } from '@/utils/feedback'
import NovelCastWall from '@/components/novel/NovelCastWall.vue'
import NovelRelationGraph from '@/components/novel/NovelRelationGraph.vue'
import {
  formatChapterPlain,
  formatUnitProsePlain,
  type BookOutlineVolumeRow,
  type NovelCastCard,
  type NovelUnitEntry,
  type NovelUnitOutlineFields,
  type NovelUnitPhase,
  type NovelUnitProseSection,
  type NovelVolumeInfo,
} from '@/types/novel-workbench'

const props = defineProps<{
  crumb: string
  readTitle: string
  readHtml: string
  readLoading: boolean
  readContent: string
  treeKind: 'book' | 'volume' | 'unit' | 'cast' | 'setup' | 'ledger'
  readingIsOutline: boolean
  readingIsProse: boolean
  readingEntry: NovelUnitEntry | null
  unitOutline: NovelUnitOutlineFields | null
  unitPhase: NovelUnitPhase | null
  chapterChips: { chapter: number; title: string }[]
  chapterSections: NovelUnitProseSection[]
  chapterChars: Record<number, number>
  highlightChapter: number | null
  hasBookOutline: boolean
  bookOutlineRows: BookOutlineVolumeRow[]
  volume: NovelVolumeInfo | null
  cast: NovelCastCard[]
  castPane: 'wall' | 'graph' | 'card'
  castIssues: string[]
  prevUnit: NovelUnitEntry | null
  nextUnit: NovelUnitEntry | null
}>()

const emit = defineEmits<{
  'open-outline': []
  'open-prose': []
  'prev-unit': []
  'next-unit': []
  'highlight-chapter': [n: number]
  'select-unit': [unitId: string]
  'open-cast': [stem?: string]
  'set-cast-pane': [pane: 'wall' | 'graph']
}>()

const { t } = useI18n()
const bodyEl = ref<HTMLElement | null>(null)
const copyFlash = ref<'chapter' | 'unit' | number | null>(null)
let copyFlashTimer: ReturnType<typeof setTimeout> | null = null

const showCastBoard = computed(() => props.treeKind === 'cast' && props.castPane !== 'card')
const showUnitCard = computed(() => props.treeKind === 'unit' && props.readingIsOutline)

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

const cardRows = computed(() => {
  const u = props.unitOutline
  if (!u) return []
  const rows: { key: string; label: string; value: string }[] = []
  const push = (key: string, label: string, value: string) => {
    if (value.trim()) rows.push({ key, label, value })
  }
  push('function', t('novelWorkbench.unitFieldFunction'), u.functionText)
  push('entry', t('novelWorkbench.unitFieldEntry'), u.entry)
  push('desire', t('novelWorkbench.unitFieldDesire'), u.desire)
  push('obstacle', t('novelWorkbench.unitFieldObstacle'), u.obstacle)
  push('choice', t('novelWorkbench.unitFieldChoice'), u.choice)
  push('payoff', t('novelWorkbench.unitFieldPayoff'), u.payoff)
  push('pleasure', t('novelWorkbench.unitFieldPleasure'), u.pleasure)
  push('forbidden', t('novelWorkbench.unitFieldForbidden'), u.forbidden)
  push('onStage', t('novelWorkbench.unitFieldOnStage'), u.onStage.join('、'))
  push('pov', t('novelWorkbench.unitFieldPov'), u.pov)
  push('hook', t('novelWorkbench.unitFieldHook'), [u.hookType, u.hookOut].filter(Boolean).join(' · '))
  return rows
})

function emptyMessage(): string {
  if (showCastBoard.value) return ''
  if (props.treeKind === 'book' && !props.hasBookOutline && !props.readContent) return t('novelWorkbench.noBookOutline')
  if (props.treeKind === 'volume' && !props.volume?.rows.length && !props.readContent) return t('novelWorkbench.volumeUnitsEmpty')
  if (props.readingIsProse && !props.readingEntry?.prose) return t('novelWorkbench.noProseHint')
  if (showUnitCard.value && !props.unitOutline) return t('novelWorkbench.noContractYet')
  if (!props.readContent && !showUnitCard.value && props.treeKind !== 'volume') return ''
  return ''
}

const showMarkdown = computed(() => {
  if (showCastBoard.value || showUnitCard.value) return false
  if (props.treeKind === 'volume' && !props.readContent) return false
  return Boolean(props.readContent)
})

function unitNavLabel(entry: NovelUnitEntry | null, fallback: string): string {
  if (!entry) return fallback
  if (entry.chapterFrom > 0 && entry.chapterTo >= entry.chapterFrom) {
    return t('novelWorkbench.unitRow', { n: entry.index, from: entry.chapterFrom, to: entry.chapterTo })
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
  toast.success(kind === 'unit' ? t('novelWorkbench.copyUnitDone') : t('novelWorkbench.copyChapterDone'))
}

async function copyActiveChapter() {
  const section = activeChapter.value
  if (!section) return
  if (props.highlightChapter !== section.chapter) emit('highlight-chapter', section.chapter)
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
        <div v-if="canCopyProse" class="novel-reader__copy">
          <button
            type="button"
            class="novel-wb-link"
            :disabled="!activeChapter"
            :title="activeChapter ? t('novelWorkbench.copyChapterTip', { n: activeChapter.chapter }) : undefined"
            @click="copyActiveChapter"
          >
            {{
              copyFlash === 'chapter' || (activeChapter && copyFlash === activeChapter.chapter)
                ? t('novelWorkbench.copied')
                : t('novelWorkbench.copyChapter')
            }}
          </button>
          <button type="button" class="novel-wb-link" @click="copyUnit">
            {{ copyFlash === 'unit' ? t('novelWorkbench.copied') : t('novelWorkbench.copyUnit') }}
          </button>
        </div>
        <div v-if="treeKind === 'unit'" class="novel-reader__nav" :aria-label="t('novelWorkbench.unitNav')">
          <button type="button" class="novel-wb-link" :disabled="!prevUnit" @click="emit('prev-unit')">
            ← {{ unitNavLabel(prevUnit, t('novelWorkbench.prevUnitNone')) }}
          </button>
          <button type="button" class="novel-wb-link" :disabled="!nextUnit" @click="emit('next-unit')">
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

    <div v-if="treeKind === 'cast' && castPane !== 'card'" class="novel-reader__tabs" role="tablist">
      <button
        type="button"
        role="tab"
        class="novel-reader__tab"
        :class="{ 'novel-reader__tab--active': castPane === 'wall' }"
        @click="emit('set-cast-pane', 'wall')"
      >
        {{ t('novelWorkbench.castWall') }}
      </button>
      <button
        type="button"
        role="tab"
        class="novel-reader__tab"
        :class="{ 'novel-reader__tab--active': castPane === 'graph' }"
        @click="emit('set-cast-pane', 'graph')"
      >
        {{ t('novelWorkbench.castGraph') }}
      </button>
    </div>
    <div v-else-if="treeKind === 'cast'" class="novel-reader__tabs">
      <button type="button" class="novel-wb-link" @click="emit('open-cast')">
        ← {{ t('novelWorkbench.castWall') }}
      </button>
    </div>

    <div v-if="treeKind === 'unit' && readingIsProse && chapterChips.length" class="novel-reader__chips">
      <div v-for="chip in chapterChips" :key="chip.chapter" class="novel-reader__chip-wrap">
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

    <div class="novel-reader__scroll">
      <div v-if="treeKind === 'book' && bookOutlineRows.length" class="novel-reader__summary">
        <div class="novel-reader__summary-title">{{ t('novelWorkbench.volumeSummary') }}</div>
        <ul class="novel-reader__summary-list">
          <li v-for="(row, i) in bookOutlineRows" :key="i">
            <strong>{{ row.vol }}</strong>
            <span v-if="row.goal"> — {{ row.goal }}</span>
          </li>
        </ul>
      </div>

      <div v-if="treeKind === 'volume'" class="novel-reader__summary">
        <div class="novel-reader__summary-title">{{ t('novelWorkbench.volumeIndex') }}</div>
        <table v-if="volume?.rows.length" class="novel-reader__table">
          <thead>
            <tr>
              <th>{{ t('novelWorkbench.colUnit') }}</th>
              <th>{{ t('novelWorkbench.colRange') }}</th>
              <th>{{ t('novelWorkbench.colFunction') }}</th>
              <th>{{ t('novelWorkbench.colHook') }}</th>
              <th>{{ t('novelWorkbench.colEndgame') }}</th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="row in volume.rows" :key="row.id" class="novel-reader__row" @click="emit('select-unit', row.id)">
              <td>{{ row.id }}</td>
              <td>{{ row.range }}</td>
              <td>{{ row.purpose }}</td>
              <td>{{ row.hookType }}</td>
              <td>{{ row.endgameBoundary }}</td>
            </tr>
          </tbody>
        </table>
        <p v-else class="novel-reader__muted">{{ t('novelWorkbench.noVolumeRows') }}</p>
        <div v-if="volume?.cast.length" class="novel-reader__chips novel-reader__chips--in">
          <span class="novel-reader__summary-title">{{ t('novelWorkbench.volumeCast') }}</span>
          <button
            v-for="stem in volume.cast"
            :key="stem"
            type="button"
            class="novel-reader__chip novel-reader__chip--cast"
            @click="emit('open-cast', stem)"
          >
            {{ stem }}
          </button>
        </div>
      </div>

      <div v-if="showCastBoard && castIssues.length" class="novel-reader__issues">
        <div v-for="(issue, i) in castIssues" :key="i">{{ issue }}</div>
      </div>
      <NovelCastWall v-if="showCastBoard && castPane === 'wall'" :cards="cast" @open="emit('open-cast', $event)" />
      <NovelRelationGraph v-if="showCastBoard && castPane === 'graph'" :cards="cast" @open="emit('open-cast', $event)" />

      <div v-if="showUnitCard && unitOutline" class="novel-reader__card">
        <div v-if="unitPhase" class="novel-phase" :class="'novel-phase--' + unitPhase">
          {{ t(`novelWorkbench.phase${unitPhase === 'pending_outline' ? 'PendingOutline' : unitPhase === 'ready' ? 'Ready' : unitPhase === 'review_fail' ? 'ReviewFail' : unitPhase === 'finalized' ? 'Finalized' : 'Drafted'}`) }}
        </div>
        <dl v-if="cardRows.length" class="novel-reader__fields">
          <template v-for="f in cardRows" :key="f.key">
            <dt>{{ f.label }}</dt>
            <dd>{{ f.value }}</dd>
          </template>
        </dl>
        <div v-if="unitOutline.scenes.length" class="novel-reader__block">
          <div class="novel-reader__summary-title">{{ t('novelWorkbench.unitScenes') }}</div>
          <ol class="novel-reader__summary-list">
            <li v-for="s in unitOutline.scenes" :key="s.id || s.beat">
              <template v-if="s.id">{{ s.id }}</template><template v-if="s.id && s.beat"> · </template>{{ s.beat }}
              <span v-if="s.where"> · {{ s.where }}</span>
              <span v-if="s.chapter"> · {{ t('novelWorkbench.chapterN', { n: s.chapter }) }}</span>
              <span v-if="s.who.length"> · {{ s.who.join('、') }}</span>
            </li>
          </ol>
        </div>
        <div v-if="unitOutline.chapters.length" class="novel-reader__block">
          <div class="novel-reader__summary-title">{{ t('novelWorkbench.unitCuts') }}</div>
          <ul class="novel-reader__summary-list">
            <li v-for="c in unitOutline.chapters" :key="c.chapter">
              {{ t('novelWorkbench.chapterN', { n: c.chapter }) }}
              <span v-if="c.title"> {{ c.title }}</span>
              <span v-if="c.cutHook"> · {{ c.cutHook }}</span>
              <span v-if="c.wordShare"> · {{ chapterChars[c.chapter] || 0 }}/{{ c.wordShare }}</span>
            </li>
          </ul>
        </div>
      </div>

      <div v-if="readLoading" class="novel-reader__empty">{{ t('novelWorkbench.loading') }}</div>
      <div v-else-if="emptyMessage()" class="novel-reader__empty">{{ emptyMessage() }}</div>
      <div
        v-else-if="showMarkdown"
        ref="bodyEl"
        class="novel-reader__body"
        @click="onBodyClick"
        v-html="readHtml"
      />
    </div>
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

.novel-reader__bar-actions,
.novel-reader__copy,
.novel-reader__nav {
  display: flex;
  flex-shrink: 0;
  flex-wrap: wrap;
  align-items: center;
  justify-content: flex-end;
  gap: 4px;
}

.novel-reader__bar-actions {
  gap: 8px;
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

.novel-reader__chips--in {
  padding: 8px 0 0;
  align-items: center;
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

.novel-reader__chip--cast {
  border-right: 1px solid color-mix(in srgb, var(--dq-border-subtle, #000) 40%, transparent);
  border-radius: 999px;
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

.novel-reader__scroll {
  flex: 1;
  min-height: 0;
  overflow: auto;
  padding: 0 18px 28px;
}

.novel-reader__summary,
.novel-reader__card,
.novel-reader__issues {
  margin: 0 0 12px;
  padding: 8px 10px;
  border-radius: 8px;
  background: color-mix(in srgb, var(--dq-accent) 6%, transparent);
  border: 1px solid color-mix(in srgb, var(--dq-accent) 12%, transparent);
}

.novel-reader__issues {
  background: color-mix(in srgb, var(--dq-danger, #dc2626) 8%, transparent);
  border-color: color-mix(in srgb, var(--dq-danger, #dc2626) 25%, transparent);
  font-size: var(--dq-font-size-caption);
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
}

.novel-reader__table {
  width: 100%;
  border-collapse: collapse;
  font-size: var(--dq-font-size-caption);
}

.novel-reader__table th,
.novel-reader__table td {
  padding: 6px 8px;
  border-bottom: 1px solid color-mix(in srgb, var(--dq-border-subtle, #000) 30%, transparent);
  text-align: left;
  vertical-align: top;
}

.novel-reader__table th {
  font-size: 11px;
  font-weight: 650;
  opacity: 0.6;
}

.novel-reader__row {
  cursor: pointer;
}

.novel-reader__row:hover {
  background: color-mix(in srgb, var(--dq-accent) 10%, transparent);
}

.novel-reader__fields {
  margin: 0;
  display: grid;
  grid-template-columns: 5.5rem 1fr;
  gap: 6px 10px;
  font-size: var(--dq-font-size-caption);
}

.novel-reader__fields dt {
  opacity: 0.55;
  font-weight: 650;
}

.novel-reader__fields dd {
  margin: 0;
  line-height: 1.4;
}

.novel-reader__block {
  margin-top: 10px;
}

.novel-reader__muted,
.novel-reader__empty {
  font-size: var(--dq-font-size-body);
  opacity: 0.7;
  line-height: 1.45;
}

.novel-reader__empty {
  padding: 24px 0;
}

.novel-reader__body {
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
