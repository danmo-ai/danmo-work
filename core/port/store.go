package port

import (
	"context"
	"time"

	"danmo-work/core/domain"
)

type Repository interface {
	Sessions()        SessionRepo
	Projects()        ProjectRepo
	LLMConfig()       LLMConfigRepo
	Approvals()       ApprovalRepo
	PendingMessages() PendingMessageRepo
	StreamEvents()    StreamEventRepo
	Turns()           TurnRepo
	TurnLogs()        TurnLogRepo
	Checkpoints()     CheckpointRepo
	FileChanges()     FileChangeRepo
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
	Usage()           UsageRepo
}

// UsageRepo persists turn/session/project token rollups (not per-call rows).
type UsageRepo interface {
	AddDelta(ctx context.Context, turnID, sessionID, projectID string, delta domain.UsageDelta, at time.Time) error
	Get(ctx context.Context, grain domain.UsageGrain, refID string) (domain.UsageRollup, error)
	ListBySession(ctx context.Context, sessionID string) ([]domain.UsageRollup, error)
	ListByProject(ctx context.Context, projectID string, grain domain.UsageGrain) ([]domain.UsageRollup, error)
	SummarizeSession(ctx context.Context, sessionID string) (domain.UsageBreakdown, error)
	SummarizeProject(ctx context.Context, projectID string) (domain.UsageBreakdown, error)
	// SummarizeScope aggregates tokens + turn count for optional project/model filters.
	SummarizeScope(ctx context.Context, projectID, model string) (domain.UsageSummary, error)
	Series(ctx context.Context, filter domain.UsageSeriesFilter) ([]domain.UsageSeriesPoint, error)
	// HasGrain reports whether any rollup exists for grain+refID (backfill gate).
	HasGrain(ctx context.Context, grain domain.UsageGrain, refID string) (bool, error)
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

// KnowledgeChapter is one bottom-up extracted logical chapter.
type KnowledgeChapter struct {
	ID      string
	KBID    string
	DocID   string
	Title   string
	Content string // full chapter Markdown
}

// KnowledgeChunkEntry is one chunk-level unit for in-memory BM25 and vector indices.
type KnowledgeChunkEntry struct {
	ID    string
	KBID  string
	DocID string
	Text  string // chunk content for BM25 tokenization
}

// KnowledgeIndexRepo persists chapters and manages in-memory BM25 + vector indices.
type KnowledgeIndexRepo interface {
	ReplaceDocChapters(ctx context.Context, docID string, chapters []KnowledgeChapter, chunks []KnowledgeChunkEntry, vectors map[string][]float32) error
	DeleteByDoc(ctx context.Context, docID string) error
	DeleteByKB(ctx context.Context, kbID string) error
	SearchBM25(ctx context.Context, kbIDs []string, query string, limit int) ([]domain.KnowledgeChunkHit, error)
	SearchVector(ctx context.Context, kbIDs []string, queryVec []float32, limit int) ([]domain.KnowledgeChunkHit, error)
	GetChaptersByIDs(ctx context.Context, chapterIDs []string) ([]domain.KnowledgeChapter, error)
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
	// Returns ok=false when the queue is empty. Only considers status=queued.
	PopFront(ctx context.Context, sessionID string) (domain.PendingMessage, bool, error)
	// ClaimSteering atomically returns and deletes all status=steering messages
	// for the session (ordered). Used at the tool→LLM soft-steer boundary.
	ClaimSteering(ctx context.Context, sessionID string) ([]domain.PendingMessage, error)
	// DemoteSteering moves leftover steering messages back to queued (turn ended).
	DemoteSteering(ctx context.Context, sessionID string) error
	Reorder(ctx context.Context, sessionID string, ids []string) error
	MaxPosition(ctx context.Context, sessionID string) (int, error)
}

type StreamEventRepo interface {
	Save(ctx context.Context, event domain.StreamEvent) error
	ListBySession(ctx context.Context, sessionID string, since int64) ([]domain.StreamEvent, error)
	MaxSeq() int64
	// DeleteBySession removes the session's event timeline (cascade cleanup).
	DeleteBySession(ctx context.Context, sessionID string) error
}

type TurnRepo interface {
	Create(ctx context.Context, t domain.TurnLog) error
	UpdateStatus(ctx context.Context, id string, status domain.TurnStatus) error
	Get(ctx context.Context, id string) (domain.TurnLog, error)
	ListBySession(ctx context.Context, sessionID string) ([]domain.TurnLog, error)
	ListByStatus(ctx context.Context, status domain.TurnStatus) ([]domain.TurnLog, error)
}

// TurnLogEntryRecord is one persisted LLM-history entry row. The SQLite
// turn_log_entries table is the single source of truth for LLM message
// reconstruction; JSONL files are rendered from these rows on demand
// (export/zip/debug) and are no longer authoritative.
type TurnLogEntryRecord struct {
	TurnID string
	Seq    int
	Type   string
	Data   map[string]any
}

// TurnLogRepo persists turn-log entries and turn metadata in the database.
// Turn metadata (goal, agent, status, nested) lives in the turns table —
// the same rows served by TurnRepo — so there is exactly one fact per turn.
type TurnLogRepo interface {
	// UpsertTurnMeta inserts the turn row (status as given) or fills blank
	// fields (session/project/agent/goal) on an existing row. Nested is only
	// ever raised to true, never cleared. The status of an existing row is
	// left untouched (CancelTurn/recovery own explicit status transitions).
	UpsertTurnMeta(ctx context.Context, t domain.TurnLog) error
	GetTurnMeta(ctx context.Context, turnID string) (domain.TurnLog, bool, error)
	// EndTurn records a terminal status but never overwrites an existing
	// terminal status — a concurrent user cancel must not be resurrected by
	// the finishing turn goroutine.
	EndTurn(ctx context.Context, turnID string, status domain.TurnStatus) error
	ListSessionTurnIDs(ctx context.Context, sessionID string, includeNested bool) ([]string, error)
	AppendEntry(ctx context.Context, e TurnLogEntryRecord) error
	ListEntries(ctx context.Context, turnID string) ([]TurnLogEntryRecord, error)
	// MaxSeq returns the highest entry seq for a turn (0 when none).
	MaxSeq(ctx context.Context, turnID string) (int, error)
	// DeleteSessionHistory removes the session's turn rows and their message
	// entries (cascade cleanup when a session is deleted).
	DeleteSessionHistory(ctx context.Context, sessionID string) error
}

// CheckpointRepo persists the latest compaction checkpoint per session
// (control plane). The checkpoint owns replay semantics: RetainFromTurnID
// decides which turn_log_entries participate in LLM context reconstruction,
// and it carries the surviving todo/plan list and file-change aggregate.
type CheckpointRepo interface {
	// Save upserts the session's checkpoint (only the latest is kept).
	Save(ctx context.Context, cp domain.CompactionCheckpoint) error
	// Get returns nil without error when the session has no checkpoint.
	Get(ctx context.Context, sessionID string) (*domain.CompactionCheckpoint, error)
	DeleteBySession(ctx context.Context, sessionID string) error
}

// FileChangeRepo persists the append-only session file-change journal
// (history plane).
type FileChangeRepo interface {
	// Append stores one record. Seq 0 means assign the next per-session seq;
	// a non-zero Seq is preserved (legacy import).
	Append(ctx context.Context, sessionID string, rec domain.FileChangeRecord) (int64, error)
	// ListAfter returns records with Seq > afterSeq in ascending order.
	ListAfter(ctx context.Context, sessionID string, afterSeq int64) ([]domain.FileChangeRecord, error)
	DeleteBySession(ctx context.Context, sessionID string) error
}

// TurnLogStore reconstructs LLM chat history (session replay + turn recovery)
// from the DB-backed turn log and renders JSONL exports for debugging.
//
// WHITELIST of allowed entry types:
//   - "user"         — user / synthetic user messages for LLM replay
//   - "assistant"    — assistant text and/or batched tool_calls
//   - "tool_call"    — legacy single tool call (still accepted on read)
//   - "tool_result"  — tool role result after Execute (success, error, or cancel)
//
// Turn start/end are table columns (turns.goal / turns.status), not entries;
// exports render synthetic "start"/"end" lines for backward-compatible JSONL.
//
// DO NOT write diagnostic, audit, or telemetry entries here (e.g. llm_error,
// step events, permission decisions). Those belong in Stream Events
// (port.EventStream) which serve the UI/SSE timeline.
//
// LoadSessionMessages rebuilds full ChatMessages from the whitelist above.
// Incomplete turns drop an unpaired trailing assistant(tool_calls)/tool_call
// unless recovery has already closed them via ListIncompleteToolCalls +
// tool_result. Compaction uses retainFromTurnID + retainSkipMessages to bound
// the replay window.
type TurnLogStore interface {
	Create(turnID, sessionID, projectID, agentID, goal string) error
	// CreateNested records a nested tool-run log (zip/debug only).
	CreateNested(turnID, sessionID, projectID, agentID, goal string) error
	Append(turnID, typ string, data map[string]any)
	EndTurn(turnID string, status domain.TurnStatus)
	ListTurnIDs(sessionID string) []string
	LoadForRecovery(turnID string) (goal string, entries []map[string]any)
	// ListIncompleteToolCalls returns tool invocations in the turn log that
	// lack a matching tool_result (authoritative open set for recovery close).
	ListIncompleteToolCalls(turnID string) []IncompleteToolCall
	// LoadSessionMessages rebuilds full LLM chat history for a session.
	// retainFromTurnID: if non-empty, only include that turn and later ones.
	// retainSkipMessages: skip this many leading messages inside retainFromTurnID.
	LoadSessionMessages(sessionID, retainFromTurnID string, retainSkipMessages int) []ChatMessage
	LoadTurnMessages(turnID string) []ChatMessage
	IsNestedToolRun(turnID string) bool
	// ListSessionEntries returns every entry of every non-nested turn in the
	// session, ordered by turn then seq (debug/inspection).
	ListSessionEntries(sessionID string) []TurnLogEntryRecord
	LoadRawLog(turnID string) ([]byte, error)
	LoadTurnLogZip(turnID string, events []domain.StreamEvent) ([]byte, error)
	// RecallToolResult returns the durable tool_result output for callID in
	// turnID (full text at execute time; compaction does not mutate the log).
	RecallToolResult(turnID, callID string) (RecalledToolResult, bool)
	// RecallToolResultInSession searches non-nested turns in the session
	// (newest first) when call_id is not found in the current turn.
	RecallToolResultInSession(sessionID, callID string) (RecalledToolResult, bool)
}

// RecalledToolResult is a tool_result read from the durable turn log.
type RecalledToolResult struct {
	TurnID          string
	CallID          string
	ToolName        string
	Output          string
	IngestTruncated bool // capped by runtime.tools.max_output_chars at execute
}

// IncompleteToolCall is a tool invocation recorded in turn JSONL without a
// matching tool_result. Used by recovery to close open pairs before resume.
type IncompleteToolCall struct {
	CallID string
	Name   string
	Input  map[string]any
}
