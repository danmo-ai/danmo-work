import { apiBaseUrl } from '@/utils/desktop'
import { fetchJSON, asArray, ApiError } from '@/api/client'

export interface GitFileChange {
  status: string
  file: string
  origFile?: string
  staged: boolean
}

export interface GitChanges {
  branch: string
  ahead?: number
  behind?: number
  hasRemote?: boolean
  changes: GitFileChange[]
  error?: string
  code?: string
}

export interface GitBranches {
  current: string
  branches: string[] | null
  error?: string
  code?: string
}

export interface GitRemote {
  name: string
  fetchUrl: string
  pushUrl?: string
}

export interface GitRemotes {
  remotes: GitRemote[]
  error?: string
  code?: string
}

export interface GitCredentialInfo {
  host: string
  user: string
  hasToken: boolean
}

export interface GitCommitInfo {
  hash: string
  short: string
  author: string
  date: string
  subject: string
}

export interface GitLog {
  commits: GitCommitInfo[]
  error?: string
  code?: string
}

export interface GitStreamEvent {
  type: 'line' | 'done' | 'error'
  data?: string
  exit?: number
}

export function gitChanges(projectId: string): Promise<GitChanges> {
  return fetchJSON<GitChanges>(`/projects/${projectId}/git-changes`).then(d => ({
    ...d,
    changes: asArray(d.changes),
  }))
}

export function gitBranches(projectId: string): Promise<GitBranches> {
  return fetchJSON<GitBranches>(`/projects/${projectId}/git-branches`)
}

export function gitCheckout(projectId: string, branch: string): Promise<GitBranches> {
  return fetchJSON<GitBranches>(`/projects/${projectId}/git-checkout`, {
    method: 'POST',
    body: JSON.stringify({ branch }),
  })
}

export function gitRemotes(projectId: string): Promise<GitRemotes> {
  return fetchJSON<GitRemotes>(`/projects/${projectId}/git-remotes`).then(d => ({
    ...d,
    remotes: asArray(d.remotes),
  }))
}

export function gitAddRemote(projectId: string, name: string, url: string): Promise<GitRemotes> {
  return fetchJSON<GitRemotes>(`/projects/${projectId}/git-remote/add`, {
    method: 'POST',
    body: JSON.stringify({ name, url }),
  })
}

export function gitCredentials(): Promise<GitCredentialInfo[]> {
  return fetchJSON<{ credentials: GitCredentialInfo[] }>(`/git-credentials`).then(d =>
    asArray(d.credentials),
  )
}

export function gitSaveCredential(
  projectId: string,
  host: string,
  username: string,
  token: string,
): Promise<GitCredentialInfo[]> {
  return fetchJSON<{ credentials: GitCredentialInfo[] }>(`/projects/${projectId}/git-credentials`, {
    method: 'POST',
    body: JSON.stringify({ host, username, token }),
  }).then(d => asArray(d.credentials))
}

export function gitDeleteCredential(host: string): Promise<{ ok: boolean }> {
  return fetchJSON<{ ok: boolean }>(`/git-credentials?host=${encodeURIComponent(host)}`, {
    method: 'DELETE',
  })
}

export function gitStage(
  projectId: string,
  files: string[],
  staged: boolean,
): Promise<GitChanges> {
  return fetchJSON<GitChanges>(`/projects/${projectId}/git-stage`, {
    method: 'POST',
    body: JSON.stringify({ files, staged }),
  }).then(d => ({ ...d, changes: asArray(d.changes) }))
}

export function gitCommit(projectId: string, message: string): Promise<GitLog> {
  return fetchJSON<GitLog>(`/projects/${projectId}/git-commit`, {
    method: 'POST',
    body: JSON.stringify({ message }),
  })
}

export function gitLog(projectId: string, limit = 20): Promise<GitLog> {
  return fetchJSON<GitLog>(`/projects/${projectId}/git-log?limit=${limit}`).then(d => ({
    ...d,
    commits: asArray(d.commits),
  }))
}

/**
 * Streams a git pull/push/fetch over SSE, invoking onEvent per parsed event.
 * Resolves with the final event (done/error) or rejects on HTTP error.
 */
export async function streamGitOp(
  projectId: string,
  op: 'pull' | 'push' | 'fetch',
  onEvent: (ev: GitStreamEvent) => void,
  signal?: AbortSignal,
): Promise<GitStreamEvent> {
  const url = `${apiBaseUrl()}/api/v1/projects/${projectId}/git-stream?op=${op}`
  let res: Response
  try {
    res = await fetch(url, { signal, headers: { Accept: 'text/event-stream' } })
  } catch (networkErr) {
    if (signal?.aborted) throw new Error('已取消')
    throw new Error(`网络请求失败: ${url} — ${networkErr instanceof Error ? networkErr.message : '未知错误'}`)
  }
  if (!res.ok) {
    let message = res.statusText || `HTTP ${res.status}`
    try {
      const parsed = (await res.json()) as { error?: string }
      if (parsed?.error) message = parsed.error
    } catch {
      /* keep status text */
    }
    throw new ApiError(message, res.status, url)
  }
  if (!res.body) throw new Error('响应无数据流')

  const reader = res.body.getReader()
  const decoder = new TextDecoder()
  let buffer = ''
  let last: GitStreamEvent = { type: 'error', data: '流已结束' }

  const dispatch = (block: string) => {
    for (const line of block.split('\n')) {
      if (!line.startsWith('data:')) continue
      const payload = line.slice(5).trim()
      if (!payload) continue
      try {
        const ev = JSON.parse(payload) as GitStreamEvent
        last = ev
        onEvent(ev)
      } catch {
        /* skip malformed frame */
      }
    }
  }

  for (;;) {
    const { done, value } = await reader.read()
    if (done) break
    buffer += decoder.decode(value, { stream: true })
    let idx: number
    while ((idx = buffer.indexOf('\n\n')) >= 0) {
      const block = buffer.slice(0, idx)
      buffer = buffer.slice(idx + 2)
      dispatch(block)
    }
  }
  if (buffer.trim()) dispatch(buffer)
  return last
}
