<script setup lang="ts">
import { ref, onMounted, onBeforeUnmount, watch, nextTick } from 'vue'
import { Terminal } from '@xterm/xterm'
import { FitAddon } from '@xterm/addon-fit'
import '@xterm/xterm/css/xterm.css'
import { apiBaseUrl } from '@/utils/desktop'
import { dqFontSizePx } from '@/utils/font-size'

const props = defineProps<{
  projectId: string
  /** When false, panel may be display:none — refit/focus when it becomes true. */
  active?: boolean
}>()

const containerRef = ref<HTMLElement | null>(null)
const status = ref<'connecting' | 'connected' | 'closed' | 'error'>('connecting')
const errorText = ref('')

let term: Terminal | null = null
let fit: FitAddon | null = null
let ws: WebSocket | null = null
let resizeObserver: ResizeObserver | null = null
let themeObserver: MutationObserver | null = null
let fitTries = 0
let disposed = false

function readCssVar(name: string, fallback: string): string {
  const value = getComputedStyle(document.documentElement).getPropertyValue(name).trim()
  return value || fallback
}

function terminalTheme() {
  // Transparent canvas — drawer glass is the only surface (no nested fill).
  return {
    background: 'rgba(0, 0, 0, 0)',
    foreground: readCssVar('--dq-label-primary', '#d4d4d4'),
    cursor: readCssVar('--dq-accent', '#0a84ff'),
    cursorAccent: readCssVar('--dq-on-accent', '#ffffff'),
    selectionBackground: readCssVar('--dq-selection-bg', readCssVar('--dq-accent-tint', 'rgba(10, 132, 255, 0.22)')),
    selectionForeground: readCssVar('--dq-label-primary', '#111111'),
    black: readCssVar('--dq-label-primary', '#111111'),
    red: readCssVar('--dq-danger', '#ff3b30'),
    green: readCssVar('--dq-success', '#34c759'),
    yellow: readCssVar('--dq-warning', '#ff9500'),
    blue: readCssVar('--dq-accent', '#007aff'),
    magenta: '#af52de',
    cyan: '#5ac8fa',
    white: readCssVar('--dq-label-primary', '#111111'),
    brightBlack: readCssVar('--dq-label-tertiary', '#888888'),
    brightRed: readCssVar('--dq-danger', '#ff3b30'),
    brightGreen: readCssVar('--dq-success', '#34c759'),
    brightYellow: readCssVar('--dq-warning', '#ff9500'),
    brightBlue: readCssVar('--dq-accent', '#007aff'),
    brightMagenta: '#af52de',
    brightCyan: '#5ac8fa',
    brightWhite: readCssVar('--dq-label-primary', '#111111'),
  }
}

function applyTheme() {
  if (!term) return
  term.options.theme = terminalTheme()
}

function wsUrl(): string {
  const httpBase = apiBaseUrl() || window.location.origin
  const u = new URL(`${httpBase}/api/v1/projects/${props.projectId}/terminal`)
  u.protocol = u.protocol === 'https:' ? 'wss:' : 'ws:'
  // Pass fitted size so the PTY/shell starts with a real geometry (avoids zsh "%").
  if (term && term.cols > 0 && term.rows > 0) {
    u.searchParams.set('cols', String(term.cols))
    u.searchParams.set('rows', String(term.rows))
  }
  return u.toString()
}

function sendResize() {
  if (!term || !ws || ws.readyState !== WebSocket.OPEN) return
  ws.send(JSON.stringify({ type: 'resize', cols: term.cols, rows: term.rows }))
}

function refit(focus = false) {
  if (disposed || !containerRef.value || !fit || !term) return
  const { width, height } = containerRef.value.getBoundingClientRect()
  if (width <= 2 || height <= 2) {
    if (fitTries < 30) {
      fitTries += 1
      window.setTimeout(() => refit(focus), 50)
    }
    return
  }
  fitTries = 0
  try {
    fit.fit()
  } catch {
    /* ignore fit races while unmounting */
  }
  sendResize()
  if (focus) term.focus()
}

function scheduleRefit(focus = false) {
  fitTries = 0
  nextTick(() => {
    requestAnimationFrame(() => refit(focus))
  })
}

function connect() {
  if (disposed) return
  if (ws) {
    ws.onclose = null
    ws.onerror = null
    ws.close()
    ws = null
  }
  status.value = 'connecting'
  errorText.value = ''
  let socket: WebSocket
  try {
    socket = new WebSocket(wsUrl())
  } catch (e) {
    status.value = 'error'
    errorText.value = e instanceof Error ? e.message : '无法创建终端连接'
    return
  }
  socket.binaryType = 'arraybuffer'
  socket.onopen = () => {
    if (disposed || ws !== socket) return
    status.value = 'connected'
    scheduleRefit(true)
  }
  socket.onmessage = (ev) => {
    if (disposed) return
    if (typeof ev.data === 'string') {
      term?.write(ev.data)
    } else {
      term?.write(new Uint8Array(ev.data as ArrayBuffer))
    }
  }
  socket.onerror = () => {
    if (disposed || ws !== socket) return
    errorText.value = '终端连接失败'
  }
  socket.onclose = () => {
    if (disposed || ws !== socket) return
    status.value = status.value === 'connecting' ? 'error' : 'closed'
    if (status.value === 'error' && !errorText.value) {
      errorText.value = '终端连接被关闭'
    }
    term?.write('\r\n\x1b[90m[终端连接已断开]\x1b[0m\r\n')
  }
  ws = socket
}

