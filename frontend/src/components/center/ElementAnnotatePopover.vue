<script setup lang="ts">
import { nextTick, ref, watch } from 'vue'
import type { InspectElementPayload } from '@/types/element-attachment'

const props = defineProps<{
  open: boolean
  payload: InspectElementPayload | null
  summary?: string
}>()

const emit = defineEmits<{
  confirm: [annotation: string]
  cancel: []
}>()

const annotation = ref('')
const inputRef = ref<HTMLTextAreaElement | null>(null)

watch(
  () => props.open,
  (v) => {
    if (v) {
      annotation.value = ''
      void nextTick(() => inputRef.value?.focus())
    }
  },
)

function confirm() {
  emit('confirm', annotation.value.trim())
}

function cancel() {
  emit('cancel')
}

function onKeydown(e: KeyboardEvent) {
  if ((e.metaKey || e.ctrlKey) && e.key === 'Enter') {
    e.preventDefault()
    confirm()
    return
  }
  if (e.key === 'Escape') {
    e.preventDefault()
    cancel()
  }
}

function shortSummary(): string {
  if (props.summary) return props.summary
  const p = props.payload
  if (!p) return ''
  const tag = p.tag || 'el'
  const text = (p.text || p.ariaLabel || '').trim()
  if (text) return `<${tag}> ${text.length > 40 ? text.slice(0, 40) + '…' : text}`
  if (p.selectors?.css) return `<${tag}> ${p.selectors.css}`
  return `<${tag}>`
}
</script>

<template>
  <div v-if="open" class="el-annotate" role="dialog" aria-label="提交到创作器" @keydown="onKeydown">
    <div class="el-annotate__backdrop" @click="cancel" />
    <div class="el-annotate__panel">
      <div class="el-annotate__header">
        <span class="el-annotate__icon" aria-hidden="true">
          <svg viewBox="0 0 24 24" width="16" height="16" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
            <circle cx="12" cy="12" r="10" />
            <line x1="22" y1="12" x2="18" y2="12" />
            <line x1="6" y1="12" x2="2" y2="12" />
            <line x1="12" y1="6" x2="12" y2="2" />
            <line x1="12" y1="22" x2="12" y2="18" />
          </svg>
        </span>
        <div class="el-annotate__title-wrap">
          <div class="el-annotate__title">提交到创作器</div>
          <div class="el-annotate__summary">{{ shortSummary() }}</div>
        </div>
      </div>
      <div v-if="payload?.screenshot" class="el-annotate__preview">
        <img :src="payload.screenshot" alt="选中元素预览" class="el-annotate__preview-img" />
      </div>
      <label class="el-annotate__label" for="el-annotate-input">批注（可选）</label>
      <textarea
        id="el-annotate-input"
        ref="inputRef"
        v-model="annotation"
        class="el-annotate__input"
        rows="3"
        placeholder="例如：字号改大、左边距加大…"
        @keydown="onKeydown"
      />
      <div class="el-annotate__actions">
        <button type="button" class="el-annotate__btn el-annotate__btn--ghost" @click="cancel">取消</button>
        <button type="button" class="el-annotate__btn el-annotate__btn--primary" @click="confirm">
          提交到创作器
        </button>
      </div>
      <div class="el-annotate__hint">⌘/Ctrl+Enter 确认 · Esc 取消</div>
    </div>
  </div>
</template>

<style scoped>
.el-annotate {
  position: absolute;
  inset: 0;
  z-index: 40;
  display: flex;
  align-items: center;
  justify-content: center;
  pointer-events: none;
}

.el-annotate__backdrop {
  position: absolute;
  inset: 0;
  background: color-mix(in srgb, var(--dq-bg-base) 45%, transparent);
  pointer-events: auto;
}

.el-annotate__panel {
  position: relative;
  width: min(360px, calc(100% - 24px));
  padding: 14px 14px 12px;
  border-radius: var(--dq-radius-menu, 12px);
  border: 1px solid var(--dq-glass-border-strong);
  background: var(--dq-glass-popover-bg);
  box-shadow: var(--dq-shadow-glass);
  -webkit-backdrop-filter: var(--dq-glass-blur-heavy);
  backdrop-filter: var(--dq-glass-blur-heavy);
  pointer-events: auto;
}

.el-annotate__header {
  display: flex;
  gap: 10px;
  align-items: flex-start;
  margin-bottom: 12px;
}

.el-annotate__icon {
  display: flex;
  align-items: center;
  justify-content: center;
  width: 28px;
  height: 28px;
  border-radius: 8px;
  color: var(--dq-accent);
  background: color-mix(in srgb, var(--dq-accent) 14%, transparent);
  flex-shrink: 0;
}

.el-annotate__title {
  font-size: 13px;
  font-weight: 600;
  color: var(--dq-label-primary);
}

.el-annotate__summary {
  margin-top: 2px;
  font-size: 12px;
  color: var(--dq-label-tertiary);
  word-break: break-all;
  line-height: 1.4;
}

.el-annotate__preview {
  margin: 0 0 12px;
  border-radius: 8px;
  border: 1px solid var(--dq-glass-border-strong);
  background: color-mix(in srgb, var(--dq-bg-base) 40%, transparent);
  overflow: hidden;
  max-height: 140px;
  display: flex;
  align-items: center;
  justify-content: center;
}

.el-annotate__preview-img {
  display: block;
  max-width: 100%;
  max-height: 140px;
  object-fit: contain;
}

.el-annotate__label {
  display: block;
  font-size: 12px;
  color: var(--dq-label-secondary);
  margin-bottom: 6px;
}

.el-annotate__input {
  width: 100%;
  box-sizing: border-box;
  resize: vertical;
  min-height: 72px;
  max-height: 160px;
  padding: 8px 10px;
  border-radius: 8px;
  border: 1px solid var(--dq-glass-border-strong);
  background: color-mix(in srgb, var(--dq-bg-base) 55%, transparent);
  color: var(--dq-label-primary);
  font-size: 13px;
  line-height: 1.45;
  font-family: inherit;
}

.el-annotate__input:focus {
  outline: none;
  border-color: var(--dq-accent);
}

.el-annotate__actions {
  display: flex;
  justify-content: flex-end;
  gap: 8px;
  margin-top: 12px;
}

.el-annotate__btn {
  appearance: none;
  border: 1px solid transparent;
  border-radius: 8px;
  padding: 6px 12px;
  font-size: 12px;
  cursor: pointer;
  font-family: inherit;
}

.el-annotate__btn--ghost {
  background: transparent;
  border-color: var(--dq-glass-border-strong);
  color: var(--dq-label-secondary);
}

.el-annotate__btn--ghost:hover {
  color: var(--dq-label-primary);
}

.el-annotate__btn--primary {
  background: var(--dq-accent);
  color: #fff;
}

.el-annotate__btn--primary:hover {
  filter: brightness(1.05);
}

.el-annotate__hint {
  margin-top: 8px;
  font-size: 11px;
  color: var(--dq-label-quaternary);
  text-align: right;
}
</style>
