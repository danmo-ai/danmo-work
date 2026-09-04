<script setup lang="ts">
import { computed, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import {
  novelBiblePath,
  novelCanonDir,
  novelCastDir,
  parseContractYaml,
  setupDocLabel,
  type NovelChapterEntry,
  type NovelChapterPhase,
  type NovelFileNode,
  isChapterPhasePending,
} from '@/types/novel-workbench'

const props = defineProps<{
  bookId: string
  treeOpen: string[]
  setupOpen: string[]
  treeSel: { kind: 'book' | 'volume' | 'setup' | 'chapter' | 'dossier'; name?: string; n?: number }
  bookOutlineSelected: boolean
  visibleVolumeFiles: NovelFileNode[]
  worldDocs: NovelFileNode[]
  castDocs: NovelFileNode[]
  treeChapters: NovelChapterEntry[]
  chapterPhases: Record<number, NovelChapterPhase>
  contractRaws: Record<number, string>
  continuityFiles: NovelFileNode[]
  reviewFiles: NovelFileNode[]
  nextVolume: number
}>()

const emit = defineEmits<{
  'update:treeOpen': [v: string[]]
  'update:setupOpen': [v: string[]]
  'select-book-outline': []
  'select-volume': [node: NovelFileNode]
  'select-setup': [path: string, name: string]
  'select-chapter': [n: number, pane: 'contract' | 'prose']
  'select-dossier': [path: string, name: string]
  'add-volume': []
}>()

const { t } = useI18n()
const pendingOnly = ref(false)

const SETUP_DOC_KEYS = new Set(['bible', 'world', 'glossary', 'reveal', 'rules', 'platform', 'goldfinger'])

function setupDocTitle(name: string): string {
  const id = setupDocLabel(name)
  if (SETUP_DOC_KEYS.has(id)) return t(`novelWorkbench.setupDoc_${id}`)
  return id
}

function volumeLabel(name: string): string {
  return name.replace(/\.md$/i, '')
}

function phaseLabel(phase: NovelChapterPhase): string {
  const map: Record<NovelChapterPhase, string> = {
    empty: 'phaseEmpty',
    contract_draft: 'phaseContractDraft',
    contract_ready: 'phaseContractReady',
    drafted: 'phaseDrafted',
    review_fail: 'phaseReviewFail',
    review_pass: 'phaseReviewPass',
    committed: 'phaseCommitted',
  }
  return t(`novelWorkbench.${map[phase]}`)
}

function chapterTreeName(entry: NovelChapterEntry): string {
  const title = parseContractYaml(props.contractRaws[entry.chapter] || '').title.trim()
  if (title) return t('novelWorkbench.chapterTitle', { n: entry.chapter, title })
  return t('novelWorkbench.chapterN', { n: entry.chapter })
}

const filteredChapters = computed(() => {
  if (!pendingOnly.value) return props.treeChapters
  return props.treeChapters.filter((e) => {
    const phase = props.chapterPhases[e.chapter] ?? 'empty'
    return isChapterPhasePending(phase)
  })
})

const dossierFiles = computed(() => [
  ...props.continuityFiles.filter((n) => !n.isDir),
  ...props.reviewFiles.filter((n) => !n.isDir),
])

const treeOpenModel = computed({
  get: () => props.treeOpen,
  set: (v) => emit('update:treeOpen', v),
})

const setupOpenModel = computed({
  get: () => props.setupOpen,
  set: (v) => emit('update:setupOpen', v),
})
</script>

<template>
  <aside class="novel-binder">
    <DqCollapse v-model="treeOpenModel">
      <DqCollapseItem name="outline" :title="t('novelWorkbench.folderOutline')">
        <button
          type="button"
          class="novel-binder__item"
          :class="{ 'novel-binder__item--on': treeSel.kind === 'book' }"
          @click="emit('select-book-outline')"
        >
          {{ t('novelWorkbench.bookOutline') }}
        </button>
        <button
          v-for="v in visibleVolumeFiles"
          :key="v.name"
          type="button"
          class="novel-binder__item"
          :class="{ 'novel-binder__item--on': treeSel.kind === 'volume' && treeSel.name === v.name }"
          @click="emit('select-volume', v)"
        >
          {{ volumeLabel(v.name) }}
        </button>
        <button type="button" class="novel-binder__item novel-binder__item--ghost" @click="emit('add-volume')">
          + {{ t('novelWorkbench.actionVolumeOutline', { n: nextVolume }) }}
        </button>
      </DqCollapseItem>

      <DqCollapseItem name="setup" :title="t('novelWorkbench.folderSetup')">
        <button
          type="button"
          class="novel-binder__item"
          :class="{ 'novel-binder__item--on': treeSel.kind === 'setup' && treeSel.name === 'book-bible.md' }"
          @click="emit('select-setup', novelBiblePath(bookId), 'book-bible.md')"
        >
          {{ t('novelWorkbench.setupDoc_bible') }}
        </button>
        <DqCollapse v-model="setupOpenModel" class="novel-binder__sub">
          <DqCollapseItem name="world" :title="t('novelWorkbench.folderSetupWorld')">
            <button
              v-for="f in worldDocs"
              :key="f.name"
              type="button"
              class="novel-binder__item novel-binder__item--nested"
              :class="{ 'novel-binder__item--on': treeSel.kind === 'setup' && treeSel.name === f.name }"
              @click="emit('select-setup', f.path || `${novelCanonDir(bookId)}/${f.name}`, f.name)"
            >
              {{ setupDocTitle(f.name) }}
            </button>
            <p v-if="!worldDocs.length" class="novel-binder__hint">{{ t('novelWorkbench.noCanonYet') }}</p>
          </DqCollapseItem>
          <DqCollapseItem name="cast" :title="t('novelWorkbench.folderSetupCast')">
            <button
              v-for="f in castDocs"
              :key="f.name"
              type="button"
              class="novel-binder__item novel-binder__item--nested"
              :class="{ 'novel-binder__item--on': treeSel.kind === 'setup' && treeSel.name === f.name }"
              @click="emit('select-setup', f.path || `${novelCastDir(bookId)}/${f.name}`, f.name)"
            >
              {{ setupDocTitle(f.name) }}
            </button>
          </DqCollapseItem>
        </DqCollapse>
      </DqCollapseItem>

      <DqCollapseItem name="prose">
        <template #title>
          <span>{{ t('novelWorkbench.folderProse') }}</span>
          <span class="novel-binder__count">{{ treeChapters.length }}</span>
        </template>
        <div v-if="treeChapters.length" class="novel-binder__filter">
          <button
            type="button"
            class="novel-binder__filter-btn"
            :class="{ 'novel-binder__filter-btn--on': pendingOnly }"
            @click="pendingOnly = !pendingOnly"
          >
            {{
              pendingOnly
                ? t('novelWorkbench.showAllChapters', { n: treeChapters.length })
                : t('novelWorkbench.showFocusChapters')
            }}
          </button>
        </div>
        <button
          v-for="entry in filteredChapters"
          :key="entry.chapter"
          type="button"
          class="novel-binder__item novel-binder__item--chapter"
          :class="{
            'novel-binder__item--on': treeSel.kind === 'chapter' && treeSel.n === entry.chapter,
            'novel-binder__item--dim': !entry.prose,
          }"
          @click="emit('select-chapter', entry.chapter, entry.prose ? 'prose' : 'contract')"
        >
          <span class="novel-binder__chapter-name">{{ chapterTreeName(entry) }}</span>
          <span
            class="novel-binder__phase"
            :class="'novel-binder__phase--' + (chapterPhases[entry.chapter] || 'empty')"
          >
            {{ phaseLabel(chapterPhases[entry.chapter] || 'empty') }}
          </span>
        </button>
        <p v-if="!filteredChapters.length" class="novel-binder__hint">
          {{ treeChapters.length ? t('novelWorkbench.noPendingChapters') : t('novelWorkbench.noChapters') }}
        </p>
      </DqCollapseItem>

      <DqCollapseItem name="dossier" :title="t('novelWorkbench.dossier')">
        <button
          v-for="f in dossierFiles"
          :key="f.path || f.name"
          type="button"
          class="novel-binder__item"
          :class="{ 'novel-binder__item--on': treeSel.kind === 'dossier' && treeSel.name === f.name }"
          @click="emit('select-dossier', f.path || f.name, f.name)"
        >
          {{ f.name }}
        </button>
        <p v-if="!dossierFiles.length" class="novel-binder__hint">{{ t('novelWorkbench.noDossierYet') }}</p>
      </DqCollapseItem>
    </DqCollapse>
  </aside>
