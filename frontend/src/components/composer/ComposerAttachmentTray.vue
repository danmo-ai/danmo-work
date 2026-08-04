<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import {
  formatBytes,
  type ComposerAttachment,
  type CodeComposerAttachment,
  type ElementComposerAttachment,
  type OfficeComposerAttachment,
} from '@/types/composer-attachment'
import { chipLabel, chipTooltip } from '@/types/element-attachment'
import { codeChipLabel, codeChipTooltip } from '@/types/code-attachment'
import { officeChipLabel, officeChipTooltip } from '@/types/office-edit-attachment'

defineProps<{
  attachments: ComposerAttachment[]
  editingId: string | null
  editingAnnotation: string
}>()

const emit = defineEmits<{
  remove: [id: string]
  'edit-start': [att: ElementComposerAttachment | CodeComposerAttachment | OfficeComposerAttachment]
  'edit-save': []
  'edit-cancel': []
  'update:editingAnnotation': [value: string]
}>()

const { t } = useI18n()
</script>

<template>
  <div v-if="attachments.length || editingId" class="att-tray">
    <div v-if="attachments.length" class="att-tray__list">
      <div
        v-for="att in attachments"
        :key="att.id"
        class="att-card"
        :class="`att-card--${att.kind}`"
      >
        <!-- Image -->
        <template v-if="att.kind === 'image'">
          <div class="att-card__thumb" :style="{ backgroundImage: `url(${att.dataUrl})` }" />
          <div class="att-card__meta">
            <span class="att-card__name" :title="att.name">{{ att.name }}</span>
            <span class="att-card__sub">{{ formatBytes(att.size) }}</span>
          </div>
        </template>

        <!-- File placeholder -->
        <template v-else-if="att.kind === 'file'">
          <div class="att-card__icon" aria-hidden="true">
            <svg viewBox="0 0 24 24" width="18" height="18" fill="none" stroke="currentColor" stroke-width="2">
              <path d="M14 2H6a2 2 0 0 0-2 2v16a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V8z" />
              <polyline points="14 2 14 8 20 8" />
            </svg>
          </div>
          <div class="att-card__meta">
            <span class="att-card__name" :title="att.name">{{ att.name }}</span>
            <span class="att-card__sub">
              {{ formatBytes(att.size) }}
              <span class="att-card__badge">{{ t('composer.attachPending') }}</span>
            </span>
          </div>
        </template>

        <!-- Code selection -->
        <template v-else-if="att.kind === 'code'">
          <div class="att-card__icon att-card__icon--code" aria-hidden="true">
            <svg viewBox="0 0 24 24" width="16" height="16" fill="none" stroke="currentColor" stroke-width="2">
              <polyline points="16 18 22 12 16 6" />
              <polyline points="8 6 2 12 8 18" />
            </svg>
          </div>
          <div class="att-card__meta" :title="codeChipTooltip(att.data)">
            <span class="att-card__name">{{ codeChipLabel(att.data) }}</span>
            <span class="att-card__sub">
              {{ t('composer.attachCode') }}
              <template v-if="att.data.annotation"> · {{ att.data.annotation }}</template>
            </span>
          </div>
          <button
            type="button"
            class="att-card__action"
            :title="t('composer.editAnnotation')"
            @click="emit('edit-start', att)"
          >
            ✎
          </button>
        </template>

        <!-- Office polish / modify -->
        <template v-else-if="att.kind === 'office'">
          <div class="att-card__icon att-card__icon--office" aria-hidden="true">
            <svg viewBox="0 0 24 24" width="16" height="16" fill="none" stroke="currentColor" stroke-width="2">
              <path d="M12 20h9" />
              <path d="M16.5 3.5a2.1 2.1 0 0 1 3 3L7 19l-4 1 1-4Z" />
            </svg>
          </div>
          <div class="att-card__meta" :title="officeChipTooltip(att.data)">
            <span class="att-card__name">{{ officeChipLabel(att.data) }}</span>
            <span class="att-card__sub">
              {{ t('composer.attachOffice') }}
              <template v-if="att.data.instruction"> · {{ att.data.instruction }}</template>
            </span>
          </div>
          <button
            type="button"
            class="att-card__action"
            :title="t('composer.editAnnotation')"
            @click="emit('edit-start', att)"
          >
            ✎
          </button>
        </template>

        <!-- DOM element -->
        <template v-else>
          <div
            v-if="att.data.screenshotDataUrl"
            class="att-card__thumb"
            :style="{ backgroundImage: `url(${att.data.screenshotDataUrl})` }"
            aria-hidden="true"
          />
          <div v-else class="att-card__icon att-card__icon--el" aria-hidden="true">
            <svg viewBox="0 0 24 24" width="16" height="16" fill="none" stroke="currentColor" stroke-width="2">
              <circle cx="12" cy="12" r="10" />
              <line x1="22" y1="12" x2="18" y2="12" />
              <line x1="6" y1="12" x2="2" y2="12" />
              <line x1="12" y1="6" x2="12" y2="2" />
              <line x1="12" y1="22" x2="12" y2="18" />
            </svg>
          </div>
          <div class="att-card__meta" :title="chipTooltip(att.data)">
            <span class="att-card__name">{{ chipLabel(att.data) }}</span>
            <span class="att-card__sub">
              {{ t('composer.attachElement') }}
              <template v-if="att.data.annotation"> · {{ att.data.annotation }}</template>
            </span>
          </div>
          <button
            type="button"
            class="att-card__action"
            :title="t('composer.editAnnotation')"
            @click="emit('edit-start', att)"
          >
            ✎
          </button>
        </template>

        <button
          type="button"
          class="att-card__remove"
          :aria-label="t('composer.removeAttachment')"
          @click="emit('remove', att.id)"
        >
          ×
        </button>
      </div>
    </div>

    <div v-if="editingId" class="att-tray__edit">
      <input
        class="att-tray__edit-input"
        :value="editingAnnotation"
        :placeholder="t('composer.annotationPlaceholder')"
        @input="emit('update:editingAnnotation', ($event.target as HTMLInputElement).value)"
        @keydown.enter.prevent="emit('edit-save')"
        @keydown.esc.prevent="emit('edit-cancel')"
      />
      <button type="button" class="att-tray__edit-btn" @click="emit('edit-save')">{{ t('common.save') }}</button>
      <button type="button" class="att-tray__edit-btn att-tray__edit-btn--ghost" @click="emit('edit-cancel')">
        {{ t('common.cancel') }}
      </button>
    </div>
  </div>
