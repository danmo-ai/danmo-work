<script setup lang="ts">
import { computed, ref, watch, nextTick } from 'vue'
import { useI18n } from 'vue-i18n'
import type { AvailableSkill } from '@/types'
import { filterAvailableSkills } from '@/types/composer-skills'

const props = defineProps<{
  skills: AvailableSkill[]
  selectedIds: string[]
  /** External filter (e.g. `@query`); when set, search box is hidden. */
  query?: string
  loading?: boolean
  /** Show internal search field (button mode). */
  showSearch?: boolean
}>()

const emit = defineEmits<{
  select: [skill: AvailableSkill]
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
  filterAvailableSkills(props.skills, effectiveQuery.value),
)

watch(
  filtered,
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

function sourceLabel(source: AvailableSkill['source']) {
  if (source === 'filesystem') return t('composer.skillSourceFilesystem')
  if (source === 'both') return t('composer.skillSourceBoth')
  return t('composer.skillSourceBound')
}

function move(delta: number) {
  const n = filtered.value.length
  if (!n) return
  activeIndex.value = (activeIndex.value + delta + n) % n
  void nextTick(() => {
    const el = listRef.value?.querySelector<HTMLElement>(`[data-idx="${activeIndex.value}"]`)
    el?.scrollIntoView({ block: 'nearest' })
  })
}

function confirmActive() {
  const sk = filtered.value[activeIndex.value]
  if (sk) emit('select', sk)
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

defineExpose({ onKeydown, confirmActive, move })
</script>

<template>
  <div
    class="skill-picker"
    role="listbox"
    :aria-label="t('composer.skillPickerTitle')"
    @keydown="onKeydown"
  >
    <div class="skill-picker__head">
      <span class="skill-picker__title">{{ t('composer.skillPickerTitle') }}</span>
      <button type="button" class="skill-picker__close" :aria-label="t('common.close')" @click="emit('close')">
        ×
      </button>
    </div>

    <div v-if="showSearch" class="skill-picker__search">
      <input
        v-model="localQuery"
        type="search"
        class="skill-picker__search-input"
        :placeholder="t('composer.skillSearchPlaceholder')"
        autocomplete="off"
        @keydown="onKeydown"
      >
    </div>

    <div v-if="loading" class="skill-picker__empty">{{ t('composer.skillLoading') }}</div>
    <div v-else-if="!skills.length" class="skill-picker__empty">{{ t('composer.skillEmpty') }}</div>
    <div v-else-if="!filtered.length" class="skill-picker__empty">{{ t('composer.skillNoMatch') }}</div>
    <ul v-else ref="listRef" class="skill-picker__list">
      <li
        v-for="(sk, idx) in filtered"
        :key="sk.id"
        :data-idx="idx"
        class="skill-picker__item"
        :class="{
          'is-active': idx === activeIndex,
          'is-selected': isSelected(sk.id),
        }"
        role="option"
        :aria-selected="isSelected(sk.id)"
        @mouseenter="activeIndex = idx"
        @click="emit('select', sk)"
      >
        <div class="skill-picker__item-main">
          <span class="skill-picker__name">{{ sk.name || sk.id }}</span>
          <span class="skill-picker__source">{{ sourceLabel(sk.source) }}</span>
        </div>
        <p v-if="sk.description" class="skill-picker__desc">{{ sk.description }}</p>
        <span v-if="isSelected(sk.id)" class="skill-picker__check" aria-hidden="true">✓</span>
      </li>
    </ul>
  </div>
</template>

<style scoped>
.skill-picker {
  display: flex;
  flex-direction: column;
  min-width: 260px;
  max-width: min(420px, 92vw);
  max-height: 280px;
  overflow: hidden;
  border: 1px solid var(--dq-border-subtle, rgba(0, 0, 0, 0.08));
  border-radius: 10px;
  /* Opaque sheet — glass popover-bg is ~24% in dark and lets stream bleed through. */
  background: var(--dq-bg-elevated, #fff);
  box-shadow: 0 12px 40px rgba(0, 0, 0, 0.45);
  color: var(--dq-label-primary, inherit);
  isolation: isolate;
}

.skill-picker__head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
  padding: 8px 10px 6px;
  border-bottom: 1px solid color-mix(in srgb, var(--dq-border-subtle, #000) 60%, transparent);
}

.skill-picker__title {
  font-size: var(--dq-font-size-body);
  font-weight: 600;
  opacity: 0.75;
}

.skill-picker__close {
  border: none;
  background: transparent;
  cursor: pointer;
  opacity: 0.5;
  font-size: var(--dq-font-size-prose);
  line-height: 1;
  padding: 0 4px;
  color: inherit;
}

.skill-picker__close:hover {
  opacity: 1;
}

.skill-picker__search {
  padding: 6px 10px;
}

.skill-picker__search-input {
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

.skill-picker__search-input:focus {
  outline: 1px solid color-mix(in srgb, var(--dq-accent) 50%, transparent);
}

.skill-picker__list {
  list-style: none;
  margin: 0;
  padding: 4px;
  overflow: auto;
  flex: 1;
  min-height: 0;
}

.skill-picker__item {
  position: relative;
  padding: 8px 10px;
  border-radius: 6px;
  cursor: pointer;
}

.skill-picker__item.is-active {
  background: color-mix(in srgb, var(--dq-accent) 10%, transparent);
}

.skill-picker__item-main {
  display: flex;
  align-items: baseline;
  gap: 8px;
  min-width: 0;
  padding-right: 18px;
}

.skill-picker__name {
  font-size: var(--dq-font-size-body);
  font-weight: 600;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.skill-picker__source {
  flex-shrink: 0;
  font-size: var(--dq-font-size-caption);
  opacity: 0.55;
}

.skill-picker__desc {
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

.skill-picker__check {
  position: absolute;
  top: 8px;
  right: 10px;
  color: var(--dq-accent);
  font-size: var(--dq-font-size-body);
}

.skill-picker__empty {
  padding: 16px 12px;
  font-size: var(--dq-font-size-body);
  opacity: 0.6;
  text-align: center;
}
</style>
