<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { useSessionsStore } from '@/stores/sessions'
import { toast } from '@/utils/feedback'

const { t } = useI18n()
const sessions = useSessionsStore()

const editingId = ref<string | null>(null)
const editDraft = ref('')

const items = computed(() => sessions.pendingMessages)

watch(
  () => sessions.currentSessionId,
  (id) => {
    editingId.value = null
    if (id) void sessions.loadPending(id)
    else void sessions.loadPending(null)
  },
  { immediate: true },
)

function preview(content: string) {
  const one = content.replace(/\s+/g, ' ').trim()
  if (one.length <= 120) return one
  return `${one.slice(0, 117)}…`
}

function startEdit(id: string, content: string) {
  editingId.value = id
  editDraft.value = content
}

function cancelEdit() {
  editingId.value = null
  editDraft.value = ''
}

async function saveEdit(id: string) {
  const content = editDraft.value.trim()
  if (!content) {
    toast.warning(t('composer.queueEmptyContent'))
    return
  }
  try {
    await sessions.updatePending(id, content)
    cancelEdit()
  } catch (e) {
    toast.error(e instanceof Error ? e.message : t('composer.queueFailed'))
  }
}

async function remove(id: string) {
  try {
    await sessions.deletePending(id)
  } catch (e) {
    toast.error(e instanceof Error ? e.message : t('composer.queueFailed'))
  }
}

async function clearAll() {
  try {
    await sessions.clearPending()
  } catch (e) {
    toast.error(e instanceof Error ? e.message : t('composer.queueFailed'))
  }
}

async function move(id: string, dir: -1 | 1) {
  const ids = items.value.map((m) => m.id)
  const idx = ids.indexOf(id)
  const next = idx + dir
  if (idx < 0 || next < 0 || next >= ids.length) return
  const reordered = [...ids]
  const [row] = reordered.splice(idx, 1)
  reordered.splice(next, 0, row)
  try {
    await sessions.reorderPending(reordered)
  } catch (e) {
    toast.error(e instanceof Error ? e.message : t('composer.queueFailed'))
  }
}

const steeringId = ref<string | null>(null)

async function steerToCurrentTurn(id: string) {
  if (steeringId.value) return
  const wasRunning = Boolean(sessions.runningTurnId)
  steeringId.value = id
  try {
    await sessions.steerPending(id)
    toast.success(
      wasRunning ? t('composer.queueSteeredInterrupt') : t('composer.queueSteered'),
    )
    // Soft-steer stays in the queue as status=steering until claimed after tools.
    await sessions.loadPending()
    if (wasRunning) await sessions.loadTurns()
  } catch (e) {
    toast.error(e instanceof Error ? e.message : t('composer.queueSteerFailed'))
  } finally {
    steeringId.value = null
  }
}
</script>

<template>
  <div v-if="items.length" class="composer-queue" role="region" :aria-label="t('composer.queuedTitle')">
    <div class="composer-queue__head">
      <span class="composer-queue__title">{{ t('composer.queuedTitle') }}</span>
      <span class="composer-queue__count">{{ items.length }}</span>
      <button type="button" class="composer-queue__clear" @click="clearAll">
        {{ t('composer.queueClear') }}
      </button>
    </div>

    <ul class="composer-queue__list">
      <li
        v-for="(msg, index) in items"
        :key="msg.id"
        class="composer-queue__item"
        :class="{ 'is-front': index === 0 }"
      >
        <template v-if="editingId === msg.id">
          <textarea
            v-model="editDraft"
            class="composer-queue__edit"
            rows="2"
            @keydown.meta.enter.prevent="saveEdit(msg.id)"
            @keydown.ctrl.enter.prevent="saveEdit(msg.id)"
            @keydown.escape.prevent="cancelEdit"
          />
          <div class="composer-queue__edit-actions">
            <button type="button" @click="saveEdit(msg.id)">{{ t('composer.queueSave') }}</button>
            <button type="button" @click="cancelEdit">{{ t('composer.queueCancel') }}</button>
          </div>
        </template>
        <template v-else>
          <div class="composer-queue__body">
            <button
              v-if="index === 0"
              type="button"
              class="composer-queue__steer"
              :disabled="steeringId === msg.id"
              :title="t('composer.queueSteerHint')"
              :aria-label="t('composer.queueSteer')"
              @click="steerToCurrentTurn(msg.id)"
            >
              <svg viewBox="0 0 24 24" width="14" height="14" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true">
                <path d="M5 12h14" />
                <path d="M13 6l6 6-6 6" />
                <path d="M5 19V5" />
              </svg>
            </button>
            <span v-else class="composer-queue__index">{{ index + 1 }}</span>
            <div class="composer-queue__main">
              <p class="composer-queue__text">{{ preview(msg.content) }}</p>
              <span v-if="index === 0 || msg.status === 'steering'" class="composer-queue__steer-label">
                {{
                  msg.status === 'steering'
                    ? t('composer.queueSteeringArmed')
                    : sessions.runningTurnId
                      ? t('composer.queueSteerNextRunning')
                      : t('composer.queueSteerNextIdle')
                }}
              </span>
            </div>
            <span v-if="msg.attachments?.length" class="composer-queue__atts">
              {{ t('composer.queueAttachments', { n: msg.attachments.length }) }}
            </span>
          </div>
          <div class="composer-queue__actions">
            <button type="button" :disabled="index === 0" :title="t('composer.queueMoveUp')" @click="move(msg.id, -1)">↑</button>
            <button type="button" :disabled="index === items.length - 1" :title="t('composer.queueMoveDown')" @click="move(msg.id, 1)">↓</button>
            <button
              type="button"
              :disabled="msg.status === 'steering'"
              :title="t('composer.queueEdit')"
              @click="startEdit(msg.id, msg.content)"
            >✎</button>
            <button type="button" :title="t('composer.queueRemove')" @click="remove(msg.id)">✕</button>
          </div>
        </template>
      </li>
    </ul>
  </div>
