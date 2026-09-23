<script setup lang="ts">
import { computed, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import {
  novelBiblePath,
  novelCanonDir,
  setupDocLabel,
  volumeId,
  volumeNumFromName,
  isUnitPhasePending,
  type NovelFileNode,
  type NovelUnitEntry,
  type NovelUnitPhase,
  type NovelVolumeInfo,
} from '@/types/novel-workbench'

const props = defineProps<{
  bookId: string
  treeSel: { kind: 'book' | 'volume' | 'unit' | 'cast' | 'setup' | 'ledger'; name?: string }
  volumes: NovelVolumeInfo[]
  units: NovelUnitEntry[]
  unitPhases: Record<string, NovelUnitPhase>
  /** Unit the primary button is about (dot marker). */
  primaryUnit: string | null
  worldDocs: NovelFileNode[]
  ledgerFiles: NovelFileNode[]
  summaryFiles: NovelFileNode[]
  castCount: number
  castIssueCount: number
}>()

const emit = defineEmits<{
  'select-book-outline': []
  'select-volume': [volume: number]
  'select-unit': [unitId: string]
  'select-cast': []
  'select-setup': [path: string, name: string]
  'select-facts': []
  'select-ledger': [node: NovelFileNode, kind: 'facts' | 'summary']
}>()

const { t } = useI18n()
const pendingOnly = ref(false)
const setupOpen = ref(false)
const ledgerOpen = ref(false)

const SETUP_DOC_KEYS = new Set(['bible', 'world', 'glossary', 'reveal', 'rules', 'platform', 'goldfinger', 'authorLore', 'style', 'lockedTerms'])

function setupDocTitle(name: string): string {
  const id = setupDocLabel(name)
  if (SETUP_DOC_KEYS.has(id)) return t(`novelWorkbench.setupDoc_${id}`)
  return id
}

function phaseLabel(phase: NovelUnitPhase): string {
  const map: Record<NovelUnitPhase, string> = {
    pending_outline: 'phasePendingOutline',
    ready: 'phaseReady',
    drafted: 'phaseDrafted',
    review_fail: 'phaseReviewFail',
    finalized: 'phaseFinalized',
  }
  return t(`novelWorkbench.${map[phase]}`)
}

function unitRange(entry: NovelUnitEntry): string {
  if (entry.chapterFrom > 0 && entry.chapterTo >= entry.chapterFrom) {
    return `ch${entry.chapterFrom}–${entry.chapterTo}`
  }
  return ''
}

function unitPurpose(entry: NovelUnitEntry): string {
  const vol = props.volumes.find((v) => v.volume === entry.volume)
  const row = vol?.rows.find((r) => r.id === entry.unitId)
  return row?.purpose ?? ''
}

/** Volumes from files ∪ volumes implied by unit ids, sorted. */
const volumeGroups = computed(() => {
  const nums = new Set<number>()
  for (const v of props.volumes) nums.add(v.volume)
  for (const u of props.units) nums.add(u.volume)
  return [...nums]
    .sort((a, b) => a - b)
    .map((volume) => {
      const units = props.units
        .filter((u) => u.volume === volume)
        .filter((u) => !pendingOnly.value || isUnitPhasePending(props.unitPhases[u.unitId] ?? 'pending_outline'))
      const all = props.units.filter((u) => u.volume === volume)
      const finalized = all.filter((u) => props.unitPhases[u.unitId] === 'finalized').length
      return {
        volume,
        hasOutline: props.volumes.some((v) => v.volume === volume),
        units,
        total: all.length,
        finalized,
      }
    })
})

const isVolumeSelected = (volume: number) =>
  props.treeSel.kind === 'volume' && volumeNumFromName(props.treeSel.name || '') === volume

const isSetupSelected = (name: string) => props.treeSel.kind === 'setup' && props.treeSel.name === name
</script>

<template>
  <aside class="novel-binder">
    <section class="novel-binder__section">
      <button
        type="button"
        class="novel-binder__item novel-binder__item--top"
        :class="{ 'novel-binder__item--on': treeSel.kind === 'book' }"
        @click="emit('select-book-outline')"
      >
        {{ t('novelWorkbench.bookOutline') }}
      </button>
    </section>

    <section class="novel-binder__section novel-binder__section--grow">
      <div class="novel-binder__head">
        <span>{{ t('novelWorkbench.folderVolumes') }}</span>
        <button
          v-if="units.length"
          type="button"
          class="novel-binder__filter-btn"
          :class="{ 'novel-binder__filter-btn--on': pendingOnly }"
          @click="pendingOnly = !pendingOnly"
        >
          {{ pendingOnly ? t('novelWorkbench.showAllUnits', { n: units.length }) : t('novelWorkbench.showPendingUnits') }}
        </button>
      </div>

      <div v-for="g in volumeGroups" :key="g.volume" class="novel-binder__vol">
        <button
          type="button"
          class="novel-binder__item novel-binder__item--vol"
          :class="{ 'novel-binder__item--on': isVolumeSelected(g.volume), 'novel-binder__item--dim': !g.hasOutline }"
          @click="emit('select-volume', g.volume)"
        >
          <span class="novel-binder__vol-name">{{ volumeId(g.volume) }}</span>
          <span class="novel-binder__vol-meta">
            {{ g.hasOutline ? t('novelWorkbench.volumeProgress', { done: g.finalized, total: g.total }) : t('novelWorkbench.volumeNoOutline') }}
          </span>
        </button>
        <button
          v-for="entry in g.units"
          :key="entry.unitId"
          type="button"
          class="novel-binder__item novel-binder__item--unit"
          :class="{
            'novel-binder__item--on': treeSel.kind === 'unit' && treeSel.name === entry.unitId,
          }"
          :title="unitPurpose(entry) || entry.unitId"
          @click="emit('select-unit', entry.unitId)"
        >
          <span class="novel-binder__dot" :class="'novel-binder__dot--' + (unitPhases[entry.unitId] || 'pending_outline')" />
          <span class="novel-binder__unit-main">
            <span class="novel-binder__unit-line">
              <span class="novel-binder__unit-id">U{{ entry.index }}</span>
              <span v-if="unitRange(entry)" class="novel-binder__unit-range">{{ unitRange(entry) }}</span>
              <span v-if="primaryUnit === entry.unitId" class="novel-binder__next">▶</span>
            </span>
            <span v-if="unitPurpose(entry)" class="novel-binder__unit-purpose">{{ unitPurpose(entry) }}</span>
          </span>
          <span class="novel-phase" :class="'novel-phase--' + (unitPhases[entry.unitId] || 'pending_outline')">
            {{ phaseLabel(unitPhases[entry.unitId] || 'pending_outline') }}
          </span>
        </button>
        <p v-if="g.hasOutline && !g.units.length" class="novel-binder__hint">
          {{ g.total ? t('novelWorkbench.noPendingUnits') : t('novelWorkbench.volumeNotAccepted') }}
        </p>
      </div>
      <p v-if="!volumeGroups.length" class="novel-binder__hint">{{ t('novelWorkbench.noVolumesYet') }}</p>
    </section>

    <section class="novel-binder__section novel-binder__section--bottom">
      <button
        type="button"
        class="novel-binder__item novel-binder__item--top"
        :class="{ 'novel-binder__item--on': treeSel.kind === 'cast' }"
        @click="emit('select-cast')"
      >
        <span>{{ t('novelWorkbench.castWall') }}</span>
        <span class="novel-binder__count">
          {{ castCount }}
          <span v-if="castIssueCount" class="novel-binder__warn" :title="t('novelWorkbench.castLintFail')">!</span>
        </span>
      </button>

      <button type="button" class="novel-binder__item novel-binder__item--top" @click="setupOpen = !setupOpen">
        <span>{{ t('novelWorkbench.folderSetup') }}</span>
        <span class="novel-binder__count">{{ setupOpen ? '−' : '+' }}</span>
      </button>
      <template v-if="setupOpen">
        <button
          type="button"
          class="novel-binder__item novel-binder__item--nested"
          :class="{ 'novel-binder__item--on': isSetupSelected('book-bible.md') }"
          @click="emit('select-setup', novelBiblePath(bookId), 'book-bible.md')"
        >
          {{ t('novelWorkbench.setupDoc_bible') }}
        </button>
        <button
          v-for="f in worldDocs"
          :key="f.name"
          type="button"
          class="novel-binder__item novel-binder__item--nested"
          :class="{ 'novel-binder__item--on': isSetupSelected(f.name) }"
          @click="emit('select-setup', f.path || `${novelCanonDir(bookId)}/${f.name}`, f.name)"
        >
          {{ setupDocTitle(f.name) }}
        </button>
        <p v-if="!worldDocs.length" class="novel-binder__hint">{{ t('novelWorkbench.noCanonYet') }}</p>
      </template>

      <button type="button" class="novel-binder__item novel-binder__item--top" @click="ledgerOpen = !ledgerOpen">
        <span>{{ t('novelWorkbench.ledger') }}</span>
        <span class="novel-binder__count">{{ ledgerOpen ? '−' : '+' }}</span>
      </button>
      <template v-if="ledgerOpen">
        <button
          type="button"
          class="novel-binder__item novel-binder__item--nested"
          :class="{ 'novel-binder__item--on': treeSel.kind === 'ledger' && treeSel.name === 'facts.md' }"
          @click="emit('select-facts')"
        >
          facts.md
        </button>
        <button
          v-for="f in summaryFiles"
          :key="'s-' + f.name"
          type="button"
          class="novel-binder__item novel-binder__item--nested"
          :class="{ 'novel-binder__item--on': treeSel.kind === 'ledger' && treeSel.name === f.name }"
          @click="emit('select-ledger', f, 'summary')"
        >
          summaries/{{ f.name }}
        </button>
        <button
          v-for="f in ledgerFiles"
          :key="'l-' + f.name"
          type="button"
          class="novel-binder__item novel-binder__item--nested novel-binder__item--dim"
          :class="{ 'novel-binder__item--on': treeSel.kind === 'ledger' && treeSel.name === f.name }"
          @click="emit('select-ledger', f, 'facts')"
        >
          {{ f.name }}
        </button>
      </template>
    </section>
  </aside>
