<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import type { ShelfBookRow } from '@/composables/useNovelBookLoader'

defineProps<{
  books: ShelfBookRow[]
  activeBookId: string | null
  loading: boolean
}>()

const emit = defineEmits<{
  open: [bookId: string]
  init: []
}>()

const { t } = useI18n()
</script>

<template>
  <div class="novel-shelf">
    <div v-if="loading" class="novel-shelf__empty">{{ t('novelWorkbench.loading') }}</div>
    <div v-else-if="!books.length" class="novel-shelf__empty">
      <p>{{ t('novelWorkbench.emptyShelf') }}</p>
      <p class="novel-shelf__hint">{{ t('novelWorkbench.emptyHint') }}</p>
      <button type="button" class="novel-wb-btn" @click="emit('init')">
        {{ t('novelWorkbench.actionInit') }}
      </button>
    </div>
    <ul v-else class="novel-shelf__grid">
      <li v-for="b in books" :key="b.id">
        <button
          type="button"
          class="novel-shelf__card"
          :class="{ 'novel-shelf__card--active': activeBookId === b.id }"
          @click="emit('open', b.id)"
        >
          <div class="novel-shelf__card-top">
            <span class="novel-shelf__title">{{ b.state?.title || b.id }}</span>
            <span v-if="activeBookId === b.id" class="novel-shelf__badge">
              {{ t('novelWorkbench.activeBook') }}
            </span>
          </div>
          <div class="novel-shelf__meta">
            <span class="novel-shelf__id">{{ b.id }}</span>
            <span v-if="b.state?.stage">· {{ b.state.stage }}</span>
          </div>
          <div v-if="b.progress" class="novel-shelf__progress">
            {{
              t('novelWorkbench.progressLabel', {
                committed: b.progress.committed,
                total: b.progress.total,
              })
            }}
          </div>
          <div v-else-if="b.state" class="novel-shelf__progress">
            {{ t('novelWorkbench.chapterN', { n: b.state.lastCommittedCh }) }}
          </div>
        </button>
      </li>
    </ul>
    <div v-if="books.length" class="novel-shelf__footer">
      <button type="button" class="novel-wb-btn novel-wb-btn--ghost" @click="emit('init')">
        {{ t('novelWorkbench.actionInit') }}
      </button>
    </div>
  </div>
</template>

<style scoped>
.novel-shelf {
  flex: 1;
  min-height: 0;
  display: flex;
  flex-direction: column;
  overflow: hidden;
}

.novel-shelf__empty {
  padding: 24px 16px;
  font-size: var(--dq-font-size-body);
  opacity: 0.75;
  line-height: 1.45;
}

.novel-shelf__hint {
  margin: 8px 0 16px;
  font-size: var(--dq-font-size-caption);
  opacity: 0.85;
}

.novel-shelf__grid {
  list-style: none;
  margin: 0;
  padding: 10px 12px;
  overflow: auto;
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(200px, 1fr));
  gap: 10px;
  flex: 1;
  min-height: 0;
  align-content: start;
}

.novel-shelf__card {
  display: flex;
  flex-direction: column;
  gap: 6px;
  width: 100%;
  margin: 0;
  padding: 14px 14px 12px;
  border: 1px solid color-mix(in srgb, var(--dq-border-subtle, #000) 55%, transparent);
  border-radius: 10px;
  background: color-mix(in srgb, var(--dq-glass-popover-bg, #fff) 70%, transparent);
  color: inherit;
  font: inherit;
  text-align: left;
  cursor: pointer;
  transition: border-color 0.15s ease, background 0.15s ease;
}

.novel-shelf__card:hover {
  border-color: color-mix(in srgb, var(--dq-accent) 45%, transparent);
  background: color-mix(in srgb, var(--dq-accent) 6%, transparent);
}

.novel-shelf__card--active {
  border-color: color-mix(in srgb, var(--dq-accent) 55%, transparent);
  background: color-mix(in srgb, var(--dq-accent) 10%, transparent);
}

.novel-shelf__card-top {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 8px;
}

.novel-shelf__title {
  font-size: var(--dq-font-size-body);
  font-weight: 650;
  line-height: 1.3;
  overflow: hidden;
  display: -webkit-box;
  -webkit-line-clamp: 2;
  -webkit-box-orient: vertical;
}

.novel-shelf__badge {
  flex-shrink: 0;
  padding: 2px 7px;
  border-radius: 4px;
  background: color-mix(in srgb, var(--dq-accent) 18%, transparent);
  color: var(--dq-accent);
  font-size: 11px;
  font-weight: 650;
}

.novel-shelf__meta {
  font-size: var(--dq-font-size-caption);
  opacity: 0.6;
}

.novel-shelf__id {
  font-family: ui-monospace, SFMono-Regular, Menlo, Consolas, monospace;
}

.novel-shelf__progress {
  margin-top: 4px;
  font-size: var(--dq-font-size-caption);
  font-weight: 600;
  opacity: 0.8;
}

.novel-shelf__footer {
  flex-shrink: 0;
  padding: 8px 12px 12px;
  border-top: 1px solid color-mix(in srgb, var(--dq-border-subtle, #000) 40%, transparent);
}
</style>
