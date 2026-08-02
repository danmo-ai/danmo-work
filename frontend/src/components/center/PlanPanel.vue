<script setup lang="ts">
import { computed } from 'vue'
import type { StreamEvent } from '@/types/mission'

const props = defineProps<{
  streamEvents: StreamEvent[]
  planTurnId?: string | null
}>()

interface TodoItem {
  content: string
  status: 'pending' | 'in_progress' | 'completed' | 'cancelled'
  priority: 'high' | 'medium' | 'low'
}

const todos = computed<TodoItem[]>(() => {
  const turnId = props.planTurnId
  for (let i = props.streamEvents.length - 1; i >= 0; i--) {
    const ev = props.streamEvents[i]
    if (turnId && ev.turnId !== turnId) continue
    if (ev.type !== 'tool.running') continue
    const p = ev.payload as Record<string, unknown> | null
    if (p?.name !== 'todowrite') continue
    const input = p?.input as Record<string, unknown> | null
    if (!input?.todos) continue
    const items = input.todos as unknown[]
    if (!Array.isArray(items) || items.length === 0) continue
    return items.map((item: any) => ({
      content: String(item.content ?? ''),
      status: String(item.status ?? 'pending') as TodoItem['status'],
      priority: String(item.priority ?? 'medium') as TodoItem['priority'],
    }))
  }
  return []
})

const groups = computed(() => {
  const items = todos.value
  const inProgress = items.filter((t) => t.status === 'in_progress')
  const pending = items.filter((t) => t.status === 'pending')
  const cancelled = items.filter((t) => t.status === 'cancelled')
  const completed = items.filter((t) => t.status === 'completed')
  return [
    { key: 'in_progress', label: '进行中', items: inProgress },
    { key: 'pending', label: '待处理', items: pending },
    { key: 'cancelled', label: '已取消', items: cancelled },
    { key: 'completed', label: '已完成', items: completed },
  ].filter((g) => g.items.length > 0)
})

const stats = computed(() => {
  const items = todos.value
  return {
    total: items.length,
    inProgress: items.filter((t) => t.status === 'in_progress').length,
    pending: items.filter((t) => t.status === 'pending').length,
    completed: items.filter((t) => t.status === 'completed').length,
    cancelled: items.filter((t) => t.status === 'cancelled').length,
  }
})

function statusIcon(status: TodoItem['status']): string {
  switch (status) {
    case 'completed': return '[x]'
    case 'in_progress': return '[>]'
    case 'cancelled': return '[-]'
    default: return '[ ]'
  }
}

function priorityColor(priority: TodoItem['priority']): string {
  switch (priority) {
    case 'high': return 'var(--dq-label-secondary)'
    case 'medium': return 'var(--dq-label-tertiary)'
    case 'low': return 'var(--dq-label-quaternary)'
  }
}
</script>

<template>
  <aside class="plan-panel">
    <div v-if="todos.length" class="plan-panel__head">
      <div class="plan-panel__stats-row">
        <span class="plan-panel__stat plan-panel__stat--progress" v-if="stats.inProgress > 0">⏳ {{ stats.inProgress }}</span>
        <span class="plan-panel__stat plan-panel__stat--pending" v-if="stats.pending > 0">○ {{ stats.pending }}</span>
        <span class="plan-panel__stat plan-panel__stat--done" v-if="stats.completed > 0">✓ {{ stats.completed }}</span>
        <span class="plan-panel__stat plan-panel__stat--cancelled" v-if="stats.cancelled > 0">✗ {{ stats.cancelled }}</span>
        <span class="plan-panel__stat plan-panel__stat--total">{{ stats.completed }}/{{ stats.total }}</span>
      </div>
    </div>

    <div v-if="!todos.length" class="plan-panel__empty">
      <div class="plan-panel__empty-icon">
        <svg viewBox="0 0 24 24" width="28" height="28" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round"><path d="M9 11l3 3L22 4"/><path d="M21 12v7a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2h11"/></svg>
      </div>
      <p>暂无计划</p>
      <p class="plan-panel__hint">Agent 会在复杂任务时自动生成计划</p>
    </div>

    <div v-else class="plan-panel__list">
      <div
        v-for="group in groups"
        :key="group.key"
        class="plan-panel__group"
      >
        <div class="plan-panel__group-label">
          {{ group.label }}
          <span class="plan-panel__group-count">{{ group.items.length }}</span>
        </div>
        <div
          v-for="(item, idx) in group.items"
          :key="idx"
          class="plan-panel__item"
          :class="{ 'is-done': item.status === 'completed', 'is-cancelled': item.status === 'cancelled', 'is-in-progress': item.status === 'in_progress' }"
        >
          <span
            class="plan-panel__item-dot"
            :style="{ background: priorityColor(item.priority) }"
          />
          <span class="plan-panel__item-text">{{ item.content }}</span>
        </div>
      </div>
    </div>

    <div v-if="todos.length" class="plan-panel__progress">
      <div class="plan-panel__progress-track">
        <div
          class="plan-panel__progress-bar"
          :class="{ 'is-animated': stats.inProgress > 0 }"
          :style="{ width: stats.total ? (stats.completed / stats.total * 100) + '%' : '0%' }"
        />
      </div>
      <span class="plan-panel__progress-text">{{ stats.completed }}/{{ stats.total }} 已完成{{ stats.inProgress > 0 ? ` · ${stats.inProgress} 进行中` : '' }}</span>
    </div>
  </aside>
</template>

