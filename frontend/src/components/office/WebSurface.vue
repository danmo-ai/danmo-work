<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import ElementAnnotatePopover from '@/components/center/ElementAnnotatePopover.vue'
import { apiBaseUrl, projectRawUrl as buildProjectRawUrl } from '@/utils/desktop'
import { renderMarkdown } from '@/utils/markdown-render'
import { toast } from '@/utils/feedback'
import {
  fromInspectPayload,
  serializePreviewConsoleReport,
  type ElementAttachment,
  type InspectElementPayload,
  type PreviewConsoleEntry,
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
  attachConsole: [text: string]
  urlChange: [displayUrl: string, loadUrl: string]
}>()

const { t } = useI18n()

const urlInput = ref('')
const loadUrl = ref('')
const refreshKey = ref(0)
const mdHtml = ref('')
const selectingElement = ref(false)
const drawingRegion = ref(false)
const annotateOpen = ref(false)
const annotatePayload = ref<InspectElementPayload | null>(null)
const annotatePayloads = ref<InspectElementPayload[]>([])
const frameRef = ref<HTMLIFrameElement | null>(null)
const consoleCount = ref(0)
const lastHighlightSelector = ref('')

const isImagePath = computed(() => /\.(png|jpe?g|gif|webp|svg|ico|bmp)$/i.test(props.path || urlInput.value))
const isHtmlPreview = computed(() => !mdHtml.value && !isImagePath.value)

function isMdUrl(url: string): boolean {
  const path = url.split('?')[0].split('#')[0]
  return /\.(md|markdown)$/i.test(path)
}

function toProxyUrl(rawUrl: string): string {
  if (rawUrl.includes('/api/v1/projects/') || rawUrl.startsWith('/')) return rawUrl
  try {
    const u = new URL(rawUrl)
    const scheme = u.protocol === 'https:' ? 'https' : 'http'
    const host = u.host.replace(/:/g, '-')
    return `${apiBaseUrl()}/api/v1/proxy/${scheme}/${host}${u.pathname}${u.search}${u.hash}`
  } catch {
    return rawUrl
  }
}

