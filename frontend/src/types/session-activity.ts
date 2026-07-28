/** Cross-session activity from GET /sessions/activity, plus local ask overlays. */
export type SessionActivityState = 'running' | 'awaiting_approval' | 'awaiting_ask' | 'idle'

export interface SessionActivityItem {
  sessionId: string
  /** Server: running | awaiting_approval. Client may overlay awaiting_ask. */
  state: SessionActivityState | string
  runningTurnId?: string
  pendingApprovalCount?: number
}

export function activityRank(state: SessionActivityState | string | undefined): number {
  switch (state) {
    case 'awaiting_approval':
    case 'awaiting_ask':
      return 0
    case 'running':
      return 1
    default:
      return 2
  }
}
