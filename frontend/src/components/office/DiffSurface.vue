<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { fetchJSON } from '@/api/client'
import { applyAiReviewHunks, fetchAiReviewDiff, revertAiReviewFile } from '@/api/aiReview'
import { toast } from '@/utils/feedback'
import { routeOfficeFile } from '@/utils/office-route'
import { useWorkspaceUiStore } from '@/stores/workspaceUi'
import { useStageAiReviewStore } from '@/stores/stageAiReview'
import { apiBaseUrl } from '@/utils/desktop'
import UnifiedDiffView from '@/components/office/UnifiedDiffView.vue'

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

const props = defineProps<{
  projectId: string
  path: string
  staged?: boolean
  reloadToken: number
  /** git (default) or ai review vs pre-turn snapshot */
  source?: 'git' | 'ai'
  sessionId?: string
  turnId?: string
}>()

const { t } = useI18n()
const workspaceUi = useWorkspaceUiStore()
const aiReview = useStageAiReviewStore()
const loading = ref(false)
const data = ref<GitDiffResult | null>(null)
const aiPatch = ref('')
const aiCanRevert = ref(false)
const aiError = ref('')
const selectedHunks = ref<number[]>([])
const acting = ref(false)

const isAi = computed(() => props.source === 'ai')

async function load() {
  if (!props.path) return
  loading.value = true
  aiError.value = ''
  try {
    if (isAi.value) {
      if (!props.sessionId || !props.turnId) {
        aiError.value = t('office.aiReviewMissing')
        aiPatch.value = ''
        return
      }
      const diff = await fetchAiReviewDiff(props.sessionId, props.turnId, props.path)
      aiPatch.value = diff.patch || ''
      aiCanRevert.value = !!diff.canRevert
      if (diff.error) aiError.value = diff.error
      data.value = null
      selectedHunks.value = []
      return
    }
    if (!props.projectId) return
    const q = new URLSearchParams({
      path: props.path,
      staged: props.staged ? '1' : '0',
    })
    data.value = await fetchJSON<GitDiffResult>(`/projects/${props.projectId}/git-diff?${q.toString()}`)
  } catch (e) {
    if (isAi.value) {
      aiError.value = e instanceof Error ? e.message : t('office.diffLoadFailed')
      aiPatch.value = ''
    } else {
      data.value = {
        path: props.path,
        staged: !!props.staged,
        patch: '',
        error: e instanceof Error ? e.message : t('office.diffLoadFailed'),
      }
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

function askAboutDiff() {
  const hint = isAi.value
    ? t('office.aiReviewAskHint')
    : props.staged
      ? t('sessions.askAboutStaged')
      : t('sessions.askAboutUnstaged')
  workspaceUi.prefillComposer(t('sessions.askAboutFilePrompt', { file: props.path, hint }))
}

async function keepAi() {
  aiReview.clearPending()
  openFile()
}

async function revertAi() {
  if (!props.sessionId || !props.turnId || acting.value) return
  acting.value = true
  try {
    await revertAiReviewFile(props.sessionId, props.turnId, props.path)
    aiReview.clearPending()
    toast.success(t('office.aiReviewReverted'))
    workspaceUi.requestStageReload()
    openFile()
  } catch (e) {
    toast.error(e instanceof Error ? e.message : t('office.aiReviewRevertFailed'))
  } finally {
    acting.value = false
  }
}

async function acceptSelectedHunks() {
  if (!props.sessionId || !props.turnId || acting.value) return
  if (!selectedHunks.value.length) {
    toast.warning(t('office.aiReviewSelectHunks'))
    return
  }
  acting.value = true
  try {
    await applyAiReviewHunks(props.sessionId, props.turnId, props.path, {
      hunkIndexes: selectedHunks.value,
    })
    aiReview.clearPending()
    toast.success(t('office.aiReviewHunksApplied'))
    workspaceUi.requestStageReload()
    openFile()
  } catch (e) {
    toast.error(e instanceof Error ? e.message : t('office.aiReviewHunksFailed'))
  } finally {
    acting.value = false
  }
}

watch(
  () =>
    [props.projectId, props.path, props.staged, props.reloadToken, props.source, props.sessionId, props.turnId] as const,
  () => {
    void load()
  },
  { immediate: true },
)
</script>

<template>
  <div class="diff-surface">
    <div class="diff-surface__toolbar">
      <span v-if="isAi" class="diff-surface__badge">{{ t('office.aiReviewBadge') }}</span>
      <template v-else>
        <span class="diff-surface__badge">{{ staged ? t('office.diffStaged') : t('office.diffUnstaged') }}</span>
        <span v-if="data?.untracked" class="diff-surface__badge diff-surface__badge--warn">{{
          t('office.diffUntracked')
        }}</span>
        <span v-if="data?.binary" class="diff-surface__badge">{{ t('office.diffBinary') }}</span>
        <span v-if="data?.truncated" class="diff-surface__badge diff-surface__badge--warn">{{
          t('office.diffTruncated')
        }}</span>
      </template>
      <span class="diff-surface__spacer" />
      <template v-if="isAi">
        <button
          v-if="selectedHunks.length"
          type="button"
          class="diff-surface__btn"
          :disabled="acting"
          @click="acceptSelectedHunks"
        >
          {{ t('office.aiReviewAcceptHunks') }}
        </button>
        <button type="button" class="diff-surface__btn" :disabled="acting" @click="keepAi">
          {{ t('office.aiReviewKeep') }}
        </button>
        <button
          type="button"
          class="diff-surface__btn"
          :disabled="acting || !aiCanRevert"
          @click="revertAi"
        >
          {{ t('office.aiReviewRevert') }}
        </button>
      </template>
      <button type="button" class="diff-surface__btn" @click="askAboutDiff">
        {{ t('sessions.askAboutFile') }}
      </button>
      <button v-if="!isAi" type="button" class="diff-surface__btn" @click="openFile">
        {{ t('office.diffOpenFile') }}
      </button>
    </div>

    <div v-if="loading" class="diff-surface__status">{{ t('office.loading') }}</div>
    <div v-else-if="isAi && aiError && !aiPatch" class="diff-surface__status">
      {{ aiError === 'hash_only' ? t('office.aiReviewHashOnly') : aiError }}
    </div>
    <div v-else-if="isAi && !aiPatch" class="diff-surface__status">{{ t('office.diffEmpty') }}</div>
    <UnifiedDiffView
      v-else-if="isAi"
      :patch="aiPatch"
      selectable-hunks
      v-model:selected-hunks="selectedHunks"
    />
    <div v-else-if="data?.error || data?.code" class="diff-surface__status">
      {{ data.code === 'git_missing' ? t('composer.gitMissing') : data.error || t('office.diffLoadFailed') }}
    </div>
    <div v-else-if="!(data?.patch)" class="diff-surface__status">{{ t('office.diffEmpty') }}</div>
    <UnifiedDiffView v-else :patch="data?.patch || ''" />
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
.diff-surface__btn:hover:not(:disabled) {
  background: color-mix(in srgb, var(--dq-label-primary) 8%, var(--dq-fill-tertiary));
}
.diff-surface__btn:disabled {
  opacity: 0.5;
  cursor: not-allowed;
}
.diff-surface__status {
  padding: 24px;
  color: var(--dq-label-tertiary);
  font-size: 13px;
}
</style>
