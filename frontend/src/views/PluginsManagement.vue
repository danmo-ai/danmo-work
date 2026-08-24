<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { usePluginsStore, type PluginInstalled } from '@/stores/plugins'
import { useMarketStore } from '@/stores/market'
import { confirm, toast } from '@/utils/feedback'
import WorkspaceShell from '@/components/common/WorkspaceShell.vue'
import MarketBrowser from '@/components/market/MarketBrowser.vue'
import MarketCatalogRail from '@/components/market/MarketCatalogRail.vue'
import { handleResourceRailArrowKeysByKey } from '@/composables/useResourceRailKeyboard'

type PageView = 'library' | 'market'

const { t, locale } = useI18n()
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
  if (!store.selectedName && sortedPlugins.value.length) {
    store.selectedName = sortedPlugins.value[0].name
  }
  try { await marketStore.loadSources() } catch { /* ignore */ }
  try { await marketStore.loadCatalog() } catch { /* ignore */ }
})

const sortedPlugins = computed(() =>
  [...store.items].sort((a, b) => a.name.localeCompare(b.name, locale.value)),
)

const pluginRailItems = computed(() =>
  sortedPlugins.value.map((plugin) => ({ key: plugin.name })),
)

const selectedNameRef = computed({
  get: () => store.selectedName,
  set: (value: string | null) => {
    store.selectedName = value
  },
})

const selectedName = computed(() => store.selectedName)

function selectPlugin(name: string | null) {
  store.selectedName = name
}

function onKeydown(e: KeyboardEvent) {
  handleResourceRailArrowKeysByKey(
    e,
    pluginRailItems.value,
    selectedNameRef,
    selectPlugin,
    pageView.value === 'library',
  )
}

function initial(name: string) {
  return name.trim().charAt(0).toUpperCase() || '?'
}

// Always true so the header (本地/市场 tabs) is always visible;
// empty states are handled inside the body slots below.
const hasSelection = computed(() => true)

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
    @keydown="onKeydown"
  >
    <template #rail>
      <div class="resource-rail__section">
        <div class="resource-rail__section-head">
          <DqSegmented
            v-model="pageView"
            block
            class="resource-rail__page-view"
            :options="pageViewOptions"
          />
        </div>
        <template v-if="pageView === 'library'">
          <div class="resource-rail__scroll">
            <button
              v-for="p in sortedPlugins"
              :key="p.name"
              type="button"
              class="resource-rail__row"
              :class="{ 'is-active': selectedName === p.name }"
              @click="selectPlugin(p.name)"
            >
              <span class="resource-rail__avatar">{{ initial(p.name) }}</span>
              <span class="resource-rail__meta">
                <span class="resource-rail__name">{{ p.name }}</span>
                <span class="resource-rail__desc">
                  v{{ p.version || '0.0.0' }}
                  <span v-if="p.builtin"> · {{ t('plugins.builtin') }}</span>
                </span>
              </span>
            </button>
            <div v-if="sortedPlugins.length === 0" class="resource-rail__empty">
              {{ t('plugins.empty') }}
            </div>
          </div>
        </template>
        <MarketCatalogRail
          v-else
          kind="plugin"
          v-model:selected-key="marketSelectedKey"
        />
      </div>
    </template>

    <template #body>
      <div v-if="pageView === 'library' && store.selected" class="detail-panel">
        <div class="detail-header">{{ store.selected.name }}</div>
        <div class="detail-section">
          <div class="detail-row"><span class="detail-label">{{ t('plugins.version') }}</span><span>{{ store.selected.version || '-' }}</span></div>
          <div v-if="store.selected.builtin" class="detail-row"><span class="detail-label">{{ t('plugins.source') }}</span><span>{{ t('plugins.builtin') }}</span></div>
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
      <DqEmpty
        v-else-if="pageView === 'library'"
        :description="t('plugins.emptyHint')"
        style="min-height: 200px; display: flex; align-items: center; justify-content: center;"
      />
      <MarketBrowser
        v-else-if="pageView === 'market'"
        kind="plugin"
        :selected-key="marketSelectedKey ?? undefined"
        @installed="onMarketInstalled"
        @uninstalled="onMarketUninstalled"
      />
    </template>

    <template v-if="pageView === 'library' && store.selected && !store.selected.builtin" #footer>
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
.resource-rail__empty {
  padding: 24px 12px;
  text-align: center;
  color: var(--dq-color-text-tertiary);
  font-size: 13px;
}

.detail-panel {
  padding: 16px 20px;
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
