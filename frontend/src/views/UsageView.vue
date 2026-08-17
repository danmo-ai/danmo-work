<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { useProjectsStore } from '@/stores/projects'
import { formatTokenCount } from '@/composables/useSessionContextUsage'
import {
  fetchProjectUsage,
  fetchUsageAgents,
  fetchUsageModels,
  fetchUsageSeries,
  fetchUsageSummary,
  type UsageBreakdown,
  type UsagePeriod,
  type UsageRollup,
  type UsageSeriesPoint,
  type UsageSummary,
} from '@/api/usage'

const { t } = useI18n()
const projects = useProjectsStore()

const period = ref<UsagePeriod>('day')
const projectId = ref('')
const modelFilter = ref('')
const loading = ref(false)
const series = ref<UsageSeriesPoint[]>([])
const models = ref<UsageRollup[]>([])
const agents = ref<UsageRollup[]>([])
const projectBreakdown = ref<UsageBreakdown | null>(null)
const summary = ref<UsageSummary>({
  promptTokens: 0,
  completionTokens: 0,
  totalTokens: 0,
  callCount: 0,
  turnCount: 0,
  avgTurnTokens: 0,
})

function barTotal(pt: UsageSeriesPoint): number {
  const n = pt.totalTokens > 0 ? pt.totalTokens : pt.promptTokens + pt.completionTokens
  return Math.max(n, 0)
}

const maxBar = computed(() => Math.max(1, ...series.value.map((p) => barTotal(p))))

function barTip(pt: UsageSeriesPoint): string {
  const cache = pt.cacheReadTokens
    ? ` · ${t('usage.legendCache')} ${formatTokenCount(pt.cacheReadTokens)}`
    : ''
  return `${t('usage.legendIn')} ${formatTokenCount(pt.promptTokens)} · ${t('usage.legendOut')} ${formatTokenCount(pt.completionTokens)}${cache} · Σ ${formatTokenCount(barTotal(pt))}`
}

const totals = computed(() => summary.value)

const sessions = computed(() => projectBreakdown.value?.sessions ?? [])

async function reload() {
  loading.value = true
  try {
    const pid = projectId.value || undefined
    const model = modelFilter.value || undefined
    const [pts, mods, ags, sum] = await Promise.all([
      fetchUsageSeries({
        period: period.value,
        projectId: pid,
        model,
        grain: model ? 'model' : 'session',
      }),
      fetchUsageModels(pid),
      fetchUsageAgents(pid),
      fetchUsageSummary({ projectId: pid, model }),
    ])
    series.value = pts
    models.value = mods
    agents.value = ags
    summary.value = sum
    if (pid) {
      projectBreakdown.value = await fetchProjectUsage(pid)
    } else {
      projectBreakdown.value = null
    }
  } catch {
    series.value = []
    models.value = []
    agents.value = []
    projectBreakdown.value = null
    summary.value = {
      promptTokens: 0,
      completionTokens: 0,
      totalTokens: 0,
      callCount: 0,
      turnCount: 0,
      avgTurnTokens: 0,
    }
  } finally {
    loading.value = false
  }
}

function formatPeriodLabel(iso: string): string {
  const d = new Date(iso)
  if (Number.isNaN(d.getTime())) return iso
  if (period.value === 'month') {
    return `${d.getUTCFullYear()}-${String(d.getUTCMonth() + 1).padStart(2, '0')}`
  }
  return d.toISOString().slice(0, 10)
}

onMounted(async () => {
  if (!projects.projects.length) await projects.loadProjects()
  await reload()
})

watch([period, projectId, modelFilter], () => {
  void reload()
})
</script>

