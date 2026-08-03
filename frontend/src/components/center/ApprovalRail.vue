<script setup lang="ts">
export interface ApprovalAnchor {
  key: string
  seq: number
  turnId: string
  kind: 'permission' | 'ask'
  pending: boolean
  label: string
  topPercent: number
}

defineProps<{
  anchors?: ApprovalAnchor[]
}>()

const emit = defineEmits<{
  jump: [anchor: ApprovalAnchor]
}>()
</script>

<template>
  <aside
    v-if="(anchors?.length ?? 0) > 0"
    class="approval-rail"
    :aria-label="$t('sessions.approvalRailLabel')"
  >
    <div class="approval-rail__track" />
    <button
      v-for="a in (anchors ?? [])"
      :key="a.key"
      type="button"
      class="approval-rail__anchor"
      :class="{
        'is-permission': a.kind === 'permission',
        'is-ask': a.kind === 'ask',
        'is-pending': a.pending,
      }"
      :style="{ top: `${a.topPercent}%` }"
      :title="a.label"
      @click="emit('jump', a)"
    >
      <span class="approval-rail__dot" />
      <span class="approval-rail__tip">
        {{ a.kind === 'permission' ? $t('sessions.approvalTip') : $t('sessions.askTip') }}
      </span>
    </button>
  </aside>
</template>

<style scoped>
.approval-rail {
  position: absolute;
  top: 12px;
  right: 6px;
  bottom: 12px;
  width: 28px;
  z-index: 4;
  pointer-events: none;
}

.approval-rail__track {
  position: absolute;
  top: 0;
  bottom: 0;
  left: 50%;
  width: 2px;
  transform: translateX(-50%);
  background: color-mix(in srgb, var(--dq-label-primary) 10%, transparent);
  border-radius: 1px;
}

.approval-rail__anchor {
  position: absolute;
  left: 50%;
  transform: translate(-50%, -50%);
  width: 18px;
  height: 18px;
  padding: 0;
  border: none;
  background: transparent;
  cursor: pointer;
  pointer-events: auto;
}

.approval-rail__dot {
  display: block;
  width: 10px;
  height: 10px;
  margin: 4px auto;
  border-radius: 50%;
  background: var(--dq-label-tertiary);
  box-shadow: 0 0 0 2px var(--dq-bg-base);
}

.approval-rail__anchor.is-permission .approval-rail__dot {
  background: var(--dq-system-orange);
}

.approval-rail__anchor.is-ask .approval-rail__dot {
  background: var(--dq-accent);
}

.approval-rail__anchor.is-pending .approval-rail__dot {
  box-shadow:
    0 0 0 2px var(--dq-bg-base),
    0 0 0 4px color-mix(in srgb, var(--dq-system-orange) 35%, transparent);
  animation: approval-pulse 1.4s ease-in-out infinite;
}

.approval-rail__tip {
  position: absolute;
  right: 22px;
  top: 50%;
  transform: translateY(-50%);
  padding: 2px 6px;
  border-radius: 4px;
  background: var(--dq-glass-popover-bg);
  border: 1px solid var(--dq-glass-border-strong);
  font-size: var(--dq-font-size-caption);
  color: var(--dq-label-secondary);
  white-space: nowrap;
  opacity: 0;
  pointer-events: none;
  transition: opacity 0.12s ease;
}

.approval-rail__anchor:hover .approval-rail__tip,
.approval-rail__anchor.is-pending .approval-rail__tip {
  opacity: 1;
}

@keyframes approval-pulse {
  0%,
  100% {
    opacity: 1;
  }
  50% {
    opacity: 0.55;
  }
}
</style>
