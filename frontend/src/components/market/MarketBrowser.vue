<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { useRouter } from 'vue-router'
import { useMarketStore } from '@/stores/market'
import { confirm, toast } from '@/utils/feedback'
import type { InstallMarketResult, MarketListing, UninstallMarketResult } from '@/types'

const props = defineProps<{
  kind: 'skill' | 'expert' | 'connector' | 'plugin'
  selectedKey?: string | null
}>()

const emit = defineEmits<{
  installed: [id: string]
  uninstalled: [id: string]
  viewInstalled: [id: string]
}>()

const { t } = useI18n()
const router = useRouter()
const store = useMarketStore()

const lastInstall = ref<InstallMarketResult | null>(null)
const lastUninstall = ref<UninstallMarketResult | null>(null)
const showDepsLog = ref(true)

const selected = computed(() => {
  if (!props.selectedKey) return null
  return (
    store.catalog.find(
      (item) => item.kind === props.kind && `${item.sourceId}:${item.id}` === props.selectedKey,
    ) ?? null
  )
})

watch(
  () => props.selectedKey,
  () => {
    lastInstall.value = null
    lastUninstall.value = null
  },
)

const enabledSources = computed(() => store.sources.filter((s) => s.enabled))

const isClawhubItem = computed(() => {
  const item = selected.value
  if (!item) return false
  const src = store.sources.find((s) => s.id === item.sourceId)
  return src?.kind === 'clawhub' || item.sourceId === 'clawhub'
})

const isTechleadsItem = computed(() => {
  const item = selected.value
  if (!item) return false
  const src = store.sources.find((s) => s.id === item.sourceId)
  const kind = src?.kind || ''
  return kind === 'techleads' || kind === 'tlc' || kind === 'tech-leads-club' || item.sourceId === 'techleads'
})

const clawhubListingURL = computed(() => {
  const item = selected.value
  if (!item || !isClawhubItem.value) return ''
  const slug = item.path || item.id.replace(/^clawhub__/, '').replace(/^[^_]+__/, '')
  if (item.author && slug) {
    return `https://clawhub.ai/${item.author}/skills/${slug}`
  }
  if (slug) return `https://clawhub.ai/skills/${slug}`
  return 'https://clawhub.ai'
})

const externalListingURL = computed(() => {
  if (isClawhubItem.value) return clawhubListingURL.value
  if (isTechleadsItem.value) return 'https://tech-leads-club.github.io/agent-skills/'
  return ''
})

const externalListingLabel = computed(() => {
  if (isClawhubItem.value) return 'market.openOnClawhub'
  if (isTechleadsItem.value) return 'market.openOnTechleads'
  return ''
})

const hasUninstallDeps = computed(() => {
  const d = selected.value?.uninstallDeps
  return !!d && Object.keys(d).length > 0
})

const installDepsRuns = computed(() => lastInstall.value?.depsRuns?.filter((r) => r.phase === 'install' || !r.phase) ?? [])

const installedConnectorIds = computed(() => {
  const result = lastInstall.value
  const ids = new Set<string>()
  if (result) {
    for (const r of result.depsRuns ?? []) {
      if (r.connectorId) ids.add(r.connectorId)
    }
    for (const id of result.installed ?? []) {
      if (result.kind === 'expert' && id === result.id) continue
      if (result.kind === 'skill' && id === result.id) continue
      if (store.catalog.some((c) => c.kind === 'connector' && c.id === id)) {
        ids.add(id)
      }
    }
  }
  // Expert catalog deps as fallback jump targets after install.
  if (props.kind === 'expert' && selected.value?.installed) {
    for (const id of selected.value.connectorDeps ?? []) ids.add(id)
  }
  return Array.from(ids)
})

async function installItem(item: MarketListing, overwrite = false) {
  lastUninstall.value = null
  try {
    const result = await store.install(item.sourceId, item.kind, item.id, overwrite)
    lastInstall.value = result
    showDepsLog.value = true
    if (item.kind === 'expert') {
      toast.success(t('market.installSuccessExpert', { name: item.name }))
    } else if (item.kind === 'connector') {
      const msg = result?.depsScript || (result?.depsRuns?.length ?? 0) > 0
        ? t('market.installSuccessConnector', { name: item.name })
        : t('market.installSuccess', { name: item.name })
      toast.success(msg)
    } else {
      toast.success(t('market.installSuccess', { name: item.name }))
    }
    const installedId = result?.installed?.find((id) => id === item.id) || result?.installed?.[0] || item.id
    emit('installed', installedId)
  } catch (e) {
    toast.error(e instanceof Error ? e.message : t('market.installFailed'))
  }
}

