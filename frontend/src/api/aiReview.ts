import { fetchJSON } from '@/api/client'

export interface AiReviewStatus {
  turnId: string
  path: string
  changed: boolean
  baseHash: string
  currentHash?: string
  hasSnapshot: boolean
  canRevert: boolean
  hashOnly?: boolean
  missingFile?: boolean
}

export interface AiReviewDiff {
  path: string
  turnId: string
  patch: string
  changed: boolean
  canRevert: boolean
  hashOnly?: boolean
  error?: string
}

export async function fetchAiReviewStatus(sessionId: string, turnId: string, path: string) {
  const q = new URLSearchParams({ turnId, path })
  return fetchJSON<AiReviewStatus>(`/sessions/${sessionId}/ai-review/status?${q}`)
}

export async function fetchAiReviewDiff(sessionId: string, turnId: string, path: string) {
  const q = new URLSearchParams({ turnId, path })
  return fetchJSON<AiReviewDiff>(`/sessions/${sessionId}/ai-review/diff?${q}`)
}

export async function revertAiReviewFile(sessionId: string, turnId: string, path: string) {
  return fetchJSON<{ ok: boolean; path: string }>(`/sessions/${sessionId}/turns/${turnId}/ai-review/revert`, {
    method: 'POST',
    body: JSON.stringify({ path }),
  })
}

export async function applyAiReviewHunks(
  sessionId: string,
  turnId: string,
  path: string,
  opts: { acceptAll?: boolean; hunkIndexes?: number[] },
) {
  return fetchJSON<{ ok: boolean; path: string }>(
    `/sessions/${sessionId}/turns/${turnId}/ai-review/apply-hunks`,
    {
      method: 'POST',
      body: JSON.stringify({
        path,
        acceptAll: !!opts.acceptAll,
        hunkIndexes: opts.hunkIndexes || [],
      }),
    },
  )
}
