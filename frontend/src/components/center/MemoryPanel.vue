<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { asArray, fetchJSON, isNotFoundError } from '@/api/client'
import { toast } from '@/utils/feedback'
import type { StreamEvent } from '@/types/mission'

export interface MemoryItem {
  id: string
  scope: 'user' | 'project' | 'agent' | string
  scopeId: string
  key: string
  content: string
  updatedAt: string
}

const props = defineProps<{
  projectId: string | null
  agentId: string | null
  streamEvents?: StreamEvent[]
}>()

const emit = defineEmits<{
  loaded: [count: number]
}>()

const { t } = useI18n()
const items = ref<MemoryItem[]>([])
const loading = ref(false)
const deletingKey = ref('')

async function load() {
  loading.value = true
  try {
    const qs = new URLSearchParams()
    if (props.projectId) qs.set('projectId', props.projectId)
    // Include all agent-scoped memories (subagents write under their own id).
    qs.set('allAgents', 'true')
    const q = qs.toString()
    items.value = asArray(await fetchJSON<MemoryItem[]>(`/memories?${q}`))
  } catch (e) {
    items.value = []
    // Outdated sidecars may lack /memories — don't spam toasts while switching sessions.
    if (!isNotFoundError(e)) {
      toast.error(e instanceof Error ? e.message : t('sessions.memoryLoadFailed'))
    }
  } finally {
    loading.value = false
    emit('loaded', items.value.length)
  }
}

async function remove(m: MemoryItem) {
  const id = `${m.scope}:${m.scopeId}:${m.key}`
  deletingKey.value = id
  try {
    const qs = new URLSearchParams({
      scope: m.scope,
      scopeId: m.scopeId,
      key: m.key,
    })
    await fetchJSON(`/memories?${qs.toString()}`, { method: 'DELETE' })
    items.value = items.value.filter((x) => !(x.scope === m.scope && x.scopeId === m.scopeId && x.key === m.key))
    emit('loaded', items.value.length)
  } catch (e) {
    toast.error(e instanceof Error ? e.message : t('sessions.memoryDeleteFailed'))
  } finally {
    deletingKey.value = ''
  }
}

const grouped = computed(() => {
  const order: Array<'user' | 'project' | 'agent'> = ['user', 'project', 'agent']
  const map: Record<string, MemoryItem[]> = { user: [], project: [], agent: [] }
  for (const m of items.value) {
    const scope = (['user', 'project', 'agent'].includes(m.scope) ? m.scope : 'user') as keyof typeof map
    map[scope].push(m)
  }
  return order
    .filter((scope) => map[scope].length > 0)
    .map((scope) => ({ scope, items: map[scope] }))
})

function scopeLabel(scope: string): string {
  if (scope === 'user') return t('sessions.memoryScopeUser')
  if (scope === 'project') return t('sessions.memoryScopeProject')
  if (scope === 'agent') return t('sessions.memoryScopeAgent')
  return scope
}

function formatTime(iso: string): string {
  if (!iso) return ''
  const d = new Date(iso)
  if (Number.isNaN(d.getTime())) return ''
  return d.toLocaleString()
}

function asRecord(v: unknown): Record<string, unknown> | null {
  if (v && typeof v === 'object' && !Array.isArray(v)) return v as Record<string, unknown>
  return null
}

watch(
  () => [props.projectId, props.agentId] as const,
  () => { void load() },
)

watch(
  () => props.streamEvents?.length ?? 0,
  (len, prev) => {
    const events = props.streamEvents
    if (!events?.length || len <= (prev ?? 0)) return
    // Refresh when any newly arrived event is a memory_update completion
    // (including nested delegate child turns on the same session stream).
    for (let i = prev ?? 0; i < events.length; i++) {
      const ev = events[i]
      if (ev?.type !== 'tool.completed') continue
      const payload = asRecord(ev.payload)
      if (String(payload?.name ?? '') === 'memory_update') {
        void load()
        return
      }
    }
  },
)

onMounted(() => { void load() })

defineExpose({
  refresh: load,
})
</script>