<template>
  <div class="usage-view float-island">
    <header class="usage-view__bar">
      <div class="usage-view__titles">
        <h1 class="usage-view__title">{{ t('usage.title') }}</h1>
        <p class="usage-view__desc">{{ t('usage.subtitle') }}</p>
      </div>
      <div class="usage-toolbar">
        <label class="usage-field">
          <span class="usage-field__label">{{ t('usage.period') }}</span>
          <DqSelect v-model="period" size="sm">
            <DqOption value="day" :label="t('usage.periodDay')" />
            <DqOption value="week" :label="t('usage.periodWeek')" />
            <DqOption value="month" :label="t('usage.periodMonth')" />
          </DqSelect>
        </label>
        <label class="usage-field">
          <span class="usage-field__label">{{ t('usage.project') }}</span>
          <DqSelect v-model="projectId" size="sm" :placeholder="t('usage.allProjects')" clearable>
            <DqOption value="" :label="t('usage.allProjects')" />
            <DqOption
              v-for="p in projects.sortedProjects"
              :key="p.id"
              :value="p.id"
              :label="p.name"
            />
          </DqSelect>
        </label>
        <label class="usage-field">
          <span class="usage-field__label">{{ t('usage.model') }}</span>
          <DqSelect v-model="modelFilter" size="sm" :placeholder="t('usage.allModels')" clearable>
            <DqOption value="" :label="t('usage.allModels')" />
            <DqOption
              v-for="m in models"
              :key="m.refId"
              :value="m.model || m.refId"
              :label="m.model || m.refId"
            />
          </DqSelect>
        </label>
      </div>
    </header>

    <div class="usage-view__body">
      <div class="usage-cards">
        <div class="usage-card">
          <div class="usage-card__label">{{ t('usage.totalTokens') }}</div>
          <div class="usage-card__value">{{ formatTokenCount(totals.totalTokens) }}</div>
        </div>
        <div class="usage-card">
          <div class="usage-card__label">{{ t('usage.promptTokens') }}</div>
          <div class="usage-card__value">{{ formatTokenCount(totals.promptTokens) }}</div>
        </div>
        <div class="usage-card">
          <div class="usage-card__label">{{ t('usage.completionTokens') }}</div>
          <div class="usage-card__value">{{ formatTokenCount(totals.completionTokens) }}</div>
        </div>
        <div class="usage-card">
          <div class="usage-card__label">{{ t('usage.cacheReadTokens') }}</div>
          <div class="usage-card__value">{{ formatTokenCount(totals.cacheReadTokens || 0) }}</div>
        </div>
        <div class="usage-card">
          <div class="usage-card__label">{{ t('usage.cacheCreationTokens') }}</div>
          <div class="usage-card__value">{{ formatTokenCount(totals.cacheCreationTokens || 0) }}</div>
        </div>
        <div class="usage-card">
          <div class="usage-card__label">{{ t('usage.callCount') }}</div>
          <div class="usage-card__value">{{ totals.callCount }}</div>
        </div>
        <div class="usage-card">
          <div class="usage-card__label">{{ t('usage.turnCount') }}</div>
          <div class="usage-card__value">{{ totals.turnCount }}</div>
        </div>
        <div class="usage-card">
          <div class="usage-card__label">{{ t('usage.avgTurnTokens') }}</div>
          <div class="usage-card__value">{{ formatTokenCount(totals.avgTurnTokens || 0) }}</div>
        </div>
      </div>

      <section class="usage-section">
        <div class="usage-section__head">
          <div>
            <h2>{{ t('usage.chartTitle') }}</h2>
            <p class="usage-hint">{{ t('usage.chartHint') }}</p>
          </div>
          <div class="usage-legend" aria-hidden="true">
            <span class="usage-legend__item"><i class="usage-legend__swatch is-in" />{{ t('usage.legendIn') }}</span>
            <span class="usage-legend__item"><i class="usage-legend__swatch is-out" />{{ t('usage.legendOut') }}</span>
          </div>
        </div>
        <div v-if="loading" class="usage-empty">{{ t('usage.loading') }}</div>
        <div v-else-if="!series.length" class="usage-empty">{{ t('usage.empty') }}</div>
        <div v-else class="usage-chart" role="img" :aria-label="t('usage.chartTitle')">
          <div v-for="pt in series" :key="pt.periodStart" class="usage-bar">
            <div
              class="usage-bar__stack"
              :style="{ height: `${Math.max(2, Math.round((barTotal(pt) / maxBar) * 100))}%` }"
              :title="barTip(pt)"
            >
              <div
                class="usage-bar__seg is-out"
                :style="{ flexGrow: Math.max(pt.completionTokens, 0) }"
              />
              <div
                class="usage-bar__seg is-in"
                :style="{ flexGrow: Math.max(pt.promptTokens, 0) }"
              />
            </div>
            <div class="usage-bar__label">{{ formatPeriodLabel(pt.periodStart) }}</div>
            <div class="usage-bar__value">{{ formatTokenCount(barTotal(pt)) }}</div>
          </div>
        </div>
      </section>

      <div class="usage-columns">
        <section class="usage-section">
          <h2>{{ t('usage.byModel') }}</h2>
          <div v-if="!models.length" class="usage-empty">{{ t('usage.empty') }}</div>
          <ul v-else class="usage-list">
            <li
              v-for="m in models"
              :key="m.refId"
              class="usage-list__item"
              :class="{ 'is-active': modelFilter === m.model }"
              @click="modelFilter = modelFilter === m.model ? '' : (m.model || '')"
            >
              <span class="usage-list__name">{{ m.model || m.refId }}</span>
              <span class="usage-list__meta">
                {{ formatTokenCount(m.totalTokens) }} · {{ m.callCount }} {{ t('usage.calls') }}
              </span>
            </li>
          </ul>
        </section>

        <section class="usage-section">
          <h2>{{ t('usage.byAgent') }}</h2>
          <div v-if="!agents.length" class="usage-empty">{{ t('usage.empty') }}</div>
          <ul v-else class="usage-list">
            <li v-for="a in agents" :key="a.refId" class="usage-list__item">
              <span class="usage-list__name">{{ a.agentId || a.refId }}</span>
              <span class="usage-list__meta">
                {{ formatTokenCount(a.totalTokens) }} · {{ a.callCount }} {{ t('usage.calls') }}
              </span>
            </li>
          </ul>
        </section>
      </div>

      <section v-if="projectId" class="usage-section">
        <h2>{{ t('usage.bySession') }}</h2>
        <div v-if="!sessions.length" class="usage-empty">{{ t('usage.empty') }}</div>
        <ul v-else class="usage-list">
          <li v-for="s in sessions" :key="s.refId" class="usage-list__item">
            <span class="usage-list__name">{{ s.sessionId || s.refId }}</span>
            <span class="usage-list__meta">
              {{ formatTokenCount(s.totalTokens) }}
              <template v-if="s.updatedAt"> · {{ new Date(s.updatedAt).toLocaleString() }}</template>
            </span>
          </li>
        </ul>
      </section>
    </div>
  </div>
