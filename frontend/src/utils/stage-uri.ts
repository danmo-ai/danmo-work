import type { FileStageState } from '@/stores/workspaceUi'

/** The Stage URI selects a renderer directly; it is not wrapped in danmo://. */
export function stageUri(stage: FileStageState): string {
  if (stage.kind === 'browser-live') return `agent://${encodeURIComponent(stage.sessionId ?? '')}`
  if (stage.kind === 'desktop') return `vnc://${encodeURIComponent(stage.sessionId ?? '')}`
  if (stage.kind === 'web' || stage.kind === 'media') return stage.url || (stage.path ? `project://${encodeURIComponent(stage.path)}` : 'https://example.com/')
  const params = new URLSearchParams({ kind: stage.kind, mode: stage.mode })
  if (stage.engine) params.set('engine', stage.engine)
  return `file://${encodeURIComponent(stage.path)}?${params.toString()}`
}

/** Parse a user-facing Stage address into its renderer and resource identity. */
export function stageFromUri(uri: string): FileStageState | null {
  const raw = uri.trim()
  try {
    const u = new URL(raw)
    const id = decodeURIComponent(u.hostname || u.pathname.replace(/^\//, ''))
    if (u.protocol === 'agent:' && id) {
      return { kind: 'browser-live', path: '', mode: 'view', engine: 'screencast', sessionId: id }
    }
    if (u.protocol === 'vnc:') {
      return { kind: 'desktop', path: '', mode: 'view', engine: 'novnc', sessionId: id }
    }
    if ((u.protocol === 'http:' || u.protocol === 'https:') && u.hostname) {
      return { kind: 'web', path: '', mode: 'view', engine: 'iframe', url: raw }
    }
    if (u.protocol === 'project:') {
      return { kind: 'web', path: id, mode: 'view', engine: 'iframe' }
    }
    if (u.protocol === 'file:' && u.searchParams.has('kind')) {
      const kind = u.searchParams.get('kind') as FileStageState['kind'] | null
      if (!kind || kind === 'browser-live' || kind === 'desktop') return null
      const mode = u.searchParams.get('mode') as FileStageState['mode'] | null
      return {
        kind,
        path: decodeURIComponent(u.pathname.replace(/^\//, '')),
        mode: mode === 'edit' || mode === 'present' ? mode : 'view',
        engine: (u.searchParams.get('engine') ?? undefined) as FileStageState['engine'],
        url: u.searchParams.get('url') ?? undefined,
      }
    }
  } catch { /* invalid addresses do not replace the active surface */ }
  return null
}