</template>

<style scoped>
.novel-binder {
  flex: 0 0 250px;
  min-width: 210px;
  max-width: 300px;
  display: flex;
  flex-direction: column;
  min-height: 0;
  overflow: hidden;
  border-right: 1px solid color-mix(in srgb, var(--dq-border-subtle, #000) 50%, transparent);
  background: color-mix(in srgb, var(--dq-glass-popover-bg, #fff) 40%, transparent);
}

.novel-binder__section {
  flex-shrink: 0;
  padding: 6px 0;
}

.novel-binder__section--grow {
  flex: 1;
  min-height: 0;
  overflow: auto;
  border-top: 1px solid color-mix(in srgb, var(--dq-border-subtle, #000) 30%, transparent);
  border-bottom: 1px solid color-mix(in srgb, var(--dq-border-subtle, #000) 30%, transparent);
}

.novel-binder__section--bottom {
  max-height: 45%;
  overflow: auto;
}

.novel-binder__head {
  display: flex;
  align-items: baseline;
  justify-content: space-between;
  gap: 8px;
  padding: 4px 12px 6px;
  font-size: 11px;
  font-weight: 700;
  letter-spacing: 0.04em;
  opacity: 0.6;
}

.novel-binder__item {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 6px;
  width: 100%;
  margin: 0;
  padding: 6px 12px;
  border: none;
  background: transparent;
  color: inherit;
  font: inherit;
  font-size: var(--dq-font-size-caption);
  text-align: left;
  cursor: pointer;
}

.novel-binder__item:hover {
  background: color-mix(in srgb, var(--dq-accent) 10%, transparent);
}

.novel-binder__item--on {
  background: color-mix(in srgb, var(--dq-accent) 14%, transparent);
  font-weight: 650;
}

.novel-binder__item--top {
  font-weight: 650;
}

.novel-binder__item--nested {
  padding-left: 26px;
}

.novel-binder__item--dim {
  opacity: 0.7;
}

.novel-binder__vol {
  padding-bottom: 4px;
}

.novel-binder__item--vol {
  padding: 5px 12px 3px;
}

.novel-binder__vol-name {
  font-size: 11px;
  font-weight: 700;
  letter-spacing: 0.04em;
}

.novel-binder__vol-meta {
  font-size: 10px;
  opacity: 0.55;
}

.novel-binder__item--unit {
  align-items: flex-start;
  padding: 5px 12px 5px 16px;
}

.novel-binder__dot {
  flex-shrink: 0;
  width: 8px;
  height: 8px;
  margin-top: 4px;
  border-radius: 50%;
  background: color-mix(in srgb, var(--dq-border-subtle, #000) 45%, transparent);
}

.novel-binder__dot--ready,
.novel-binder__dot--drafted {
  background: var(--dq-accent);
}

.novel-binder__dot--review_fail {
  background: var(--dq-danger, #dc2626);
}

.novel-binder__dot--finalized {
  background: var(--dq-success, #16a34a);
}

.novel-binder__unit-main {
  flex: 1;
  min-width: 0;
  display: flex;
  flex-direction: column;
  gap: 1px;
}

.novel-binder__unit-line {
  display: flex;
  align-items: baseline;
  gap: 6px;
}

.novel-binder__unit-id {
  font-weight: 650;
}

.novel-binder__unit-range {
  font-size: 11px;
  opacity: 0.6;
}

.novel-binder__next {
  font-size: 9px;
  color: var(--dq-accent);
}

.novel-binder__unit-purpose {
  font-size: 11px;
  opacity: 0.7;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.novel-binder__count {
  font-weight: 400;
  opacity: 0.55;
  font-size: 11px;
}

.novel-binder__warn {
  display: inline-block;
  min-width: 14px;
  margin-left: 4px;
  padding: 0 4px;
  border-radius: 999px;
  background: color-mix(in srgb, var(--dq-danger, #dc2626) 18%, transparent);
  color: var(--dq-danger, #dc2626);
  font-weight: 700;
  text-align: center;
  opacity: 1;
}

.novel-binder__hint {
  margin: 0;
  padding: 2px 12px 4px 26px;
  font-size: 11px;
  opacity: 0.55;
  line-height: 1.35;
}

.novel-binder__filter-btn {
  margin: 0;
  padding: 0;
  border: none;
  background: transparent;
  color: var(--dq-accent);
  font: inherit;
  font-size: 11px;
  font-weight: 500;
  letter-spacing: 0;
  cursor: pointer;
  opacity: 1;
}

.novel-binder__filter-btn--on {
  font-weight: 650;
}
</style>
