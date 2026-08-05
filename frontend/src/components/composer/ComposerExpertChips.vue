<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import type { Agent } from '@/types'

defineProps<{
  experts: Agent[]
}>()

const emit = defineEmits<{
  remove: [id: string]
}>()

const { t } = useI18n()
</script>

<template>
  <div v-if="experts.length" class="expert-chips" role="list" :aria-label="t('composer.selectedExperts')">
    <div
      v-for="a in experts"
      :key="a.id"
      class="expert-chip"
      role="listitem"
    >
      <span class="expert-chip__badge" aria-hidden="true">{{ t('composer.expertBadge') }}</span>
      <span class="expert-chip__name" :title="a.description || a.name">{{ a.name || a.id }}</span>
      <button
        type="button"
        class="expert-chip__remove"
        :aria-label="t('composer.removeExpert', { name: a.name || a.id })"
        @click="emit('remove', a.id)"
      >
        ×
      </button>
    </div>
  </div>
</template>

<style scoped>
.expert-chips {
  display: flex;
  flex-wrap: wrap;
  gap: 6px;
  padding: 8px 12px 0;
}

.expert-chip {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  max-width: 100%;
  padding: 3px 6px 3px 8px;
  border: 1px solid color-mix(in srgb, var(--dq-accent) 28%, transparent);
  border-radius: 6px;
  background: color-mix(in srgb, var(--dq-accent) 8%, transparent);
  font-size: var(--dq-font-size-body);
  line-height: 1.3;
  color: var(--dq-text-primary, inherit);
}

.expert-chip__badge {
  flex-shrink: 0;
  font-size: var(--dq-font-size-caption);
  font-weight: 600;
  letter-spacing: 0.02em;
  color: var(--dq-accent);
  opacity: 0.9;
}

.expert-chip__name {
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.expert-chip__remove {
  flex-shrink: 0;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 18px;
  height: 18px;
  margin: 0;
  padding: 0;
  border: none;
  border-radius: 4px;
  background: transparent;
  color: inherit;
  opacity: 0.55;
  cursor: pointer;
  font-size: var(--dq-font-size-body);
  line-height: 1;
}

.expert-chip__remove:hover {
  opacity: 1;
  background: color-mix(in srgb, var(--dq-text-primary, #000) 8%, transparent);
}
</style>