function projectRawUrl(filePath: string): string {
  return buildProjectRawUrl(props.projectId, filePath)
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
    mdHtml.value = `<p style="color:red">${t('office.mdLoadFailed', { error: e })}</p>`
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

function postToFrame(data: Record<string, unknown>) {
  frameRef.value?.contentWindow?.postMessage(data, '*')
}

function startElementSelect() {
  if (!isHtmlPreview.value) {
    toast.warning(t('sessions.designModeHtmlOnly'))
    return
  }
  drawingRegion.value = false
  selectingElement.value = true
  postToFrame({ type: 'dq-inspect-start' })
}

function startDrawRegion() {
  if (!isHtmlPreview.value) {
    toast.warning(t('sessions.designModeHtmlOnly'))
    return
  }
  selectingElement.value = true
  drawingRegion.value = true
  postToFrame({ type: 'dq-inspect-draw' })
}

function stopElementSelect() {
  selectingElement.value = false
  drawingRegion.value = false
  postToFrame({ type: 'dq-inspect-stop' })
}

function requestConsoleDump() {
  if (!isHtmlPreview.value) {
    toast.warning(t('sessions.designModeHtmlOnly'))
    return
  }
  if (!consoleCount.value) {
    toast.info(t('sessions.designModeConsoleEmpty'))
    return
  }
  postToFrame({ type: 'dq-inspect-console-dump' })
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
  if (data.type === 'dq-inspect-console') {
    consoleCount.value = Number(data.count) || 0
    return
  }
  if (data.type === 'dq-inspect-console-data') {
    const entries = (Array.isArray(data.entries) ? data.entries : []) as PreviewConsoleEntry[]
    emit('attachConsole', serializePreviewConsoleReport(entries))
    toast.success(t('sessions.designModeConsoleSent'))
    return
  }
  if (data.type === 'dq-inspect-cancel') {
    selectingElement.value = false
    drawingRegion.value = false
    return
  }
  if (data.type !== 'dq-inspect-selected') return
  const payload = data as InspectElementPayload
  if (!payload.tag && !payload.text && !payload.outerHTML && !payload.html) return
  if (payload.additive) {
    selectingElement.value = true
    annotatePayloads.value = [...annotatePayloads.value, payload]
    annotatePayload.value = payload
    annotateOpen.value = true
    return
  }
  selectingElement.value = false
  drawingRegion.value = false
  annotatePayloads.value = annotatePayloads.value.length
    ? [...annotatePayloads.value, payload]
    : [payload]
  annotatePayload.value = payload
  annotateOpen.value = true
}

function onAnnotateConfirm(annotation: string) {
  const list = annotatePayloads.value.length
    ? annotatePayloads.value
    : annotatePayload.value
      ? [annotatePayload.value]
      : []
  annotateOpen.value = false
  annotatePayload.value = null
  annotatePayloads.value = []
  if (!list.length) return
  const pageUrl = urlInput.value || list[0]?.page?.url || ''
  const sourceFile = resolveProjectSourceFile()
  for (const raw of list) {
    const att = fromInspectPayload(raw, {
      annotation: raw.suggestedAnnotation && !annotation ? raw.suggestedAnnotation : annotation,
      sourceFile,
      pageUrl,
    })
    if (att.selectors?.css) lastHighlightSelector.value = att.selectors.css
    emit('attachElement', att)
  }
  toast.success(t('office.previewAttached'))
}

function onAnnotateCancel() {
  annotateOpen.value = false
  annotatePayload.value = null
  annotatePayloads.value = []
}

function onFrameLoad() {
  consoleCount.value = 0
  if (lastHighlightSelector.value) {
    postToFrame({ type: 'dq-inspect-highlight', selector: lastHighlightSelector.value })
  }
}

function onPreviewKeydown(e: KeyboardEvent) {
  if (e.key !== 'Escape') return
  if (annotateOpen.value) return
  if (selectingElement.value || drawingRegion.value) {
    e.preventDefault()
    stopElementSelect()
  }
}

onMounted(() => {
  window.addEventListener('message', handleInspectMessage)
  window.addEventListener('keydown', onPreviewKeydown, true)
})

onUnmounted(() => {
  window.removeEventListener('message', handleInspectMessage)
  window.removeEventListener('keydown', onPreviewKeydown, true)
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
  <div class="web-surface">
    <p v-if="selectingElement" class="web-surface__hint">{{ drawingRegion ? t('sessions.designModeHintDraw') : t('sessions.designModeHint') }}</p>
    <div class="web-surface__bar">
      <input
        v-model="urlInput"
        class="web-surface__input"
        :placeholder="t('office.previewUrlPlaceholder')"
        @keydown.enter="navigate"
      />
      <button class="web-surface__btn" :title="t('office.previewRefresh')" @click="refresh">
        <svg viewBox="0 0 24 24" width="14" height="14" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><polyline points="23 4 23 10 17 10"/><path d="M20.49 15a9 9 0 1 1-2.12-9.36L23 10"/></svg>
      </button>
      <button class="web-surface__btn" :title="t('office.previewGo')" @click="navigate">
        <svg viewBox="0 0 24 24" width="14" height="14" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
          <polyline points="9 18 15 12 9 6"/>
        </svg>
      </button>
      <button
        class="web-surface__btn"
        :class="{ 'is-active': selectingElement && !drawingRegion }"
        :title="selectingElement && !drawingRegion ? t('sessions.designModeOn') : t('sessions.designModeOff')"
        @click="selectingElement && !drawingRegion ? stopElementSelect() : startElementSelect()"
      >
        <svg viewBox="0 0 24 24" width="14" height="14" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><circle cx="12" cy="12" r="10"/><line x1="22" y1="12" x2="18" y2="12"/><line x1="6" y1="12" x2="2" y2="12"/><line x1="12" y1="6" x2="12" y2="2"/><line x1="12" y1="22" x2="12" y2="18"/></svg>
      </button>
      <button
        class="web-surface__btn"
        :class="{ 'is-active': drawingRegion }"
        :title="t('sessions.designModeDraw')"
        @click="drawingRegion ? stopElementSelect() : startDrawRegion()"
      >
        <svg viewBox="0 0 24 24" width="14" height="14" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M12 19l7-7 3 3-7 7-3-3z"/><path d="M18 13l-1.5-7.5L2 2l3.5 14.5L13 18l5-5z"/><path d="M2 2l7.586 7.586"/></svg>
      </button>
      <button
        class="web-surface__btn web-surface__btn--console"
        :title="t('sessions.designModeConsole')"
        @click="requestConsoleDump"
      >
        <svg viewBox="0 0 24 24" width="14" height="14" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><polyline points="4 17 10 11 4 5"/><line x1="12" y1="19" x2="20" y2="19"/></svg>
        <span v-if="consoleCount" class="web-surface__badge">{{ consoleCount > 9 ? '9+' : consoleCount }}</span>
      </button>
      <button class="web-surface__btn" :title="t('office.previewClear')" @click="clearPreview">
        <svg viewBox="0 0 24 24" width="14" height="14" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><line x1="18" y1="6" x2="6" y2="18"/><line x1="6" y1="6" x2="18" y2="18"/></svg>
      </button>
    </div>
    <div class="web-surface__stage">
      <div
        v-if="mdHtml"
        class="web-surface__md dq-prose"
        v-html="mdHtml"
      />
      <img
        v-else-if="isImagePath && loadUrl"
        :key="`${loadUrl}:${refreshKey}`"
        class="web-surface__img"
        :src="loadUrl"
        :alt="path || urlInput"
      />
      <iframe
        v-else
        ref="frameRef"
        :key="`${loadUrl || 'empty'}:${refreshKey}`"
        class="web-surface__frame"
        :src="loadUrl || 'about:blank'"
        @load="onFrameLoad"
      />
      <ElementAnnotatePopover
        :open="annotateOpen"
        :payload="annotatePayload"
        :payloads="annotatePayloads"
        @confirm="onAnnotateConfirm"
        @cancel="onAnnotateCancel"
      />
    </div>
  </div>
</template>

<style scoped>
.web-surface {
  flex: 1;
  min-height: 0;
  display: flex;
  flex-direction: column;
  overflow: hidden;
  background: var(--dq-bg-base);
  color: var(--dq-label-primary);
}
.web-surface__hint {
  margin: 0;
  padding: 6px 12px;
  font-size: var(--dq-font-size-body);
  color: var(--dq-accent);
  background: color-mix(in srgb, var(--dq-accent) 10%, transparent);
  border-bottom: 1px solid color-mix(in srgb, var(--dq-accent) 20%, transparent);
}
.web-surface__bar {
  flex-shrink: 0;
  display: flex;
  align-items: center;
  gap: 4px;
  padding: 6px 8px;
  border-bottom: 1px solid var(--dq-separator-light);
  background: color-mix(in srgb, var(--dq-bg-elevated) 45%, transparent);
}
.web-surface__input {
  flex: 1;
  height: 28px;
  padding: 0 10px;
  border-radius: 6px;
  border: 1px solid var(--dq-border);
  background: var(--dq-bg-elevated);
  color: var(--dq-label-primary);
  font-size: var(--dq-font-size-body);
  outline: none;
  font-family: var(--dq-font-mono, ui-monospace, monospace);
}
.web-surface__input::placeholder {
  color: var(--dq-label-quaternary);
}
.web-surface__input:focus {
  border-color: var(--dq-accent);
  box-shadow: 0 0 0 3px color-mix(in srgb, var(--dq-accent) 12%, transparent);
}
.web-surface__btn {
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
.web-surface__btn:hover {
  background: var(--dq-accent);
  color: var(--dq-on-accent);
}
.web-surface__btn.is-active {
  background: var(--dq-accent);
  color: var(--dq-on-accent);
}
.web-surface__btn--console {
  position: relative;
}
.web-surface__badge {
  position: absolute;
  top: -4px;
  right: -4px;
  min-width: 14px;
  height: 14px;
  padding: 0 3px;
  border-radius: 7px;
  background: var(--dq-system-orange, #f59e0b);
  color: #fff;
  font-size: 9px;
  line-height: 14px;
  text-align: center;
}
.web-surface__stage {
  position: relative;
  flex: 1 1 auto;
  min-height: 0;
  overflow: hidden;
  background: var(--dq-bg-base);
}
.web-surface__frame {
  position: absolute;
  inset: 0;
  width: 100%;
  height: 100%;
  border: none;
  display: block;
  background: var(--dq-bg-base);
}
.web-surface__img {
  position: absolute;
  inset: 0;
  max-width: 100%;
  max-height: 100%;
  margin: auto;
  object-fit: contain;
}
.web-surface__md {
  position: absolute;
  inset: 0;
  overflow-y: auto;
  padding: 24px 32px;
  color: var(--dq-label-primary);
}
</style>
