<script setup lang="ts">
import { onBeforeUnmount, ref, shallowRef, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { projectRawUrl } from '@/utils/desktop'
import { loadSfcComponent } from '@/utils/sfc-preview'

const props = defineProps<{
  projectId: string
  path: string
  reloadToken?: number
}>()

const { t } = useI18n()
const component = shallowRef<unknown>(null)
const errorText = ref('')
const source = ref('')
const loading = ref(false)
let runToken = 0

async function load() {
  const token = ++runToken
  loading.value = true
  errorText.value = ''
  component.value = null
  try {
    const resp = await fetch(projectRawUrl(props.projectId, props.path), { cache: 'no-store' })
    if (!resp.ok) throw new Error(`HTTP ${resp.status} ${resp.statusText}`)
    const text = await resp.text()
    if (token !== runToken) return
    source.value = text
    component.value = await loadSfcComponent(text, props.path)
  } catch (e) {
    if (token === runToken) errorText.value = e instanceof Error ? e.message : String(e)
  } finally {
    if (token === runToken) loading.value = false
  }
}

watch(() => [props.path, props.reloadToken], () => void load(), { immediate: true })
onBeforeUnmount(() => { runToken++ })

defineExpose({ reload: load })
</script>

<template>
  <div class="vue-sfc">
    <div v-if="errorText" class="vue-sfc__error" role="alert">
      <p class="vue-sfc__error-title">{{ t('office.vuePreviewFailed') }}</p>
      <pre class="vue-sfc__error-text">{{ errorText }}</pre>
      <pre v-if="source" class="vue-sfc__source">{{ source }}</pre>
    </div>
    <div v-else-if="loading && !component" class="vue-sfc__empty">{{ t('office.vuePreviewCompiling') }}</div>
    <div v-else class="vue-sfc__mount">
      <component :is="component" v-if="component" />
    </div>
  </div>
</template>

<style scoped>
.vue-sfc {
  display: flex;
  flex-direction: column;
  flex: 1;
  min-height: 0;
  overflow: auto;
  background: var(--dq-bg-base);
}

.vue-sfc__mount {
  padding: 16px;
}

.vue-sfc__empty {
  padding: 24px 16px;
  font-size: var(--dq-font-size-body);
  color: var(--dq-label-tertiary);
  text-align: center;
}

.vue-sfc__error {
  margin: 12px;
  padding: 12px;
  border: 1px solid var(--dq-warning-surface-border, var(--dq-border));
  border-radius: 8px;
  background: var(--dq-warning-surface, transparent);
  color: var(--dq-label-primary);
  font-size: var(--dq-font-size-caption);
}

.vue-sfc__error-title {
  margin: 0 0 8px;
  font-weight: 600;
  color: var(--dq-warning, var(--dq-label-primary));
}

.vue-sfc__error-text,
.vue-sfc__source {
  margin: 0;
  max-height: 320px;
  overflow: auto;
  font: var(--dq-font-size-caption) var(--dq-font-mono);
  white-space: pre-wrap;
  word-break: break-word;
}

.vue-sfc__source {
  margin-top: 10px;
  padding-top: 10px;
  border-top: 1px solid var(--dq-separator-light);
  color: var(--dq-label-tertiary);
}
</style>
