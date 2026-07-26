<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import type { AvailableSkill } from '@/types'

defineProps<{
  skills: AvailableSkill[]
}>()

const emit = defineEmits<{
  remove: [id: string]
}>()

const { t } = useI18n()
</script>

<template>
  <div v-if="skills.length" class="skill-chips" role="list" :aria-label="t('composer.selectedSkills')">
    <div
      v-for="sk in skills"
      :key="sk.id"
      class="skill-chip"
      role="listitem"
    >
      <span class="skill-chip__badge" aria-hidden="true">{{ t('composer.skillBadge') }}</span>
      <span class="skill-chip__name" :title="sk.description || sk.name">{{ sk.name || sk.id }}</span>
      <button
        type="button"
        class="skill-chip__remove"
        :aria-label="t('composer.removeSkill', { name: sk.name || sk.id })"
        @click="emit('remove', sk.id)"
      >
        ×
      </button>
    </div>
  </div>
</template>

<style scoped>
.skill-chips {
  display: flex;
  flex-wrap: wrap;
  gap: 6px;
  padding: 8px 12px 0;
}

.skill-chip {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  max-width: 100%;
  padding: 3px 6px 3px 8px;
  border: 1px solid color-mix(in srgb, var(--dq-accent) 28%, transparent);
  border-radius: 6px;
  background: color-mix(in srgb, var(--dq-accent) 8%, transparent);
  font-size: 12px;
  line-height: 1.3;
  color: var(--dq-text-primary, inherit);
}

.skill-chip__badge {
  flex-shrink: 0;
  font-size: 10px;
  font-weight: 600;
  letter-spacing: 0.02em;
  color: var(--dq-accent);
  opacity: 0.9;
}

.skill-chip__name {
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.skill-chip__remove {
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
  font-size: 14px;
  line-height: 1;
}

.skill-chip__remove:hover {
  opacity: 1;
  background: color-mix(in srgb, var(--dq-text-primary, #000) 8%, transparent);
}
</style>
