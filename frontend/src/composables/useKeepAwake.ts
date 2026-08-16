import { watch } from 'vue'
import { isTauriRuntime } from '@/utils/desktop'

/**
 * Keeps the machine awake while `isActive` returns true (desktop shell only).
 * Calls the Rust `prevent_sleep` / `allow_sleep` commands; the OS assertion
 * (IOPM / SetThreadExecutionState / systemd-inhibit) is held until released
 * or the app exits. Browser runtimes are a no-op.
 *
 * Returns a cleanup that stops watching and releases the assertion.
 */
export function useKeepAwake(isActive: () => boolean): (() => void) | undefined {
  if (!isTauriRuntime()) return undefined

  let held = false
  let chain: Promise<void> = Promise.resolve()

  function setHeld(on: boolean) {
    chain = chain.then(async () => {
      if (on === held) return
      try {
        const { invoke } = await import('@tauri-apps/api/core')
        await invoke(on ? 'prevent_sleep' : 'allow_sleep')
        held = on
      } catch {
        /* transient failure; next toggle retries */
      }
    })
  }

  const stop = watch(isActive, (on) => setHeld(Boolean(on)), { immediate: true })

  return () => {
    stop()
    setHeld(false)
  }
}
