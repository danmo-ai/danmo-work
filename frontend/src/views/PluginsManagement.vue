<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { usePluginsStore, type PluginInstalled } from '@/stores/plugins'
import { useMarketStore } from '@/stores/market'
import { confirm, toast } from '@/utils/feedback'
import WorkspaceShell from '@/components/common/WorkspaceShell.vue'
import MarketBrowser from '@/components/market/MarketBrowser.vue'
import MarketCatalogRail from '@/components/market/MarketCatalogRail.vue'

type PageView = 'library' | 'market'

const { t } = useI18n()
const store = usePluginsStore()
const marketStore = useMarketStore()

const pageView = ref<PageView>('library')
const pageViewOptions = computed(() => [
  { label: t('market.library'), value: 'library' as const },
  { label: t('market.tab'), value: 'market' as const },
])
const marketSelectedKey = ref<string | null>(null)

onMounted(async () => {
  await store.load()
  try { await marketStore.loadSources() } catch { /* ignore */ }
})

const sortedPlugins = computed(() =>
  [...store.items].sort((a, b) => a.name.localeCompare(b.name, 'zh-CN')),
)

const selectedName = computed(() => store.selectedName)

function selectPlugin(name: string | null) {
  store.selectedName = name
}

const hasSelection = computed(
  () =>
    (pageView.value === 'market' && !!marketSelectedKey.value) ||
    (pageView.value === 'library' && !!selectedName.value),
)

async function handleUninstall() {
  if (!selectedName.value) return
  const name = selectedName.value
  const ok = await confirm(
    t('plugins.confirmUninstall', { name }),
    t('plugins.uninstallTitle'),
  )
  if (!ok) return
  await store.uninstall(name)
  toast(t('plugins.uninstalled', { name }))
}

async function onMarketInstalled() {
  await store.load()
}

async function onMarketUninstalled() {
  await store.load()
}

function formatComponents(p: PluginInstalled) {
  const c = p.components
  const parts: string[] = []
  if (c.skills?.length) parts.push(`Skills: ${c.skills.join(', ')}`)
  if (c.experts?.length) parts.push(`Experts: ${c.experts.join(', ')}`)
  if (c.mcp?.length) parts.push(`MCP: ${c.mcp.join(', ')}`)
  if (c.knowledge?.length) parts.push(`Knowledge: ${c.knowledge.join(', ')}`)
  return parts
}

const marketSelected = computed(() => {
  if (!marketSelectedKey.value) return null
  return (
    marketStore.catalog.find(
      (item) => item.kind === 'plugin' && `${item.sourceId}:${item.id}` === marketSelectedKey.value,
    ) ?? null
  )
})
</script>