</template>

<style scoped>
.composer-queue {
  display: flex;
  flex-direction: column;
  gap: 8px;
  margin-bottom: 8px;
  padding: 10px 12px;
  border-radius: 14px;
  border: 1px solid color-mix(in srgb, var(--dq-accent) 22%, transparent);
  background: color-mix(in srgb, var(--dq-accent) 5%, var(--dq-bg-elevated, var(--dq-bg-base)));
}

.composer-queue__head {
  display: flex;
  align-items: center;
  gap: 8px;
}

.composer-queue__title {
  font-size: var(--dq-font-size-footnote);
  font-weight: 600;
  color: var(--dq-label-primary);
}

.composer-queue__count {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  min-width: 18px;
  height: 18px;
  padding: 0 6px;
  border-radius: 999px;
  font-size: var(--dq-font-size-caption);
  font-weight: 600;
  color: var(--dq-accent);
  background: color-mix(in srgb, var(--dq-accent) 14%, transparent);
}

.composer-queue__clear {
  margin-left: auto;
  border: 0;
  background: transparent;
  color: var(--dq-label-secondary);
  font-size: var(--dq-font-size-caption);
  cursor: pointer;
}

.composer-queue__clear:hover {
  color: var(--dq-danger, #dc2626);
}

.composer-queue__list {
  list-style: none;
  margin: 0;
  padding: 0;
  display: flex;
  flex-direction: column;
  gap: 6px;
}

.composer-queue__item {
  display: flex;
  flex-direction: column;
  gap: 6px;
  padding: 8px 10px;
  border-radius: 10px;
  border: 1px solid color-mix(in srgb, var(--dq-label-primary) 10%, transparent);
  background: color-mix(in srgb, var(--dq-label-primary) 3%, transparent);
}

.composer-queue__item.is-front {
  border-color: color-mix(in srgb, var(--dq-accent) 35%, transparent);
  background: color-mix(in srgb, var(--dq-accent) 8%, transparent);
}

.composer-queue__body {
  display: flex;
  align-items: flex-start;
  gap: 8px;
  min-width: 0;
}

.composer-queue__main {
  flex: 1;
  min-width: 0;
  display: flex;
  flex-direction: column;
  gap: 2px;
}

.composer-queue__steer {
  flex-shrink: 0;
  width: 26px;
  height: 26px;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  border: 0;
  border-radius: 8px;
  color: var(--dq-color-white);
  background: var(--dq-accent);
  cursor: pointer;
}

.composer-queue__steer:hover:not(:disabled) {
  filter: brightness(1.06);
}

.composer-queue__steer:disabled {
  opacity: 0.55;
  cursor: wait;
}

.composer-queue__steer-label {
  font-size: var(--dq-font-size-caption);
  color: var(--dq-accent);
  font-weight: 500;
}

.composer-queue__index {
  flex-shrink: 0;
  width: 18px;
  height: 18px;
  border-radius: 999px;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  font-size: 11px;
  font-weight: 650;
  color: var(--dq-accent);
  background: color-mix(in srgb, var(--dq-accent) 12%, transparent);
}

.composer-queue__text {
  margin: 0;
  flex: 1;
  min-width: 0;
  font-size: var(--dq-font-size-body);
  line-height: 1.4;
  color: var(--dq-label-primary);
  word-break: break-word;
}

.composer-queue__atts {
  flex-shrink: 0;
  font-size: var(--dq-font-size-caption);
  color: var(--dq-label-tertiary);
}

.composer-queue__actions {
  display: flex;
  gap: 4px;
  margin-left: auto;
}

.composer-queue__actions button,
.composer-queue__edit-actions button {
  border: 0;
  background: color-mix(in srgb, var(--dq-label-primary) 6%, transparent);
  color: var(--dq-label-secondary);
  border-radius: 6px;
  padding: 2px 7px;
  font-size: var(--dq-font-size-caption);
  cursor: pointer;
}

.composer-queue__actions button:disabled {
  opacity: 0.35;
  cursor: not-allowed;
}

.composer-queue__actions button:hover:not(:disabled),
.composer-queue__edit-actions button:hover {
  color: var(--dq-label-primary);
  background: color-mix(in srgb, var(--dq-label-primary) 10%, transparent);
}

.composer-queue__edit {
  width: 100%;
  resize: vertical;
  min-height: 48px;
  padding: 7px 9px;
  border-radius: 8px;
  border: 1px solid color-mix(in srgb, var(--dq-accent) 35%, transparent);
  background: var(--dq-bg-base, transparent);
  color: var(--dq-label-primary);
  font: inherit;
}

.composer-queue__edit-actions {
  display: flex;
  gap: 6px;
}
</style>
