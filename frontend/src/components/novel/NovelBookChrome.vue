<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import type { GateStatus, NovelBookPipeline, NovelPipelineStepId } from '@/types/novel-workbench'

const props = defineProps<{
  title: string
  pipeline: NovelBookPipeline | null
  loading: boolean
  focusMode: boolean
}>()

const emit = defineEmits<{
  back: []
  refresh: []
  'toggle-focus': []
}>()

const { t } = useI18n()

function stepperLabel(id: NovelPipelineStepId | string): string {
  const map: Record<string, string> = {
    init: 'stepperInit',
    setup: 'stepperSetup',
    outline: 'stepperBookOutline',
    volume: 'stepperVolume',
    contract: 'stepperContract',
    write: 'stepperWriting',
    review: 'stepperReview',
    commit: 'stepperCommit',
    idle: 'stepperWriting',
  }
  return t(`novelWorkbench.${map[id] ?? 'stepperWriting'}`)
}

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

const stepLabel = computed(() =>
  props.pipeline ? stepperLabel(props.pipeline.step) : '',
)

const progressPct = computed(() => props.pipeline?.progress.percent ?? 0)
</script>

<template>
  <header class="novel-chrome">
    <button type="button" class="novel-wb-link" @click="emit('back')">
      ← {{ t('novelWorkbench.backShelf') }}
    </button>
    <div class="novel-chrome__main">
      <span class="novel-chrome__title">{{ title }}</span>
      <div v-if="pipeline" class="novel-chrome__progress" :title="stepLabel">
        <span class="novel-chrome__step">{{ stepLabel }}</span>
        <div class="novel-chrome__bar" role="progressbar" :aria-valuenow="progressPct" aria-valuemin="0" aria-valuemax="100">
          <div class="novel-chrome__bar-fill" :style="{ width: `${progressPct}%` }" />
        </div>
        <span class="novel-chrome__pct">
          {{
            t('novelWorkbench.progressLabel', {
              committed: pipeline.progress.committed,
              total: pipeline.progress.totalWithContract || pipeline.progress.committed || 0,
            })
          }}
        </span>
      </div>
    </div>
    <div v-if="pipeline" class="novel-chrome__gates" :aria-label="t('novelWorkbench.gatePanel')">
      <span
        class="novel-chrome__gate"
        :class="'novel-chrome__gate--' + pipeline.gates.knowledge"
        :title="`${t('novelWorkbench.gateKnowledge')} · ${gateLabel(pipeline.gates.knowledge)}`"
      >{{ t('novelWorkbench.gateKnowledge') }}</span>
      <span
        class="novel-chrome__gate"
        :class="'novel-chrome__gate--' + pipeline.gates.asset"
        :title="`${t('novelWorkbench.gateAsset')} · ${gateLabel(pipeline.gates.asset)}`"
      >{{ t('novelWorkbench.gateAsset') }}</span>
      <span
        class="novel-chrome__gate"
        :class="'novel-chrome__gate--' + pipeline.gates.qc"
        :title="`${t('novelWorkbench.gateQc')} · ${gateLabel(pipeline.gates.qc)}`"
      >{{ t('novelWorkbench.gateQc') }}</span>
    </div>
    <button
      type="button"
      class="novel-wb-link"
      :aria-pressed="focusMode"
      :title="t('novelWorkbench.focusMode')"
      @click="emit('toggle-focus')"
    >
      {{ focusMode ? t('novelWorkbench.exitFocus') : t('novelWorkbench.focusMode') }}
    </button>
    <button type="button" class="novel-wb-link" :disabled="loading" @click="emit('refresh')">
      {{ t('novelWorkbench.refresh') }}
    </button>
  </header>
</template>

<style scoped>
.novel-chrome {
  flex-shrink: 0;
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 8px 10px;
  border-bottom: 1px solid color-mix(in srgb, var(--dq-border-subtle, #000) 50%, transparent);
  min-width: 0;
}

.novel-chrome__main {
  flex: 1;
  min-width: 0;
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.novel-chrome__title {
  font-size: var(--dq-font-size-body);
  font-weight: 650;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.novel-chrome__progress {
  display: flex;
  align-items: center;
  gap: 8px;
  min-width: 0;
}

.novel-chrome__step {
  flex-shrink: 0;
  padding: 1px 7px;
  border-radius: 4px;
  background: color-mix(in srgb, var(--dq-accent) 14%, transparent);
  color: var(--dq-accent);
  font-size: 11px;
  font-weight: 650;
}

.novel-chrome__bar {
  flex: 1;
  min-width: 40px;
  max-width: 120px;
  height: 4px;
  border-radius: 999px;
  background: color-mix(in srgb, var(--dq-border-subtle, #000) 40%, transparent);
  overflow: hidden;
}

.novel-chrome__bar-fill {
  height: 100%;
  border-radius: inherit;
  background: var(--dq-accent);
  transition: width 0.2s ease;
}

.novel-chrome__pct {
  flex-shrink: 0;
  font-size: 11px;
  opacity: 0.65;
  white-space: nowrap;
}

.novel-chrome__gates {
  display: flex;
  flex-shrink: 0;
  gap: 4px;
}

.novel-chrome__gate {
  padding: 2px 6px;
  border-radius: 4px;
  font-size: 10px;
  font-weight: 650;
  letter-spacing: 0.02em;
  background: color-mix(in srgb, var(--dq-border-subtle, #000) 25%, transparent);
  opacity: 0.85;
}

.novel-chrome__gate--pass {
  background: color-mix(in srgb, var(--dq-success, #16a34a) 18%, transparent);
  color: var(--dq-success, #16a34a);
}

.novel-chrome__gate--fail {
  background: color-mix(in srgb, var(--dq-danger, #dc2626) 18%, transparent);
  color: var(--dq-danger, #dc2626);
}

.novel-chrome__gate--skipped,
.novel-chrome__gate--unknown {
  opacity: 0.55;
}
</style>
