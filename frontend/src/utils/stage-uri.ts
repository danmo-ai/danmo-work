import type { FileStageState } from '@/stores/workspaceUi'

/**
 * The Stage URI selects a renderer directly; it is not wrapped in danmo://.
 *
 * Two families only:
 *   - `https://…`      外部网址，经后端代理渲染
 *   - `file:///…`      项目内文件，由 `/projects/:id/raw/…` 提供；kind/engine
 *                      决定用哪个 surface，所以图片、Vue 组件都不会降级
 * 另有 `browser://`（Agent 浏览器）和 `vnc://`（沙箱桌面）两个非文件目标。
 */
export function stageUri(stage: FileStageState): string {
  if (stage.kind === 'browser-live') return `browser://${encodeURIComponent(stage.sessionId ?? '')}`
  if (stage.kind === 'desktop') return `vnc://${encodeURIComponent(stage.sessionId ?? '')}`
  if (stage.kind === 'web' || stage.kind === 'media') {
    if (stage.url) return stage.url // external address is its own identity
    if (!stage.path) return '' // nothing loaded yet — bar shows the placeholder
  }
  const params = new URLSearchParams({ kind: stage.kind, mode: stage.mode })
  if (stage.engine) params.set('engine', stage.engine)
  // file:///<path> keeps one leading slash so relative project paths parse back
  // through `pathname` instead of being read as a hostname.
  const path = stage.path.split('/').map(encodeURIComponent).join('/')
  return `file:///${path}?${params.toString()}`
}

/** Parse a user-facing Stage address into its renderer and resource identity. */
export function stageFromUri(uri: string): FileStageState | null {
  const raw = uri.trim()
  try {
    const u = new URL(raw)
    const id = decodeURIComponent(u.hostname || u.pathname.replace(/^\//, ''))
    // `browser://<sessionId>` is the only accepted scheme for the agent browser.
    if (u.protocol === 'browser:' && id) {
      return { kind: 'browser-live', path: '', mode: 'view', engine: 'screencast', sessionId: id }
    }
    if (u.protocol === 'vnc:') {
      return { kind: 'desktop', path: '', mode: 'view', engine: 'novnc', sessionId: id }
    }
    if ((u.protocol === 'http:' || u.protocol === 'https:') && u.hostname) {
      return { kind: 'web', path: '', mode: 'view', engine: 'iframe', url: raw }
    }
    // Project files have exactly one form: file:///<path>?kind=&mode=&engine=.
    if (u.protocol === 'file:' && u.searchParams.has('kind')) {
      const kind = u.searchParams.get('kind') as FileStageState['kind'] | null
      if (!kind || kind === 'browser-live' || kind === 'desktop') return null
      const mode = u.searchParams.get('mode') as FileStageState['mode'] | null
      const engine = u.searchParams.get('engine') as FileStageState['engine'] | null
      const url = u.searchParams.get('url')
      const parsed: FileStageState = {
        kind,
        path: decodeURIComponent(u.pathname.replace(/^\//, '')),
        mode: mode === 'edit' || mode === 'present' ? mode : 'view',
      }
      // Omit absent keys so round-tripped state compares equal to the original.
      if (engine) parsed.engine = engine
      if (url) parsed.url = url
      return parsed
    }
  } catch { /* invalid addresses do not replace the active surface */ }
  return null
}
