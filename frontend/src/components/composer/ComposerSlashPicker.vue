<script setup lang="ts">
import { computed, ref, watch, nextTick } from 'vue'
import { useI18n } from 'vue-i18n'
import {
  filterSlashCommands,
  type ComposerSlashCommand,
} from '@/types/composer-slash'

const props = defineProps<{
  commands: ComposerSlashCommand[]
  query?: string
}>()

const emit = defineEmits<{
  select: [cmd: ComposerSlashCommand]
  close: []
}>()

const { t } = useI18n()
const activeIndex = ref(0)
const listRef = ref<HTMLElement | null>(null)

const filtered = computed(() => filterSlashCommands(props.commands, props.query ?? ''))

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
  const cmd = filtered.value[activeIndex.value]
  if (cmd) emit('select', cmd)
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
    class="slash-picker"
    role="listbox"
    :aria-label="t('composer.slash.title')"
    @keydown="onKeydown"
  >
    <div class="slash-picker__head">
      <span class="slash-picker__title">{{ t('composer.slash.title') }}</span>
      <button type="button" class="slash-picker__close" :aria-label="t('common.close')" @click="emit('close')">
        ×
      </button>
    </div>
    <div v-if="!filtered.length" class="slash-picker__empty">{{ t('composer.slash.noMatch') }}</div>
    <ul v-else ref="listRef" class="slash-picker__list">
      <li
        v-for="(cmd, idx) in filtered"
        :key="cmd.id"
        :data-idx="idx"
        class="slash-picker__item"
        :class="{ 'is-active': idx === activeIndex }"
        role="option"
        :aria-selected="idx === activeIndex"
        @mouseenter="activeIndex = idx"
        @click="emit('select', cmd)"
      >
        <div class="slash-picker__item-main">
          <span class="slash-picker__trigger">/{{ cmd.trigger }}</span>
          <span class="slash-picker__label">{{ t(cmd.labelKey) }}</span>
        </div>
        <p class="slash-picker__desc">{{ t(cmd.descriptionKey) }}</p>
      </li>
    </ul>
  </div>
</template>

<style scoped>
.slash-picker {
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

.slash-picker__head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
  padding: 8px 10px 6px;
  border-bottom: 1px solid color-mix(in srgb, var(--dq-border-subtle, #000) 60%, transparent);
}

.slash-picker__title {
  font-size: var(--dq-font-size-body);
  font-weight: 600;
  opacity: 0.75;
}

.slash-picker__close {
  border: none;
  background: transparent;
  cursor: pointer;
  opacity: 0.5;
  font-size: var(--dq-font-size-prose);
  line-height: 1;
  padding: 0 4px;
  color: inherit;
}

.slash-picker__close:hover {
  opacity: 1;
}

.slash-picker__list {
  list-style: none;
  margin: 0;
  padding: 4px;
  overflow: auto;
  flex: 1;
  min-height: 0;
}

.slash-picker__item {
  padding: 8px 10px;
  border-radius: 6px;
  cursor: pointer;
}

.slash-picker__item.is-active {
  background: color-mix(in srgb, var(--dq-accent) 10%, transparent);
}

.slash-picker__item-main {
  display: flex;
  align-items: baseline;
  gap: 8px;
  min-width: 0;
}

.slash-picker__trigger {
  font-size: var(--dq-font-size-body);
  font-weight: 700;
  font-family: var(--dq-font-mono);
  color: var(--dq-accent);
}

.slash-picker__label {
  font-size: var(--dq-font-size-body);
  opacity: 0.8;
}

.slash-picker__desc {
  margin: 2px 0 0;
  font-size: var(--dq-font-size-caption);
  line-height: 1.35;
  opacity: 0.65;
}

.slash-picker__empty {
  padding: 16px 12px;
  font-size: var(--dq-font-size-body);
  opacity: 0.6;
  text-align: center;
}
</style>
