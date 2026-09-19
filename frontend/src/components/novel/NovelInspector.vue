<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import {
  parseUnitOutlineYaml,
  type GateStatus,
  type NovelBookPipeline,
  type NovelChapterPhase,
  type NovelFileNode,
  type NovelPipelinePhase,
  type NovelStageAction,
  type NovelUnitOutlineFields,
} from '@/types/novel-workbench'

export type DeskPrimary = {
  action: NovelStageAction
  unitId?: string
  label: string
  allowed: boolean
}

const props = defineProps<{
  pipeline: NovelBookPipeline | null
  treeKind: 'book' | 'volume' | 'setup' | 'unit' | 'dossier'
  readingIsOutline: boolean
  readingIsProse: boolean
  unitId: string | null
  outlineRaw: string
  unitPhase: NovelChapterPhase | null
  highlightChapter: number | null
  proseChars: number
  chapterChars: Record<number, number>
  castDocs: NovelFileNode[]
  continuityFiles: NovelFileNode[]
  deskPrimary: DeskPrimary | null
  unitPrimary: DeskPrimary | null
  moreActions: { action: NovelStageAction; label: string; allowed: boolean; unitId?: string }[]
  blockers: string[]
  setupShowsGoldfinger: boolean
  hasBookOutline: boolean
  selectedVolumeNum: number
  nextVolume: number
}>()

const emit = defineEmits<{
  action: [action: NovelStageAction, unitId?: string, opts?: { volume?: number }]
  'open-cast': [node: NovelFileNode]
  'open-ledger': [node: NovelFileNode]
}>()

const { t } = useI18n()
const tab = ref<'overview' | 'contract' | 'canon' | 'actions'>('actions')

watch(
  () => props.treeKind,
  (kind) => {
    if (kind === 'unit') tab.value = props.readingIsOutline ? 'contract' : 'actions'
    else if (kind === 'setup') tab.value = 'canon'
    else tab.value = 'overview'
  },
)

watch(
  () => props.highlightChapter,
  (n) => {
    if (n != null && props.treeKind === 'unit') tab.value = 'contract'
  },
)

function gateLabel(status: GateStatus | string): string {
  switch (status) {
    case 'pass':
      return t('novelWorkbench.gatePass')
    case 'fail':
      return t('novelWorkbench.gateFail')
    case 'skipped':
      return t('novelWorkbench.gateSkipped')
    default:
      return t('novelWorkbench.gateUnknown')
  }
}

function stageHint(phase: NovelPipelinePhase | undefined): string {
  switch (phase) {
    case 'init':
      return t('novelWorkbench.stageHintInit')
    case 'setup':
      return t('novelWorkbench.stageHintSetup')
    case 'outline':
      return t('novelWorkbench.stageHintOutline')
    case 'writing':
      return t('novelWorkbench.stageHintWriting')
    case 'review':
      return t('novelWorkbench.stageHintReview')
    default:
      return t('novelWorkbench.stageHintIdle')
  }
}

function stepLabel(id: string | undefined): string {
  const map: Record<string, string> = {
    init: 'stepperInit',
    setup: 'stepperSetup',
    outline: 'stepperBookOutline',
    volume: 'stepperVolume',
    contract: 'stepperContract',
    write: 'stepperWriting',
    review: 'stepperReview',
    commit: 'stepperCommit',
  }
  return t(`novelWorkbench.${map[id ?? ''] ?? 'stepperWriting'}`)
}

function phaseLabel(phase: NovelChapterPhase): string {
  const map: Record<NovelChapterPhase, string> = {
    empty: 'phaseEmpty',
    contract_draft: 'phaseContractDraft',
    contract_ready: 'phaseContractReady',
    drafted: 'phaseDrafted',
    review_fail: 'phaseReviewFail',
    review_pass: 'phaseReviewPass',
    committed: 'phaseCommitted',
  }
  return t(`novelWorkbench.${map[phase]}`)
}

