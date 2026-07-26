<script setup lang="ts">
import { ref, watch, computed } from 'vue'
import { useSessionsStore } from '@/stores/sessions'
import { fetchJSON, asArray } from '@/api/client'
import Skeleton from '@/components/common/Skeleton.vue'

const sessions = useSessionsStore()

interface FileNode {
  name: string
  path: string
  isDir: boolean
  size?: number
  children?: FileNode[]
}

const props = defineProps<{
  projectId: string
}>()

const emit = defineEmits<{
  selectFile: [path: string]
}>()

const rootNodes = ref<FileNode[]>([])
const loading = ref(false)
const expanded = ref<Record<string, boolean>>({})
const selectedPath = ref<string | null>(null)
const childrenCache = ref<Record<string, FileNode[]>>({})

const projectId = computed(() => props.projectId)

watch(projectId, (id) => {
  if (id) loadRoot()
}, { immediate: true })

async function loadRoot() {
  if (!projectId.value) return
  loading.value = true
  try {
    rootNodes.value = asArray(await fetchJSON<FileNode[]>(`/projects/${projectId.value}/files`))
  } catch {
    rootNodes.value = []
  } finally {
    loading.value = false
  }
}

async function toggleDir(dirPath: string) {
  if (expanded.value[dirPath]) {
    expanded.value[dirPath] = false
    return
  }
  expanded.value[dirPath] = true
  if (!childrenCache.value[dirPath]) {
    try {
      childrenCache.value[dirPath] = asArray(
        await fetchJSON<FileNode[]>(`/projects/${projectId.value}/files?path=${encodeURIComponent(dirPath)}`),
      )
    } catch {
      childrenCache.value[dirPath] = []
    }
  }
}

function selectFile(path: string) {
  selectedPath.value = path
  emit('selectFile', path)
}