</template>

<style scoped>
.att-tray {
  display: flex;
  flex-direction: column;
  gap: 8px;
  padding: 10px 14px 0;
}

.att-tray__list {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
}

.att-card {
  position: relative;
  display: flex;
  align-items: center;
  gap: 8px;
  min-width: 160px;
  max-width: 240px;
  padding: 6px 28px 6px 6px;
  border-radius: 10px;
  border: 1px solid color-mix(in srgb, var(--dq-label-primary) 10%, transparent);
  background: color-mix(in srgb, var(--dq-label-primary) 4%, transparent);
}

.att-card__thumb {
  width: 36px;
  height: 36px;
  border-radius: 6px;
  background-size: cover;
  background-position: center;
  flex-shrink: 0;
}

.att-card__icon {
  width: 36px;
  height: 36px;
  border-radius: 6px;
  display: flex;
  align-items: center;
  justify-content: center;
  background: color-mix(in srgb, var(--dq-label-primary) 8%, transparent);
  color: var(--dq-label-secondary);
  flex-shrink: 0;
}

.att-card__icon--el {
  color: var(--dq-accent);
}

.att-card__icon--code {
  color: var(--dq-accent);
}

.att-card__icon--office {
  color: var(--dq-accent);
}

.att-card__meta {
  min-width: 0;
  display: flex;
  flex-direction: column;
  gap: 2px;
}

.att-card__name {
  font-size: var(--dq-font-size-body);
  font-weight: 600;
  color: var(--dq-label-primary);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.att-card__sub {
  font-size: var(--dq-font-size-caption);
  color: var(--dq-label-tertiary);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.att-card__badge {
  margin-left: 4px;
  font-size: var(--dq-font-size-caption);
  opacity: 0.8;
}

.att-card__action,
.att-card__remove {
  position: absolute;
  top: 4px;
  border: 0;
  background: transparent;
  color: var(--dq-label-tertiary);
  cursor: pointer;
  font-size: var(--dq-font-size-body);
  line-height: 1;
  padding: 2px 4px;
}

.att-card__action {
  right: 22px;
}

.att-card__remove {
  right: 4px;
  font-size: var(--dq-font-size-body);
}

.att-card__action:hover,
.att-card__remove:hover {
  color: var(--dq-label-primary);
}

.att-tray__edit {
  display: flex;
  gap: 6px;
  align-items: center;
}

.att-tray__edit-input {
  flex: 1;
  height: 30px;
  padding: 0 10px;
  border-radius: 6px;
  border: 1px solid var(--dq-border);
  background: var(--dq-bg-elevated);
  color: var(--dq-label-primary);
  font-size: var(--dq-font-size-body);
}

.att-tray__edit-btn {
  height: 30px;
  padding: 0 10px;
  border-radius: 6px;
  border: 1px solid var(--dq-border);
  background: var(--dq-fill-tertiary);
  color: var(--dq-label-primary);
  font-size: var(--dq-font-size-body);
  cursor: pointer;
}

.att-tray__edit-btn--ghost {
  background: transparent;
}
</style>