function reconnect() {
  term?.reset()
  connect()
}

onMounted(() => {
  disposed = false
  term = new Terminal({
    cursorBlink: true,
    fontSize: Math.max(12, dqFontSizePx('--dq-font-size-caption')),
    fontFamily: 'ui-monospace, "SF Mono", Monaco, Menlo, Consolas, monospace',
    theme: terminalTheme(),
    scrollback: 5000,
    allowProposedApi: true,
  })
  fit = new FitAddon()
  term.loadAddon(fit)
  if (containerRef.value) {
    term.open(containerRef.value)
    applyTheme()
    try {
      fit.fit()
    } catch {
      /* ignore */
    }
  }
  term.onData((data) => {
    if (ws?.readyState === WebSocket.OPEN) {
      ws.send(JSON.stringify({ type: 'input', data }))
    }
  })
  term.onResize(() => sendResize())

  resizeObserver = new ResizeObserver(() => scheduleRefit(false))
  if (containerRef.value) resizeObserver.observe(containerRef.value)

  themeObserver = new MutationObserver(() => applyTheme())
  themeObserver.observe(document.documentElement, { attributes: true, attributeFilter: ['class'] })

  // Fit first, then connect so the server can StartWithSize(cols, rows).
  scheduleRefit(false)
  window.setTimeout(() => {
    if (!disposed) connect()
  }, 0)
  window.setTimeout(() => scheduleRefit(true), 120)
  window.setTimeout(() => scheduleRefit(true), 360)
})

watch(
  () => props.projectId,
  () => {
    term?.reset()
    connect()
    scheduleRefit(true)
  },
)

watch(
  () => props.active,
  (active) => {
    if (active) scheduleRefit(true)
  },
)

onBeforeUnmount(() => {
  disposed = true
  themeObserver?.disconnect()
  themeObserver = null
  resizeObserver?.disconnect()
  resizeObserver = null
  if (ws) {
    ws.onclose = null
    ws.onerror = null
    ws.close()
    ws = null
  }
  term?.dispose()
  term = null
  fit = null
})

defineExpose({ refit: () => scheduleRefit(true) })
</script>

<template>
  <div class="terminal-panel">
    <div ref="containerRef" class="terminal-panel__term" />
    <div v-if="status === 'closed' || status === 'error'" class="terminal-panel__overlay">
      <p v-if="errorText" class="terminal-panel__error">{{ errorText }}</p>
      <DqButton size="sm" class="terminal-panel__reconnect" @click="reconnect">重新连接</DqButton>
    </div>
    <div v-else-if="status === 'connecting'" class="terminal-panel__status">正在连接终端…</div>
  </div>
</template>

<style scoped>
.terminal-panel {
  position: absolute;
  inset: 0;
  display: flex;
  flex-direction: column;
  overflow: hidden;
  background: transparent;
}

.terminal-panel__term {
  flex: 1 1 auto;
  min-height: 0;
  width: 100%;
  height: 100%;
  padding: 8px 4px 8px 10px;
  background: transparent;
}

.terminal-panel__term :deep(.xterm),
.terminal-panel__term :deep(.xterm-screen),
.terminal-panel__term :deep(.xterm-helpers) {
  height: 100%;
  width: 100%;
  padding: 0;
  background: transparent !important;
}

.terminal-panel__term :deep(.xterm-viewport) {
  overflow-y: auto !important;
  background: transparent !important;
}

.terminal-panel__term :deep(canvas) {
  /* Ensure WebGL/canvas path doesn't paint an opaque slab */
  background: transparent !important;
}

.terminal-panel__status {
  position: absolute;
  left: 12px;
  bottom: 10px;
  z-index: 2;
  font-size: var(--dq-font-size-caption);
  color: var(--dq-label-tertiary);
  pointer-events: none;
}

.terminal-panel__overlay {
  position: absolute;
  inset: 0;
  z-index: 3;
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  gap: 10px;
  background: var(--dq-overlay-medium);
  -webkit-backdrop-filter: blur(6px);
  backdrop-filter: blur(6px);
}

.terminal-panel__error {
  margin: 0 16px;
  text-align: center;
  font-size: var(--dq-font-size-body);
  color: var(--dq-label-primary);
}

.terminal-panel__reconnect {
  border-radius: var(--dq-radius-button);
}
</style>
