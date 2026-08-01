import { createApp } from 'vue'
import { createPinia } from 'pinia'
import { registerDqIcons } from '@danqing/dq-shell'
import App from './App.vue'
import { router } from './router'
import { installDanQingUi } from './plugins/dq-ui'
import { i18n } from './i18n'
import { useThemeStore } from './stores/theme'
import '@danqing/dq-tokens/dq-mac.css'
import '@danqing/dq-tokens/dq-linear-dark.css'
import '@danqing/dq-tokens/dq-china-red-dark.css'
import '@danqing/dq-tokens/dq-shadcn-dark.css'
import '@danqing/dq-tokens/dq-shadcn-light.css'
import '@danqing/dq-tokens/dq-catppuccin.css'
import '@danqing/dq-tokens/dq-tokyo-night.css'
import '@danqing/dq-tokens/dq-minimal-light.css'
import '@danqing/dq-tokens/dq-dracula.css'
import '@danqing/dq-tokens/dq-nord-dark.css'
import '@danqing/dq-tokens/dq-catppuccin-latte.css'
import '@danqing/dq-tokens/dq-nord-light.css'
import '@danqing/dq-tokens/dq-github-light.css'
import '@danqing/dq-tokens/dq-glass.css'
import '@danqing/dq-tokens/dq-agent.css'
import '@danqing/dq-ui/style.css'
import '@danqing/dq-shell/style.css'
import '@/styles/work.css'

// Theme class is managed by theme store; default flash before init is Tokyo Night
document.documentElement.classList.add('dq-tokyo-night', 'dark')

function isRecoverableRuntimeError(err: unknown): boolean {
  if (!err) return false
  // API / network failures must not blank the whole desktop shell.
  if (typeof err === 'object' && err !== null && 'status' in err && 'path' in err) return true
  const msg = (err instanceof Error ? err.message : String(err)).toLowerCase()
  return (
    msg.includes('not found') ||
    msg.includes('readonly database') ||
    msg.includes('database not writable') ||
    msg.includes('网络请求失败') ||
    msg.includes('failed to fetch') ||
    msg.includes('http ')
  )
}

function showFatalError(err: unknown) {
  if (isRecoverableRuntimeError(err)) {
    // eslint-disable-next-line no-console
    console.error('[runtime]', err)
    return
  }
  const container = document.getElementById('app')
  const msg = err instanceof Error ? err.message : String(err)
  const stack = err instanceof Error ? err.stack : ''
  if (container) {
    container.innerHTML = `<div style="padding:20px;font-family:ui-monospace,monospace;color:var(--dq-danger, #ff453a);background:var(--dq-bg-page, #0a0a0a);white-space:pre-wrap;word-break:break-word;"><strong>Fatal Error:</strong> ${msg}\n${stack}</div>`
  }
  // eslint-disable-next-line no-console
  console.error('[FATAL]', err)
}

window.addEventListener('error', (e) => showFatalError(e.error))
window.addEventListener('unhandledrejection', (e) => {
  if (isRecoverableRuntimeError(e.reason)) {
    e.preventDefault()
  }
  showFatalError(e.reason)
})

const app = createApp(App)
registerDqIcons(app)
const pinia = createPinia()
app.use(pinia)
app.use(router)
app.use(i18n)
installDanQingUi(app)

// Initialize theme from stored preference
const themeStore = useThemeStore()
themeStore.init()

app.mount('#app')

app.config.errorHandler = (err, vm, info) => {
  // eslint-disable-next-line no-console
  console.error('[Vue Error]', err, info, vm)
  showFatalError(err)
}
