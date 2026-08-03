import { computed, type Ref } from 'vue'
import { useSessionsStore } from '@/stores/sessions'
import { useModelConfigStore } from '@/stores/modelLimits'
import type { StreamEvent } from '@/types/mission'

function asRecord(v: unknown): Record<string, unknown> | null {
  if (v && typeof v === 'object' && !Array.isArray(v)) return v as Record<string, unknown>
  return null
}

function num(v: unknown): number {
  const n = Number(v)
  return Number.isFinite(n) ? n : 0
}

export interface CompactionEntry {
  seq: number
  turnsCompacted: number
  tokensBefore: number
  tokensAfter: number
  filePath: string
}

export function useSessionContextUsage(modelId?: Ref<string> | (() => string)) {
  const sessions = useSessionsStore()
  const modelConfig = useModelConfigStore()

  const resolveModelId = () => {
    if (!modelId) return sessions.selectedModelId
    return typeof modelId === 'function' ? modelId() : modelId.value
  }

  const latestUsage = computed(() => {
    let prompt = 0
    let completion = 0
    let total = 0
    let seq = 0
    for (const ev of sessions.streamEvents) {
      if (ev.type !== 'llm.usage') continue
      const p = asRecord(ev.payload)
      prompt = num(p?.promptTokens ?? p?.prompt_tokens)
      completion = num(p?.completionTokens ?? p?.completion_tokens)
      total = num(p?.totalTokens ?? p?.total_tokens) || prompt + completion
      seq = ev.seq
    }
    return { promptTokens: prompt, completionTokens: completion, totalTokens: total, seq }
  })

  const sessionTotalTokens = computed(() => {
    let sum = 0
    for (const ev of sessions.streamEvents) {
      if (ev.type !== 'llm.usage') continue
      const p = asRecord(ev.payload)
      const total = num(p?.totalTokens ?? p?.total_tokens)
      const prompt = num(p?.promptTokens ?? p?.prompt_tokens)
      const completion = num(p?.completionTokens ?? p?.completion_tokens)
      sum += total > 0 ? total : prompt + completion
    }
    return sum
  })

  const compactionHistory = computed<CompactionEntry[]>(() => {
    const list: CompactionEntry[] = []
    for (const ev of sessions.streamEvents) {
      if (ev.type !== 'context.compacted') continue
      const p = asRecord(ev.payload)
      list.push({
        seq: ev.seq,
        turnsCompacted: num(p?.turnsCompacted ?? p?.turns_compacted),
        tokensBefore: num(p?.tokensBefore ?? p?.tokens_before),
        tokensAfter: num(p?.tokensAfter ?? p?.tokens_after),
        filePath: String(p?.filePath ?? p?.file_path ?? ''),
      })
    }
    return list
  })

  const contextWindow = computed(() => {
    const mid = resolveModelId()
    // selectedModelId is "provider/model" or "provider/model/effort";
    // model-config entries are usually bare names like "deepseek-v4-pro".
    const parts = mid.split('/')
    const providerModel = parts.length >= 2 ? `${parts[0]}/${parts[1]}` : mid
    const bareModel = parts.length >= 2 ? parts[1] : mid
    const cfg = modelConfig.models.find(
      (m) => m.model === mid || m.model === providerModel || m.model === bareModel,
    )
    return cfg?.context_window && cfg.context_window > 0 ? cfg.context_window : 128000
  })

  /** Best estimate of context fill: latest prompt tokens vs window. */
  const usedTokens = computed(() => latestUsage.value.promptTokens || latestUsage.value.totalTokens)

  const usageRatio = computed(() => {
    const window = contextWindow.value
    if (window <= 0) return 0
    return Math.min(1, usedTokens.value / window)
  })

  const usageLevel = computed<'ok' | 'warn' | 'critical'>(() => {
    const r = usageRatio.value
    if (r >= 0.9) return 'critical'
    if (r >= 0.7) return 'warn'
    return 'ok'
  })

  function tokensForTurn(turnId: string, events?: StreamEvent[]): number {
    const source = events ?? sessions.streamEvents
    let sum = 0
    for (const ev of source) {
      if (ev.type !== 'llm.usage' || ev.turnId !== turnId) continue
      const p = asRecord(ev.payload)
      const total = num(p?.totalTokens ?? p?.total_tokens)
      const prompt = num(p?.promptTokens ?? p?.prompt_tokens)
      const completion = num(p?.completionTokens ?? p?.completion_tokens)
      sum += total > 0 ? total : prompt + completion
    }
    return sum
  }

  return {
    latestUsage,
    sessionTotalTokens,
    compactionHistory,
    contextWindow,
    usedTokens,
    usageRatio,
    usageLevel,
    tokensForTurn,
  }
}

export function formatTokenCount(n: number): string {
  if (n >= 1_000_000) return `${(n / 1_000_000).toFixed(1)}M`
  if (n >= 1000) return `${(n / 1000).toFixed(1)}k`
  return String(Math.round(n))
}