const unit = computed((): NovelUnitOutlineFields => parseUnitOutlineYaml(props.outlineRaw || ''))

const unitRows = computed(() => {
  const u = unit.value
  const rows: { key: string; label: string; value: string }[] = []
  const push = (key: string, label: string, value: string) => {
    if (value.trim()) rows.push({ key, label, value })
  }
  push('function', t('novelWorkbench.unitFieldFunction'), u.functionText)
  push('desire', t('novelWorkbench.unitFieldDesire'), u.desire)
  push('obstacle', t('novelWorkbench.unitFieldObstacle'), u.obstacle)
  push('pleasure', t('novelWorkbench.unitFieldPleasure'), u.pleasure)
  const target = u.wordTarget.trim()
  const words = target
    ? t('novelWorkbench.unitWords', { actual: props.proseChars, target })
    : String(props.proseChars || '')
  push('words', t('novelWorkbench.unitFieldWords'), words)
  return rows
})

const primary = computed(() => {
  if (props.treeKind === 'unit') return props.unitPrimary
  return props.deskPrimary
})

const ledgerFile = computed(
  () => props.continuityFiles.find((f) => /ledger/i.test(f.name)) ?? props.continuityFiles[0] ?? null,
)

function runPrimary() {
  const p = primary.value
  if (!p?.allowed) return
  if (p.action === 'volume') {
    emit('action', p.action, p.unitId, { volume: props.nextVolume })
    return
  }
  emit('action', p.action, p.unitId)
}
</script>