</template>

<style scoped>
.novel-binder {
  flex: 0 0 240px;
  min-width: 200px;
  max-width: 280px;
  overflow: auto;
  border-right: 1px solid color-mix(in srgb, var(--dq-border-subtle, #000) 50%, transparent);
  padding: 6px 0 16px;
  background: color-mix(in srgb, var(--dq-glass-popover-bg, #fff) 40%, transparent);
}

.novel-binder :deep(.dq-collapse-item__header) {
  padding: 6px 12px;
  font-size: var(--dq-font-size-caption);
  font-weight: 650;
  border-radius: 6px;
}

.novel-binder :deep(.dq-collapse-item__header:hover) {
  background: color-mix(in srgb, var(--dq-accent) 8%, transparent);
}

.novel-binder :deep(.dq-collapse-item__title) {
  display: flex;
  align-items: baseline;
  justify-content: space-between;
  gap: 8px;
}

.novel-binder__sub {
  margin-left: 4px;
}

.novel-binder__sub :deep(.dq-collapse-item__header) {
  padding-left: 28px;
  font-weight: 600;
}

.novel-binder__item {
  display: flex;
  align-items: baseline;
  justify-content: space-between;
  gap: 6px;
  width: 100%;
  margin: 0;
  padding: 6px 12px 6px 28px;
  border: none;
  background: transparent;
  color: inherit;
  font: inherit;
  font-size: var(--dq-font-size-caption);
  text-align: left;
  cursor: pointer;
}

.novel-binder__item:hover {
  background: color-mix(in srgb, var(--dq-accent) 10%, transparent);
}

.novel-binder__item--on {
  background: color-mix(in srgb, var(--dq-accent) 14%, transparent);
  font-weight: 650;
}

.novel-binder__item--ghost {
  opacity: 0.65;
}

.novel-binder__item--nested {
  padding-left: 40px;
}

.novel-binder__item--dim {
  opacity: 0.78;
}

.novel-binder__item--chapter {
  flex-direction: column;
  align-items: stretch;
  gap: 3px;
}

.novel-binder__chapter-name {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.novel-binder__phase {
  align-self: flex-start;
  padding: 1px 6px;
  border-radius: 3px;
  font-size: 10px;
  font-weight: 650;
  background: color-mix(in srgb, var(--dq-border-subtle, #000) 28%, transparent);
  opacity: 0.9;
}

.novel-binder__phase--contract_ready,
.novel-binder__phase--drafted {
  background: color-mix(in srgb, var(--dq-accent) 16%, transparent);
  color: var(--dq-accent);
}

.novel-binder__phase--review_fail {
  background: color-mix(in srgb, var(--dq-danger, #dc2626) 16%, transparent);
  color: var(--dq-danger, #dc2626);
}

.novel-binder__phase--review_pass {
  background: color-mix(in srgb, #ca8a04 18%, transparent);
  color: #a16207;
}

.novel-binder__phase--committed {
  background: color-mix(in srgb, var(--dq-success, #16a34a) 16%, transparent);
  color: var(--dq-success, #16a34a);
}

.novel-binder__count {
  opacity: 0.5;
  font-weight: 400;
}

.novel-binder__hint {
  margin: 0;
  padding: 2px 12px 2px 28px;
  font-size: var(--dq-font-size-caption);
  opacity: 0.55;
  line-height: 1.35;
}

.novel-binder__filter {
  padding: 2px 12px 6px 28px;
}

.novel-binder__filter-btn {
  margin: 0;
  padding: 2px 0;
  border: none;
  background: transparent;
  color: var(--dq-accent);
  font: inherit;
  font-size: 11px;
  cursor: pointer;
}

.novel-binder__filter-btn--on {
  font-weight: 650;
}
</style>
