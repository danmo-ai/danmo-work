<script setup lang="ts">
import { computed } from 'vue'

export interface FileNode {
  name: string
  path: string
  isDir: boolean
  size?: number
  children?: FileNode[]
}

const props = defineProps<{
  nodes: FileNode[]
  depth: number
  expanded: Record<string, boolean>
  childrenCache: Record<string, FileNode[]>
  selectedPath: string | null
}>()

const emit = defineEmits<{
  toggle: [path: string]
  select: [path: string]
}>()

defineOptions({ name: 'FileTreeNodes' })

const padStyle = computed(() => ({
  paddingLeft: `${12 + props.depth * 12}px`,
}))

function fileIcon(node: FileNode): string {
  const size = 'width="14" height="14"'
  const svg = (d: string) =>
    `<svg viewBox="0 0 24 24" ${size} fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">${d}</svg>`

  if (node.isDir) {
    return svg(
      '<path d="M22 19a2 2 0 0 1-2 2H4a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2h5l2 3h9a2 2 0 0 1 2 2z"/>',
    )
  }

  const ext = node.name.split('.').pop()?.toLowerCase()
  const icons: Record<string, string> = {
    md: '<path d="M14 2H6a2 2 0 0 0-2 2v16a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V8z"/><polyline points="14 2 14 8 20 8"/><line x1="16" y1="13" x2="8" y2="13"/><line x1="16" y1="17" x2="8" y2="17"/><polyline points="10 9 9 9 8 9"/>',
    txt: '<path d="M14 2H6a2 2 0 0 0-2 2v16a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V8z"/><polyline points="14 2 14 8 20 8"/><line x1="16" y1="13" x2="8" y2="13"/><line x1="16" y1="17" x2="8" y2="17"/>',
    yaml: '<path d="M16 4h2a2 2 0 0 1 2 2v14a2 2 0 0 1-2 2h-2"/><path d="M8 20H6a2 2 0 0 1-2-2V6a2 2 0 0 1 2-2h2"/>',
    yml: '<path d="M16 4h2a2 2 0 0 1 2 2v14a2 2 0 0 1-2 2h-2"/><path d="M8 20H6a2 2 0 0 1-2-2V6a2 2 0 0 1 2-2h2"/>',
    json: '<path d="M16 4h2a2 2 0 0 1 2 2v14a2 2 0 0 1-2 2h-2"/><path d="M8 20H6a2 2 0 0 1-2-2V6a2 2 0 0 1 2-2h2"/>',
  }
  return svg(
    icons[ext ?? ''] ||
      '<path d="M14 2H6a2 2 0 0 0-2 2v16a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V8z"/><polyline points="14 2 14 8 20 8"/>',
  )
}

function formatSize(bytes: number): string {
  if (bytes < 1024) return `${bytes} B`
  if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(1)} KB`
  return `${(bytes / 1024 / 1024).toFixed(1)} MB`
}

function onRow(node: FileNode) {
  if (node.isDir) emit('toggle', node.path)
  else emit('select', node.path)
}
</script>

<template>
  <ul class="file-tree-nodes">
    <li
      v-for="node in nodes"
      :key="node.path"
      class="file-tree-nodes__item"
      :class="{ 'is-dir': node.isDir, 'is-selected': selectedPath === node.path }"
    >
      <div class="file-tree-nodes__row" :style="padStyle" @click="onRow(node)">
        <span v-if="node.isDir" class="file-tree-nodes__arrow">{{ expanded[node.path] ? '▾' : '▸' }}</span>
        <span v-else class="file-tree-nodes__arrow-spacer" />
        <span class="file-tree-nodes__icon" v-html="fileIcon(node)" />
        <span class="file-tree-nodes__name" :title="node.path">{{ node.name }}</span>
        <span v-if="!node.isDir && node.size" class="file-tree-nodes__size">{{ formatSize(node.size) }}</span>
      </div>
      <FileTreeNodes
        v-if="node.isDir && expanded[node.path] && childrenCache[node.path]"
        :nodes="childrenCache[node.path]"
        :depth="depth + 1"
        :expanded="expanded"
        :children-cache="childrenCache"
        :selected-path="selectedPath"
        @toggle="emit('toggle', $event)"
        @select="emit('select', $event)"
      />
    </li>
  </ul>
</template>

<style scoped>
.file-tree-nodes {
  list-style: none;
  margin: 0;
  padding: 0;
}

.file-tree-nodes__item.is-selected > .file-tree-nodes__row {
  background: color-mix(in srgb, var(--dq-accent) 15%, transparent);
  color: var(--dq-label-primary);
}

.file-tree-nodes__row {
  display: flex;
  align-items: center;
  gap: 4px;
  padding-top: 5px;
  padding-right: 16px;
  padding-bottom: 5px;
  cursor: pointer;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
  color: var(--dq-label-primary);
}

.file-tree-nodes__row:hover {
  background: var(--dq-fill-on-glass-hover);
}

.file-tree-nodes__arrow {
  flex-shrink: 0;
  width: 14px;
  font-size: var(--dq-font-size-caption);
  color: var(--dq-label-tertiary);
}

.file-tree-nodes__arrow-spacer {
  width: 14px;
  flex-shrink: 0;
}

.file-tree-nodes__icon {
  flex-shrink: 0;
  width: 18px;
  height: 18px;
  display: flex;
  align-items: center;
  justify-content: center;
}

.file-tree-nodes__icon :deep(svg) {
  pointer-events: none;
}

.file-tree-nodes__name {
  flex: 1;
  overflow: hidden;
  text-overflow: ellipsis;
}

.file-tree-nodes__size {
  flex-shrink: 0;
  font-size: var(--dq-font-size-caption);
  color: var(--dq-label-quaternary);
}
</style>
