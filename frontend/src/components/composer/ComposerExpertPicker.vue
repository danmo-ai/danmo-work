<script setup lang="ts">
import { computed, ref, watch, nextTick } from 'vue'
import { useI18n } from 'vue-i18n'
import type { Agent } from '@/types'
import {
  filterSummonableExperts,
  groupSummonableExperts,
  type ExpertCategoryGroup,
} from '@/types/composer-experts'

const props = defineProps<{
  agents: Agent[]
  selectedIds: string[]
  excludeAgentId?: string | null
  /** External filter (e.g. `@query`); when set with showSearch=false, search box is hidden. */
  query?: string
  /** Show internal search field (button mode). */
  showSearch?: boolean
  /** Hide the close button (e.g. when stacked under a shared popover). */
  hideClose?: boolean
}>()

const emit = defineEmits<{
  select: [agent: Agent]
  close: []
}>()

const { t } = useI18n()
const localQuery = ref('')
const activeIndex = ref(0)
const listRef = ref<HTMLElement | null>(null)

const effectiveQuery = computed(() =>
  props.showSearch ? localQuery.value : (props.query ?? ''),
)

const filtered = computed(() =>
  filterSummonableExperts(props.agents, effectiveQuery.value, props.excludeAgentId),
)

const grouped = computed((): ExpertCategoryGroup[] =>
  groupSummonableExperts(props.agents, effectiveQuery.value, props.excludeAgentId),
)

/** Flat index → agent for keyboard nav (matches visual order in grouped list). */
const flatAgents = computed(() => grouped.value.flatMap((g) => g.agents))

watch(
  flatAgents,
  (list) => {
    if (activeIndex.value >= list.length) activeIndex.value = Math.max(0, list.length - 1)
  },
  { immediate: true },
)

watch(
  () => props.query,
  () => {
    activeIndex.value = 0
  },
)

function isSelected(id: string) {
  return props.selectedIds.includes(id)
}

function sourceLabel(a: Agent) {
  if (a.marketSource) return t('composer.expertMarket')
  if (a.builtin) return t('composer.expertBuiltin')
  return t('composer.expertCustom')
}

function categoryLabel(id: string) {
  const key = `teams.category.${id}`
  const label = t(key)
  return label === key ? t('teams.category.other') : label
}

function flatIndexOf(agentId: string) {
  return flatAgents.value.findIndex((a) => a.id === agentId)
}

function move(delta: number) {
  const n = flatAgents.value.length
  if (!n) return
  activeIndex.value = (activeIndex.value + delta + n) % n
  void nextTick(() => {
    const el = listRef.value?.querySelector<HTMLElement>(`[data-idx="${activeIndex.value}"]`)
    el?.scrollIntoView({ block: 'nearest' })
  })
}

function confirmActive() {
  const a = flatAgents.value[activeIndex.value]
  if (a) emit('select', a)
}

function onKeydown(e: KeyboardEvent) {
  if (e.key === 'ArrowDown') {
    e.preventDefault()
    move(1)
    return
  }
  if (e.key === 'ArrowUp') {
    e.preventDefault()
    move(-1)
    return
  }
  if (e.key === 'Enter') {
    e.preventDefault()
    confirmActive()
    return
  }
  if (e.key === 'Escape') {
    e.preventDefault()
    emit('close')
  }
}

defineExpose({ onKeydown, confirmActive, move, filtered, grouped })
</script>

<template>
  <div
    class="expert-picker"
    role="listbox"
    :aria-label="t('composer.expertPickerTitle')"
    @keydown="onKeydown"
  >
    <div class="expert-picker__head">
      <span class="expert-picker__title">{{ t('composer.expertPickerTitle') }}</span>
      <button
        v-if="!hideClose"
        type="button"
        class="expert-picker__close"
        :aria-label="t('common.close')"
        @click="emit('close')"
      >
        ×
      </button>
    </div>

    <div v-if="showSearch" class="expert-picker__search">
      <input
        v-model="localQuery"
        type="search"
        class="expert-picker__search-input"
        :placeholder="t('composer.expertSearchPlaceholder')"
        autocomplete="off"
        @keydown="onKeydown"
      >
    </div>

    <div v-if="!agents.length" class="expert-picker__empty">{{ t('composer.expertEmpty') }}</div>
    <div v-else-if="!flatAgents.length" class="expert-picker__empty">{{ t('composer.expertNoMatch') }}</div>
    <div v-else ref="listRef" class="expert-picker__list">
      <div v-for="group in grouped" :key="group.id" class="expert-picker__group">
        <div class="expert-picker__group-title">{{ categoryLabel(group.id) }}</div>
        <ul class="expert-picker__group-list">
          <li
            v-for="a in group.agents"
            :key="a.id"
            :data-idx="flatIndexOf(a.id)"
            class="expert-picker__item"
            :class="{
              'is-active': flatIndexOf(a.id) === activeIndex,
              'is-selected': isSelected(a.id),
            }"
            role="option"
            :aria-selected="isSelected(a.id)"
            @mouseenter="activeIndex = flatIndexOf(a.id)"
            @click="emit('select', a)"
          >
            <div class="expert-picker__item-main">
              <span class="expert-picker__name">{{ a.name || a.id }}</span>
              <span class="expert-picker__source">{{ sourceLabel(a) }}</span>
            </div>
            <p v-if="a.description" class="expert-picker__desc">{{ a.description }}</p>
            <span v-if="isSelected(a.id)" class="expert-picker__check" aria-hidden="true">✓</span>
          </li>
        </ul>
      </div>
    </div>
  </div>
