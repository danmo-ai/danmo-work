<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import type {
  BookOutlineVolumeRow,
  NovelChapterEntry,
  VolumeUnitRow,
} from '@/types/novel-workbench'

const props = defineProps<{
  crumb: string
  readTitle: string
  readHtml: string
  readLoading: boolean
  readContent: string
  treeKind: 'book' | 'volume' | 'setup' | 'chapter' | 'dossier'
  readingIsContract: boolean
  readingIsProse: boolean
  readingEntry: NovelChapterEntry | null
  hasBookOutline: boolean
  bookOutlineRows: BookOutlineVolumeRow[]
  volumeUnits: VolumeUnitRow[]
  prevChapter: NovelChapterEntry | null
  nextChapter: NovelChapterEntry | null
}>()

const emit = defineEmits<{
  'open-contract': []
  'open-prose': []
  'prev-chapter': []
  'next-chapter': []
}>()

const { t } = useI18n()

const emptyMessage = computed(() => {
  if (props.treeKind === 'book' && !props.hasBookOutline) return t('novelWorkbench.noBookOutline')
  if (props.treeKind === 'volume' && !props.readContent) return t('novelWorkbench.volumeUnitsEmpty')
  if (props.readingIsProse && !props.readingEntry?.prose) return t('novelWorkbench.noProseYet')
  if (props.readingIsContract && !props.readingEntry?.contract) return t('novelWorkbench.noContractYet')
  return ''
})

const showUnits =
  computed(() => props.treeKind === 'book' && props.bookOutlineRows.length > 0)
const showVolumeUnits =
  computed(() => props.treeKind === 'volume' && props.volumeUnits.length > 0)
</script>

<template>
  <section class="novel-reader">
    <div class="novel-reader__bar">
      <div class="novel-reader__head">
        <div class="novel-reader__crumb">{{ crumb }}</div>
        <div class="novel-reader__file">{{ readTitle }}</div>
      </div>
      <div
        v-if="treeKind === 'chapter'"
        class="novel-reader__nav"
        :aria-label="t('novelWorkbench.chapterNav')"
      >
        <button
          type="button"
          class="novel-wb-link"
          :disabled="!prevChapter"
          :title="prevChapter ? t('novelWorkbench.prevChapter', { n: prevChapter.chapter }) : t('novelWorkbench.prevChapterNone')"
          @click="emit('prev-chapter')"
        >
          ← {{ prevChapter ? t('novelWorkbench.chapterN', { n: prevChapter.chapter }) : t('novelWorkbench.prevChapterNone') }}
        </button>
        <button
          type="button"
          class="novel-wb-link"
          :disabled="!nextChapter"
          :title="nextChapter ? t('novelWorkbench.nextChapter', { n: nextChapter.chapter }) : t('novelWorkbench.nextChapterNone')"
          @click="emit('next-chapter')"
        >
          {{ nextChapter ? t('novelWorkbench.chapterN', { n: nextChapter.chapter }) : t('novelWorkbench.nextChapterNone') }} →
        </button>
      </div>
    </div>

    <div v-if="treeKind === 'chapter'" class="novel-reader__tabs" role="tablist">
      <button
        type="button"
        role="tab"
        class="novel-reader__tab"
        :class="{
          'novel-reader__tab--active': readingIsContract,
          'novel-reader__tab--missing': !readingEntry?.contract,
        }"
        :aria-selected="readingIsContract"
        @click="emit('open-contract')"
      >
        {{ t('novelWorkbench.badgeContract') }}
      </button>
      <button
        type="button"
        role="tab"
        class="novel-reader__tab"
        :class="{
          'novel-reader__tab--active': readingIsProse,
          'novel-reader__tab--missing': !readingEntry?.prose,
        }"
        :aria-selected="readingIsProse"
        @click="emit('open-prose')"
      >
        {{ t('novelWorkbench.badgeProse') }}
      </button>
    </div>

    <div v-if="showUnits" class="novel-reader__summary">
      <div class="novel-reader__summary-title">{{ t('novelWorkbench.volumeSummary') }}</div>
      <ul class="novel-reader__summary-list">
        <li v-for="(row, i) in bookOutlineRows" :key="i">
          <strong>{{ row.vol }}</strong>
          <span v-if="row.goal"> — {{ row.goal }}</span>
        </li>
      </ul>
    </div>

    <div v-if="showVolumeUnits" class="novel-reader__summary">
      <div class="novel-reader__summary-title">{{ t('novelWorkbench.unitSummary') }}</div>
      <ul class="novel-reader__summary-list">
        <li v-for="(u, i) in volumeUnits" :key="i">
          <strong>{{ u.id || u.range }}</strong>
          <span v-if="u.range && u.id"> · {{ u.range }}</span>
          <span v-if="u.purpose"> — {{ u.purpose }}</span>
        </li>
      </ul>
    </div>

    <div v-if="readLoading" class="novel-reader__empty">{{ t('novelWorkbench.loading') }}</div>
    <div v-else-if="emptyMessage" class="novel-reader__empty">{{ emptyMessage }}</div>
    <div v-else class="novel-reader__body" v-html="readHtml" />
  </section>
