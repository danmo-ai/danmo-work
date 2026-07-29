package port

import (
	"context"
	"danmo-work/core/domain"
)

type Repository interface {
	Agents()          AgentRepo
	Skills()          SkillRepo
	SkillFiles()      SkillFileRepo
	Sessions()        SessionRepo
	Projects()        ProjectRepo
	LLMConfig()       LLMConfigRepo
	Approvals()       ApprovalRepo
	PendingMessages() PendingMessageRepo
	StreamEvents()    StreamEventRepo
	Turns()           TurnRepo
	MCPServers()      MCPServerRepo
	Secrets()         SecretStore
	Automations()     AutomationRepo
	Memories()        MemoryRepo
	KnowledgeBases()  KnowledgeBaseRepo
	KnowledgeDocs()   KnowledgeDocRepo
	KnowledgeIndex()  KnowledgeIndexRepo
	WeixinAccounts()  WeixinAccountRepo
	WeixinBindings()  WeixinBindingRepo
	ChannelBindings() ChannelBindingRepo
	AppMeta()         AppMetaRepo
}

// KnowledgeBaseRepo persists knowledge-base catalog metadata.
type KnowledgeBaseRepo interface {
	List(ctx context.Context) ([]domain.KnowledgeBase, error)
	Get(ctx context.Context, id string) (domain.KnowledgeBase, error)
	Upsert(ctx context.Context, b domain.KnowledgeBase) error
	Delete(ctx context.Context, id string) error
}

// KnowledgeDocRepo persists document metadata (content lives on disk).
type KnowledgeDocRepo interface {
	ListByKB(ctx context.Context, kbID string) ([]domain.KnowledgeDoc, error)
	ListAll(ctx context.Context) ([]domain.KnowledgeDoc, error)
	Get(ctx context.Context, id string) (domain.KnowledgeDoc, error)
	Upsert(ctx context.Context, d domain.KnowledgeDoc) error
	Delete(ctx context.Context, id string) error
	DeleteByKB(ctx context.Context, kbID string) error
	CountByKB(ctx context.Context, kbID string) (int, error)
}

// KnowledgeChapter is one indexed chapter row (not exposed over HTTP).
type KnowledgeChapter struct {
	Path      string
	KBID      string
	DocID     string
	Title     string
	Content   string
	Embedding []float32 // optional; empty when vector hybrid off
}

// KnowledgeIndexRepo maintains chapter FTS (+ optional vectors) for search_kb.
type KnowledgeIndexRepo interface {
	ReplaceDocChapters(ctx context.Context, docID string, chapters []KnowledgeChapter) error
	DeleteByDoc(ctx context.Context, docID string) error
	DeleteByKB(ctx context.Context, kbID string) error
	SearchBM25(ctx context.Context, kbIDs []string, query string, limit int) ([]domain.KnowledgeChapterHit, error)
	SearchVector(ctx context.Context, kbIDs []string, queryVec []float32, limit int) ([]domain.KnowledgeChapterHit, error)
	ListChapterEmbeddings(ctx context.Context, kbIDs []string) ([]KnowledgeChapter, error)
}

// AutomationRepo persists scheduled / webhook automations.
type AutomationRepo interface {
	List(ctx context.Context) ([]domain.Automation, error)
	Get(ctx context.Context, id string) (domain.Automation, error)
	Upsert(ctx context.Context, a domain.Automation) error
	Delete(ctx context.Context, id string) error
	ListEnabled(ctx context.Context) ([]domain.Automation, error)
}

// ChannelBindingRepo maps (channel, account, peer) → Teams session for non-Weixin channels.
type ChannelBindingRepo interface {
	GetByPeer(ctx context.Context, channelType, accountID, peerID string) (domain.ChannelBinding, error)
	Upsert(ctx context.Context, b domain.ChannelBinding) error
	UpdateMeta(ctx context.Context, channelType, accountID, peerID string, meta map[string]string) error
	DeleteByAccount(ctx context.Context, channelType, accountID string) error
}

// WeixinAccountRepo persists logged-in iLink bot accounts.
type WeixinAccountRepo interface {
	List(ctx context.Context) ([]domain.WeixinAccount, error)
	Get(ctx context.Context, accountID string) (domain.WeixinAccount, error)
	Upsert(ctx context.Context, a domain.WeixinAccount) error
	Delete(ctx context.Context, accountID string) error
	UpdateSyncBuf(ctx context.Context, accountID, syncBuf string) error
	UpdateProjectID(ctx context.Context, accountID, projectID string) error
}

