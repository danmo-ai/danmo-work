<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import ElementAnnotatePopover from '@/components/center/ElementAnnotatePopover.vue'
import { apiBaseUrl } from '@/utils/desktop'
import { renderMarkdown } from '@/utils/markdown-render'
import { toast } from '@/utils/feedback'
import {
  fromInspectPayload,
  type ElementAttachment,
  type InspectElementPayload,
} from '@/types/element-attachment'

const props = defineProps<{
  projectId: string
  path: string
  /** Initial / controlled load URL (project raw or already-proxied). */
  url?: string
  reloadToken?: number
}>()

const emit = defineEmits<{
  attachElement: [att: ElementAttachment]
  urlChange: [displayUrl: string, loadUrl: string]
}>()

const { t } = useI18n()

const urlInput = ref('')
const loadUrl = ref('')
const refreshKey = ref(0)
const mdHtml = ref('')
const selectingElement = ref(false)
const annotateOpen = ref(false)
const annotatePayload = ref<InspectElementPayload | null>(null)
const frameRef = ref<HTMLIFrameElement | null>(null)

const isImagePath = computed(() => /\.(png|jpe?g|gif|webp|svg|ico|bmp)$/i.test(props.path || urlInput.value))

function isMdUrl(url: string): boolean {
  const path = url.split('?')[0].split('#')[0]
  return /\.(md|markdown)$/i.test(path)
}

function toProxyUrl(rawUrl: string): string {
  if (rawUrl.includes('/api/v1/projects/') || rawUrl.startsWith('/')) return rawUrl
  try {
    const u = new URL(rawUrl)
    const host = u.host.replace(/:/g, '-')
    return `${apiBaseUrl()}/api/v1/proxy/${host}${u.pathname}${u.search}${u.hash}`
  } catch {
    return rawUrl
  }
}

function projectRawUrl(filePath: string): string {
  return `${apiBaseUrl()}/api/v1/projects/${props.projectId}/raw/${encodeURIComponent(filePath)}`
}

async function loadMdContent(urlOrPath: string) {
  try {
    let apiPath = urlOrPath
    try {
      const u = new URL(urlOrPath)
      apiPath = u.pathname + u.search
    } catch {
      /* relative path */
    }
    const resp = await fetch(`${apiBaseUrl()}${apiPath}`)
    if (!resp.ok) throw new Error(resp.statusText)
    const text = await resp.text()
    mdHtml.value = renderMarkdown(text)
  } catch (e) {
    mdHtml.value = `<p style="color:red">加载 Markdown 失败: ${e}</p>`
  }
}

function applyLoad(display: string, proxied: string, opts?: { emit?: boolean }) {
  urlInput.value = display
  loadUrl.value = proxied
  refreshKey.value++
  if (opts?.emit !== false) {
    emit('urlChange', display, proxied)
  }
  if (isMdUrl(proxied) || isMdUrl(display)) {
    void loadMdContent(proxied)
  } else {
    mdHtml.value = ''
  }
}

function syncFromProps() {
  if (props.url) {
    const display = props.path || props.url
    applyLoad(display, props.url, { emit: false })
    return
  }
  if (props.path) {
    applyLoad(props.path, projectRawUrl(props.path), { emit: false })
  }
}

watch(
  () => [props.path, props.url, props.reloadToken] as const,
  () => syncFromProps(),
  { immediate: true },
)