<template>
  <aside class="novel-insp">
    <div class="novel-insp__tabs" role="tablist">
      <button
        type="button"
        role="tab"
        class="novel-insp__tab"
        :class="{ 'novel-insp__tab--on': tab === 'overview' }"
        @click="tab = 'overview'"
      >
        {{ t('novelWorkbench.inspOverview') }}
      </button>
      <button
        type="button"
        role="tab"
        class="novel-insp__tab"
        :class="{ 'novel-insp__tab--on': tab === 'contract' }"
        :disabled="treeKind !== 'unit'"
        @click="tab = 'contract'"
      >
        {{ t('novelWorkbench.inspContract') }}
      </button>
      <button
        type="button"
        role="tab"
        class="novel-insp__tab"
        :class="{ 'novel-insp__tab--on': tab === 'canon' }"
        @click="tab = 'canon'"
      >
        {{ t('novelWorkbench.inspCanon') }}
      </button>
      <button
        type="button"
        role="tab"
        class="novel-insp__tab"
        :class="{ 'novel-insp__tab--on': tab === 'actions' }"
        @click="tab = 'actions'"
      >
        {{ t('novelWorkbench.inspActions') }}
      </button>
    </div>

    <div class="novel-insp__body">
      <template v-if="tab === 'overview'">
        <div v-if="pipeline" class="novel-insp__block">
          <div class="novel-insp__label">{{ t('novelWorkbench.currentStep') }}</div>
          <div class="novel-insp__value">{{ stepLabel(pipeline.step) }}</div>
          <p class="novel-insp__hint">{{ stageHint(pipeline.phase) }}</p>
          <div class="novel-insp__progress">
            {{
              t('novelWorkbench.progressLabel', {
                committed: pipeline.progress.committed,
                total: pipeline.progress.totalWithContract || pipeline.progress.committed,
              })
            }}
            · {{ pipeline.progress.percent }}%
          </div>
        </div>
        <div v-if="pipeline" class="novel-insp__block">
          <div class="novel-insp__label">{{ t('novelWorkbench.gatePanel') }}</div>
          <ul class="novel-insp__gates">
            <li :class="'is-' + pipeline.gates.knowledge">
              {{ t('novelWorkbench.gateKnowledge') }} · {{ gateLabel(pipeline.gates.knowledge) }}
            </li>
            <li :class="'is-' + pipeline.gates.asset">
              {{ t('novelWorkbench.gateAsset') }} · {{ gateLabel(pipeline.gates.asset) }}
            </li>
            <li :class="'is-' + pipeline.gates.qc">
              {{ t('novelWorkbench.gateQc') }} · {{ gateLabel(pipeline.gates.qc) }}
            </li>
          </ul>
        </div>
        <div v-if="unitPhase" class="novel-insp__block">
          <div class="novel-insp__label">{{ t('novelWorkbench.chapterPhase') }}</div>
          <div class="novel-insp__value">{{ phaseLabel(unitPhase) }}</div>
        </div>
      </template>

      <template v-else-if="tab === 'contract'">
        <div v-if="!outlineRaw" class="novel-insp__empty">{{ t('novelWorkbench.noContractYet') }}</div>
        <template v-else>
          <dl v-if="unitRows.length" class="novel-insp__fields">
            <template v-for="f in unitRows" :key="f.key">
              <dt>{{ f.label }}</dt>
              <dd>{{ f.value }}</dd>
            </template>
          </dl>
          <div v-if="unit.scenes.length" class="novel-insp__block">
            <div class="novel-insp__label">{{ t('novelWorkbench.unitScenes') }}</div>
            <ul class="novel-insp__cuts">
              <li v-for="s in unit.scenes" :key="s.id">
                {{ s.id }} · {{ s.beat }} · {{ t('novelWorkbench.chapterN', { n: s.chapter }) }}
              </li>
            </ul>
          </div>
          <div v-if="unit.chapters.length" class="novel-insp__block">
            <div class="novel-insp__label">{{ t('novelWorkbench.unitCuts') }}</div>
            <ul class="novel-insp__cuts">
              <li
                v-for="c in unit.chapters"
                :key="c.chapter"
                :class="{ 'novel-insp__cut--on': highlightChapter === c.chapter }"
              >
                <span>{{ t('novelWorkbench.chapterN', { n: c.chapter }) }}</span>
                <span v-if="c.cutHook"> · {{ c.cutHook }}</span>
                <span v-if="c.wordShare"> · {{ chapterChars[c.chapter] || 0 }}/{{ c.wordShare }}</span>
              </li>
            </ul>
          </div>
        </template>
      </template>

      <template v-else-if="tab === 'canon'">
        <div class="novel-insp__block">
          <div class="novel-insp__label">{{ t('novelWorkbench.folderSetupCast') }}</div>
          <button
            v-for="f in castDocs"
            :key="f.name"
            type="button"
            class="novel-insp__link"
            @click="emit('open-cast', f)"
          >
            {{ f.name.replace(/\.md$/i, '') }}
          </button>
          <p v-if="!castDocs.length" class="novel-insp__empty">{{ t('novelWorkbench.noCastYet') }}</p>
        </div>
        <div v-if="ledgerFile" class="novel-insp__block">
          <div class="novel-insp__label">{{ t('novelWorkbench.continuityLedger') }}</div>
          <button type="button" class="novel-insp__link" @click="emit('open-ledger', ledgerFile)">
            {{ ledgerFile.name }}
          </button>
        </div>
      </template>

      <template v-else>
        <p class="novel-insp__inject" :title="t('novelWorkbench.modelTip')">
          {{ t('novelWorkbench.injectHint', { action: primary?.label || t('novelWorkbench.primaryCta') }) }}
        </p>

        <div v-if="blockers.length" class="novel-insp__blockers">
          <div class="novel-insp__label">{{ t('novelWorkbench.blockersTitle') }}</div>
          <ul>
            <li v-for="(b, i) in blockers" :key="i">{{ b }}</li>
          </ul>
        </div>

        <button
          v-if="primary"
          type="button"
          class="novel-wb-btn novel-wb-btn--cta"
          :disabled="!primary.allowed"
          @click="runPrimary()"
        >
          {{ t('novelWorkbench.primaryCta') }} · {{ primary.label }}
        </button>

        <!-- Context secondary actions for non-chapter selections -->
        <div v-if="treeKind === 'book'" class="novel-insp__more-stack">
          <button type="button" class="novel-wb-btn" @click="emit('action', 'outline')">
            {{ hasBookOutline ? t('novelWorkbench.actionReviseBookOutline') : t('novelWorkbench.actionOutline') }}
          </button>
          <button type="button" class="novel-wb-btn novel-wb-btn--ghost" @click="emit('action', 'volume', undefined, { volume: nextVolume })">
            {{ t('novelWorkbench.actionVolumeOutline', { n: nextVolume }) }}
          </button>
          <button type="button" class="novel-wb-btn novel-wb-btn--ghost" @click="emit('action', 'assets')">
            {{ t('novelWorkbench.actionAssets') }}
          </button>
        </div>

        <div v-else-if="treeKind === 'volume'" class="novel-insp__more-stack">
          <button type="button" class="novel-wb-btn" @click="emit('action', 'volume', undefined, { volume: selectedVolumeNum })">
            {{ t('novelWorkbench.actionReviseVolumeOutline', { n: selectedVolumeNum }) }}
          </button>
          <button type="button" class="novel-wb-btn novel-wb-btn--ghost" @click="emit('action', 'outline')">
            {{ hasBookOutline ? t('novelWorkbench.actionReviseBookOutline') : t('novelWorkbench.actionOutline') }}
          </button>
        </div>

        <div v-else-if="treeKind === 'setup'" class="novel-insp__more-stack">
          <button type="button" class="novel-wb-btn" @click="emit('action', 'assets')">
            {{ t('novelWorkbench.actionAssets') }}
          </button>
          <button
            v-if="setupShowsGoldfinger"
            type="button"
            class="novel-wb-btn novel-wb-btn--ghost"
            @click="emit('action', 'goldfinger')"
          >
            {{ t('novelWorkbench.actionGoldfinger') }}
          </button>
          <button type="button" class="novel-wb-btn novel-wb-btn--ghost" @click="emit('action', 'outline')">
            {{ hasBookOutline ? t('novelWorkbench.actionReviseBookOutline') : t('novelWorkbench.actionOutline') }}
          </button>
        </div>

        <details v-if="treeKind === 'unit' && moreActions.length" class="novel-insp__more">
          <summary>{{ t('novelWorkbench.moreActions') }}</summary>
          <div class="novel-insp__more-stack">
            <button
              v-for="(a, i) in moreActions"
              :key="i"
              type="button"
              class="novel-wb-btn novel-wb-btn--ghost"
              :disabled="!a.allowed"
              @click="emit('action', a.action, a.unitId)"
            >
              {{ a.label }}
            </button>
          </div>
        </details>

        <p class="novel-insp__model" :title="t('novelWorkbench.modelTip')">ⓘ {{ t('novelWorkbench.modelTipShort') }}</p>
      </template>
    </div>
  </aside>
