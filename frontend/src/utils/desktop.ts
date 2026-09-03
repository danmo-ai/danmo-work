/** True when UI runs inside Tauri (desktop shell). */
export function isTauriRuntime(): boolean {
  const w = window as Window & { __TAURI_INTERNALS__?: unknown; __TAURI__?: unknown };
  return Boolean(w.__TAURI_INTERNALS__ ?? w.__TAURI__);
}

/**
 * Resolve the API base URL for backend requests.
 * - VITE_API_BASE_URL (build-time) takes highest priority.
 * - In Tauri desktop runtime the webview may load from a custom protocol
 *   (e.g. tauri://localhost) where relative URLs cannot reach the Go backend,
 *   so we fall back to the absolute localhost address.
 * - Otherwise return empty string (same-origin, proxied by Vite dev server).
 */
export function apiBaseUrl(): string {
  return import.meta.env.VITE_API_BASE_URL ?? (isTauriRuntime() ? 'http://127.0.0.1:7801' : '');
}

/**
 * Project file URL for iframe / media preview (`/projects/:id/raw/...`).
 * Encodes each path segment but keeps `/` so relative HTML assets resolve
 * against the file's directory (backend also injects `<base>` for HTML).
 */
export function projectRawUrl(projectId: string, filePath: string): string {
  const encoded = filePath
    .replace(/\\/g, '/')
    .split('/')
    .map((seg) => encodeURIComponent(seg))
    .join('/')
  return `${apiBaseUrl()}/api/v1/projects/${projectId}/raw/${encoded}`
}

/**
 * Wait until the local Go backend accepts HTTP (desktop first-launch race).
 * Sidecar spawn ≠ ready: migrate/SQLite can delay listen on first open.
 * Non-desktop runtimes return true immediately.
 */
export async function waitForBackend(opts?: {
  timeoutMs?: number
  intervalMs?: number
}): Promise<boolean> {
  if (!isTauriRuntime()) return true

  const timeoutMs = opts?.timeoutMs ?? 45_000
  const intervalMs = opts?.intervalMs ?? 250
  const url = `${apiBaseUrl()}/api/v1/version`
  const deadline = Date.now() + timeoutMs

  while (Date.now() < deadline) {
    try {
      const res = await fetch(url, { method: 'GET', cache: 'no-store' })
      if (res.ok) return true
    } catch {
      /* connection refused / not listening yet */
    }
    await new Promise((r) => setTimeout(r, intervalMs))
  }
  return false
}

/** Overlay title bar + transparent window styles (macOS Tauri only). */
export function installTauriMacosShell(): void {
  if (!isTauriRuntime()) return;
  const platform = navigator.platform.toLowerCase();
  const ua = navigator.userAgent.toLowerCase();
  if (!platform.includes('mac') && !ua.includes('mac')) return;
  document.documentElement.classList.add('dq-tauri-macos');
}

export type SaveBlobFilter = { name: string; extensions: string[] }

export type SaveBlobResult =
  | { ok: true; path?: string; method: 'dialog' | 'picker' | 'download' }
  | { ok: false; cancelled: true }

const EXT_MIME: Record<string, string> = {
  zip: 'application/zip',
  pdf: 'application/pdf',
  md: 'text/markdown',
  txt: 'text/plain',
  json: 'application/json',
  png: 'image/png',
  jpg: 'image/jpeg',
  jpeg: 'image/jpeg',
}

function pickerAccept(filters?: SaveBlobFilter[]): Record<string, string[]> {
  const list = filters?.length ? filters : [{ name: 'Zip', extensions: ['zip'] }]
  const accept: Record<string, string[]> = {}
  for (const f of list) {
    for (const ext of f.extensions) {
      const key = ext.replace(/^\./, '').toLowerCase()
      const mime = EXT_MIME[key] || 'application/octet-stream'
      const dotted = `.${key}`
      ;(accept[mime] ??= []).push(dotted)
    }
  }
  return accept
}

async function resolveBlob(blobOrFactory: Blob | (() => Promise<Blob>)): Promise<Blob> {
  return typeof blobOrFactory === 'function' ? await blobOrFactory() : blobOrFactory
}

/**
 * Save a Blob with a location prompt when possible.
 * - Tauri desktop: native Save dialog, then write via sidecar command.
 * - Chromium: File System Access `showSaveFilePicker`.
 * - Otherwise: browser default download (usually ~/Downloads), no folder picker.
 *
 * Pass a factory `() => Promise<Blob>` to open the dialog first (keeps the
 * user-gesture chain), then build bytes only after the path is chosen.
 */
export async function saveBlobAs(
  blobOrFactory: Blob | (() => Promise<Blob>),
  fileName: string,
  opts?: { filters?: SaveBlobFilter[]; defaultPath?: string },
): Promise<SaveBlobResult> {
  const filters = opts?.filters ?? [{ name: 'Zip', extensions: ['zip'] }]
  const suggestedName = fileName.replace(/\\/g, '/').split('/').pop() || fileName

  if (isTauriRuntime()) {
    const { save } = await import('@tauri-apps/plugin-dialog')
    const { invoke } = await import('@tauri-apps/api/core')
    const path = await save({
      defaultPath: opts?.defaultPath || suggestedName,
      filters,
    })
    if (!path) return { ok: false, cancelled: true }
    const blob = await resolveBlob(blobOrFactory)
    const buf = new Uint8Array(await blob.arrayBuffer())
    await invoke('write_file_bytes', { path, contents: Array.from(buf) })
    return { ok: true, path, method: 'dialog' }
  }

  const w = window as Window & {
    showSaveFilePicker?: (options?: {
      suggestedName?: string
      types?: Array<{ description?: string; accept: Record<string, string[]> }>
    }) => Promise<FileSystemFileHandle>
  }
  if (typeof w.showSaveFilePicker === 'function') {
    try {
      const handle = await w.showSaveFilePicker({
        suggestedName,
        types: [
          {
            description: filters[0]?.name ?? 'File',
            accept: pickerAccept(filters),
          },
        ],
      })
      const blob = await resolveBlob(blobOrFactory)
      const writable = await handle.createWritable()
      await writable.write(blob)
      await writable.close()
      return { ok: true, path: handle.name, method: 'picker' }
    } catch (e) {
      // User cancelled, or API denied — don't fall through to silent download.
      if (e instanceof DOMException && e.name === 'AbortError') {
        return { ok: false, cancelled: true }
      }
      throw e
    }
  }

  const blob = await resolveBlob(blobOrFactory)
  const objectUrl = URL.createObjectURL(blob)
  const a = document.createElement('a')
  a.href = objectUrl
  a.download = suggestedName
  a.style.display = 'none'
  document.body.appendChild(a)
  a.click()
  a.remove()
  window.setTimeout(() => URL.revokeObjectURL(objectUrl), 1000)
  return { ok: true, method: 'download' }
}