</template>

<style scoped>
.novel-reader {
  flex: 1;
  min-width: 0;
  display: flex;
  flex-direction: column;
  overflow: hidden;
  background: color-mix(in srgb, var(--dq-glass-popover-bg, #fff) 88%, transparent);
}

.novel-reader__bar {
  flex-shrink: 0;
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 10px;
  padding: 10px 14px 6px;
}

.novel-reader__head {
  min-width: 0;
}

.novel-reader__crumb {
  font-size: var(--dq-font-size-caption);
  font-weight: 650;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.novel-reader__file {
  margin-top: 2px;
  font-size: 11px;
  opacity: 0.5;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.novel-reader__nav {
  display: flex;
  flex-shrink: 0;
  gap: 4px;
}

.novel-reader__tabs {
  flex-shrink: 0;
  display: flex;
  gap: 4px;
  padding: 0 14px 8px;
}

.novel-reader__tab {
  margin: 0;
  padding: 5px 12px;
  border: 1px solid color-mix(in srgb, var(--dq-border-subtle, #000) 45%, transparent);
  border-radius: 6px;
  background: transparent;
  color: inherit;
  font: inherit;
  font-size: var(--dq-font-size-caption);
  cursor: pointer;
}

.novel-reader__tab--active {
  border-color: color-mix(in srgb, var(--dq-accent) 50%, transparent);
  background: color-mix(in srgb, var(--dq-accent) 12%, transparent);
  font-weight: 650;
}

.novel-reader__tab--missing {
  opacity: 0.55;
}

.novel-reader__summary {
  flex-shrink: 0;
  margin: 0 14px 8px;
  padding: 8px 10px;
  border-radius: 8px;
  background: color-mix(in srgb, var(--dq-accent) 6%, transparent);
  border: 1px solid color-mix(in srgb, var(--dq-accent) 12%, transparent);
}

.novel-reader__summary-title {
  font-size: 11px;
  font-weight: 650;
  opacity: 0.7;
  margin-bottom: 4px;
}

.novel-reader__summary-list {
  margin: 0;
  padding-left: 16px;
  font-size: var(--dq-font-size-caption);
  line-height: 1.45;
  opacity: 0.9;
}

.novel-reader__empty {
  padding: 24px 16px;
  font-size: var(--dq-font-size-body);
  opacity: 0.7;
  line-height: 1.45;
}

.novel-reader__body {
  flex: 1;
  min-height: 0;
  overflow: auto;
  padding: 4px 18px 28px;
  max-width: 42rem;
  width: 100%;
  margin: 0 auto;
  font-size: 15px;
  line-height: 1.7;
}

.novel-reader__body :deep(h1),
.novel-reader__body :deep(h2),
.novel-reader__body :deep(h3) {
  line-height: 1.3;
  margin: 1.1em 0 0.45em;
}

.novel-reader__body :deep(p) {
  margin: 0.55em 0;
}

.novel-reader__body :deep(pre),
.novel-reader__body :deep(.novel-wb__pre) {
  overflow: auto;
  padding: 10px 12px;
  border-radius: 8px;
  background: color-mix(in srgb, var(--dq-border-subtle, #000) 12%, transparent);
  font-size: 12.5px;
  line-height: 1.45;
}
</style>
