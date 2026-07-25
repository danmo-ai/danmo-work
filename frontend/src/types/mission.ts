import type { Agent, Skill } from '../types'

export type { Agent, Skill }

export type SessionStatus = 'active' | 'archived' | 'running' | 'awaiting_approval' | 'completed' | 'failed' | 'blocked'

export interface Session {
  id: string
  title?: string
  projectId?: string
  agentId?: string
  modelId?: string
  content: string
  status: SessionStatus
  summary?: string
  createdAt: string
  updatedAt: string
}

export interface UpdateSessionPayload {
  title?: string
  projectId?: string
  status?: SessionStatus
  modelId?: string
  agentId?: string
}

export interface TurnLog {
  id: string
  sessionId: string
  agentId: string
  goal: string
  status: 'running' | 'completed' | 'failed' | 'cancelled' | 'timeout'
}

export interface CreateSessionPayload {
  content: string
  agentId?: string
  projectId?: string
  modelId?: string
}

export interface SendMessagePayload {
  userInput: string
}

/** SSE / UI timeline event — not LLM chat history. */
export interface StreamEvent {
  seq: number
  type: string
  sessionId: string
  turnId?: string
  runId?: string
  payload: unknown
  createdAt: string
}

/** Worker card from team dispatch — one AgentRun. */
export interface WorkerCard {
  runId: string
  traceId: string
  agentId: string
  status: string
  stepsUsed: number
}

/** Persisted agent loop metadata — not LLM messages. */
export interface AgentRun {
  id: string
  sessionId: string
  agentId: string
  parentId?: string
  goal: string
  status: string
  stepsUsed: number
  traceId?: string
  createdAt: string
  updatedAt: string
}

export type LLMProviderType = 'openai' | 'anthropic' | 'local' | 'mock'

export interface LLMModelRef {
  name: string
  enabled: boolean
}

export interface LLMProviderConfig {
  id: string
  provider: LLMProviderType
  name: string
  apiKey?: string
  baseUrl?: string
  models?: LLMModelRef[]
  createdAt: string
  updatedAt: string
}

export interface UpsertLLMProviderConfigRequest {
  provider: LLMProviderType
  name: string
  apiKey?: string
  baseUrl?: string
  models?: LLMModelRef[]
}

export interface LLMProviderPreset {
  id: string
  name: string
  provider: LLMProviderType
  baseUrl: string
  icon: string
  description: string
}

export interface LLMModel {
  id: string
  name: string
  providerId: string
  provider: string
  enabled: boolean
  availableEfforts: string[]
  /** True when the model accepts image / multimodal input. */
  vision?: boolean
}

export interface ModelConfig {
  model: string
  context_window?: number
  max_output?: number
  temperature?: number
  top_p?: number
  frequency_penalty?: number
  presence_penalty?: number
  stop?: string[]
  available_efforts?: string[]
  thinking_mode?: string
  effort_budget_tokens?: Record<string, number>
  /** Accepts image content parts when true. */
  vision?: boolean
}

export type SearchProvider =
  | 'duckduckgo'
  | 'bing'
  | 'brave'
  | 'tavily'
  | 'bocha'
  | 'metaso'
  | 'searxng'
  | 'baidu'
  | 'volcengine'
  | 'sofya'

export interface SearchConfig {
  provider: SearchProvider
  baseUrl?: string
  apiKey?: string
  timeoutMs: number
  maxResults: number
  proxy?: string
  userAgent?: string
  htmlFallback?: boolean
}

export interface UpsertSearchConfigRequest {
  provider: SearchProvider
  baseUrl?: string
  apiKey?: string
  timeoutMs?: number
  maxResults?: number
  proxy?: string
  userAgent?: string
  htmlFallback?: boolean
}

export interface ConfigFile {
  data: {
    dir: string
    database: string
    store: string
  }
  server: {
    listenAddr: string
  }
  instance: {
    id: string
  }
  runtime: {
    autoApprove: boolean
    sandbox?: {
      enabled: boolean
      mode: 'read-only' | 'workspace-write' | 'danger-full-access'
      network: 'deny' | 'allow' | 'allowlist'
      backend?: string
      shell?: 'auto' | 'bash' | 'cmd' | string
    }
    browser?: {
      enabled: boolean
      executablePath?: string
      cdpUrl?: string
    }
    turn: {
      doomLoopThreshold: number
      maxStepsDefault: number
      maxLLMFailures: number
    }
    tools?: {
      maxOutputChars: number
    }
    team: {
      maxDelegationDepth: number
    }
    memory: {
      readTopK: number
    }
    knowledge: {
      searchTopK: number
    }
    compaction: {
      enabled: boolean
      model: string
      maxTokens: number
      triggerRatio: number
      cutTokens: number
      turnInterval: number
      subInterval: number
      toolTruncate: number
    }
  }
  search: SearchConfig
  llm: {
    providers: LLMProviderPreset[]
    models?: ModelConfig[]
  }
  market?: ConfigMarketSection
  channels?: {
    weixin?: {
      enabled: boolean
      defaultAgentId?: string
      defaultModelId?: string
      autoApprove?: boolean
    }
    feishu?: {
      enabled: boolean
      defaultAgentId?: string
      defaultModelId?: string
      autoApprove?: boolean
      domain?: string
      appId?: string
      projectId?: string
    }
    wecom?: {
      enabled: boolean
      defaultAgentId?: string
      defaultModelId?: string
      autoApprove?: boolean
      botId?: string
      projectId?: string
      wsUrl?: string
    }
  }
}

export interface MarketSourceConfig {
  id: string
  name: string
  kind: string
  platform?: string
  repo?: string
  ref?: string
  catalogPath?: string
  token?: string
  enabled: boolean
  priority: number
}

export interface ConfigMarketSection {
  cacheTtlHours: number
  sources: MarketSourceConfig[]
}

export interface SandboxStatus {
  enabled: boolean
  mode: string
  network: string
  backend: string
  degraded: boolean
  degradedReason?: string
  platform: string
  capabilities?: string[]
  shell?: string
  shellPath?: string
  coreutilsBin?: string
}

export interface BrowserStatus {
  available: boolean
  enabled: boolean
  engine: string
  path?: string
  mode: string
  degradedReason?: string
}

export interface UpdateConfigFileRequest {
  data?: ConfigFile['data']
  server?: ConfigFile['server']
  instance?: ConfigFile['instance']
  runtime?: ConfigFile['runtime']
  search?: UpsertSearchConfigRequest
  market?: ConfigMarketSection
  channels?: ConfigFile['channels']
}

export interface RuntimeConfigForm {
  autoApprove: boolean
  sandboxEnabled: boolean
  sandboxMode: 'read-only' | 'workspace-write' | 'danger-full-access'
  sandboxNetwork: 'deny' | 'allow' | 'allowlist'
  sandboxBackend?: string
  sandboxShell?: 'auto' | 'bash' | 'cmd' | string
  browserEnabled?: boolean
  browserExecutablePath?: string
  browserCdpUrl?: string
  doomLoopThreshold: number
  maxStepsDefault: number
  maxLLMFailures: number
  maxToolOutputChars: number
  maxDelegationDepth: number
  readTopK: number
  searchTopK: number
  compactionEnabled: boolean
  compactionMaxTokens: number
  compactionTriggerRatio: number
  compactionCutTokens: number
  compactionTurnInterval: number
  compactionSubInterval: number
  compactionToolTruncate: number
}

