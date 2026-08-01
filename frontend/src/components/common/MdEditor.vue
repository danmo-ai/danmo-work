<script setup lang="ts">
import { computed, nextTick, ref, watch } from 'vue'
import { MdEditor as MdEditorV3, type ExposeParam, type Themes } from 'md-editor-v3'
import 'md-editor-v3/lib/style.css'
import { useI18n } from 'vue-i18n'
import { storeToRefs } from 'pinia'
import { useThemeStore, THEME_OPTIONS } from '@/stores/theme'

const model = defineModel<string>({ default: '' })

const props = withDefaults(
  defineProps<{
    placeholder?: string
    rows?: number
    label?: string
  }>(),
  {
    placeholder: '',
    rows: 12,
    label: '',
  },
)

type MdMode = 'edit' | 'preview' | 'split'

const { t, locale } = useI18n()
const themeStore = useThemeStore()
const { currentTheme } = storeToRefs(themeStore)

const mode = ref<MdMode>('edit')
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
    <div v-if="label" class="work-md__label-row">
      <span class="work-md__label">{{ label }}</span>
      <div class="work-md__tabs" role="tablist" aria-label="Markdown view">
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

    <div v-else class="work-md__tabs work-md__tabs--solo" role="tablist" aria-label="Markdown view">
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
  border: 1px solid var(--dq-border-subtle);
  border-radius: 10px;
  overflow: hidden;
  background: var(--dq-bg-elevated);
}

.work-md__editor {
  border: none !important;
  border-radius: 0 !important;
}

.work-md :deep(.md-editor) {
  height: 100%;
  border: none;
  border-radius: 0;
  --md-bk-color: var(--dq-bg-elevated);
  --md-color: var(--dq-label-primary);
  --md-bk-color-outstand: var(--dq-bg-base);
  --md-border-color: var(--dq-separator-light);
  --md-scrollbar-bg-color: transparent;
  --md-scrollbar-thumb-color: color-mix(in srgb, var(--dq-label-quaternary) 50%, transparent);
}

.work-md :deep(.md-editor-toolbar-wrapper) {
  display: none;
}

.work-md :deep(.md-editor-content) {
  height: 100%;
}
</style>