// AppMetaRepo is a small key/value store for one-shot migrations / flags.
type AppMetaRepo interface {
	Get(ctx context.Context, key string) (string, bool, error)
	Set(ctx context.Context, key, value string) error
}

// WeixinBindingRepo maps Weixin peer users to Teams sessions (1:1).
type WeixinBindingRepo interface {
	List(ctx context.Context) ([]domain.WeixinBinding, error)
	GetByPeer(ctx context.Context, accountID, peerUserID string) (domain.WeixinBinding, error)
	GetBySession(ctx context.Context, sessionID string) (domain.WeixinBinding, error)
	Upsert(ctx context.Context, b domain.WeixinBinding) error
	UpdateContextToken(ctx context.Context, accountID, peerUserID, token string) error
	Count(ctx context.Context) (int, error)
	DeleteByAccount(ctx context.Context, accountID string) error
}

// MemoryRepo persists agent-authored durable memories (memory_update / memory_read).
type MemoryRepo interface {
	Upsert(ctx context.Context, m domain.Memory) (domain.Memory, error)
	GetByKey(ctx context.Context, scope domain.MemoryScope, scopeID, key string) (domain.Memory, error)
	Search(ctx context.Context, q domain.MemoryQuery) ([]domain.Memory, error)
	Delete(ctx context.Context, scope domain.MemoryScope, scopeID, key string) error
}

// TableStoreRepo persists schema-free agent business rows in a separate SQLite
// data-plane database (store.db), isolated from the control-plane work.db.
type TableStoreRepo interface {
	Upsert(ctx context.Context, row domain.TableRow) (domain.TableRow, error)
	Get(ctx context.Context, scope domain.TableScope, scopeID, table, key string) (domain.TableRow, error)
	Query(ctx context.Context, q domain.TableQuery) ([]domain.TableRow, error)
	Delete(ctx context.Context, scope domain.TableScope, scopeID, table, key string) error
	CountTable(ctx context.Context, scope domain.TableScope, scopeID, table string) (int64, error)
	ListTables(ctx context.Context, scopes []domain.TableScopeRef) ([]domain.TableInfo, error)
	Close() error
}

type AgentRepo interface {
	List(ctx context.Context) ([]domain.Agent, error)
	Get(ctx context.Context, id string) (domain.Agent, error)
	Upsert(ctx context.Context, a domain.Agent) error
	Delete(ctx context.Context, id string) error
}

type SkillRepo interface {
	List(ctx context.Context) ([]domain.Skill, error)
	Get(ctx context.Context, id string) (domain.Skill, error)
	Upsert(ctx context.Context, s domain.Skill) error
	Delete(ctx context.Context, id string) error
}

type SkillFileRepo interface {
	ListBySkill(ctx context.Context, skillID string) ([]domain.SkillFile, error)
	Get(ctx context.Context, skillID, path string) (domain.SkillFile, error)
	Upsert(ctx context.Context, f domain.SkillFile) error
	Delete(ctx context.Context, skillID, path string) error
	DeleteBySkill(ctx context.Context, skillID string) error
}

type SessionRepo interface {
	Create(ctx context.Context, s domain.Session) error
	Update(ctx context.Context, s domain.Session) error
	Delete(ctx context.Context, id string) error
	Get(ctx context.Context, id string) (domain.Session, error)
	List(ctx context.Context) ([]domain.Session, error)
	ListByProject(ctx context.Context, projectID string) ([]domain.Session, error)
}

type ProjectRepo interface {
	Create(ctx context.Context, p domain.Project) error
	Update(ctx context.Context, p domain.Project) error
	Get(ctx context.Context, id string) (domain.Project, error)
	List(ctx context.Context) ([]domain.Project, error)
	Delete(ctx context.Context, id string) error
}

type LLMConfigRepo interface {
	GetAll(ctx context.Context) ([]domain.LLMProviderConfig, error)
	GetByID(ctx context.Context, id string) (domain.LLMProviderConfig, error)
	Upsert(ctx context.Context, cfg domain.LLMProviderConfig) error
	Delete(ctx context.Context, id string) error
}

type SearchConfigStore interface {
	Get(ctx context.Context) (domain.SearchConfig, error)
	Upsert(ctx context.Context, cfg domain.SearchConfig) error
}

type ConfigStore interface {
	Load(ctx context.Context) (*domain.ConfigFile, error)
	Save(ctx context.Context, cfg *domain.ConfigFile) error
}