</template>

<style scoped>
.expert-picker {
  display: flex;
  flex-direction: column;
  min-width: 260px;
  max-width: min(420px, 92vw);
  max-height: 280px;
  overflow: hidden;
  border: 1px solid var(--dq-border-subtle, rgba(0, 0, 0, 0.08));
  border-radius: var(--dq-composer-radius, var(--dq-radius-menu, 16px));
  /* Opaque sheet — glass popover-bg is ~24% in dark and lets stream bleed through. */
  background: var(--dq-bg-elevated, #fff);
  box-shadow: none;
  color: var(--dq-label-primary, inherit);
  isolation: isolate;
}

.expert-picker__head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
  padding: 8px 10px 6px;
  border-bottom: 1px solid color-mix(in srgb, var(--dq-border-subtle, #000) 60%, transparent);
}

.expert-picker__title {
  font-size: var(--dq-font-size-body);
  font-weight: 600;
  opacity: 0.75;
}

.expert-picker__close {
  border: none;
  background: transparent;
  cursor: pointer;
  opacity: 0.5;
  font-size: var(--dq-font-size-prose);
  line-height: 1;
  padding: 0 4px;
  color: inherit;
}

.expert-picker__close:hover {
  opacity: 1;
}

.expert-picker__search {
  padding: 6px 10px;
}

.expert-picker__search-input {
  width: 100%;
  box-sizing: border-box;
  padding: 6px 8px;
  border: 1px solid color-mix(in srgb, var(--dq-border-subtle, #000) 70%, transparent);
  border-radius: 6px;
  background: transparent;
  color: inherit;
  font: inherit;
  font-size: var(--dq-font-size-body);
}

.expert-picker__search-input:focus {
  outline: 1px solid color-mix(in srgb, var(--dq-accent) 50%, transparent);
}

.expert-picker__list {
  list-style: none;
  margin: 0;
  padding: 4px;
  overflow: auto;
  flex: 1;
  min-height: 0;
}

.expert-picker__group + .expert-picker__group {
  margin-top: 4px;
}

.expert-picker__group-title {
  padding: 6px 10px 2px;
  font-size: var(--dq-font-size-caption);
  font-weight: 600;
  opacity: 0.5;
  letter-spacing: 0.02em;
}

.expert-picker__group-list {
  list-style: none;
  margin: 0;
  padding: 0;
}

.expert-picker__item {
  position: relative;
  padding: 8px 10px;
  border-radius: 6px;
  cursor: pointer;
}

.expert-picker__item.is-active {
  background: color-mix(in srgb, var(--dq-accent) 10%, transparent);
}

.expert-picker__item-main {
  display: flex;
  align-items: baseline;
  gap: 8px;
  min-width: 0;
  padding-right: 18px;
}

.expert-picker__name {
  font-size: var(--dq-font-size-body);
  font-weight: 600;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.expert-picker__source {
  flex-shrink: 0;
  font-size: var(--dq-font-size-caption);
  opacity: 0.55;
}

.expert-picker__desc {
  margin: 2px 0 0;
  padding-right: 18px;
  font-size: var(--dq-font-size-caption);
  line-height: 1.35;
  opacity: 0.65;
  display: -webkit-box;
  -webkit-line-clamp: 2;
  -webkit-box-orient: vertical;
  overflow: hidden;
}

.expert-picker__check {
  position: absolute;
  top: 8px;
  right: 10px;
  color: var(--dq-accent);
  font-size: var(--dq-font-size-body);
}

.expert-picker__empty {
  padding: 16px 12px;
  font-size: var(--dq-font-size-body);
  opacity: 0.6;
  text-align: center;
}
</style>
