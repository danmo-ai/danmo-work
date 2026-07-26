import { apiBaseUrl } from '@/utils/desktop'

const base = apiBaseUrl()

/** Go 空 slice 常序列化为 JSON null，列表接口统一归一成 [] */
export function asArray<T>(data: T[] | null | undefined): T[] {
  return Array.isArray(data) ? data : []
}

/** HTTP API failure with status for soft-fail / not-found handling. */
export class ApiError extends Error {
  readonly status: number

  constructor(status: number, message: string) {
    super(message)
    this.name = 'ApiError'
    this.status = status
  }
}

/** True when an API/network error represents HTTP 404 / missing resource. */
export function isNotFoundError(err: unknown): boolean {
  if (err instanceof ApiError) return err.status === 404
  const msg = err instanceof Error ? err.message : String(err)
  return /\b404\b|not found/i.test(msg)
}

export async function fetchJSON<T>(path: string, init?: RequestInit): Promise<T> {
  const url = `${base}/api/v1${path}`
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
    const text = await res.text().catch(() => '')
    let message = res.statusText || `HTTP ${res.status}`
    if (text) {
      try {
        const err = JSON.parse(text) as { error?: string }
        if (err?.error) message = err.error
      } catch {
        // Gin default 404 body is plain text: "404 page not found"
        message = text.trim() || message
      }
    }
    throw new ApiError(res.status, message)
  }
  if (res.status === 204) return undefined as T
  return res.json() as Promise<T>
}