function fileIcon(node: FileNode): string {
  const size = 'width="14" height="14"'
  const svg = (d: string) => `<svg viewBox="0 0 24 24" ${size} fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">${d}</svg>`

  // Directory
  if (node.isDir) return svg('<path d="M22 19a2 2 0 0 1-2 2H4a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2h5l2 3h9a2 2 0 0 1 2 2z"/>')

  const ext = node.name.split('.').pop()?.toLowerCase()

  // Svg body by category
  const icons: Record<string, string> = {
    // Web
    html: '<polyline points="16 18 22 12 16 6"/><polyline points="8 6 2 12 8 18"/>',
    htm: '<polyline points="16 18 22 12 16 6"/><polyline points="8 6 2 12 8 18"/>',
    css: '<path d="M12 2l3.09 6.26L22 9.27l-5 4.87L18.18 21 12 17.77 5.82 21 7 14.14 2 9.27l6.91-1.01L12 2z"/>',
    scss: '<path d="M12 2l3.09 6.26L22 9.27l-5 4.87L18.18 21 12 17.77 5.82 21 7 14.14 2 9.27l6.91-1.01L12 2z"/>',
    less: '<path d="M12 2l3.09 6.26L22 9.27l-5 4.87L18.18 21 12 17.77 5.82 21 7 14.14 2 9.27l6.91-1.01L12 2z"/>',
    // Scripts
    js: '<path d="M14.7 6.3a1 1 0 0 0 0 1.4l1.6 1.6a1 1 0 0 0 1.4 0l3.77-3.77a6 6 0 0 1-7.94 7.94l-6.91 6.91a2.12 2.12 0 0 1-3-3l6.91-6.91a6 6 0 0 1 7.94-7.94l-3.76 3.76z"/>',
    ts: '<rect x="3" y="3" width="18" height="18" rx="2" ry="2"/><line x1="9" y1="9" x2="15" y2="9"/><line x1="9" y1="13" x2="15" y2="13"/><line x1="9" y1="17" x2="12" y2="17"/>',
    jsx: '<path d="M14.7 6.3a1 1 0 0 0 0 1.4l1.6 1.6a1 1 0 0 0 1.4 0l3.77-3.77a6 6 0 0 1-7.94 7.94l-6.91 6.91a2.12 2.12 0 0 1-3-3l6.91-6.91a6 6 0 0 1 7.94-7.94l-3.76 3.76z"/>',
    tsx: '<rect x="3" y="3" width="18" height="18" rx="2" ry="2"/><line x1="9" y1="9" x2="15" y2="9"/><line x1="9" y1="13" x2="15" y2="13"/><line x1="9" y1="17" x2="12" y2="17"/>',
    vue: '<path d="M12 2l-9 18h18l-9-18z"/><path d="M12 2l3 6h-6l3-6z"/>',
    py: '<path d="M12 2a10 10 0 1 0 10 10 4 4 0 0 1-5-5 4 4 0 0 1-5-5"/><path d="M8.5 8.5v.01"/><path d="M15.5 15.5v.01"/>',
    go: '<circle cx="12" cy="12" r="10"/><line x1="2" y1="12" x2="22" y2="12"/><path d="M12 2a15.3 15.3 0 0 1 4 10 15.3 15.3 0 0 1-4 10 15.3 15.3 0 0 1-4-10 15.3 15.3 0 0 1 4-10z"/>',
    rs: '<circle cx="12" cy="12" r="10"/><path d="M10 8h4a2 2 0 0 1 2 2v0a2 2 0 0 1-2 2"/><path d="M10 16h4a2 2 0 0 1 2 2v0a2 2 0 0 1-2 2h-4"/><line x1="10" y1="8" x2="10" y2="22"/>',
    // Data
    json: '<path d="M16 4h2a2 2 0 0 1 2 2v14a2 2 0 0 1-2 2h-2"/><path d="M8 20H6a2 2 0 0 1-2-2V6a2 2 0 0 1 2-2h2"/>',
    yaml: '<path d="M16 4h2a2 2 0 0 1 2 2v14a2 2 0 0 1-2 2h-2"/><path d="M8 20H6a2 2 0 0 1-2-2V6a2 2 0 0 1 2-2h2"/>',
    yml: '<path d="M16 4h2a2 2 0 0 1 2 2v14a2 2 0 0 1-2 2h-2"/><path d="M8 20H6a2 2 0 0 1-2-2V6a2 2 0 0 1 2-2h2"/>',
    toml: '<path d="M16 4h2a2 2 0 0 1 2 2v14a2 2 0 0 1-2 2h-2"/><path d="M8 20H6a2 2 0 0 1-2-2V6a2 2 0 0 1 2-2h2"/>',
    sql: '<ellipse cx="12" cy="5" rx="9" ry="3"/><path d="M21 12c0 1.66-4 3-9 3s-9-1.34-9-3"/><path d="M3 5v14c0 1.66 4 3 9 3s9-1.34 9-3V5"/>',
    // Docs
    md: '<path d="M14 2H6a2 2 0 0 0-2 2v16a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V8z"/><polyline points="14 2 14 8 20 8"/><line x1="16" y1="13" x2="8" y2="13"/><line x1="16" y1="17" x2="8" y2="17"/><polyline points="10 9 9 9 8 9"/>',
    txt: '<path d="M14 2H6a2 2 0 0 0-2 2v16a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V8z"/><polyline points="14 2 14 8 20 8"/><line x1="16" y1="13" x2="8" y2="13"/><line x1="16" y1="17" x2="8" y2="17"/>',
    // Images
    svg: '<rect x="3" y="3" width="18" height="18" rx="2" ry="2"/><circle cx="8.5" cy="8.5" r="1.5"/><polyline points="21 15 16 10 5 21"/>',
    png: '<rect x="3" y="3" width="18" height="18" rx="2" ry="2"/><circle cx="8.5" cy="8.5" r="1.5"/><polyline points="21 15 16 10 5 21"/>',
    jpg: '<rect x="3" y="3" width="18" height="18" rx="2" ry="2"/><circle cx="8.5" cy="8.5" r="1.5"/><polyline points="21 15 16 10 5 21"/>',
    jpeg: '<rect x="3" y="3" width="18" height="18" rx="2" ry="2"/><circle cx="8.5" cy="8.5" r="1.5"/><polyline points="21 15 16 10 5 21"/>',
    gif: '<rect x="3" y="3" width="18" height="18" rx="2" ry="2"/><circle cx="8.5" cy="8.5" r="1.5"/><polyline points="21 15 16 10 5 21"/>',
    ico: '<rect x="3" y="3" width="18" height="18" rx="2" ry="2"/><circle cx="8.5" cy="8.5" r="1.5"/><polyline points="21 15 16 10 5 21"/>',
    webp: '<rect x="3" y="3" width="18" height="18" rx="2" ry="2"/><circle cx="8.5" cy="8.5" r="1.5"/><polyline points="21 15 16 10 5 21"/>',
    // Shell
    sh: '<polyline points="4 17 10 11 4 5"/><line x1="12" y1="19" x2="20" y2="19"/>',
    bash: '<polyline points="4 17 10 11 4 5"/><line x1="12" y1="19" x2="20" y2="19"/>',
    zsh: '<polyline points="4 17 10 11 4 5"/><line x1="12" y1="19" x2="20" y2="19"/>',
    // Config
    lock: '<rect x="3" y="11" width="18" height="11" rx="2" ry="2"/><path d="M7 11V7a5 5 0 0 1 10 0v4"/>',
    gitignore: '<circle cx="12" cy="12" r="10"/><line x1="15" y1="9" x2="9" y2="15"/><line x1="9" y1="9" x2="15" y2="15"/>',
  }

  // Default: file icon
  return svg(icons[ext ?? ''] || '<path d="M14 2H6a2 2 0 0 0-2 2v16a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V8z"/><polyline points="14 2 14 8 20 8"/>')
}

