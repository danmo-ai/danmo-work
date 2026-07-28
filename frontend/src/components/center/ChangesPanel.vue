<script setup lang="ts">
import { ref, watch, computed, onMounted } from 'vue'
import { useSessionsStore } from '@/stores/sessions'
import { useWorkspaceUiStore } from '@/stores/workspaceUi'
import { fetchJSON } from '@/api/client'
import { toast } from '@/utils/feedback'
import Skeleton from '@/components/common/Skeleton.vue'

interface GitFileChange {
  status: string
  file: string
  origFile?: string
  staged: boolean
}

interface GitChanges {
  branch: string
  changes: GitFileChange[]
  error?: string
  code?: string
}

interface GitBranches {
  current: string
  branches: string[] | null
  error?: string
  code?: string
}

const sessions = useSessionsStore()
const workspaceUi = useWorkspaceUiStore()
const data = ref<GitChanges | null>(null)
const loading = ref(false)
const branches = ref<GitBranches | null>(null)
const selectedBranch = ref('')
const switching = ref(false)
const filterQuery = ref('')

async function loadChanges() {
  if (!sessions.selectedProjectId) {
    data.value = null
    return
  }
  loading.value = true
  try {
    data.value = await fetchJSON<GitChanges>(`/projects/${sessions.selectedProjectId}/git-changes`)
  } catch {
    data.value = { branch: '', changes: [], error: '加载失败' }
  } finally {
    loading.value = false
  }
}

async function loadBranches() {
  if (!sessions.selectedProjectId) {
    branches.value = null
    selectedBranch.value = ''
    return
  }
  try {
    branches.value = await fetchJSON<GitBranches>(`/projects/${sessions.selectedProjectId}/git-branches`)
    selectedBranch.value = branches.value?.current ?? ''
  } catch {
    branches.value = null
    selectedBranch.value = ''
  }
}

async function onSelectBranch(branch: unknown) {
  const target = typeof branch === 'string' ? branch : ''
  if (!sessions.selectedProjectId || !target || target === branches.value?.current) return
  switching.value = true
  try {
    branches.value = await fetchJSON<GitBranches>(`/projects/${sessions.selectedProjectId}/git-checkout`, {
      method: 'POST',
      body: JSON.stringify({ branch: target }),
    })
    selectedBranch.value = branches.value?.current ?? target
    toast.success(`已切换到分支 ${selectedBranch.value}`)
    await loadChanges()
  } catch (e) {
    toast.error(e instanceof Error ? e.message : '切换分支失败')
    selectedBranch.value = branches.value?.current ?? ''
  } finally {
    switching.value = false
  }
}

function refresh() {
  loadChanges()
  loadBranches()
}

watch(() => sessions.selectedProjectId, refresh)
onMounted(refresh)

function statusLabel(s: string): string {
  switch (s) {
    case 'M': return 'M'
    case 'A': return 'A'
    case 'D': return 'D'
    case 'R': return 'R'
    case 'C': return 'C'
    case '??': return '?'
    default: return s
  }
}

function statusType(s: string): string {
  switch (s) {
    case 'M': return 'modified'
    case 'A': return 'added'
    case 'D': return 'deleted'
    case 'R': return 'renamed'
    case 'C': return 'copied'
    case '??': return 'untracked'
    default: return 'modified'
  }
}

function matchesFilter(c: GitFileChange): boolean {
  const q = filterQuery.value.trim().toLowerCase()
  if (!q) return true
  return c.file.toLowerCase().includes(q) || (c.origFile?.toLowerCase().includes(q) ?? false)
}

const stagedChanges = computed(() => data.value?.changes?.filter(c => c.staged && matchesFilter(c)) ?? [])
const unstagedChanges = computed(() => data.value?.changes?.filter(c => !c.staged && matchesFilter(c)) ?? [])
const branchList = computed(() => branches.value?.branches ?? [])
const totalCount = computed(() => data.value?.changes?.length ?? 0)

