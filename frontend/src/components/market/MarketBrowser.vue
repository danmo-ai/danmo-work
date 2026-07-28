<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { useMarketStore } from '@/stores/market'
import { confirm, toast } from '@/utils/feedback'
import type { MarketListing } from '@/types'

const props = defineProps<{
  kind: 'skill' | 'expert'
  selectedKey?: string | null
}>()

const emit = defineEmits<{
  installed: [id: string]
  uninstalled: [id: string]
}>()

const { t } = useI18n()
const store = useMarketStore()

const selected = computed(() => {
  if (!props.selectedKey) return null
  return (
    store.catalog.find(
      (item) => item.kind === props.kind && `${item.sourceId}:${item.id}` === props.selectedKey,
    ) ?? null
  )
})

const enabledSources = computed(() => store.sources.filter((s) => s.enabled))

const isClawhubItem = computed(() => {
  const item = selected.value
  if (!item) return false
  const src = store.sources.find((s) => s.id === item.sourceId)
  return src?.kind === 'clawhub' || item.sourceId === 'clawhub'
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

async function installItem(item: MarketListing, overwrite = false) {
  try {
    await store.install(item.sourceId, item.kind, item.id, overwrite)
    toast.success(t('market.installSuccess', { name: item.name }))
    emit('installed', item.id)
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
  try {
    await store.uninstall(item.kind, item.id)
    toast.success(t('market.uninstallSuccess', { name: item.name }))
    emit('uninstalled', item.id)
  } catch (e) {
    toast.error(e instanceof Error ? e.message : t('market.uninstallFailed'))
  }
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
          v-if="clawhubListingURL"
          class="market-card__link"
          :href="clawhubListingURL"
          target="_blank"
          rel="noopener noreferrer"
        >
          {{ $t('market.openOnClawhub') }}
        </a>
      </div>
      <div v-if="selected.skillDeps?.length" class="market-card__deps">
        {{ $t('market.skillDeps') }}:
        <code v-for="dep in selected.skillDeps" :key="dep">{{ dep }}</code>
      </div>
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
  font-size: 12px;
  color: var(--dq-text-secondary, #888);
}
.market-browser__warnings {
  display: flex;
  flex-direction: column;
  gap: 4px;
}
.market-browser__error {
  margin: 0;
  font-size: 12px;
  color: var(--dq-danger, #dc2626);
  line-height: 1.4;
  word-break: break-word;
}
.market-card {
  border: 1px solid var(--dq-border, rgba(0, 0, 0, 0.08));
  border-radius: 10px;
  padding: 16px 18px;
  background: var(--dq-surface, transparent);
  max-width: 640px;
}
.market-card__head {
  display: flex;
  align-items: center;
  gap: 8px;
  flex-wrap: wrap;
}
.market-card__title {
  margin: 0;
  font-size: 18px;
  font-weight: 600;
}
.market-card__badge {
  font-size: 11px;
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
  font-size: 14px;
  color: var(--dq-text-secondary, #666);
  line-height: 1.5;
}
.market-card__compat {
  margin: 8px 0 0;
  font-size: 12px;
  color: #b45309;
  line-height: 1.4;
}
.market-card__meta {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
  margin-top: 12px;
  font-size: 12px;
  color: var(--dq-text-tertiary, #999);
  align-items: center;
}
.market-card__meta code {
  font-size: 11px;
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
  font-size: 12px;
  color: var(--dq-text-secondary, #666);
  display: flex;
  flex-wrap: wrap;
  gap: 6px;
  align-items: center;
}
.market-card__deps code {
  font-size: 11px;
}
.market-card__actions {
  margin-top: 16px;
  display: flex;
  gap: 8px;
  flex-wrap: wrap;
}
</style>