</template>

<style scoped>
.novel-insp {
  flex: 0 0 260px;
  min-width: 220px;
  max-width: 320px;
  display: flex;
  flex-direction: column;
  border-left: 1px solid color-mix(in srgb, var(--dq-border-subtle, #000) 50%, transparent);
  background: color-mix(in srgb, var(--dq-glass-popover-bg, #fff) 55%, transparent);
  min-height: 0;
}

.novel-insp__tabs {
  flex-shrink: 0;
  display: flex;
  gap: 2px;
  padding: 8px 8px 0;
  border-bottom: 1px solid color-mix(in srgb, var(--dq-border-subtle, #000) 40%, transparent);
}

.novel-insp__tab {
  flex: 1;
  margin: 0;
  padding: 6px 4px;
  border: none;
  border-bottom: 2px solid transparent;
  background: transparent;
  color: inherit;
  font: inherit;
  font-size: 11px;
  font-weight: 600;
  cursor: pointer;
  opacity: 0.65;
}

.novel-insp__tab--on {
  opacity: 1;
  border-bottom-color: var(--dq-accent);
  color: var(--dq-accent);
}

.novel-insp__tab:disabled {
  opacity: 0.3;
  cursor: not-allowed;
}

.novel-insp__body {
  flex: 1;
  min-height: 0;
  overflow: auto;
  padding: 10px 12px 16px;
  display: flex;
  flex-direction: column;
  gap: 10px;
}

.novel-insp__block {
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.novel-insp__label {
  font-size: 11px;
  font-weight: 650;
  opacity: 0.6;
  letter-spacing: 0.02em;
}

.novel-insp__value {
  font-size: var(--dq-font-size-body);
  font-weight: 650;
}

.novel-insp__hint {
  margin: 0;
  font-size: var(--dq-font-size-caption);
  line-height: 1.4;
  opacity: 0.75;
}

.novel-insp__progress {
  font-size: var(--dq-font-size-caption);
  opacity: 0.7;
}

.novel-insp__gates {
  list-style: none;
  margin: 0;
  padding: 0;
  display: flex;
  flex-direction: column;
  gap: 4px;
  font-size: var(--dq-font-size-caption);
}

.novel-insp__gates li {
  padding: 4px 8px;
  border-radius: 5px;
  background: color-mix(in srgb, var(--dq-border-subtle, #000) 18%, transparent);
}

.novel-insp__gates li.is-pass {
  background: color-mix(in srgb, var(--dq-success, #16a34a) 14%, transparent);
}

.novel-insp__gates li.is-fail {
  background: color-mix(in srgb, var(--dq-danger, #dc2626) 14%, transparent);
}

.novel-insp__fields {
  margin: 0;
  display: grid;
  grid-template-columns: auto 1fr;
  gap: 6px 10px;
  font-size: var(--dq-font-size-caption);
}

.novel-insp__fields dt {
  opacity: 0.55;
  font-weight: 650;
}

.novel-insp__fields dd {
  margin: 0;
  line-height: 1.4;
}

.novel-insp__raw {
  margin: 0;
  white-space: pre-wrap;
  font-size: 11px;
  line-height: 1.4;
  opacity: 0.85;
}

.novel-insp__link {
  display: block;
  width: 100%;
  margin: 0;
  padding: 5px 0;
  border: none;
  background: transparent;
  color: var(--dq-accent);
  font: inherit;
  font-size: var(--dq-font-size-caption);
  text-align: left;
  cursor: pointer;
}

.novel-insp__empty {
  margin: 0;
  font-size: var(--dq-font-size-caption);
  opacity: 0.6;
}

.novel-insp__inject {
  margin: 0;
  font-size: var(--dq-font-size-caption);
  opacity: 0.7;
  line-height: 1.35;
}

.novel-insp__blockers {
  padding: 8px 10px;
  border-radius: 8px;
  background: color-mix(in srgb, var(--dq-danger, #dc2626) 10%, transparent);
  font-size: var(--dq-font-size-caption);
}

.novel-insp__blockers ul {
  margin: 4px 0 0;
  padding-left: 16px;
}

.novel-insp__more {
  font-size: var(--dq-font-size-caption);
}

.novel-insp__more summary {
  cursor: pointer;
  opacity: 0.7;
  font-weight: 650;
}

.novel-insp__more-stack {
  display: flex;
  flex-direction: column;
  gap: 6px;
  margin-top: 6px;
}

.novel-insp__model {
  margin: 8px 0 0;
  font-size: 11px;
  opacity: 0.5;
  line-height: 1.35;
  cursor: help;
}

.novel-insp__cuts {
  margin: 4px 0 0;
  padding-left: 16px;
  font-size: var(--dq-font-size-caption);
  line-height: 1.45;
}

.novel-insp__cut--on {
  background: color-mix(in srgb, var(--dq-accent) 14%, transparent);
  border-radius: 4px;
  font-weight: 650;
}
</style>
