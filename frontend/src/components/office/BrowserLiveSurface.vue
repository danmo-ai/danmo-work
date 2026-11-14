<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { apiBaseUrl } from '@/utils/desktop'

const props = defineProps<{
  sessionId: string
  active?: boolean
}>()

const emit = defineEmits<{
  /** The Stage header renders the live status (single chrome row). */
  statusState: [state: { status: string; label: string; pageTitle: string; pageUrl: string }]
}>()

const { t } = useI18n()
const frameUrl = ref('')
const pageUrl = ref('')
const pageTitle = ref('')
const status = ref<'connecting' | 'live' | 'closed' | 'error'>('connecting')
const errorText = ref('')
let ws: WebSocket | null = null
let disposed = false
let reconnectTimer: ReturnType<typeof setTimeout> | null = null
let reconnectAttempts = 0

const statusLabel = computed(() => t(`sessions.browserLive.${status.value}`))

function socketUrl(): string {
  const httpBase = apiBaseUrl() || window.location.origin
  const u = new URL(`${httpBase}/api/v1/sessions/${encodeURIComponent(props.sessionId)}/browser/live`)
  u.protocol = u.protocol === 'https:' ? 'wss:' : 'ws:'
  return u.toString()
}

function disconnect() {
  if (reconnectTimer) clearTimeout(reconnectTimer)
  reconnectTimer = null
  const socket = ws
  ws = null
  if (socket) {
    socket.onmessage = null
    socket.onclose = null
    socket.onerror = null
    socket.close()
  }
}

function connect() {
  disconnect()
  if (disposed || props.active === false || !props.sessionId) return
  status.value = 'connecting'
  errorText.value = ''
  const socket = new WebSocket(socketUrl())
  socket.binaryType = 'blob'
  ws = socket
  socket.onmessage = async (event) => {
    if (disposed || ws !== socket) return
    if (typeof event.data === 'string') {
      try {
        const data = JSON.parse(event.data) as { state?: string; url?: string; title?: string; error?: string }
        if (data.url !== undefined) pageUrl.value = data.url
        if (data.title !== undefined) pageTitle.value = data.title
        if (data.error) errorText.value = data.error
        if (data.state === 'closed' || data.state === 'error') status.value = data.state
        else if (data.state === 'live') status.value = 'live'
      } catch { /* malformed status does not affect the next frame */ }
      return
    }
    const bytes = event.data instanceof Blob ? await event.data.arrayBuffer() : event.data
    const blob = new Blob([bytes], { type: 'image/jpeg' })
    const next = URL.createObjectURL(blob)
    if (frameUrl.value) URL.revokeObjectURL(frameUrl.value)
    frameUrl.value = next
    reconnectAttempts = 0
    status.value = 'live'
  }
  socket.onerror = () => {
    if (!disposed && ws === socket) errorText.value = t('sessions.browserLive.connectFailed')
  }
  socket.onclose = () => {
    if (disposed || ws !== socket) return
    if (status.value === 'closed') return
    status.value = 'error'
    if (props.active !== false && reconnectAttempts < 3) {
      reconnectAttempts += 1
      reconnectTimer = setTimeout(connect, 1000 * reconnectAttempts)
    }
  }
}

onMounted(connect)
onBeforeUnmount(() => {
  disposed = true
  disconnect()
  if (frameUrl.value) URL.revokeObjectURL(frameUrl.value)
})
watch(() => [props.sessionId, props.active], connect)

/** Mirror live status into the Stage header instead of drawing a second bar. */
watch(
  [status, statusLabel, pageTitle, pageUrl],
  () => {
    emit('statusState', {
      status: status.value,
      label: statusLabel.value,
      pageTitle: pageTitle.value,
      pageUrl: pageUrl.value,
    })
  },
  { immediate: true },
)
</script>

<template>
  <div class="browser-live">
    <div class="browser-live__viewport">
      <img v-if="frameUrl" class="browser-live__frame" :src="frameUrl" :alt="pageTitle || t('sessions.browserLive.title')">
      <div v-if="!frameUrl || status === 'error'" class="browser-live__empty">
        <template v-if="errorText">{{ errorText }}</template>
        <template v-else>{{ t('sessions.browserLive.waiting') }}</template>
      </div>
    </div>
  </div>
</template>

<style scoped>
.browser-live { display: flex; flex: 1; min-height: 0; flex-direction: column; background: var(--dq-bg-base); }
.browser-live__viewport { position: relative; display: flex; flex: 1; min-height: 0; align-items: center; justify-content: center; overflow: auto; background: color-mix(in srgb, var(--dq-bg-base) 96%, var(--dq-label-primary)); }
.browser-live__frame { display: block; width: 100%; height: 100%; object-fit: contain; }
.browser-live__empty { position: absolute; inset: 0; display: flex; align-items: center; justify-content: center; padding: 24px; text-align: center; color: var(--dq-label-tertiary); background: var(--dq-bg-base); }
</style>