function changeLabel(c: GitFileChange): string {
  if (c.origFile) return `${c.file} ← ${c.origFile}`
  return c.file
}

function openDiff(c: GitFileChange) {
  workspaceUi.openStage({
    kind: 'diff',
    path: c.file,
    mode: 'view',
    staged: c.staged,
  })
}

function copyPath(file: string, e: Event) {
  e.stopPropagation()
  void navigator.clipboard?.writeText(file).catch(() => {})
  toast.success('已复制路径')
}

defineExpose({ refresh, totalCount })
</script>

<template>
  <aside class="changes-panel">
    <div v-if="!sessions.selectedProjectId" class="changes-panel__empty">
      <p>未关联项目</p>
    </div>

    <template v-else>
      <div v-if="branchList.length" class="changes-panel__branch">
        <span class="changes-panel__branch-icon">⎇</span>
        <DqSelect
          v-model="selectedBranch"
          class="changes-panel__branch-select"
          :disabled="switching"
          @update:model-value="onSelectBranch"
        >
          <DqOption v-for="b in branchList" :key="b" :value="b" :label="b" />
        </DqSelect>
        <span v-if="switching" class="changes-panel__branch-switching">切换中...</span>
      </div>

      <div v-if="loading" class="changes-panel__empty changes-panel__loading">
        <Skeleton variant="text" width="50%" />
        <Skeleton variant="card" width="100%" />
        <Skeleton variant="card" width="100%" />
      </div>

      <div v-else-if="data?.error || data?.code" class="changes-panel__empty">
        <p>{{ data.code === 'git_missing' ? $t('composer.gitMissing') : (data.error || $t('composer.gitUnavailable')) }}</p>
      </div>

      <div v-else-if="!data?.changes?.length" class="changes-panel__empty">
        <p>{{ $t('sessions.noChanges') }}</p>
        <p class="changes-panel__hint">{{ $t('sessions.noChangesHint') }}</p>
      </div>

      <div v-else class="changes-panel__list">
        <div class="changes-panel__filter">
          <DqInput v-model="filterQuery" size="sm" :placeholder="$t('sessions.filterChanges')" />
        </div>
        <template v-if="stagedChanges.length">
          <div class="changes-panel__group-label">{{ $t('sessions.changesStaged') }}</div>
          <button
            v-for="c in stagedChanges"
            :key="'s-' + c.file"
            type="button"
            class="changes-panel__item"
            :class="`is-${statusType(c.status)}`"
            @click="openDiff(c)"
          >
            <span class="changes-panel__item-status">{{ statusLabel(c.status) }}</span>
            <span class="changes-panel__item-file" :title="changeLabel(c)">{{ changeLabel(c) }}</span>
            <span
              class="changes-panel__item-copy"
              :title="$t('sessions.copyPath')"
              @click="copyPath(c.file, $event)"
            >⎘</span>
          </button>
        </template>

        <template v-if="unstagedChanges.length">
          <div class="changes-panel__group-label">{{ $t('sessions.changesUnstaged') }}</div>
          <button
            v-for="c in unstagedChanges"
            :key="'u-' + c.file"
            type="button"
            class="changes-panel__item"
            :class="`is-${statusType(c.status)}`"
            @click="openDiff(c)"
          >
            <span class="changes-panel__item-status">{{ statusLabel(c.status) }}</span>
            <span class="changes-panel__item-file" :title="changeLabel(c)">{{ changeLabel(c) }}</span>
            <span
              class="changes-panel__item-copy"
              :title="$t('sessions.copyPath')"
              @click="copyPath(c.file, $event)"
            >⎘</span>
          </button>
        </template>
      </div>
    </template>
  </aside>
</template>

