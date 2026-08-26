<script setup lang="ts">
import { computed, nextTick, ref, watch } from 'vue'
import { MdEditor as MdEditorV3, type ExposeParam, type Themes } from 'md-editor-v3'
import 'md-editor-v3/lib/style.css'
import '@/styles/md-editor-overrides.css'
import { useI18n } from 'vue-i18n'
import { storeToRefs } from 'pinia'
import { useThemeStore, THEME_OPTIONS } from '@/stores/theme'

const model = defineModel<string>({ default: '' })

type MdMode = 'edit' | 'preview' | 'split'

const props = withDefaults(
  defineProps<{
    placeholder?: string
    rows?: number
    label?: string
    /** Initial view mode; remount with :key when changing. */
    initialMode?: MdMode
    /** Hide Edit / Preview / Split tabs (parent controls mode). */
    hideModeTabs?: boolean
  }>(),
  {
    placeholder: '',
    rows: 12,
    label: '',
    initialMode: 'edit',
    hideModeTabs: false,
  },
)

const { t, locale } = useI18n()
const themeStore = useThemeStore()
const { currentTheme } = storeToRefs(themeStore)

const mode = ref<MdMode>(props.initialMode)
const mdRef = ref<ExposeParam>()
const editorId = `work-md-${Math.random().toString(36).slice(2, 10)}`

const modes = computed(() => [
  { id: 'edit' as const, label: t('common.edit') },
  { id: 'preview' as const, label: t('common.preview') },
  { id: 'split' as const, label: t('common.splitView') },
])

const mdTheme = computed<Themes>(() => {
  const opt = THEME_OPTIONS.find((o) => o.id === currentTheme.value)
  return opt?.dark ? 'dark' : 'light'
})

const mdLanguage = computed(() => (String(locale.value).startsWith('zh') ? 'zh-CN' : 'en-US'))

const editorMinHeight = computed(() => `${Math.max(240, Math.round(props.rows * 22))}px`)

function applyMode(m: MdMode) {
  const ed = mdRef.value
  if (!ed) return
  if (m === 'edit') {
    ed.togglePreviewOnly(false)
    ed.togglePreview(false)
  } else if (m === 'preview') {
    ed.togglePreviewOnly(true)
  } else {
    ed.togglePreviewOnly(false)
    ed.togglePreview(true)
  }
}

function onRemount() {
  applyMode(mode.value)
}

watch(mode, (m) => {
  void nextTick(() => applyMode(m))
})
</script>

<template>
  <div class="work-md">
    <div v-if="label || !hideModeTabs" class="work-md__label-row">
      <span v-if="label" class="work-md__label">{{ label }}</span>
      <div
        v-if="!hideModeTabs"
        class="work-md__tabs"
        :class="{ 'work-md__tabs--solo': !label }"
        role="tablist"
        aria-label="Markdown view"
      >
        <button
          v-for="m in modes"
          :key="m.id"
          type="button"
          class="work-md__tab"
          :class="{ 'is-active': mode === m.id }"
          role="tab"
          :aria-selected="mode === m.id"
          @click="mode = m.id"
        >
          {{ m.label }}
        </button>
      </div>
    </div>

    <div class="work-md__body" :style="{ minHeight: editorMinHeight }">
      <MdEditorV3
        :id="editorId"
        ref="mdRef"
        v-model="model"
        class="work-md__editor"
        :theme="mdTheme"
        :language="mdLanguage"
        :placeholder="placeholder || t('common.markdownPlaceholder')"
        :preview="mode === 'split'"
        :preview-only="mode === 'preview'"
        :toolbars="[]"
        :footers="[]"
        :style="{ height: '100%', minHeight: editorMinHeight }"
        @on-remount="onRemount"
      />
    </div>
  </div>
</template>

<style scoped>
.work-md {
  display: flex;
  flex-direction: column;
  gap: 8px;
  min-height: 0;
  height: 100%;
}

.work-md__label-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
}

.work-md__label {
  font-size: var(--dq-font-size-footnote);
  font-weight: 500;
  color: var(--dq-label-secondary);
}

.work-md__tabs {
  display: inline-flex;
  gap: 2px;
  padding: 2px;
  border-radius: 8px;
  background: var(--dq-fill-tertiary);
}

.work-md__tabs--solo {
  align-self: flex-start;
}

.work-md__tab {
  padding: 4px 10px;
  border: none;
  border-radius: 6px;
  background: transparent;
  color: var(--dq-label-tertiary);
  font: inherit;
  font-size: var(--dq-font-size-footnote);
  font-weight: 500;
  cursor: pointer;
  transition:
    color 0.15s ease,
    background 0.15s ease;
}

.work-md__tab:hover {
  color: var(--dq-label-secondary);
}

.work-md__tab.is-active {
  background: var(--dq-bg-elevated);
  color: var(--dq-label-primary);
  box-shadow: 0 1px 2px color-mix(in srgb, var(--dq-mask) 12%, transparent);
}

.work-md__body {
  flex: 1;
  min-height: 0;
  border: 1px solid var(--dq-glass-border, var(--dq-border-subtle));
  border-radius: 12px;
  overflow: hidden;
  background: color-mix(in srgb, var(--dq-bg-elevated) 22%, transparent);
  box-shadow: inset 0 1px 0 color-mix(in srgb, var(--dq-label-primary) 4%, transparent);
}

