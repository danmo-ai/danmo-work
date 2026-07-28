<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { fetchJSON } from '@/api/client'
import { toast } from '@/utils/feedback'
import { routeOfficeFile } from '@/utils/office-route'
import { useWorkspaceUiStore } from '@/stores/workspaceUi'
import { apiBaseUrl } from '@/utils/desktop'

interface GitDiffResult {
  path: string
  staged: boolean
  patch: string
  truncated?: boolean
  binary?: boolean
  untracked?: boolean
  error?: string
  code?: string
}

interface DiffLine {
  type: 'meta' | 'hunk' | 'add' | 'del' | 'ctx' | 'empty'
  text: string
}

const props = defineProps<{
  projectId: string
  path: string
  staged?: boolean
  reloadToken: number
}>()

const { t } = useI18n()
const workspaceUi = useWorkspaceUiStore()
const loading = ref(false)
const data = ref<GitDiffResult | null>(null)

const lines = computed<DiffLine[]>(() => {
  const patch = data.value?.patch || ''
  if (!patch) return []
  return patch.split('\n').map((text) => {
    if (text.startsWith('+++') || text.startsWith('---') || text.startsWith('diff ') || text.startsWith('index ') || text.startsWith('new file') || text.startsWith('deleted file') || text.startsWith('old mode') || text.startsWith('new mode') || text.startsWith('similarity ') || text.startsWith('rename ')) {
      return { type: 'meta' as const, text }
    }
    if (text.startsWith('@@')) return { type: 'hunk' as const, text }
    if (text.startsWith('+')) return { type: 'add' as const, text }
    if (text.startsWith('-')) return { type: 'del' as const, text }
    if (text === '') return { type: 'empty' as const, text: ' ' }
    return { type: 'ctx' as const, text }
  })
})

async function load() {
  if (!props.projectId || !props.path) return
  loading.value = true
  try {
    const q = new URLSearchParams({
      path: props.path,
      staged: props.staged ? '1' : '0',
    })
    data.value = await fetchJSON<GitDiffResult>(
      `/projects/${props.projectId}/git-diff?${q.toString()}`,
    )
  } catch (e) {
    data.value = {
      path: props.path,
      staged: !!props.staged,
      patch: '',
      error: e instanceof Error ? e.message : t('office.diffLoadFailed'),
    }
  } finally {
    loading.value = false
  }
}

function openFile() {
  if (!props.projectId || !props.path) return
  if (data.value?.code === 'not_found') {
    toast.warning(t('office.diffFileMissing'))
    return
  }
  const routed = routeOfficeFile(props.path)
  if (routed.kind === 'preview') {
    const url = `${apiBaseUrl()}/api/v1/projects/${props.projectId}/raw/${encodeURIComponent(props.path)}`
    workspaceUi.openStage({ ...routed, url })
    return
  }
  workspaceUi.openStage(routed)
}

watch(
  () => [props.projectId, props.path, props.staged, props.reloadToken] as const,
  () => {
    void load()
  },
  { immediate: true },
)
</script>

<template>
  <div class="diff-surface">
    <div class="diff-surface__toolbar">
      <span class="diff-surface__badge">{{ staged ? t('office.diffStaged') : t('office.diffUnstaged') }}</span>
      <span v-if="data?.untracked" class="diff-surface__badge diff-surface__badge--warn">{{
        t('office.diffUntracked')
      }}</span>
      <span v-if="data?.binary" class="diff-surface__badge">{{ t('office.diffBinary') }}</span>
      <span v-if="data?.truncated" class="diff-surface__badge diff-surface__badge--warn">{{
        t('office.diffTruncated')
      }}</span>
      <span class="diff-surface__spacer" />
      <button type="button" class="diff-surface__btn" @click="openFile">
        {{ t('office.diffOpenFile') }}
      </button>
    </div>

    <div v-if="loading" class="diff-surface__status">{{ t('office.loading') }}</div>
    <div v-else-if="data?.error || data?.code" class="diff-surface__status">
      {{ data.code === 'git_missing' ? t('composer.gitMissing') : data.error || t('office.diffLoadFailed') }}
    </div>
    <div v-else-if="!lines.length" class="diff-surface__status">{{ t('office.diffEmpty') }}</div>
    <pre v-else class="diff-surface__patch"><code><span
      v-for="(line, i) in lines"
      :key="i"
      class="diff-surface__line"
      :class="`is-${line.type}`"
    >{{ line.text }}
</span></code></pre>
  </div>
</template>

<style scoped>
.diff-surface {
  display: flex;
  flex-direction: column;
  min-height: 0;
  flex: 1;
  background: var(--dq-bg-base);
}
.diff-surface__toolbar {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 6px 10px;
  border-bottom: 1px solid var(--dq-separator-light);
  background: color-mix(in srgb, var(--dq-bg-elevated) 40%, transparent);
  flex-wrap: wrap;
}
.diff-surface__badge {
  font-size: 11px;
  font-weight: 600;
  padding: 2px 6px;
  border-radius: 4px;
  background: var(--dq-accent-tint);
  color: var(--dq-accent);
}
.diff-surface__badge--warn {
  background: color-mix(in srgb, var(--dq-system-orange) 18%, transparent);
  color: var(--dq-system-orange);
}
.diff-surface__spacer {
  flex: 1;
}
.diff-surface__btn {
  height: 26px;
  padding: 0 10px;
  border: 1px solid var(--dq-border);
  border-radius: 6px;
  background: var(--dq-fill-tertiary);
  color: var(--dq-label-primary);
  font-size: 12px;
  cursor: pointer;
}
.diff-surface__btn:hover {
  background: color-mix(in srgb, var(--dq-label-primary) 8%, var(--dq-fill-tertiary));
}
.diff-surface__status {
  padding: 24px;
  color: var(--dq-label-tertiary);
  font-size: 13px;
}
.diff-surface__patch {
  margin: 0;
  padding: 12px 0;
  overflow: auto;
  flex: 1;
  min-height: 0;
  font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, 'Liberation Mono', monospace;
  font-size: 12px;
  line-height: 1.45;
}
.diff-surface__line {
  display: block;
  padding: 0 12px;
  white-space: pre;
}
.diff-surface__line.is-add {
  background: color-mix(in srgb, var(--dq-system-green, #34c759) 14%, transparent);
  color: color-mix(in srgb, var(--dq-system-green, #248a3d) 70%, var(--dq-label-primary));
}
.diff-surface__line.is-del {
  background: color-mix(in srgb, var(--dq-system-red, #ff3b30) 12%, transparent);
  color: color-mix(in srgb, var(--dq-system-red, #d70015) 70%, var(--dq-label-primary));
}
.diff-surface__line.is-hunk {
  background: color-mix(in srgb, var(--dq-accent) 10%, transparent);
  color: var(--dq-accent);
}
.diff-surface__line.is-meta {
  color: var(--dq-label-tertiary);
}
</style>