async function uninstallItem(item: MarketListing) {
  try {
    await confirm(t('market.uninstallConfirm', { name: item.name }), t('market.uninstall'), { type: 'warning' })
  } catch {
    return
  }
  let runCleanup = false
  if (item.kind === 'connector' && hasUninstallDeps.value) {
    try {
      await confirm(t('market.uninstallCleanupConfirm', { name: item.name }), t('market.uninstallCleanup'), {
        type: 'warning',
      })
      runCleanup = true
    } catch {
      runCleanup = false
    }
  }
  try {
    const result = await store.uninstall(item.kind, item.id, {
      runCleanup,
      sourceId: item.sourceId,
    })
    lastUninstall.value = result
    lastInstall.value = null
    showDepsLog.value = true
    toast.success(t('market.uninstallSuccess', { name: item.name }))
    emit('uninstalled', item.id)
  } catch (e) {
    toast.error(e instanceof Error ? e.message : t('market.uninstallFailed'))
  }
}

function openConnector(id: string) {
  void router.push({ name: 'mcpServers', query: { id } })
}
</script>

<template>
  <div class="market-browser">
    <p v-if="enabledSources.length" class="market-browser__sources">
      {{ $t('market.sourcesLabel') }}:
      <span v-for="(s, i) in enabledSources" :key="s.id">
        {{ s.name }}<template v-if="i < enabledSources.length - 1"> · </template>
      </span>
    </p>

    <div v-if="store.warnings.length" class="market-browser__warnings">
      <p v-for="(w, i) in store.warnings" :key="i" class="market-browser__error">{{ w }}</p>
    </div>
    <p v-else-if="store.error && !selected" class="market-browser__error">{{ store.error }}</p>

    <DqEmpty v-if="!store.loading && !selected" :description="store.error || $t('market.emptySelection')" />

    <article v-else-if="selected" class="market-card">
      <div class="market-card__head">
        <h3 class="market-card__title">{{ selected.name }}</h3>
        <span v-if="selected.installed" class="market-card__badge">{{ $t('market.installed') }}</span>
        <span v-if="selected.compatibility" class="market-card__badge market-card__badge--warn" :title="selected.compatibility">
          {{ $t('market.compatWarn') }}
        </span>
      </div>
      <p class="market-card__desc">{{ selected.description || selected.id }}</p>
      <p v-if="selected.compatibility" class="market-card__compat">
        {{ $t('market.compatHint') }}: {{ selected.compatibility }}
      </p>
      <div class="market-card__meta">
        <code>{{ selected.id }}</code>
        <span v-if="selected.version">v{{ selected.version }}</span>
        <span>{{ selected.sourceName || selected.sourceId }}</span>
        <span v-if="selected.author">{{ selected.author }}</span>
        <a
          v-if="externalListingURL"
          class="market-card__link"
          :href="externalListingURL"
          target="_blank"
          rel="noopener noreferrer"
        >
          {{ $t(externalListingLabel) }}
        </a>
      </div>
      <div v-if="selected.skillDeps?.length" class="market-card__deps">
        {{ $t('market.skillDeps') }}:
        <code v-for="dep in selected.skillDeps" :key="'s-'+dep">{{ dep }}</code>
      </div>
      <div v-if="selected.connectorDeps?.length" class="market-card__deps">
        {{ $t('market.connectorDeps') }}:
        <button
          v-for="dep in selected.connectorDeps"
          :key="'c-'+dep"
          type="button"
          class="market-card__dep-link"
          @click="openConnector(dep)"
        >
          {{ dep }}
        </button>
      </div>
      <div v-if="selected.deps && Object.keys(selected.deps).length" class="market-card__deps">
        {{ $t('market.depsScriptLabel') }}:
        <code v-for="(path, platform) in selected.deps" :key="'d-'+platform">{{ platform }}:{{ path }}</code>
      </div>
      <div v-if="selected.uninstallDeps && Object.keys(selected.uninstallDeps).length" class="market-card__deps">
        {{ $t('market.uninstallDepsLabel') }}:
        <code v-for="(path, platform) in selected.uninstallDeps" :key="'u-'+platform">{{ platform }}:{{ path }}</code>
      </div>
      <p v-if="kind === 'connector' && selected.deps && Object.keys(selected.deps).length && !selected.installed" class="market-card__next">
        {{ $t('market.installRunsDeps') }}
      </p>
      <p v-if="kind === 'expert' && selected.installed" class="market-card__next">
        {{ $t('market.installNextStepExpert') }}
      </p>
      <p v-else-if="kind === 'connector' && selected.installed" class="market-card__next">
        {{ $t('market.installNextStepConnector') }}
      </p>
      <div class="market-card__actions">
        <template v-if="!selected.installed">
          <DqButton
            type="primary"
            :loading="store.installing === `${selected.kind}:${selected.id}`"
            @click="installItem(selected)"
          >
            {{ $t('market.install') }}
          </DqButton>
        </template>
        <template v-else>
          <DqButton
            v-if="kind === 'expert' || kind === 'connector'"
            type="primary"
            @click="emit('viewInstalled', selected.id)"
          >
            {{ kind === 'connector' ? $t('market.viewInstalledConnector') : $t('market.viewInstalled') }}
          </DqButton>
          <DqButton
            :loading="store.installing === `${selected.kind}:${selected.id}`"
            @click="installItem(selected, true)"
          >
            {{ $t('market.reinstall') }}
          </DqButton>
          <DqButton
            type="danger"
            :loading="store.installing === `${selected.kind}:${selected.id}`"
            @click="uninstallItem(selected)"
          >
            {{ $t('market.uninstall') }}
          </DqButton>
        </template>
      </div>

      <div v-if="kind === 'expert' && installedConnectorIds.length" class="market-card__links">
        <p class="market-card__links-title">{{ $t('market.openInstalledConnectors') }}</p>
        <div class="market-card__actions">
          <DqButton
            v-for="cid in installedConnectorIds"
            :key="cid"
            size="sm"
            @click="openConnector(cid)"
          >
            {{ cid }}
          </DqButton>
        </div>
      </div>

      <div v-if="(installDepsRuns.length || lastUninstall?.cleanupLog) && showDepsLog" class="market-card__log">
        <div class="market-card__log-head">
          <span>{{ $t('market.depsLogTitle') }}</span>
          <button type="button" class="market-card__log-toggle" @click="showDepsLog = false">{{ $t('common.close') }}</button>
        </div>
        <div v-for="(run, i) in installDepsRuns" :key="'ir-'+i" class="market-card__log-block">
          <div class="market-card__log-meta">
            <code>{{ run.connectorId }}</code>
            <span v-if="run.script">{{ run.script }}</span>
          </div>
          <pre class="market-card__log-pre">{{ run.log || '—' }}</pre>
        </div>
        <div v-if="lastUninstall?.cleanupScript || lastUninstall?.cleanupLog" class="market-card__log-block">
          <div class="market-card__log-meta">
            <code>{{ lastUninstall.id }}</code>
            <span v-if="lastUninstall.cleanupScript">{{ lastUninstall.cleanupScript }}</span>
          </div>
          <pre class="market-card__log-pre">{{ lastUninstall.cleanupLog || '—' }}</pre>
        </div>
      </div>
    </article>
  </div>
