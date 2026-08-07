<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { asArray, fetchJSON } from '@/api/client'
import Skeleton from '@/components/common/Skeleton.vue'
import FileTreeNodes, { type FileNode } from '@/components/center/FileTreeNodes.vue'

const props = defineProps<{
  projectId: string
}>()

const emit = defineEmits<{
  selectFile: [path: string]
}>()

const { t } = useI18n()

const rootNodes = ref<FileNode[]>([])
const loading = ref(false)
const expanded = ref<Record<string, boolean>>({})
const selectedPath = ref<string | null>(null)
const childrenCache = ref<Record<string, FileNode[]>>({})

const projectId = computed(() => props.projectId)

watch(
  projectId,
  (id) => {
    if (id) void refresh()
  },
  { immediate: true },
)

async function refresh() {
  if (!projectId.value) return
  loading.value = true
  expanded.value = {}
  childrenCache.value = {}
  selectedPath.value = null
  try {
    rootNodes.value = asArray(await fetchJSON<FileNode[]>(`/projects/${projectId.value}/files`))
  } catch {
    rootNodes.value = []
  } finally {
    loading.value = false
  }
}

async function ensureChildren(dirPath: string) {
  if (childrenCache.value[dirPath]) return
  try {
    childrenCache.value[dirPath] = asArray(
      await fetchJSON<FileNode[]>(
        `/projects/${projectId.value}/files?path=${encodeURIComponent(dirPath)}`,
      ),
    )
  } catch {
    childrenCache.value[dirPath] = []
  }
}

async function toggleDir(dirPath: string) {
  if (expanded.value[dirPath]) {
    expanded.value[dirPath] = false
    return
  }
  expanded.value[dirPath] = true
  await ensureChildren(dirPath)
}

function selectFile(path: string) {
  selectedPath.value = path
  emit('selectFile', path)
}

defineExpose({ refresh })
</script>

<template>
  <div class="file-tree">
    <div class="file-tree__toolbar">
      <button
        type="button"
        class="file-tree__refresh"
        :disabled="loading || !projectId"
        :title="t('common.refresh')"
        @click="refresh"
      >
        {{ t('common.refresh') }}
      </button>
    </div>
    <div v-if="loading" class="file-tree__loading">
      <Skeleton variant="text" width="70%" />
      <Skeleton variant="text" width="55%" />
      <Skeleton variant="text" width="62%" />
    </div>
    <div v-else-if="!rootNodes.length" class="file-tree__empty">
      <p>{{ t('sessions.filesEmpty') }}</p>
    </div>
    <FileTreeNodes
      v-else
      class="file-tree__list"
      :nodes="rootNodes"
      :depth="0"
      :expanded="expanded"
      :children-cache="childrenCache"
      :selected-path="selectedPath"
      @toggle="toggleDir"
      @select="selectFile"
    />
  </div>
</template>

<style scoped>
.file-tree {
  display: flex;
  flex-direction: column;
  height: 100%;
  min-height: 0;
  overflow: hidden;
  font-size: var(--dq-font-size-body);
}

.file-tree__toolbar {
  flex-shrink: 0;
  display: flex;
  justify-content: flex-end;
  padding: 6px 10px 0;
}

.file-tree__refresh {
  margin: 0;
  padding: 2px 8px;
  border: none;
  border-radius: 4px;
  background: transparent;
  color: var(--dq-accent);
  font: inherit;
  font-size: var(--dq-font-size-caption);
  cursor: pointer;
}

.file-tree__refresh:disabled {
  opacity: 0.4;
  cursor: not-allowed;
}

.file-tree__loading,
.file-tree__empty {
  padding: 24px 16px;
  text-align: center;
  color: var(--dq-label-tertiary);
}

.file-tree__loading {
  display: flex;
  flex-direction: column;
  gap: var(--dq-space-sm);
  text-align: left;
}

.file-tree__list {
  flex: 1;
  overflow-y: auto;
}
</style>