<style scoped>
.plan-panel {
  display: flex;
  flex-direction: column;
  height: 100%;
  min-height: 0;
  overflow: hidden;
  /* Drawer owns the glass fill — keep panel clear to avoid stacked frost */
  background: transparent;
  font-size: var(--dq-font-size-body);
}

.plan-panel__head {
  flex-shrink: 0;
  display: flex;
  align-items: center;
  justify-content: flex-end;
  gap: 8px;
  padding: 10px 12px 6px;
  border-bottom: 1px solid var(--dq-separator-light);
}

.plan-panel__title {
  display: flex;
  align-items: center;
  gap: 6px;
  font-size: var(--dq-font-size-body);
  font-weight: 600;
  color: var(--dq-label-primary);
  white-space: nowrap;
}

.plan-panel__title svg {
  color: var(--dq-label-tertiary);
}

.plan-panel__stats-row {
  display: flex;
  align-items: center;
  gap: 6px;
  font-size: var(--dq-font-size-caption);
  font-variant-numeric: tabular-nums;
}

.plan-panel__stat {
  font-weight: 500;
  padding: 1px 5px;
  border-radius: 999px;
  line-height: 1.4;
}

.plan-panel__stat--progress {
  color: var(--dq-accent);
  background: color-mix(in srgb, var(--dq-accent) 8%, transparent);
}

.plan-panel__stat--pending {
  color: var(--dq-label-tertiary);
  background: color-mix(in srgb, var(--dq-label-primary) 6%, transparent);
}

.plan-panel__stat--done {
  color: var(--dq-success);
  background: color-mix(in srgb, var(--dq-success) 8%, transparent);
}

.plan-panel__stat--cancelled {
  color: var(--dq-label-quaternary);
  background: color-mix(in srgb, var(--dq-label-primary) 4%, transparent);
}

.plan-panel__stat--total {
  color: var(--dq-label-secondary);
  font-weight: 600;
  background: color-mix(in srgb, var(--dq-label-primary) 8%, transparent);
}

.plan-panel__empty-icon {
  margin-bottom: 8px;
  opacity: 0.3;
  color: var(--dq-label-tertiary);
}

.plan-panel__empty {
  padding: 24px 16px;
  text-align: center;
  color: var(--dq-label-tertiary);
}

.plan-panel__hint {
  margin-top: 6px;
  font-size: var(--dq-font-size-caption);
  color: var(--dq-label-quaternary);
  line-height: 1.5;
}

.plan-panel__list {
  flex: 1;
  overflow-y: auto;
  padding: 6px 0;
}

.plan-panel__group {
  padding: 2px 0;
}

.plan-panel__group-label {
  padding: 6px 16px 2px;
  font-size: var(--dq-font-size-caption);
  font-weight: 600;
  letter-spacing: 0.06em;
  text-transform: uppercase;
  color: var(--dq-label-quaternary);
  display: flex;
  align-items: center;
  gap: 6px;
}

.plan-panel__group-count {
  font-size: var(--dq-font-size-caption);
  font-weight: 600;
  padding: 1px 6px;
  border-radius: 999px;
  background: color-mix(in srgb, var(--dq-label-primary) 6%, transparent);
  color: var(--dq-label-secondary);
}

.plan-panel__item {
  display: flex;
  align-items: flex-start;
  gap: 8px;
  padding: 5px 16px;
  line-height: 1.45;
  color: var(--dq-label-primary);
}

.plan-panel__item.is-in-progress {
  background: color-mix(in srgb, var(--dq-accent) 4%, transparent);
}

.plan-panel__item.is-done {
  color: var(--dq-label-secondary);
}

.plan-panel__item.is-done .plan-panel__item-text {
  text-decoration: line-through;
  text-decoration-color: color-mix(in srgb, var(--dq-label-secondary) 55%, transparent);
}

.plan-panel__item.is-cancelled {
  color: var(--dq-label-tertiary);
  opacity: 0.6;
}

.plan-panel__item.is-cancelled .plan-panel__item-text {
  text-decoration: line-through;
}

.plan-panel__item-dot {
  flex-shrink: 0;
  width: 6px;
  height: 6px;
  border-radius: 50%;
  margin-top: 6px;
  opacity: 0.5;
}

.plan-panel__item.is-done .plan-panel__item-dot {
  opacity: 0.3;
}

.plan-panel__item.is-cancelled .plan-panel__item-dot {
  opacity: 0.2;
}

.plan-panel__item-text {
  word-break: break-word;
}

.plan-panel__progress {
  flex-shrink: 0;
  padding: 10px 16px 12px;
  border-top: 1px solid var(--dq-separator-light);
}

.plan-panel__progress-track {
  height: 4px;
  border-radius: 2px;
  background: var(--dq-fill-on-glass-hover);
  overflow: hidden;
}

.plan-panel__progress-bar {
  height: 100%;
  border-radius: 2px;
  background: var(--dq-accent);
  transition: width 0.4s ease;
}

.plan-panel__progress-bar.is-animated {
  background: linear-gradient(90deg, var(--dq-accent), color-mix(in srgb, var(--dq-accent) 70%, var(--dq-success)), var(--dq-accent));
  background-size: 200% 100%;
  animation: progress-shimmer 2s ease-in-out infinite;
}

@keyframes progress-shimmer {
  0%, 100% { background-position: 0% 50%; }
  50% { background-position: 100% 50%; }
}

.plan-panel__progress-text {
  display: block;
  margin-top: 6px;
  font-size: var(--dq-font-size-caption);
  color: var(--dq-label-tertiary);
  text-align: center;
}
</style>
