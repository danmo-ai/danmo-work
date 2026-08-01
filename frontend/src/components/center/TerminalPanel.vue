<script setup lang="ts">
import { ref, onMounted, onBeforeUnmount, watch } from 'vue'
import { Terminal } from '@xterm/xterm'
import { FitAddon } from '@xterm/addon-fit'
import '@xterm/xterm/css/xterm.css'
import { apiBaseUrl } from '@/utils/desktop'
import { dqFontSizePx } from '@/utils/font-size'

const props = defineProps<{ projectId: string }>()

const containerRef = ref<HTMLElement | null>(null)
const status = ref<'connecting' | 'connected' | 'closed'>('connecting')

let term: Terminal | null = null
let fit: FitAddon | null = null
let ws: WebSocket | null = null
let resizeObserver: ResizeObserver | null = null
let themeObserver: MutationObserver | null = null

function readCssVar(name: string, fallback: string): string {
  const value = getComputedStyle(document.documentElement).getPropertyValue(name).trim()
  return value || fallback
}

function terminalTheme() {
  return {
    background: readCssVar('--dq-bg-base', '#1e1e1e'),
    foreground: readCssVar('--dq-label-primary', '#d4d4d4'),
    cursor: readCssVar('--dq-accent', '#d4d4d4'),
    selectionBackground: readCssVar('--dq-accent-tint', 'rgba(10, 132, 255, 0.35)'),
    black: readCssVar('--dq-bg-page', '#000000'),
    brightWhite: readCssVar('--dq-label-primary', '#ffffff'),
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
  return u.toString()
}

function sendResize() {
  if (!term || !ws || ws.readyState !== WebSocket.OPEN) return
  ws.send(JSON.stringify({ type: 'resize', cols: term.cols, rows: term.rows }))
}

function connect() {
  if (ws) {
    ws.onclose = null
    ws.close()
    ws = null
  }
  status.value = 'connecting'
  const socket = new WebSocket(wsUrl())
  socket.binaryType = 'arraybuffer'
  socket.onopen = () => {
    status.value = 'connected'
    fit?.fit()
    sendResize()
    term?.focus()
  }
  socket.onmessage = (ev) => {
    if (typeof ev.data === 'string') {
      term?.write(ev.data)
    } else {
      term?.write(new Uint8Array(ev.data as ArrayBuffer))
    }
  }
  socket.onclose = () => {
    if (ws !== socket) return
    status.value = 'closed'
    term?.write('\r\n\x1b[90m[终端连接已断开]\x1b[0m\r\n')
  }
  ws = socket
}

function reconnect() {
  term?.reset()
  connect()
}

onMounted(() => {
  term = new Terminal({
    cursorBlink: true,
    fontSize: dqFontSizePx('--dq-font-size-caption'),
    fontFamily: 'var(--dq-font-mono), "SF Mono", Monaco, Menlo, Consolas, monospace',
    theme: terminalTheme(),
    scrollback: 5000,
  })
  fit = new FitAddon()
  term.loadAddon(fit)
  if (containerRef.value) {
    term.open(containerRef.value)
    fit.fit()
  }
  term.onData((data) => {
    if (ws?.readyState === WebSocket.OPEN) {
      ws.send(JSON.stringify({ type: 'input', data }))
    }
  })
  term.onResize(() => sendResize())

  resizeObserver = new ResizeObserver(() => {
    if (!containerRef.value) return
    const { width, height } = containerRef.value.getBoundingClientRect()
    if (width > 0 && height > 0) fit?.fit()
  })
  if (containerRef.value) resizeObserver.observe(containerRef.value)

  // Re-apply when html class list changes (theme switch)
  themeObserver = new MutationObserver(() => applyTheme())
  themeObserver.observe(document.documentElement, { attributes: true, attributeFilter: ['class'] })

  connect()
})

watch(() => props.projectId, () => {
  term?.reset()
  connect()
})

onBeforeUnmount(() => {
  themeObserver?.disconnect()
  themeObserver = null
  resizeObserver?.disconnect()
  resizeObserver = null
  if (ws) {
    ws.onclose = null
    ws.close()
    ws = null
  }
  term?.dispose()
  term = null
  fit = null
})
</script>

<template>
  <div class="terminal-panel">
    <div ref="containerRef" class="terminal-panel__term" />
    <div v-if="status === 'closed'" class="terminal-panel__overlay">
      <DqButton class="terminal-panel__reconnect" @click="reconnect">重新连接</DqButton>
    </div>
  </div>
</template>

<style scoped>
.terminal-panel {
  position: relative;
  flex: 1;
  min-height: 0;
  min-width: 0;
  display: flex;
  flex-direction: column;
  overflow: hidden;
  background: var(--dq-bg-base);
}

.terminal-panel__term {
  flex: 1;
  min-height: 0;
  padding: 6px 0 6px 8px;
}

.terminal-panel__term :deep(.xterm) {
  height: 100%;
}

.terminal-panel__overlay {
  position: absolute;
  inset: 0;
  display: flex;
  align-items: center;
  justify-content: center;
  background: var(--dq-overlay-medium);
}

.terminal-panel__reconnect {
  border-radius: var(--dq-radius-button);
}
</style>
