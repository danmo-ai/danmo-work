export interface Agent {
  id: string
  name: string
  description?: string
  persona?: string
  mode?: 'primary' | 'subagent'
  systemPrompt?: string
  steps?: number
  skillIds?: string[]
  tools?: ToolBinding[]
  /** Connector ids (exact; API field mcpServers). Used when inheritAmbient is false. */
  mcpServers?: string[]
  knowledgeIds?: string[]
  canDelegate?: boolean
  /** Ambient layer: FS skills + all enabled connectors. Default: primary true, subagent false. */
  inheritAmbient?: boolean
  builtin?: boolean
  marketSource?: string
}

export interface CreateAgentPayload {
  id: string
  name: string
  description?: string
  persona?: string
  mode?: 'primary' | 'subagent'
  systemPrompt?: string
  steps?: number
  skillIds?: string[]
  tools?: ToolBinding[]
  mcpServers?: string[]
  knowledgeIds?: string[]
  canDelegate?: boolean
  inheritAmbient?: boolean
}

export interface UpdateAgentPayload {
  name?: string
  description?: string
  persona?: string
  mode?: 'primary' | 'subagent'
  systemPrompt?: string
  steps?: number
  skillIds?: string[]
  tools?: ToolBinding[]
  mcpServers?: string[]
  knowledgeIds?: string[]
  canDelegate?: boolean
  inheritAmbient?: boolean
}

export type RiskLevel = 'low' | 'medium' | 'high' | 'external'

export type PermissionMode = 'discuss' | 'plan' | 'interactive' | 'auto'

export type MCPAuthMode = 'none' | 'headers' | 'oauth'

export interface ConnectorCatalogEntry {
  id: string
  name: string
  description: string
  category: string
  transport: 'stdio' | 'sse' | 'streamable-http'
  url?: string
  command?: string
  args?: string
  auth: MCPAuthMode
  docsUrl?: string
  oauthAuthorizeUrl?: string
  oauthTokenUrl?: string
  oauthScopes?: string
  region?: string
  tags?: string[]
}

export interface Skill {
  id: string
  name: string
  description?: string
  license?: string
  compatibility?: string
  metadata?: Record<string, string>
  allowedTools?: string
  keywords?: string[]
  toolIds?: string[]
  systemHint?: string
  body?: string
  sourcePath?: string
  builtin?: boolean
  marketSource?: string
  templateDiverged?: boolean
}

/** Skill usable for an agent turn (bound ∪ filesystem), from available-skills API. */
export type AvailableSkillSource = 'bound' | 'filesystem' | 'both'

export interface AvailableSkill extends Skill {
  source: AvailableSkillSource
}

export interface ToolBinding {
  toolId: string
  name?: string
  riskLevel?: RiskLevel
}

export interface Tool {
  id: string
  name: string
  description?: string
  type: 'builtin' | 'mcp'
  mcpServer?: string
  riskLevel?: RiskLevel
  schema?: string
}

export interface SkillFile {
  id: string
  skillId: string
  path: string
  content?: string
  size: number
}

export interface KnowledgeBase {
  id: string
  name: string
  description?: string
  documentCount: number
  updatedAt: string
}

export interface Project {
  id: string
  name: string
  directory: string
  createdAt: string
  updatedAt: string
}

export interface KnowledgeDocument {
  id: string
  knowledgeBaseId: string
  /** Backend JSON field alias */
  kbId?: string
  title: string
  content?: string
  path?: string
  updatedAt: string
}

export interface MCPToolDef {
  name: string
  description: string
  enabled: boolean
  inputSchema?: Record<string, unknown>
}

export interface MCPServer {
  id: string
  name: string
  description?: string
  transport: 'stdio' | 'sse' | 'streamable-http'
  command?: string
  args?: string
  url?: string
  env?: string
  headers?: Record<string, string>
  auth?: MCPAuthMode
  secretHeadersRef?: Record<string, string>
  oauthClientId?: string
  oauthAuthorizeUrl?: string
  oauthTokenUrl?: string
  oauthScopes?: string
  oauthStatus?: string
  catalogId?: string
  marketSource?: string
  enabledTools?: string[]
  discoveredTools?: MCPToolDef[]
  toolTimeout?: number
  /** When false, tools are bound-only (not mounted for ambient agents). Default true. */
  ambientMount?: boolean
  status: 'connected' | 'disconnected' | 'error'
  enabled: boolean
}

export type AutomationTrigger = 'schedule' | 'event' | 'webhook' | 'manual'

export interface Automation {
  id: string
  name: string
  description?: string
  enabled: boolean
  trigger: AutomationTrigger
  schedule?: string
  eventType?: string
  webhookPath?: string
  agentId?: string
  projectId?: string
  modelId?: string
  prompt: string
  lastRunAt?: string
  nextRunAt?: string
  lastTurnId?: string
  lastStatus?: string
}

export interface TimelineEvent {
  id: string
  sessionId: string
  type: string
  payload: unknown
  createdAt: string
}

export interface ApprovalRequest {
  id: string
  summary: string
  highRiskItems: { type: string; id: string; displayName: string }[]
  status: string
  runId: string
  sessionId: string
}

export interface TodoItem {
  id: string
  title: string
  done: boolean
  sessionId?: string
}

export interface WorkspaceArtifact {
  id: string
  sessionId?: string
  title: string
  kind: 'report' | 'note' | 'pin' | string
  content?: string
  createdAt: string
}

export interface ExecutionPlan {
  runId: string
  skillIds: string[]
  toolIds: string[]
  rationale: string
  evaluatedRisk: RiskLevel
  highRiskItems?: { type: string; id: string; displayName: string }[]
}

export interface MarketSource {
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

export interface MarketListing {
  kind: 'skill' | 'expert' | 'connector' | 'bundle'
  id: string
  name: string
  description?: string
  keywords?: string[]
  category?: string
  version?: string
  license?: string
  author?: string
  path: string
  skillDeps?: string[]
  updatedAt?: string
  compatibility?: string
  sourceId: string
  sourceName?: string
  installed?: boolean
}

export interface InstallMarketResult {
  kind: string
  id: string
  sourceId: string
  ref?: string
  version?: string
  installed?: string[]
  skipped?: string[]
}
