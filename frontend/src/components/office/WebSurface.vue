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
  /** Lets the Stage header render this surface's toolbar (single chrome row). */
  toolbarState: [state: { selecting: boolean; drawing: boolean; consoleCount: number }]
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

/**
 * Re-fetch the current resource. Navigation is owned by the Stage address bar
 * (one URI, one bar), so this surface carries no address input of its own.
 */
function refresh() {
  refreshKey.value++
  syncFromProps()
}

function clearPreview() {
  urlInput.value = ''
  loadUrl.value = ''
  mdHtml.value = ''
  refreshKey.value++
  emit('urlChange', '', '')
}

/** The Stage header renders these controls, so it needs their state. */
watch(
  [selectingElement, drawingRegion, consoleCount],
  () => {
    emit('toolbarState', {
      selecting: selectingElement.value,
      drawing: drawingRegion.value,
      consoleCount: consoleCount.value,
    })
  },
  { immediate: true },
)

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
  refresh,
  clearPreview,
  startElementSelect,
  startDrawRegion,
  stopElementSelect,
  requestConsoleDump,
})
</script>

<template>
  <div class="web-surface">
    <p v-if="selectingElement" class="web-surface__hint">{{ drawingRegion ? t('sessions.designModeHintDraw') : t('sessions.designModeHint') }}</p>
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
