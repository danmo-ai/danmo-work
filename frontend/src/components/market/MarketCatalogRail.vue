<script setup lang="ts">
import { computed, onMounted, watch } from 'vue'
import { useMarketStore } from '@/stores/market'
import { uiLocale } from '@/stores/locale'
import type { MarketListing } from '@/types'

const props = defineProps<{
  kind: 'skill' | 'expert' | 'connector' | 'plugin'
}>()

const selectedKey = defineModel<string | null>('selectedKey', { default: null })

const store = useMarketStore()

const items = computed(() =>
  store.catalog
    .filter((item) => item.kind === props.kind)
    .slice()
    .sort((a, b) => a.name.localeCompare(b.name, uiLocale())),
)

const availableItems = computed(() => items.value.filter((item) => !item.installed))
const installedItems = computed(() => items.value.filter((item) => item.installed))

function listingKey(item: MarketListing) {
  return `${item.sourceId}:${item.id}`
}

function select(item: MarketListing) {
  selectedKey.value = listingKey(item)
}

function initial(name: string) {
  return name.trim().charAt(0).toUpperCase() || '?'
}

function ensureSelection() {
  if (!items.value.length) {
    selectedKey.value = null
    return
  }
  if (selectedKey.value && items.value.some((item) => listingKey(item) === selectedKey.value)) {
    return
  }
  const prefer = installedItems.value[0] ?? availableItems.value[0] ?? items.value[0]
  selectedKey.value = listingKey(prefer)
}

onMounted(async () => {
  await Promise.all([store.loadSources(), store.loadCatalog()])
  ensureSelection()
})

watch(items, () => ensureSelection())
</script>

<template>
  <div class="market-rail">
    <div class="market-rail__head">
      <span class="market-rail__title">{{ $t('market.catalog') }}</span>
      <DqIconButton :aria-label="$t('market.refresh')" :disabled="store.loading" @click="store.loadCatalog(true)">
        <svg viewBox="0 0 24 24" width="14" height="14" fill="none" stroke="currentColor" stroke-width="2">
          <path d="M21 12a9 9 0 1 1-2.64-6.36" stroke-linecap="round" stroke-linejoin="round" />
          <path d="M21 3v6h-6" stroke-linecap="round" stroke-linejoin="round" />
        </svg>
      </DqIconButton>
    </div>

    <DqEmpty
      v-if="!store.loading && !items.length"
      class="market-rail__empty"
      :description="store.error || $t('market.empty')"
    />

    <div v-else class="market-rail__body">
      <div v-if="availableItems.length" class="market-rail__group">
        <div class="market-rail__group-title">{{ $t('market.available') }}</div>
        <nav class="market-rail__list" :aria-label="$t('market.available')">
          <button
            v-for="item in availableItems"
            :key="listingKey(item)"
            type="button"
            class="resource-rail__row"
            :class="{ 'is-active': selectedKey === listingKey(item) }"
            @click="select(item)"
          >
            <span class="resource-rail__avatar">{{ initial(item.name) }}</span>
            <span class="resource-rail__meta">
              <span class="resource-rail__name">
                {{ item.name }}
                <span v-if="item.compatibility" class="market-rail__warn" :title="item.compatibility">!</span>
              </span>
              <span class="resource-rail__desc">{{ item.sourceName || item.sourceId }}</span>
            </span>
          </button>
        </nav>
      </div>

      <div v-if="installedItems.length" class="market-rail__group">
        <div class="market-rail__group-title">{{ $t('market.installed') }}</div>
        <nav class="market-rail__list" :aria-label="$t('market.installed')">
          <button
            v-for="item in installedItems"
            :key="listingKey(item)"
            type="button"
            class="resource-rail__row"
            :class="{ 'is-active': selectedKey === listingKey(item) }"
            @click="select(item)"
          >
            <span class="resource-rail__avatar">{{ initial(item.name) }}</span>
            <span class="resource-rail__meta">
              <span class="resource-rail__name">
                {{ item.name }}
                <span v-if="item.compatibility" class="market-rail__warn" :title="item.compatibility">!</span>
              </span>
              <span class="resource-rail__desc">{{ item.sourceName || item.sourceId }}</span>
            </span>
          </button>
        </nav>
      </div>
    </div>
  </div>
</template>

<style scoped>
.market-rail {
  display: flex;
  flex-direction: column;
  flex: 1;
  min-height: 0;
  overflow: hidden;
}

.market-rail__head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
  padding: 10px 10px 6px 14px;
  flex-shrink: 0;
}

.market-rail__title {
  font-size: var(--dq-font-size-caption);
  font-weight: 600;
  letter-spacing: 0.06em;
  text-transform: uppercase;
  color: var(--dq-label-tertiary);
  line-height: 1.3;
}

.market-rail__empty {
  padding: 20px 12px;
}

.market-rail__body {
  flex: 1;
  min-height: 0;
  overflow-y: auto;
  padding: 0 6px 8px;
}

.market-rail__group + .market-rail__group {
  margin-top: 10px;
}

.market-rail__group-title {
  padding: 6px 10px 6px;
  font-size: var(--dq-font-size-caption);
  font-weight: 600;
  letter-spacing: 0.04em;
  text-transform: uppercase;
  color: var(--dq-label-tertiary);
  line-height: 1.3;
}

.market-rail__list {
  display: flex;
  flex-direction: column;
}

.market-rail__warn {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  margin-left: 4px;
  width: 14px;
  height: 14px;
  border-radius: 999px;
  font-size: var(--dq-font-size-caption);
  font-weight: 700;
  background: rgba(217, 119, 6, 0.15);
  color: #b45309;
  vertical-align: middle;
}
</style>