<style scoped>
.changes-panel {
  display: flex;
  flex-direction: column;
  height: 100%;
  min-height: 0;
  overflow: hidden;
  background: var(--work-glass-bg);
  font-size: var(--dq-font-size-body);
}

.changes-panel__empty {
  padding: 32px 16px;
  text-align: center;
  color: var(--dq-label-tertiary);
}

.changes-panel__loading {
  display: flex;
  flex-direction: column;
  gap: var(--dq-space-sm);
  text-align: left;
}

.changes-panel__hint {
  margin-top: 8px;
  font-size: var(--dq-font-size-caption);
  color: var(--dq-label-quaternary);
  line-height: 1.5;
}

.changes-panel__list {
  flex: 1;
  overflow-y: auto;
  padding: 8px 0;
}

.changes-panel__filter {
  padding: 4px 12px 8px;
}

.changes-panel__item {
  width: 100%;
  border: none;
  background: transparent;
  text-align: left;
  cursor: pointer;
  font: inherit;
}

.changes-panel__branch {
  flex-shrink: 0;
  display: flex;
  align-items: center;
  gap: 6px;
  padding: 8px 14px;
  font-size: var(--dq-font-size-footnote);
  font-weight: 600;
  color: var(--dq-label-primary);
  border-bottom: 1px solid var(--dq-separator-light);
}

.changes-panel__branch-icon {
  font-size: var(--dq-font-size-secondary);
  color: var(--dq-accent);
}

.changes-panel__branch-select {
  flex: 1;
  min-width: 0;
  font-family: var(--dq-font-mono);
}

.changes-panel__branch-switching {
  flex-shrink: 0;
  font-size: var(--dq-font-size-caption);
  font-weight: 400;
  color: var(--dq-label-tertiary);
}

.changes-panel__group-label {
  padding: 8px 14px 4px;
  font-size: var(--dq-font-size-caption);
  font-weight: 600;
  letter-spacing: 0.06em;
  text-transform: uppercase;
  color: var(--dq-label-quaternary);
}

.changes-panel__item {
  display: flex;
  align-items: flex-start;
  gap: 8px;
  padding: 6px 14px;
  line-height: 1.45;
  color: var(--dq-label-primary);
}

.changes-panel__item:hover {
  background: color-mix(in srgb, var(--dq-label-primary) 4%, transparent);
}

.changes-panel__item-status {
  flex-shrink: 0;
  width: 18px;
  height: 18px;
  display: flex;
  align-items: center;
  justify-content: center;
  border-radius: 4px;
  font-size: var(--dq-font-size-caption);
  font-weight: 700;
  font-family: var(--dq-font-mono);
  color: var(--dq-color-white);
  background: color-mix(in srgb, var(--dq-label-secondary) 60%, transparent);
}

.changes-panel__item.is-modified .changes-panel__item-status {
  background: var(--dq-system-orange);
}

.changes-panel__item.is-added .changes-panel__item-status {
  background: var(--dq-success);
}

.changes-panel__item.is-deleted .changes-panel__item-status {
  background: var(--dq-danger);
}

.changes-panel__item.is-renamed .changes-panel__item-status,
.changes-panel__item.is-copied .changes-panel__item-status {
  background: var(--dq-accent);
}

.changes-panel__item.is-untracked .changes-panel__item-status {
  background: color-mix(in srgb, var(--dq-label-tertiary) 80%, transparent);
}

.changes-panel__item-file {
  flex: 1;
  min-width: 0;
  word-break: break-word;
  font-size: var(--dq-font-size-footnote);
  font-family: var(--dq-font-mono);
}

.changes-panel__item-copy {
  flex-shrink: 0;
  opacity: 0;
  font-size: 12px;
  color: var(--dq-label-tertiary);
  padding: 0 2px;
  line-height: 1.45;
}

.changes-panel__item:hover .changes-panel__item-copy {
  opacity: 1;
}

.changes-panel__item-copy:hover {
  color: var(--dq-accent);
}
</style>