<template>
  <WorkspaceShell
    :has-selection="hasSelection"
    :custom-rail="true"
  >
    <template #header>
      <div style="padding-left: 8px;">
        <DqSegmented
          :value="pageView"
          :options="pageViewOptions"
          @change="(v: string) => { pageView = v as PageView; store.selectedName = null; marketSelectedKey = null }"
        />
      </div>
    </template>

    <template v-if="pageView === 'library'" #rail>
      <div class="resource-rail">
        <div
          v-for="p in sortedPlugins"
          :key="p.name"
          class="resource-rail__row"
          :class="{ 'resource-rail__row--active': selectedName === p.name }"
          @click="selectPlugin(p.name)"
        >
          <div class="resource-rail__avatar">&#128268;</div>
          <div class="resource-rail__meta">
            <div class="resource-rail__title">{{ p.name }}</div>
            <div class="resource-rail__subtitle">v{{ p.version || '0.0.0' }}</div>
          </div>
        </div>
        <div v-if="sortedPlugins.length === 0" class="resource-rail__empty">
          {{ t('plugins.empty') }}
        </div>
      </div>
    </template>

    <template v-if="pageView === 'market'" #rail>
      <MarketCatalogRail
        kind="plugin"
        v-model:selected-key="marketSelectedKey"
      />
    </template>

    <template #empty>
      <DqEmpty
        :description="pageView === 'library' ? t('plugins.emptyHint') : t('market.emptyHint')"
        style="min-height: 200px; display: flex; align-items: center; justify-content: center;"
      />
    </template>

    <template #body>
      <div v-if="pageView === 'library' && store.selected" class="detail-panel">
        <div class="detail-header">{{ store.selected.name }}</div>
        <div class="detail-section">
          <div class="detail-row"><span class="detail-label">{{ t('plugins.version') }}</span><span>{{ store.selected.version || '-' }}</span></div>
          <div class="detail-row"><span class="detail-label">{{ t('plugins.author') }}</span><span>{{ store.selected.author?.name || '-' }}</span></div>
          <div class="detail-row"><span class="detail-label">{{ t('plugins.license') }}</span><span>{{ store.selected.license || '-' }}</span></div>
          <div v-if="store.selected.homepage" class="detail-row"><span class="detail-label">{{ t('plugins.homepage') }}</span><span>{{ store.selected.homepage }}</span></div>
          <div v-if="store.selected.repository" class="detail-row"><span class="detail-label">{{ t('plugins.repository') }}</span><span>{{ store.selected.repository }}</span></div>
          <div class="detail-row">
            <span class="detail-label">{{ t('plugins.description') }}</span>
            <span>{{ store.selected.description || '-' }}</span>
          </div>
          <div v-if="store.selected.keywords?.length" class="detail-row">
            <span class="detail-label">{{ t('plugins.keywords') }}</span>
            <span>
              <DqTag v-for="kw in store.selected.keywords" :key="kw" size="small" style="margin-right: 4px;">{{ kw }}</DqTag>
            </span>
          </div>
        </div>
        <div class="detail-section" v-if="formatComponents(store.selected).length">
          <div class="detail-section-title">{{ t('plugins.components') }}</div>
          <div v-for="comp in formatComponents(store.selected)" :key="comp" class="detail-row">
            <span>{{ comp }}</span>
          </div>
        </div>
        <div class="detail-section">
          <div class="detail-row"><span class="detail-label">{{ t('plugins.installedAt') }}</span><span>{{ store.selected.installedAt }}</span></div>
        </div>
      </div>
      <div v-if="pageView === 'market'" style="height: 100%;">
        <MarketBrowser
          kind="plugin"
          :selected-key="marketSelectedKey ?? undefined"
          @installed="onMarketInstalled"
          @uninstalled="onMarketUninstalled"
        />
      </div>
    </template>

    <template v-if="pageView === 'library' && store.selected" #footer>
      <div style="display: flex; gap: 8px; justify-content: flex-end; width: 100%;">
        <DqButton
          theme="danger"
          :loading="store.uninstalling === store.selected.name"
          @click="handleUninstall"
        >
          {{ t('plugins.uninstall') }}
        </DqButton>
      </div>
    </template>
  </WorkspaceShell>
</template>

<style scoped>
.resource-rail {
  display: flex;
  flex-direction: column;
  height: 100%;
  overflow-y: auto;
}
.resource-rail__row {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 8px 12px;
  cursor: pointer;
  border-radius: 6px;
  transition: background 0.15s;
}
.resource-rail__row:hover {
  background: var(--dq-color-fill-secondary);
}
.resource-rail__row--active {
  background: var(--dq-color-fill-brand-light);
}
.resource-rail__avatar {
  width: 28px;
  height: 28px;
  display: flex;
  align-items: center;
  justify-content: center;
  font-size: 14px;
  border-radius: 6px;
  background: var(--dq-color-fill-secondary);
  flex-shrink: 0;
}
.resource-rail__meta {
  flex: 1;
  min-width: 0;
}
.resource-rail__title {
  font-size: 13px;
  font-weight: 500;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}
.resource-rail__subtitle {
  font-size: 11px;
  color: var(--dq-color-text-tertiary);
}
.resource-rail__empty {
  padding: 24px 12px;
  text-align: center;
  color: var(--dq-color-text-tertiary);
  font-size: 13px;
}

.detail-panel {
  padding: 16px 20px;
  overflow-y: auto;
  height: 100%;
}
.detail-header {
  font-size: 18px;
  font-weight: 600;
  margin-bottom: 16px;
}
.detail-section {
  margin-bottom: 16px;
}
.detail-section-title {
  font-size: 13px;
  font-weight: 600;
  color: var(--dq-color-text-secondary);
  margin-bottom: 8px;
  text-transform: uppercase;
  letter-spacing: 0.5px;
}
.detail-row {
  display: flex;
  align-items: flex-start;
  gap: 12px;
  padding: 6px 0;
  font-size: 13px;
  line-height: 1.5;
}
.detail-label {
  color: var(--dq-color-text-tertiary);
  min-width: 80px;
  flex-shrink: 0;
}
</style>
