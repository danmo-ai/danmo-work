import { defineStore } from 'pinia'
import { ref } from 'vue'

const AFTER_TURN_KEY = 'stage-ai-review-after-turn'

export type AiReviewAfterTurn = 'banner' | 'off'

function readAfterTurn(): AiReviewAfterTurn {
  try {
    const v = localStorage.getItem(AFTER_TURN_KEY)
    if (v === 'off' || v === 'banner') return v
  } catch {
    /* ignore */
  }
  return 'banner'
}

export interface PendingAiReview {
  sessionId: string
  turnId: string
  path: string
  canRevert: boolean
}

export const useStageAiReviewStore = defineStore('stageAiReview', () => {
  const afterTurn = ref<AiReviewAfterTurn>(readAfterTurn())
  const pending = ref<PendingAiReview | null>(null)

  function setAfterTurn(mode: AiReviewAfterTurn) {
    afterTurn.value = mode
    try {
      localStorage.setItem(AFTER_TURN_KEY, mode)
    } catch {
      /* ignore */
    }
  }

  function setPending(next: PendingAiReview | null) {
    pending.value = next
  }

  function clearPending() {
    pending.value = null
  }

  return { afterTurn, pending, setAfterTurn, setPending, clearPending }
})