<template>
  <aside class="memory-panel">
    <div class="memory-panel__toolbar">
      <span class="memory-panel__count">{{ t('sessions.memoryCount', { n: items.length }) }}</span>
      <button type="button" class="memory-panel__refresh" :disabled="loading" @click="load">
        {{ loading ? t('sessions.memoryLoading') : t('sessions.memoryRefresh') }}
      </button>
    </div>

    <div v-if="loading && !items.length" class="memory-panel__empty">
      <p>{{ t('sessions.memoryLoading') }}</p>
    </div>

    <div v-else-if="!items.length" class="memory-panel__empty">
      <p>{{ t('sessions.noMemories') }}</p>
      <p class="memory-panel__hint">{{ t('sessions.noMemoriesHint') }}</p>
    </div>

    <div v-else class="memory-panel__list">
      <section v-for="group in grouped" :key="group.scope" class="memory-group">
        <h3 class="memory-group__title">{{ scopeLabel(group.scope) }}</h3>
        <article
          v-for="m in group.items"
          :key="`${m.scope}:${m.scopeId}:${m.key}`"
          class="memory-card"
        >
          <div class="memory-card__head">
            <span class="memory-card__key" :title="m.key">{{ m.key }}</span>
            <button
              type="button"
              class="memory-card__delete"
              :disabled="deletingKey === `${m.scope}:${m.scopeId}:${m.key}`"
              :title="t('sessions.memoryDelete')"
              @click="remove(m)"
            >
              ×
            </button>
          </div>
          <p class="memory-card__content">{{ m.content }}</p>
          <div class="memory-card__meta">
            <span v-if="m.scope !== 'user' && m.scopeId" class="memory-card__scope-id">{{ m.scopeId }}</span>
            <span v-if="m.updatedAt">{{ formatTime(m.updatedAt) }}</span>
          </div>
        </article>
      </section>
    </div>
  </aside>
</template>

<style scoped>
.memory-panel {
  display: flex;
  flex-direction: column;
  min-height: 0;
  height: 100%;
  overflow: hidden;
}

.memory-panel__toolbar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
  padding: 8px 12px;
  border-bottom: 1px solid var(--dq-separator-light);
  flex-shrink: 0;
}

.memory-panel__count {
  font-size: var(--dq-font-size-footnote);
  color: var(--dq-label-tertiary);
  font-weight: 600;
}

.memory-panel__refresh {
  border: none;
  background: transparent;
  color: var(--dq-accent);
  font-size: var(--dq-font-size-footnote);
  font-weight: 600;
  cursor: pointer;
  padding: 2px 4px;
}

.memory-panel__refresh:disabled {
  opacity: 0.5;
  cursor: default;
}

.memory-panel__empty {
  padding: 32px 16px;
  text-align: center;
  color: var(--dq-label-tertiary);
  font-size: var(--dq-font-size-body);
}

.memory-panel__hint {
  margin-top: 8px;
  font-size: var(--dq-font-size-footnote);
  color: var(--dq-label-quaternary, var(--dq-label-tertiary));
  line-height: 1.4;
}

.memory-panel__list {
  flex: 1;
  min-height: 0;
  overflow: auto;
  padding: 8px;
  display: flex;
  flex-direction: column;
  gap: 12px;
}

.memory-group__title {
  margin: 0 0 6px;
  padding: 0 4px;
  font-size: var(--dq-font-size-caption);
  font-weight: 700;
  letter-spacing: 0.04em;
  text-transform: uppercase;
  color: var(--dq-label-tertiary);
}

.memory-card {
  padding: 10px 12px;
  margin-bottom: 6px;
  border-radius: 10px;
  background: color-mix(in srgb, var(--dq-label-primary) 4%, transparent);
  border: 1px solid var(--dq-separator-light);
}

.memory-card:hover {
  background: color-mix(in srgb, var(--dq-label-primary) 7%, transparent);
}

.memory-card__head {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 8px;
}

.memory-card__key {
  font-size: var(--dq-font-size-footnote);
  font-weight: 700;
  color: var(--dq-label-primary);
  word-break: break-all;
}

.memory-card__delete {
  flex-shrink: 0;
  width: 22px;
  height: 22px;
  border: none;
  border-radius: 6px;
  background: transparent;
  color: var(--dq-label-tertiary);
  cursor: pointer;
  font-size: var(--dq-font-size-prose);
  line-height: 1;
}

.memory-card__delete:hover {
  color: var(--dq-color-danger, #e5484d);
  background: color-mix(in srgb, var(--dq-label-primary) 6%, transparent);
}

.memory-card__content {
  margin: 6px 0 0;
  font-size: var(--dq-font-size-footnote);
  color: var(--dq-label-secondary);
  line-height: 1.45;
  white-space: pre-wrap;
  word-break: break-word;
}

.memory-card__meta {
  margin-top: 8px;
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 8px;
  font-size: var(--dq-font-size-caption);
  color: var(--dq-label-tertiary);
}

.memory-card__scope-id {
  font-family: var(--dq-font-mono, ui-monospace, monospace);
  opacity: 0.9;
}
</style>
