<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import type { NovelBookPipeline, NovelStageAction, NovelUnitPhase } from '@/types/novel-workbench'

export type DeskAction = {
  action: NovelStageAction
  unitId?: string
  volume?: number
  batchUnits?: string[]
  stem?: string
  label: string
  allowed: boolean
  blockers: string[]
  recommended?: boolean
}

export type InjectionPreview = {
  genre: string
  subgenre: string
  unitId: string
  onStage: string[]
  pov: string
}

const props = defineProps<{
  pipeline: NovelBookPipeline | null
  /** Book/volume-level recommended CTA when no unit desk is shown. */
  primary: DeskAction | null
  jumpNote: string
  injection: InjectionPreview | null
  /** Always-visible unit production keys (six). */
  unitActions: DeskAction[]
  /** Cast fix etc. — secondary, not the only production entry. */
  extraActions: DeskAction[]
  unitPhase: NovelUnitPhase | null
  proseChars: number
  wordTarget: string
  castIssues: string[]
}>()

const emit = defineEmits<{
  action: [desk: DeskAction]
}>()

const { t } = useI18n()

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

const genreLine = computed(() => {
  const g = props.injection?.genre || t('novelWorkbench.injectionNone')
  const sub = props.injection?.subgenre
  return sub ? `${g} · ${sub}` : g
})

const unitLine = computed(() => props.injection?.unitId || t('novelWorkbench.injectionNone'))

const castLine = computed(() => {
  const stems = props.injection?.onStage ?? []
  if (!stems.length) return t('novelWorkbench.injectionNone')
  const pov = props.injection?.pov
  const list = stems.join('、')
  return pov ? `${list}（POV ${pov}）` : list
})

const wordLine = computed(() => {
  if (!props.unitPhase) return ''
  if (!props.wordTarget && !props.proseChars) return phaseLabel(props.unitPhase)
  return `${phaseLabel(props.unitPhase)} · ${t('novelWorkbench.unitWords', {
    actual: props.proseChars,
    target: props.wordTarget || '—',
  })}`
})

const showUnitDesk = computed(() => props.unitActions.length > 0)
</script>