</template>

<style scoped>
.usage-view {
  display: flex;
  flex-direction: column;
  height: 100%;
  min-height: 0;
  overflow: hidden;
}
.usage-view__bar {
  flex: 0 0 auto;
  display: flex;
  flex-wrap: wrap;
  justify-content: space-between;
  gap: 12px 20px;
  align-items: flex-end;
  padding: 16px 20px 12px;
  border-bottom: 1px solid color-mix(in srgb, var(--dq-label-primary) 10%, transparent);
}
.usage-view__title {
  margin: 0;
  font-size: var(--dq-font-size-title, 1.1rem);
  font-weight: 600;
}
.usage-view__desc {
  margin: 4px 0 0;
  color: var(--dq-label-secondary);
  font-size: var(--dq-font-size-caption);
}
.usage-toolbar {
  display: flex;
  flex-wrap: wrap;
  gap: 12px;
  align-items: flex-end;
}
.usage-field {
  display: flex;
  flex-direction: column;
  gap: 4px;
  min-width: 160px;
}
.usage-field__label {
  font-size: var(--dq-font-size-caption);
  color: var(--dq-label-secondary);
}
.usage-view__body {
  flex: 1 1 auto;
  min-height: 0;
  overflow: auto;
  display: flex;
  flex-direction: column;
  gap: 20px;
  padding: 16px 20px 32px;
}
.usage-cards {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(140px, 1fr));
  gap: 12px;
}
.usage-card {
  padding: 12px 14px;
  border-radius: 10px;
  background: color-mix(in srgb, var(--dq-label-primary) 5%, transparent);
}
.usage-card__label {
  font-size: var(--dq-font-size-caption);
  color: var(--dq-label-tertiary);
}
.usage-card__value {
  margin-top: 6px;
  font-family: var(--dq-font-mono, ui-monospace, monospace);
  font-size: 1.25rem;
  font-variant-numeric: tabular-nums;
}
.usage-section h2 {
  margin: 0 0 6px;
  font-size: var(--dq-font-size-body);
  font-weight: 600;
}
.usage-section__head {
  display: flex;
  flex-wrap: wrap;
  justify-content: space-between;
  gap: 8px 16px;
  align-items: flex-start;
  margin-bottom: 8px;
}
.usage-section__head .usage-hint {
  margin-bottom: 0;
}
.usage-hint {
  margin: 0 0 12px;
  font-size: var(--dq-font-size-caption);
  color: var(--dq-label-tertiary);
}
.usage-legend {
  display: inline-flex;
  gap: 12px;
  align-items: center;
  font-size: var(--dq-font-size-caption);
  color: var(--dq-label-secondary);
}
.usage-legend__item {
  display: inline-flex;
  align-items: center;
  gap: 6px;
}
.usage-legend__swatch {
  width: 10px;
  height: 10px;
  border-radius: 2px;
  display: inline-block;
}
.usage-legend__swatch.is-in {
  background: var(--dq-accent);
}
.usage-legend__swatch.is-out {
  background: color-mix(in srgb, var(--dq-accent) 45%, var(--dq-label-tertiary));
}
.usage-empty {
  color: var(--dq-label-tertiary);
  font-size: var(--dq-font-size-caption);
  padding: 12px 0;
}
.usage-chart {
  display: flex;
  align-items: flex-end;
  gap: 8px;
  min-height: 160px;
  padding: 8px 4px 0;
  overflow-x: auto;
}
.usage-bar {
  flex: 0 0 48px;
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: flex-end;
  height: 160px;
}
.usage-bar__stack {
  width: 28px;
  min-height: 2px;
  border-radius: 4px 4px 0 0;
  overflow: hidden;
  display: flex;
  flex-direction: column;
  justify-content: flex-end;
}
.usage-bar__seg {
  width: 100%;
  min-height: 0;
  flex-basis: 0;
}
.usage-bar__seg.is-in {
  background: var(--dq-accent);
}
.usage-bar__seg.is-out {
  background: color-mix(in srgb, var(--dq-accent) 45%, var(--dq-label-tertiary));
}
.usage-bar__label {
  margin-top: 6px;
  font-size: 10px;
  color: var(--dq-label-tertiary);
  white-space: nowrap;
}
.usage-bar__value {
  font-size: 10px;
  font-family: var(--dq-font-mono, ui-monospace, monospace);
  color: var(--dq-label-secondary);
}
.usage-columns {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 16px;
}
@media (max-width: 800px) {
  .usage-columns {
    grid-template-columns: 1fr;
  }
}
.usage-list {
  list-style: none;
  margin: 0;
  padding: 0;
  display: flex;
  flex-direction: column;
  gap: 4px;
}
.usage-list__item {
  display: flex;
  justify-content: space-between;
  gap: 12px;
  padding: 8px 10px;
  border-radius: 8px;
  cursor: pointer;
}
.usage-list__item:hover,
.usage-list__item.is-active {
  background: color-mix(in srgb, var(--dq-label-primary) 6%, transparent);
}
.usage-list__name {
  font-size: var(--dq-font-size-body);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.usage-list__meta {
  flex: 0 0 auto;
  font-size: var(--dq-font-size-caption);
  font-family: var(--dq-font-mono, ui-monospace, monospace);
  color: var(--dq-label-secondary);
}
</style>