function refresh() {
  const display = urlInput.value.trim()
  if (!display) return
  if (display === props.path || (!/^https?:\/\//i.test(display) && !display.includes('/api/'))) {
    applyLoad(props.path || display, projectRawUrl(props.path || display))
    return
  }
  let url = display
  if (!/^https?:\/\//i.test(url) && !url.startsWith('/')) {
    url = 'https://' + url
  }
  applyLoad(url.startsWith('http') ? url : display, toProxyUrl(url))
}

function navigate() {
  refresh()
}

function clearPreview() {
  urlInput.value = ''
  loadUrl.value = ''
  mdHtml.value = ''
  refreshKey.value++
  emit('urlChange', '', '')
}

function startElementSelect() {
  selectingElement.value = true
  frameRef.value?.contentWindow?.postMessage({ type: 'dq-inspect-start' }, '*')
}

function stopElementSelect() {
  selectingElement.value = false
  frameRef.value?.contentWindow?.postMessage({ type: 'dq-inspect-stop' }, '*')
}

function resolveProjectSourceFile(): string | undefined {
  if (props.path) return props.path
  const userUrl = urlInput.value || loadUrl.value
  if (!userUrl.includes('/api/v1/projects/')) return undefined
  const marker = '/raw/'
  const idx = userUrl.indexOf(marker)
  if (idx === -1) return undefined
  try {
    return decodeURIComponent(userUrl.slice(idx + marker.length).split('?')[0].split('#')[0])
  } catch {
    return userUrl.slice(idx + marker.length).split('?')[0]
  }
}

function handleInspectMessage(ev: MessageEvent) {
  const data = ev.data
  if (!data || typeof data !== 'object') return
  if (data.type === 'dq-inspect-cancel') {
    selectingElement.value = false
    return
  }
  if (data.type !== 'dq-inspect-selected') return
  selectingElement.value = false
  const payload = data as InspectElementPayload
  if (!payload.tag && !payload.text && !payload.outerHTML && !payload.html) return
  annotatePayload.value = payload
  annotateOpen.value = true
}

function onAnnotateConfirm(annotation: string) {
  const raw = annotatePayload.value
  annotateOpen.value = false
  annotatePayload.value = null
  if (!raw) return
  const pageUrl = urlInput.value || raw.page?.url || ''
  const att = fromInspectPayload(raw, {
    annotation,
    sourceFile: resolveProjectSourceFile(),
    pageUrl,
  })
  emit('attachElement', att)
  toast.success(t('office.previewAttached'))
}

function onAnnotateCancel() {
  annotateOpen.value = false
  annotatePayload.value = null
}

onMounted(() => {
  window.addEventListener('message', handleInspectMessage)
})

onUnmounted(() => {
  window.removeEventListener('message', handleInspectMessage)
  stopElementSelect()
})

defineExpose({
  navigate,
  refresh,
  clearPreview,
  startElementSelect,
  stopElementSelect,
  selectingElement,
  urlInput,
})
</script>

<template>
  <div class="preview-surface">
    <p v-if="selectingElement" class="preview-surface__hint">{{ t('sessions.designModeHint') }}</p>
    <div class="preview-surface__bar">
      <input
        v-model="urlInput"
        class="preview-surface__input"
        :placeholder="t('office.previewUrlPlaceholder')"
        @keydown.enter="navigate"
      />
      <button class="preview-surface__btn" :title="t('office.previewRefresh')" @click="refresh">
        <svg viewBox="0 0 24 24" width="14" height="14" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><polyline points="23 4 23 10 17 10"/><path d="M20.49 15a9 9 0 1 1-2.12-9.36L23 10"/></svg>
      </button>
      <button class="preview-surface__btn" :title="t('office.previewGo')" @click="navigate">
        <svg viewBox="0 0 24 24" width="14" height="14" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
          <polyline points="9 18 15 12 9 6"/>
        </svg>
      </button>
      <button
        class="preview-surface__btn"
        :class="{ 'is-active': selectingElement }"
        :title="selectingElement ? t('sessions.designModeOn') : t('sessions.designModeOff')"
        @click="selectingElement ? stopElementSelect() : startElementSelect()"
      >
        <svg viewBox="0 0 24 24" width="14" height="14" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><circle cx="12" cy="12" r="10"/><line x1="22" y1="12" x2="18" y2="12"/><line x1="6" y1="12" x2="2" y2="12"/><line x1="12" y1="6" x2="12" y2="2"/><line x1="12" y1="22" x2="12" y2="18"/></svg>
      </button>
      <button class="preview-surface__btn" :title="t('office.previewClear')" @click="clearPreview">
        <svg viewBox="0 0 24 24" width="14" height="14" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><line x1="18" y1="6" x2="6" y2="18"/><line x1="6" y1="6" x2="18" y2="18"/></svg>
      </button>
    </div>
    <div class="preview-surface__stage">
      <div
        v-if="mdHtml"
        class="preview-surface__md dq-prose"
        v-html="mdHtml"
      />
      <img
        v-else-if="isImagePath && loadUrl"
        :key="`${loadUrl}:${refreshKey}`"
        class="preview-surface__img"
        :src="loadUrl"
        :alt="path || urlInput"
      />
      <iframe
        v-else
        ref="frameRef"
        :key="`${loadUrl || 'empty'}:${refreshKey}`"
        class="preview-surface__frame"
        :src="loadUrl || 'about:blank'"
      />
      <ElementAnnotatePopover
        :open="annotateOpen"
        :payload="annotatePayload"
        @confirm="onAnnotateConfirm"
        @cancel="onAnnotateCancel"
      />
    </div>
  </div>
</template>

<style scoped>
.preview-surface {
  flex: 1;
  min-height: 0;
  display: flex;
  flex-direction: column;
  overflow: hidden;
  background: var(--dq-bg-base);
  color: var(--dq-label-primary);
}
.preview-surface__hint {
  margin: 0;
  padding: 6px 12px;
  font-size: 12px;
  color: var(--dq-accent);
  background: color-mix(in srgb, var(--dq-accent) 10%, transparent);
  border-bottom: 1px solid color-mix(in srgb, var(--dq-accent) 20%, transparent);
}
.preview-surface__bar {
  flex-shrink: 0;
  display: flex;
  align-items: center;
  gap: 4px;
  padding: 6px 8px;
  border-bottom: 1px solid var(--dq-separator-light);
  background: color-mix(in srgb, var(--dq-bg-elevated) 45%, transparent);
}
.preview-surface__input {
  flex: 1;
  height: 28px;
  padding: 0 10px;
  border-radius: 6px;
  border: 1px solid var(--dq-border);
  background: var(--dq-bg-elevated);
  color: var(--dq-label-primary);
  font-size: 12px;
  outline: none;
  font-family: var(--dq-font-mono, ui-monospace, monospace);
}
.preview-surface__input::placeholder {
  color: var(--dq-label-quaternary);
}
.preview-surface__input:focus {
  border-color: var(--dq-accent);
  box-shadow: 0 0 0 3px color-mix(in srgb, var(--dq-accent) 12%, transparent);
}
.preview-surface__btn {
  flex-shrink: 0;
  display: flex;
  align-items: center;
  justify-content: center;
  width: 28px;
  height: 28px;
  border-radius: 6px;
  border: none;
  background: var(--dq-fill-tertiary);
  color: var(--dq-label-secondary);
  cursor: pointer;
}
.preview-surface__btn:hover {
  background: var(--dq-accent);
  color: #fff;
}
.preview-surface__btn.is-active {
  background: var(--dq-accent);
  color: #fff;
}
.preview-surface__stage {
  position: relative;
  flex: 1 1 auto;
  min-height: 0;
  overflow: hidden;
  background: var(--dq-bg-base);
}
.preview-surface__frame {
  position: absolute;
  inset: 0;
  width: 100%;
  height: 100%;
  border: none;
  display: block;
  background: var(--dq-bg-base);
}
.preview-surface__img {
  position: absolute;
  inset: 0;
  max-width: 100%;
  max-height: 100%;
  margin: auto;
  object-fit: contain;
}
.preview-surface__md {
  position: absolute;
  inset: 0;
  overflow-y: auto;
  padding: 24px 32px;
  color: var(--dq-label-primary);
}
</style>