type ApprovalRepo interface {
	Create(ctx context.Context, a domain.Approval) error
	Get(ctx context.Context, id string) (domain.Approval, error)
	Update(ctx context.Context, a domain.Approval) error
	ListByStatus(ctx context.Context, status string) ([]domain.Approval, error)
}

// PendingMessageRepo persists editable next-turn queues per session.
type PendingMessageRepo interface {
	ListBySession(ctx context.Context, sessionID string) ([]domain.PendingMessage, error)
	Get(ctx context.Context, id string) (domain.PendingMessage, error)
	Create(ctx context.Context, m domain.PendingMessage) error
	Update(ctx context.Context, m domain.PendingMessage) error
	Delete(ctx context.Context, id string) error
	DeleteBySession(ctx context.Context, sessionID string) error
	// PopFront marks the lowest-position queued message as sending and returns it.
	// Returns ok=false when the queue is empty.
	PopFront(ctx context.Context, sessionID string) (domain.PendingMessage, bool, error)
	Reorder(ctx context.Context, sessionID string, ids []string) error
	MaxPosition(ctx context.Context, sessionID string) (int, error)
}

type StreamEventRepo interface {
	Save(ctx context.Context, event domain.StreamEvent) error
	ListBySession(ctx context.Context, sessionID string, since int64) ([]domain.StreamEvent, error)
	MaxSeq() int64
}

type TurnRepo interface {
	Create(ctx context.Context, t domain.TurnLog) error
	UpdateStatus(ctx context.Context, id string, status domain.TurnStatus) error
	Get(ctx context.Context, id string) (domain.TurnLog, error)
	ListBySession(ctx context.Context, sessionID string) ([]domain.TurnLog, error)
	ListByStatus(ctx context.Context, status domain.TurnStatus) ([]domain.TurnLog, error)
}

type MCPServerRepo interface {
	List(ctx context.Context) ([]domain.MCPServer, error)
	Get(ctx context.Context, id string) (domain.MCPServer, error)
	Upsert(ctx context.Context, s domain.MCPServer) error
	Delete(ctx context.Context, id string) error
}

// TurnLogStore persists turn-level JSONL entries used exclusively for LLM
// message reconstruction (session history + turn recovery) and offline
// debugging (zip download).
//
// WHITELIST of allowed entry types:
//   - "start"        — written by Create (skipped on reopen/resume of existing file)
//   - "user"         — user / synthetic user messages for LLM replay
//   - "assistant"    — assistant text and/or batched tool_calls
//   - "tool_call"    — legacy single tool call (still accepted on read)
//   - "tool_result"  — tool role result after Execute (success, error, or cancel)
//   - "end"          — written by EndTurn
//
// DO NOT write diagnostic, audit, or telemetry entries here (e.g. llm_error,
// step events, permission decisions). Those belong in Stream Events
// (port.EventStream) which serve the UI/SSE timeline.
//
// LoadSessionMessages rebuilds full ChatMessages from the whitelist above
// (user / assistant / tool_call / tool_result). Incomplete turns drop an
// unpaired trailing assistant(tool_calls)/tool_call. Compaction uses
// retainFromTurnID + retainSkipMessages to bound the replay window.
type TurnLogStore interface {
	Create(turnID, sessionID, projectID, agentID, goal string) error
	// CreateNested writes a nested tool-run log under tool_runs/ (zip/debug only).
	CreateNested(turnID, sessionID, projectID, agentID, goal string) error
	Append(turnID, typ string, data map[string]any)
	EndTurn(turnID string, status domain.TurnStatus)
	LastStatus(sessionID string) domain.TurnStatus
	ListTurns(sessionID string) []domain.TurnLog
	ListTurnIDs(sessionID string) []string
	LoadForRecovery(turnID string) (goal string, entries []map[string]any)
	// LoadSessionMessages rebuilds full LLM chat history for a session.
	// retainFromTurnID: if non-empty, only include that turn and later ones.
	// retainSkipMessages: skip this many leading messages inside retainFromTurnID.
	LoadSessionMessages(sessionID, retainFromTurnID string, retainSkipMessages int) []ChatMessage
	LoadTurnMessages(turnID string) []ChatMessage
	IsNestedToolRun(turnID string) bool
	LoadRawLog(turnID string) ([]byte, error)
	LoadTurnLogZip(turnID string, events []domain.StreamEvent) ([]byte, error)
}
