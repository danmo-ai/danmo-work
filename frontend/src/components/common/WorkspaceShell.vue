<script setup lang="ts">
import { useI18n } from 'vue-i18n'

const { t } = useI18n()

withDefaults(
  defineProps<{
    title?: string
    count?: number
    countLabel?: string
    createLabel?: string
    hasSelection?: boolean
    /** When true, skip default count/create rail head — use #rail for full rail chrome */
    customRail?: boolean
    /** Collapse the left resource rail (e.g. while a glass drawer owns focus). */
    hideRail?: boolean
  }>(),
  {
    customRail: false,
    hideRail: false,
  },
)

defineEmits<{
  create: []
  keydown: [event: KeyboardEvent]
}>()
</script>

<template>
  <div
    class="resource-shell float-island"
    :class="{ 'resource-shell--rail-hidden': hideRail }"
    tabindex="-1"
    @keydown.capture="$emit('keydown', $event)"
  >
    <aside v-show="!hideRail" class="resource-rail">
      <template v-if="!customRail">
        <div class="resource-rail__head">
          <span class="resource-rail__count">{{ count }}</span>
          <DqIconButton :aria-label="createLabel ?? t('common.new')" @click="$emit('create')">
            <svg viewBox="0 0 24 24" width="16" height="16" fill="none" stroke="currentColor" stroke-width="2">
              <path d="M12 5v14M5 12h14" stroke-linecap="round" />
            </svg>
          </DqIconButton>
        </div>
      </template>
      <div class="resource-rail__body">
        <slot name="rail" />
      </div>
    </aside>

    <main class="resource-workspace">
      <div v-if="!hasSelection" class="resource-workspace__empty">
        <slot name="empty">
          <DqEmpty :description="t('navigation.selectOrCreateProject')" />
        </slot>
      </div>
      <template v-else>
        <header class="resource-workspace__bar">
          <slot name="header" />
        </header>
        <div class="resource-workspace__scroll">
          <slot name="body" />
        </div>
        <footer class="resource-workspace__footer">
          <slot name="footer" />
        </footer>
      </template>
    </main>
  </div>
</template>