<template>
  <aside class="novel-insp">
    <div class="novel-insp__body">
      <div v-if="showUnitDesk" class="novel-insp__block">
        <div class="novel-insp__label">{{ t('novelWorkbench.unitDeskTitle') }}</div>
        <p v-if="wordLine" class="novel-insp__hint">{{ wordLine }}</p>
        <div class="novel-insp__desk">
          <button
            v-for="(a, i) in unitActions"
            :key="i"
            type="button"
            class="novel-wb-btn novel-insp__desk-btn"
            :class="a.recommended ? 'novel-wb-btn--cta' : 'novel-wb-btn--ghost'"
            :disabled="!a.allowed"
            :title="a.blockers.length ? a.blockers.join(' · ') : a.recommended ? t('novelWorkbench.recommendedHint') : ''"
            @click="emit('action', a)"
          >
            <span v-if="a.recommended" class="novel-insp__rec">{{ t('novelWorkbench.recommendedBadge') }}</span>
            {{ a.label }}
          </button>
        </div>
        <p class="novel-insp__inject">{{ t('novelWorkbench.unitDeskHint') }}</p>
      </div>

      <div v-else class="novel-insp__block">
        <div class="novel-insp__label">{{ t('novelWorkbench.primaryCta') }}</div>
        <button
          v-if="primary"
          type="button"
          class="novel-wb-btn novel-wb-btn--cta novel-insp__cta"
          :disabled="!primary.allowed"
          :title="primary.blockers.join(' · ')"
          @click="emit('action', primary)"
        >
          {{ primary.label }}
        </button>
        <p v-if="jumpNote" class="novel-insp__hint">{{ jumpNote }}</p>
        <p class="novel-insp__inject">{{ t('novelWorkbench.injectHint', { action: primary?.label || '' }) }}</p>
      </div>

      <div
        v-if="(showUnitDesk ? unitActions : primary ? [primary] : []).some((a) => a.blockers.length && !a.allowed)"
        class="novel-insp__blockers"
      >
        <div class="novel-insp__label">{{ t('novelWorkbench.blockersTitle') }}</div>
        <ul>
          <template v-for="(a, i) in showUnitDesk ? unitActions : primary ? [primary] : []" :key="'b' + i">
            <li v-for="(b, j) in a.allowed ? [] : a.blockers" :key="j">
              {{ a.label }}：{{ b }}
            </li>
          </template>
        </ul>
      </div>

      <div v-if="castIssues.length" class="novel-insp__blockers">
        <div class="novel-insp__label">{{ t('novelWorkbench.castLintFail') }}</div>
        <ul>
          <li v-for="(b, i) in castIssues" :key="i">{{ b }}</li>
        </ul>
      </div>

      <div class="novel-insp__block">
        <div class="novel-insp__label">{{ t('novelWorkbench.injectionTitle') }}</div>
        <dl class="novel-insp__inject-rows">
          <dt>{{ t('novelWorkbench.injectionGenre') }}</dt>
          <dd>{{ genreLine }}</dd>
          <dt>{{ t('novelWorkbench.injectionUnit') }}</dt>
          <dd>{{ unitLine }}</dd>
          <dt>{{ t('novelWorkbench.injectionCast') }}</dt>
          <dd>{{ castLine }}</dd>
        </dl>
      </div>

      <div v-if="extraActions.length" class="novel-insp__block">
        <div class="novel-insp__label">{{ t('novelWorkbench.extraActions') }}</div>
        <div class="novel-insp__desk">
          <button
            v-for="(a, i) in extraActions"
            :key="'x' + i"
            type="button"
            class="novel-wb-btn novel-wb-btn--ghost novel-insp__desk-btn"
            :disabled="!a.allowed"
            :title="a.blockers.join(' · ')"
            @click="emit('action', a)"
          >
            {{ a.label }}
          </button>
        </div>
      </div>

      <p class="novel-insp__model" :title="t('novelWorkbench.modelTip')">ⓘ {{ t('novelWorkbench.modelTipShort') }}</p>
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

.novel-insp__body {
  flex: 1;
  min-height: 0;
  overflow: auto;
  padding: 12px 12px 16px;
  display: flex;
  flex-direction: column;
  gap: 12px;
}

.novel-insp__block {
  display: flex;
  flex-direction: column;
  gap: 6px;
}

.novel-insp__label {
  font-size: 11px;
  font-weight: 650;
  opacity: 0.6;
  letter-spacing: 0.02em;
}

.novel-insp__hint,
.novel-insp__inject {
  margin: 0;
  font-size: var(--dq-font-size-caption);
  line-height: 1.4;
  opacity: 0.75;
}

.novel-insp__cta {
  width: 100%;
}

.novel-insp__desk {
  display: flex;
  flex-direction: column;
  gap: 6px;
}

.novel-insp__desk-btn {
  width: 100%;
  position: relative;
  text-align: left;
}

.novel-insp__rec {
  display: inline-block;
  margin-right: 4px;
  font-size: 10px;
  font-weight: 700;
  opacity: 0.85;
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

.novel-insp__inject-rows {
  margin: 0;
  display: grid;
  grid-template-columns: auto 1fr;
  gap: 4px 8px;
  font-size: var(--dq-font-size-caption);
}

.novel-insp__inject-rows dt {
  opacity: 0.55;
  font-weight: 650;
}

.novel-insp__inject-rows dd {
  margin: 0;
  line-height: 1.4;
  word-break: break-word;
}

.novel-insp__model {
  margin: 0;
  font-size: 11px;
  opacity: 0.5;
  line-height: 1.35;
  cursor: help;
}
</style>