.work-md__editor {
  border: none !important;
  border-radius: 0 !important;
  background: transparent !important;
}

/* Beat md-editor-v3 .md-editor-dark hardcodes (#000 / #999) so app theme tokens win. */
.work-md :deep(.md-editor),
.work-md :deep(.md-editor-dark) {
  height: 100%;
  border: none !important;
  border-radius: 0 !important;
  background: transparent !important;
  --md-color: var(--dq-label-primary);
  --md-hover-color: var(--dq-label-secondary);
  --md-bk-color: transparent;
  --md-bk-color-outstand: color-mix(in srgb, var(--dq-bg-elevated) 40%, transparent);
  --md-bk-hover-color: color-mix(in srgb, var(--dq-label-primary) 6%, transparent);
  --md-border-color: var(--dq-glass-border, var(--dq-separator-light));
  --md-border-hover-color: color-mix(in srgb, var(--dq-label-primary) 18%, transparent);
  --md-border-active-color: var(--dq-accent);
  --md-scrollbar-bg-color: transparent;
  --md-scrollbar-thumb-color: color-mix(in srgb, var(--dq-label-quaternary) 55%, transparent);
  --md-scrollbar-thumb-hover-color: color-mix(in srgb, var(--dq-label-tertiary) 55%, transparent);
  --md-scrollbar-thumb-active-color: color-mix(in srgb, var(--dq-label-secondary) 50%, transparent);
}

.work-md :deep(.md-editor-toolbar-wrapper) {
  display: none;
}

.work-md :deep(.md-editor-content),
.work-md :deep(.md-editor-content-wrapper),
.work-md :deep(.md-editor-input-wrapper) {
  height: 100%;
  width: 100% !important;
  max-width: none !important;
  background: transparent !important;
  flex: 1 1 auto !important;
}

.work-md :deep(.md-editor-catalog-editor),
.work-md :deep(.md-editor-catalog-flat) {
  display: none !important;
}

.work-md :deep(.cm-editor),
.work-md :deep(.cm-scroller),
.work-md :deep(.cm-gutters) {
  background: transparent !important;
  color: var(--dq-label-primary);
}

.work-md :deep(.cm-editor) {
  height: 100%;
}

.work-md :deep(.cm-scroller) {
  font-family: ui-sans-serif, system-ui, -apple-system, 'Segoe UI', sans-serif;
  line-height: 1.7;
}

.work-md :deep(.cm-content) {
  padding: 16px 18px !important;
  caret-color: var(--dq-accent);
  min-height: 100%;
}

.work-md :deep(.cm-line) {
  font-size: 14px;
}

/* Markdown source tokens — softer hierarchy than stock CM dark. */
.work-md :deep(.cm-header),
.work-md :deep(.ͼ1 .cm-header),
.work-md :deep(.cm-heading) {
  color: var(--dq-label-primary) !important;
  font-weight: 650;
}

.work-md :deep(.cm-strong) {
  color: var(--dq-label-primary) !important;
  font-weight: 650;
}

.work-md :deep(.cm-emphasis) {
  color: var(--dq-label-secondary) !important;
}

.work-md :deep(.cm-comment),
.work-md :deep(.cm-meta) {
  color: var(--dq-label-tertiary) !important;
}

.work-md :deep(.cm-string),
.work-md :deep(.cm-url),
.work-md :deep(.cm-link) {
  color: var(--dq-accent) !important;
}

.work-md :deep(.cm-builtin),
.work-md :deep(.cm-keyword),
.work-md :deep(.cm-type),
.work-md :deep(.cm-atom) {
  color: color-mix(in srgb, var(--dq-accent) 75%, var(--dq-label-primary)) !important;
}

.work-md :deep(.cm-activeLine),
.work-md :deep(.cm-activeLineGutter) {
  background: color-mix(in srgb, var(--dq-accent) 8%, transparent) !important;
}

.work-md :deep(.cm-selectionBackground),
.work-md :deep(.cm-editor ::selection) {
  background: color-mix(in srgb, var(--dq-accent) 28%, transparent) !important;
}

.work-md :deep(.cm-cursor) {
  border-left-color: var(--dq-accent) !important;
}

.work-md :deep(.md-editor-preview-wrapper),
.work-md :deep(.md-editor-preview) {
  background: transparent !important;
  color: var(--dq-label-primary);
}

.work-md :deep(.md-editor-preview) {
  padding: 16px 20px;
  font-size: 14px;
  line-height: 1.7;
}

.work-md :deep(.md-editor-preview h1),
.work-md :deep(.md-editor-preview h2),
.work-md :deep(.md-editor-preview h3),
.work-md :deep(.md-editor-preview h4) {
  color: var(--dq-label-primary);
  border-color: var(--dq-separator-light);
}

.work-md :deep(.md-editor-preview code),
.work-md :deep(.md-editor-preview pre) {
  background: color-mix(in srgb, var(--dq-label-primary) 7%, transparent);
  border-radius: 6px;
}

.work-md :deep(.md-editor-preview a) {
  color: var(--dq-accent);
}
</style>