</template>

<style scoped>
.market-browser {
  display: flex;
  flex-direction: column;
  gap: 12px;
  min-height: 0;
  height: 100%;
  overflow: auto;
  padding: 4px 2px 16px;
}
.market-browser__sources {
  margin: 0;
  font-size: var(--dq-font-size-caption);
  color: var(--dq-text-secondary, #888);
}
.market-browser__warnings {
  display: flex;
  flex-direction: column;
  gap: 4px;
}
.market-browser__error {
  margin: 0;
  font-size: var(--dq-font-size-caption);
  color: var(--dq-danger, #dc2626);
  line-height: 1.4;
  word-break: break-word;
}
.market-card {
  border: 1px solid var(--dq-border, rgba(0, 0, 0, 0.08));
  border-radius: 10px;
  padding: 16px 18px;
  background: var(--dq-surface, transparent);
  width: 100%;
}
.market-card__head {
  display: flex;
  align-items: center;
  gap: 8px;
  flex-wrap: wrap;
}
.market-card__title {
  margin: 0;
  font-size: var(--dq-font-size-title);
  font-weight: 600;
}
.market-card__badge {
  font-size: var(--dq-font-size-caption);
  padding: 1px 6px;
  border-radius: 999px;
  background: var(--dq-accent-soft, rgba(16, 185, 129, 0.12));
  color: var(--dq-accent, #059669);
}
.market-card__badge--warn {
  background: rgba(217, 119, 6, 0.12);
  color: #b45309;
}
.market-card__desc {
  margin: 10px 0 0;
  font-size: var(--dq-font-size-body);
  color: var(--dq-text-secondary, #666);
  line-height: 1.5;
}
.market-card__compat {
  margin: 8px 0 0;
  font-size: var(--dq-font-size-caption);
  color: #b45309;
  line-height: 1.4;
}
.market-card__meta {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
  margin-top: 12px;
  font-size: var(--dq-font-size-caption);
  color: var(--dq-text-tertiary, #999);
  align-items: center;
}
.market-card__meta code {
  font-size: var(--dq-font-size-caption);
}
.market-card__link {
  color: var(--dq-accent, #059669);
  text-decoration: none;
}
.market-card__link:hover {
  text-decoration: underline;
}
.market-card__deps {
  margin-top: 10px;
  font-size: var(--dq-font-size-caption);
  color: var(--dq-text-secondary, #666);
  display: flex;
  flex-wrap: wrap;
  gap: 6px;
  align-items: center;
}
.market-card__deps code {
  font-size: var(--dq-font-size-caption);
}
.market-card__dep-link {
  font: inherit;
  font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace;
  font-size: var(--dq-font-size-caption);
  color: var(--dq-accent, #059669);
  background: transparent;
  border: none;
  padding: 0;
  cursor: pointer;
  text-decoration: underline;
}
.market-card__next {
  margin: 12px 0 0;
  font-size: var(--dq-font-size-caption);
  line-height: 1.45;
  color: var(--dq-text-secondary, #666);
}
.market-card__actions {
  margin-top: 16px;
  display: flex;
  gap: 8px;
  flex-wrap: wrap;
}
.market-card__links {
  margin-top: 14px;
  padding-top: 12px;
  border-top: 1px solid var(--dq-border, rgba(0, 0, 0, 0.06));
}
.market-card__links-title {
  margin: 0;
  font-size: var(--dq-font-size-caption);
  color: var(--dq-text-secondary, #666);
}
.market-card__links .market-card__actions {
  margin-top: 8px;
}
.market-card__log {
  margin-top: 14px;
  border: 1px solid var(--dq-border, rgba(0, 0, 0, 0.08));
  border-radius: 8px;
  overflow: hidden;
  background: color-mix(in srgb, var(--dq-bg-elevated, #f8f8f8) 80%, transparent);
}
.market-card__log-head {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 8px 10px;
  font-size: var(--dq-font-size-caption);
  color: var(--dq-text-secondary, #666);
  border-bottom: 1px solid var(--dq-border, rgba(0, 0, 0, 0.06));
}
.market-card__log-toggle {
  font: inherit;
  font-size: var(--dq-font-size-caption);
  border: none;
  background: transparent;
  color: var(--dq-accent, #059669);
  cursor: pointer;
}
.market-card__log-block + .market-card__log-block {
  border-top: 1px solid var(--dq-border, rgba(0, 0, 0, 0.06));
}
.market-card__log-meta {
  display: flex;
  gap: 8px;
  flex-wrap: wrap;
  padding: 6px 10px 0;
  font-size: var(--dq-font-size-caption);
  color: var(--dq-text-tertiary, #999);
}
.market-card__log-pre {
  margin: 0;
  padding: 8px 10px 10px;
  max-height: 220px;
  overflow: auto;
  font-size: 11px;
  line-height: 1.45;
  white-space: pre-wrap;
  word-break: break-word;
  font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace;
}
</style>