function formatSize(bytes: number): string {
  if (bytes < 1024) return `${bytes} B`
  if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(1)} KB`
  return `${(bytes / 1024 / 1024).toFixed(1)} MB`
}

defineExpose({ refresh: loadRoot })
</script>

<template>
  <div class="file-tree">
    <div v-if="loading" class="file-tree__loading">
      <Skeleton variant="text" width="70%" />
      <Skeleton variant="text" width="55%" />
      <Skeleton variant="text" width="62%" />
    </div>
    <div v-else-if="!rootNodes.length" class="file-tree__empty">
      <p>暂无文件</p>
    </div>
    <ul v-else class="file-tree__list">
      <template v-for="node in rootNodes" :key="node.path">
        <li class="file-tree__item" :class="{ 'is-dir': node.isDir, 'is-selected': selectedPath === node.path }">
          <div
            class="file-tree__row"
            @click="node.isDir ? toggleDir(node.path) : selectFile(node.path)"
          >
            <span v-if="node.isDir" class="file-tree__arrow">{{ expanded[node.path] ? '▾' : '▸' }}</span>
            <span v-else class="file-tree__arrow-spacer" />
            <span class="file-tree__icon" v-html="fileIcon(node)" />
            <span class="file-tree__name" :title="node.path">{{ node.name }}</span>
            <span v-if="!node.isDir && node.size" class="file-tree__size">{{ formatSize(node.size) }}</span>
          </div>
          <ul v-if="node.isDir && expanded[node.path] && childrenCache[node.path]" class="file-tree__children">
            <li
              v-for="child in childrenCache[node.path]"
              :key="child.path"
              class="file-tree__item"
              :class="{ 'is-dir': child.isDir, 'is-selected': selectedPath === child.path }"
            >
              <div
                class="file-tree__row file-tree__row--child"
                @click="child.isDir ? toggleDir(child.path) : selectFile(child.path)"
              >
                <span v-if="child.isDir" class="file-tree__arrow">{{ expanded[child.path] ? '▾' : '▸' }}</span>
                <span v-else class="file-tree__arrow-spacer" />
                <span class="file-tree__icon" v-html="fileIcon(child)" />
                <span class="file-tree__name" :title="child.path">{{ child.name }}</span>
                <span v-if="!child.isDir && child.size" class="file-tree__size">{{ formatSize(child.size) }}</span>
              </div>
            </li>
          </ul>
        </li>
      </template>
    </ul>
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
  list-style: none;
  margin: 0;
  padding: 4px 0;
}

.file-tree__children {
  list-style: none;
  margin: 0;
  padding: 0 0 0 12px;
}

.file-tree__item.is-selected > .file-tree__row {
  background: color-mix(in srgb, var(--dq-accent) 15%, transparent);
  color: var(--dq-label-primary);
}

.file-tree__row {
  display: flex;
  align-items: center;
  gap: 4px;
  padding: 5px 16px;
  cursor: pointer;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
  color: var(--dq-label-primary);
}

.file-tree__row:hover {
  background: var(--dq-fill-on-glass-hover);
}

.file-tree__arrow {
  flex-shrink: 0;
  width: 14px;
  font-size: var(--dq-font-size-caption);
  color: var(--dq-label-tertiary);
}

.file-tree__arrow-spacer {
  width: 14px;
  flex-shrink: 0;
}

.file-tree__icon {
  flex-shrink: 0;
  width: 18px;
  height: 18px;
  display: flex;
  align-items: center;
  justify-content: center;
}

.file-tree__icon :deep(svg) {
  pointer-events: none;
}

.file-tree__name {
  flex: 1;
  overflow: hidden;
  text-overflow: ellipsis;
}

.file-tree__size {
  flex-shrink: 0;
  font-size: var(--dq-font-size-caption);
  color: var(--dq-label-quaternary);
}

</style>
