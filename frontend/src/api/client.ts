import { apiBaseUrl } from '@/utils/desktop'

/** Go 空 slice 常序列化为 JSON null，列表接口统一归一成 [] */
export function asArray<T>(data: T[] | null | undefined): T[] {
  return Array.isArray(data) ? data : []
}

export class ApiError extends Error {
  readonly status: number
  readonly path: string

  constructor(message: string, status: number, path: string) {
    super(message)
    this.name = 'ApiError'
    this.status = status
    this.path = path
  }
}

export function isNotFoundError(err: unknown): boolean {
  if (err instanceof ApiError) return err.status === 404
  const msg = err instanceof Error ? err.message : String(err ?? '')
  const lower = msg.toLowerCase()
  return lower.includes('not found') || lower.includes('404')
}

async function parseErrorMessage(res: Response): Promise<string> {
  const raw = await res.text().catch(() => '')
  let message = res.statusText || `HTTP ${res.status}`
  if (raw) {
    try {
      const parsed = JSON.parse(raw) as { error?: string }
      if (parsed?.error) message = parsed.error
      else message = raw
    } catch {
      message = raw.trim() || message
    }
  }
  return message
}

export async function fetchJSON<T>(path: string, init?: RequestInit): Promise<T> {
  // Resolve per request — module-load-time apiBaseUrl() can miss Tauri globals
  // and send /api calls to tauri://localhost (404 "not found").
  const url = `${apiBaseUrl()}/api/v1${path}`
  let res: Response
  try {
    res = await fetch(url, {
      headers: { 'Content-Type': 'application/json', ...(init?.headers as Record<string, string>) },
      ...init,
    })
  } catch (networkErr) {
    throw new Error(`网络请求失败: ${url} — ${networkErr instanceof Error ? networkErr.message : '未知错误'}`)
  }
  if (!res.ok) {
    throw new ApiError(await parseErrorMessage(res), res.status, path)
  }
  if (res.status === 204) return undefined as T
  return res.json() as Promise<T>
}

/** Multipart upload into a project directory (do not set Content-Type; browser sets boundary). */
export async function uploadProjectFile(
  projectId: string,
  file: File,
): Promise<{ ok: boolean; path: string; size: number }> {
  const path = `/projects/${projectId}/files/upload`
  const url = `${apiBaseUrl()}/api/v1${path}`
  const body = new FormData()
  body.append('file', file, file.name)
  let res: Response
  try {
    res = await fetch(url, { method: 'POST', body })
  } catch (networkErr) {
    throw new Error(`网络请求失败: ${url} — ${networkErr instanceof Error ? networkErr.message : '未知错误'}`)
  }
  if (!res.ok) {
    throw new ApiError(await parseErrorMessage(res), res.status, path)
  }
  return res.json() as Promise<{ ok: boolean; path: string; size: number }>
}
