package runtime

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"sort"
	"strings"
	"sync"
	"time"

	"danmo-work/core/domain"
	"danmo-work/core/port"
	"danmo-work/core/runtime/permission"
	"danmo-work/core/runtime/tool"
	"danmo-work/core/runtime/tool/builtin"
	"danmo-work/core/service"
)

var _ port.Engine = (*Engine)(nil)

func evalModeEnabled() bool {
	v := strings.TrimSpace(strings.ToLower(os.Getenv("WORK_EVAL_MODE")))
	return v == "1" || v == "true" || v == "yes"
}

type engineRunCfg struct {
	autoApprove            bool
	teamMaxDelegationDepth int
	knowledgeSearchTopK    int
	memoryReadTopK         int
}

type Engine struct {
	sessions      *service.SessionManager
	turns         *service.TurnManager
	projects      *service.ProjectManager
	approvals     *service.ApprovalManager
	turnLog       *service.TurnLogManager
	agents        *service.AgentManager
	skills        *service.SkillManager
	knowledge     *builtin.Knowledge
	memories      port.MemoryRepo
	tableStore    port.TableStoreRepo
	tableBudget   *builtin.TableTurnBudget
	llm           port.LLMProvider
	stream        port.EventStream
	sandbox       port.Sandbox
	execution     port.ExecutionBackend
	turnRunner    *TurnRunner
	toolCatalog   *tool.Registry
	compactionMgr *CompactionManager
	modelLimits   *ModelConfigRegistry
	configStore   port.ConfigStore
	mcpCaller     port.MCPCaller
	// githubMCPReady reports whether the builtin github connector has usable auth.
	githubMCPReady func(ctx context.Context) bool
	dataDir       string
	turnMessages  map[string][]Message
	mu            sync.Mutex
	approvalWait  map[string]chan ApprovalOutcome
	approvalMeta  map[string]approvalMeta
	sessionPerm   map[string]sessionPermState
	askUserWait   map[string]chan string
	cancel        map[string]context.CancelFunc
	activeTurns   map[string]string // session ID -> in-flight turn ID
	readSkill     *builtin.ReadSkill
	// preTurnSnapshot runs before tools (AI review). Optional.
	preTurnSnapshot func(ctx context.Context, projectID, sessionID, turnID, userInput string, extraPaths []string)
}

type approvalMeta struct {
	SessionID string
	Reason    string
	Domain    string
}

type sessionPermState struct {
	AllowNetwork    bool
	AllowedDomains  []string
}

// ApprovalOutcome is returned when a pending approval is resolved.
type ApprovalOutcome struct {
	Approved bool
	Scope    string // once | session
	Reason   string
}

func (e *Engine) loadRunCfg(ctx context.Context) engineRunCfg {
	cfg := engineRunCfg{
		teamMaxDelegationDepth: 3,
		knowledgeSearchTopK:    3,
		memoryReadTopK:         10,
	}
	if e.configStore != nil {
		if c, err := e.configStore.Load(ctx); err == nil {
			rt := c.Runtime
			cfg.autoApprove = rt.AutoApprove
			cfg.teamMaxDelegationDepth = rt.Team.MaxDelegationDepth
			cfg.knowledgeSearchTopK = rt.Knowledge.SearchTopK
			if rt.Memory.ReadTopK > 0 {
				cfg.memoryReadTopK = rt.Memory.ReadTopK
			}
		}
	}
	return cfg
}

func (e *Engine) isAutoApprove() bool {
	if e.configStore != nil {
		if c, err := e.configStore.Load(context.Background()); err == nil {
			return c.Runtime.AutoApprove
		}
	}
	return false
}

func NewEngine(sessions *service.SessionManager, turns *service.TurnManager, projects *service.ProjectManager, approvals *service.ApprovalManager, turnLog *service.TurnLogManager, agents *service.AgentManager, skills *service.SkillManager, knowledge *builtin.Knowledge, memories port.MemoryRepo, llm port.LLMProvider, stream port.EventStream, checkpointStore CompactionCheckpointStore, configStore port.ConfigStore, dataDir string) *Engine {
	catalog := tool.NewRegistry()
	gate := permission.NewGate(nil)
	turnRunner := NewTurnRunner(llm, stream, gate, tool.NewRegistry(), configStore)

	modelLimits := NewModelConfigRegistry()
	modelLimits.LoadFromConfig(context.Background(), configStore)

	e := &Engine{
		sessions:      sessions,
		turns:         turns,
		projects:      projects,
		approvals:     approvals,
		turnLog:       turnLog,
		agents:        agents,
		skills:        skills,
		knowledge:     knowledge,
		memories:      memories,
		llm:           llm,
		stream:        stream,
		turnRunner:    turnRunner,
		toolCatalog:   catalog,
		compactionMgr: NewCompactionManager(llm, stream, configStore, checkpointStore, modelLimits),
		modelLimits:   modelLimits,
		configStore:   configStore,
		dataDir:       dataDir,
		turnMessages:  make(map[string][]Message),
		approvalWait:  make(map[string]chan ApprovalOutcome),
		approvalMeta:  make(map[string]approvalMeta),
		sessionPerm:   make(map[string]sessionPermState),
		askUserWait:   make(map[string]chan string),
		cancel:        make(map[string]context.CancelFunc),
		activeTurns:   make(map[string]string),
	}
	turnRunner.Approval = e
	turnRunner.SandboxStatus = e.sandboxStatus
	turnRunner.EffectiveIsolation = e.effectiveIsolation
	turnRunner.SessionAllowNetwork = e.sessionAllowsNetwork
	turnRunner.SessionAllowDomains = e.sessionAllowDomains
	turnRunner.GrantSessionDomains = e.grantSessionDomains
	turnRunner.GrantTurnDomains = e.grantTurnDomains
	turnRunner.ClearTurnDomains = e.clearTurnDomains
	return e
}

// SetTableStore wires the isolated agent table-store data plane (store.db).
func (e *Engine) SetTableStore(store port.TableStoreRepo) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.tableStore = store
	if e.tableBudget == nil {
		e.tableBudget = builtin.NewTableTurnBudget()
	}
}

func (e *Engine) tableQuotas() domain.ConfigTableSection {
	q := domain.DefaultTableSection()
	if e.configStore == nil {
		return q
	}
	c, err := e.configStore.Load(context.Background())
	if err != nil {
		return q
	}
	return resolveEngineTableQuotas(c.Runtime.Table)
}

func resolveEngineTableQuotas(got domain.ConfigTableSection) domain.ConfigTableSection {
	q := domain.DefaultTableSection()
	if got.MaxRowsPerUpsert > 0 {
		q.MaxRowsPerUpsert = got.MaxRowsPerUpsert
	}
	if got.MaxRowsPerTurn > 0 {
		q.MaxRowsPerTurn = got.MaxRowsPerTurn
	}
	if got.MaxRowsPerTable > 0 {
		q.MaxRowsPerTable = got.MaxRowsPerTable
	}
	if got.MaxRowBytes > 0 {
		q.MaxRowBytes = got.MaxRowBytes
	}
	if got.MaxTablesPerScope > 0 {
		q.MaxTablesPerScope = got.MaxTablesPerScope
	}
	if got.QueryDefaultLimit > 0 {
		q.QueryDefaultLimit = got.QueryDefaultLimit
	}
	if got.QueryMaxLimit > 0 {
		q.QueryMaxLimit = got.QueryMaxLimit
	}
	if got.MaxRowChars > 0 {
		q.MaxRowChars = got.MaxRowChars
	}
	return q
}

func (e *Engine) coreTableTools() []tool.Handler {
	e.mu.Lock()
	store := e.tableStore
	budget := e.tableBudget
	e.mu.Unlock()
	if store == nil {
		return nil
	}
	if budget == nil {
		budget = builtin.NewTableTurnBudget()
		e.mu.Lock()
		e.tableBudget = budget
		e.mu.Unlock()
	}
	quotas := func() domain.ConfigTableSection { return e.tableQuotas() }
	return []tool.Handler{
		&builtin.TableUpsert{Store: store, Quotas: quotas, Budget: budget},
		&builtin.TableGet{Store: store, Quotas: quotas},
		&builtin.TableQuery{Store: store, Quotas: quotas},
		&builtin.TableDelete{Store: store},
		&builtin.TableList{Store: store},
	}
}

// SetFileChangeStore wires the session file-change journal into the turn runner and compaction manager.
func (e *Engine) SetFileChangeStore(store interface {
	FileChangeAppender
	FileChangeJournal
}) {
	if store == nil {
		return
	}
	e.turnRunner.FileChanges = store
	e.compactionMgr.SetFileChangeJournal(store)
}

// SetPreTurnSnapshot wires AI-review pre-turn file snapshots (office-edit + explicit paths).
func (e *Engine) SetPreTurnSnapshot(fn func(ctx context.Context, projectID, sessionID, turnID, userInput string, extraPaths []string)) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.preTurnSnapshot = fn
}

// SetSandbox wires the process sandbox used for policy decisions and tool execution status.
func (e *Engine) SetSandbox(sb port.Sandbox) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.sandbox = sb
}

// SetExecution wires the LocalOS / OCI execution backend for EffectiveIsolation.
func (e *Engine) SetExecution(ex port.ExecutionBackend) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.execution = ex
}

func (e *Engine) sandboxStatus() domain.SandboxStatus {
	e.mu.Lock()
	sb := e.sandbox
	e.mu.Unlock()
	if sb == nil {
		return domain.SandboxStatus{Enabled: false, Backend: domain.SandboxBackendDisabled}
	}
	return sb.Status()
}

func (e *Engine) environmentStatus() domain.EnvironmentStatus {
	e.mu.Lock()
	ex := e.execution
	e.mu.Unlock()
	if ex == nil {
		return domain.EnvironmentStatus{}
	}
	return ex.Status()
}

func (e *Engine) effectiveIsolation() domain.EffectiveIsolation {
	e.mu.Lock()
	sb := e.sandbox
	ex := e.execution
	e.mu.Unlock()
	sbSt := domain.SandboxStatus{Enabled: false, Backend: domain.SandboxBackendDisabled}
	if sb != nil {
		sbSt = sb.Status()
	}
	envSt := domain.EnvironmentStatus{}
	if ex != nil {
		envSt = ex.Status()
	}
	return domain.ComputeEffectiveIsolation(sbSt, envSt)
}

func (e *Engine) sessionAllowsNetwork(sessionID string) bool {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.sessionPerm[sessionID].AllowNetwork
}

func (e *Engine) sessionAllowDomains(sessionID string) []string {
	e.mu.Lock()
	defer e.mu.Unlock()
	d := e.sessionPerm[sessionID].AllowedDomains
	if len(d) == 0 {
		return nil
	}
	out := make([]string, len(d))
	copy(out, d)
	return out
}

func (e *Engine) grantSessionDomains(sessionID string, domains []string) {
	e.mu.Lock()
	sb := e.sandbox
	e.mu.Unlock()
	if sb != nil {
		sb.GrantSessionDomains(sessionID, domains)
	}
}

func (e *Engine) grantTurnDomains(turnID string, domains []string) {
	e.mu.Lock()
	sb := e.sandbox
	e.mu.Unlock()
	if sb != nil {
		sb.GrantTurnDomains(turnID, domains)
	}
}

func (e *Engine) clearTurnDomains(turnID string) {
	e.mu.Lock()
	sb := e.sandbox
	e.mu.Unlock()
	if sb != nil {
		sb.ClearTurnDomains(turnID)
	}
}

// RevokeSessionNetworkGrants clears Soft + Hard session network grants.
func (e *Engine) RevokeSessionNetworkGrants(sessionID string) {
	e.mu.Lock()
	delete(e.sessionPerm, sessionID)
	sb := e.sandbox
	e.mu.Unlock()
	if sb != nil {
		sb.RevokeSessionDomains(sessionID)
	}
}

func (e *Engine) RegisterTool(h tool.Handler) {
	if rs, ok := h.(*builtin.ReadSkill); ok {
		e.readSkill = rs
	}
	e.toolCatalog.Register(h)
}

// SetGitHubMCPReady wires readiness checks for the builtin github expert pack.
func (e *Engine) SetGitHubMCPReady(fn func(ctx context.Context) bool) {
	e.githubMCPReady = fn
}

// SetMCPCaller wires the MCP tool executor used by catalog handlers.
func (e *Engine) SetMCPCaller(c port.MCPCaller) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.mcpCaller = c
}

// ReplaceMCPServer implements port.MCPToolSync.
func (e *Engine) ReplaceMCPServer(serverID string, tools []domain.MCPToolBinding, ambientMount bool) {
	e.mu.Lock()
	caller := e.mcpCaller
	e.mu.Unlock()
	handlers := make([]tool.Handler, 0, len(tools))
	for _, b := range tools {
		handlers = append(handlers, tool.NewMCPHandler(b, func(ctx context.Context, sid, name string, args map[string]any) (string, error) {
			if caller == nil {
				return "", fmt.Errorf("mcp caller not configured")
			}
			return caller.CallTool(ctx, sid, name, args)
		}))
	}
	e.toolCatalog.ReplaceServerAmbient(serverID, ambientMount, handlers...)
}

// RemoveMCPServer implements port.MCPToolSync.
func (e *Engine) RemoveMCPServer(serverID string) {
	e.toolCatalog.RemoveServer(serverID)
}

func (e *Engine) StartSession(ctx context.Context, s domain.Session, attachments []domain.UserAttachment) {
	turnID := newRuntimeID("turn")
	atts := append([]domain.UserAttachment(nil), attachments...)
	if err := e.reserveSessionTurn(s.ID, turnID); err != nil {
		e.publishTurnFailed(ctx, s.ID, turnID, domain.TurnFailed, err.Error())
		return
	}
	go func() {
		defer e.finishSessionTurn(s.ID, turnID)
		turnCtx, cancel := context.WithCancel(context.Background())
		e.mu.Lock()
		e.cancel[turnID] = cancel
		e.mu.Unlock()
		defer func() {
			e.mu.Lock()
			delete(e.cancel, turnID)
			e.mu.Unlock()
			cancel()
		}()

		agent, err := e.agents.Get(ctx, s.AgentID)
		if err != nil {
			e.publishTurnFailed(ctx, s.ID, turnID, domain.TurnFailed, err.Error())
			return
		}
		agentPtr := *agent

		reg, skills := e.setupRegistry(s, agentPtr, s.PlanMode)
		rep, err := e.runTurn(turnCtx, s.ID, turnID, s.Content, s.ModelID, s.ProjectID, agentPtr, reg, skills, atts, nil, s.PlanMode)
		e.turnLog.EndTurn(turnID, turnStatus(err, rep))
	}()
}

func (e *Engine) StartTurn(ctx context.Context, sessionID, userInput, agentID, modelID string, attachments []domain.UserAttachment) (string, error) {
	turnID := newRuntimeID("turn")
	if err := e.reserveSessionTurn(sessionID, turnID); err != nil {
		return "", err
	}
	atts := append([]domain.UserAttachment(nil), attachments...)
	extraSnap := service.SnapshotPathsFromCtx(ctx)
	// Reset session status to active so UI shows "运行中"
	e.updateSessionStatus(sessionID, domain.SessionStatusActive)
	go func() {
		defer e.finishSessionTurn(sessionID, turnID)
		// Do not use the HTTP request ctx — it is cancelled when StartTurn returns.
		bg := context.Background()
		s, err := e.sessions.Get(bg, sessionID)
		if err != nil {
			return
		}
		targetAgentID := agentID
		if targetAgentID == "" {
			targetAgentID = s.AgentID
		}
		targetModelID := modelID
		if targetModelID == "" {
			targetModelID = s.ModelID
		}
		agent, err := e.agents.Get(bg, targetAgentID)
		if err != nil {
			return
		}
		agentPtr := *agent

		turnCtx, cancel := context.WithCancel(context.Background())
		e.mu.Lock()
		e.cancel[turnID] = cancel
		e.mu.Unlock()
		defer func() {
			e.mu.Lock()
			delete(e.cancel, turnID)
			e.mu.Unlock()
			cancel()
		}()

		reg, skills := e.setupRegistry(s, agentPtr, s.PlanMode)
		rep, err := e.runTurn(turnCtx, sessionID, turnID, userInput, targetModelID, s.ProjectID, agentPtr, reg, skills, atts, extraSnap, s.PlanMode)
		e.turnLog.EndTurn(turnID, turnStatus(err, rep))
	}()
	return turnID, nil
}

func (e *Engine) CancelTurn(ctx context.Context, turnID string) {
	bg := context.Background()

	// Interrupt the in-memory goroutine when this process owns it.
	e.mu.Lock()
	cancel, ok := e.cancel[turnID]
	e.mu.Unlock()
	if ok {
		cancel()
	}

	t, err := e.turns.Get(bg, turnID)
	if err != nil {
		return
	}

	// Child/delegate turns share the parent context and may not be in e.cancel.
	// Cancel every other running turn in the session and clear their DB status
	// so Composer leaves the "running" state immediately.
	turns, _ := e.turns.ListBySession(bg, t.SessionID)
	for _, other := range turns {
		if other.ID == turnID || other.Status != domain.TurnRunning {
			continue
		}
		e.mu.Lock()
		c, found := e.cancel[other.ID]
		e.mu.Unlock()
		if found {
			c()
		}
		_ = e.turns.UpdateStatus(bg, other.ID, domain.TurnCancelled)
		e.publishTurnFailed(bg, t.SessionID, other.ID, domain.TurnCancelled, "cancelled")
		// Do not wait for a stuck child goroutine to finishSessionTurn.
		e.releaseSessionTurn(t.SessionID, other.ID)
	}

	// Eagerly persist cancel even when cancel() was called above. Previously we
	// returned after cancel() and relied on the turn goroutine to update DB —
	// if that goroutine was blocked (or owned by another process sharing the
	// DB), the Composer stop button looked broken while status stayed "running".
	if t.Status == domain.TurnRunning {
		_ = e.turns.UpdateStatus(bg, turnID, domain.TurnCancelled)
		e.publishTurnFailed(bg, t.SessionID, turnID, domain.TurnCancelled, "cancelled")
		e.updateSessionStatus(t.SessionID, domain.SessionStatusCompleted)
	}
	// Always clear the in-memory reservation. finishSessionTurn may never run if
	// the turn goroutine is blocked ignoring cancel (e.g. hung LLM stream).
	e.releaseSessionTurn(t.SessionID, turnID)
}

func (e *Engine) ResumeTurn(ctx context.Context, sessionID, turnID string) error {
	t, err := e.turns.Get(ctx, turnID)
	if err != nil {
		return fmt.Errorf("load turn %s: %w", turnID, err)
	}
	if t.SessionID != sessionID {
		return fmt.Errorf("turn %s belongs to session %s, not %s", turnID, t.SessionID, sessionID)
	}
	if err := e.reserveSessionTurn(sessionID, turnID); err != nil {
		return err
	}
	go func() {
		defer e.finishSessionTurn(sessionID, turnID)
		cfg := e.loadRunCfg(ctx)

		s, err := e.sessions.Get(ctx, sessionID)
		if err != nil {
			return
		}
		agent, err := e.agents.Get(ctx, s.AgentID)
		if err != nil {
			return
		}
		agentPtr := *agent

		turnCtx, cancel := context.WithCancel(context.Background())
		// Reset session status to active so UI shows "运行中" during resume
		e.updateSessionStatus(sessionID, domain.SessionStatusActive)

		e.mu.Lock()
		e.cancel[turnID] = cancel
		e.mu.Unlock()
		defer func() {
			e.mu.Lock()
			delete(e.cancel, turnID)
			e.mu.Unlock()
			cancel()
		}()

		// Close crash-orphaned tool pairs before rebuilding history so resume
		// sees real failures (same contract as RecoverRunning).
		e.closeIncompleteToolPairs(sessionID, turnID)

		goal := ""
		if g, entries := e.turnLog.LoadForRecovery(turnID); g != "" || len(entries) > 0 {
			goal = g
		}
		if goal == "" {
			if t, err := e.turns.Get(ctx, turnID); err == nil && t.Goal != "" {
				goal = t.Goal
			}
		}
		if goal == "" {
			goal = s.Content
		}

		reg, skills := e.setupRegistry(s, agentPtr, s.PlanMode)
		runner := e.spawnTurnRunner(turnID, reg, skills, agentPtr.Tools)
		e.stream.Publish(turnCtx, sessionID, turnID, domain.EventTurnStarted, domain.TurnStartedPayload{
			TurnID: turnID, AgentID: agentPtr.ID, Goal: goal,
		})
		e.stream.Publish(turnCtx, sessionID, turnID, domain.EventUserMessage, domain.UserMessagePayload{Content: goal})
		// Create reopens existing JSONL for append (no duplicate start) when present.
		_ = e.turnLog.Create(turnID, sessionID, s.ProjectID, agentPtr.ID, goal)
		_ = e.turns.Create(turnCtx, domain.TurnLog{ID: turnID, SessionID: sessionID, AgentID: agentPtr.ID, Goal: goal, Status: domain.TurnRunning})
		_ = e.turns.UpdateStatus(turnCtx, turnID, domain.TurnRunning)

		checkpoint := e.compactionMgr.Recover(turnCtx, sessionID)
		checkpointText := ""
		activeTodos := ""
		fileChanges := ""
		if checkpoint != nil {
			if checkpoint.Summary != "" {
				checkpointText = checkpoint.Summary
			}
			activeTodos = formatActiveTodos(checkpoint.Todos)
			fileChanges = formatFileChanges(checkpoint.FileChanges)
		}

		sys := buildSystemPrompt(agentPtr.SystemPrompt, skills, e.delegatableAgents(agentPtr), agentPtr.CanDelegate, s.PlanMode, checkpointText, activeTodos, fileChanges, e.sandboxStatus(), e.environmentStatus())
		messages := []Message{{Role: RoleSystem, Content: sys}}
		if hits := e.knowledge.Search(agentPtr.KnowledgeIDs, goal, cfg.knowledgeSearchTopK); len(hits) > 0 {
			content := ""
			for _, h := range hits {
				content += h + "\n"
			}
			messages = append(messages, Message{Role: RoleSystem, Content: content})
		}

		// Full session history from disk, including this turn's complete tool prefix.
		history := e.loadRetainedHistory(sessionID, checkpoint)
		messages = append(messages, history...)
		// Deduplicate against THIS turn's JSONL only. Scanning the whole
		// session history would false-positive when the goal text matches an
		// earlier turn's user message and skip appending this turn's goal.
		thisTurn := chatMessagesToRuntime(e.turnLog.LoadTurnMessages(turnID))
		if !historyHasUserGoal(thisTurn, goal) {
			e.turnLog.Append(turnID, "user", map[string]any{"content": goal})
			messages = append(messages, Message{Role: RoleUser, Content: goal})
		}

		workDir := e.resolveWorkDir(turnCtx, s.ProjectID)
		rep, _, err := runner.Run(turnCtx, TurnContext{
			SessionID: sessionID, TurnID: turnID, Agent: agentPtr,
			Model: s.ModelID, MaxSteps: agentPtr.Steps, WorkDir: workDir, ProjectID: s.ProjectID, Messages: messages,
			Path: []domain.TurnPathEntry{{TurnID: turnID, AgentID: agentPtr.ID}},
			PlanMode: s.PlanMode,
		})

		e.clearSessionTurnMessages(sessionID)
		e.turnLog.EndTurn(turnID, turnStatus(err, rep))
		e.afterTurn(sessionID, turnID, agentPtr.ID, rep, err, nil, s.ModelID)
	}()
	return nil
}

func (e *Engine) reserveSessionTurn(sessionID, turnID string) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.activeTurns == nil {
		e.activeTurns = make(map[string]string)
	}
	if activeTurnID := e.activeTurns[sessionID]; activeTurnID != "" {
		return fmt.Errorf("%w: session %s is running turn %s", port.ErrSessionTurnRunning, sessionID, activeTurnID)
	}
	e.activeTurns[sessionID] = turnID
	return nil
}

func (e *Engine) releaseSessionTurn(sessionID, turnID string) {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.activeTurns[sessionID] == turnID {
		delete(e.activeTurns, sessionID)
	}
}

func (e *Engine) ActiveTurnID(sessionID string) string {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.activeTurns == nil {
		return ""
	}
	return e.activeTurns[sessionID]
}

func (e *Engine) finishSessionTurn(sessionID, turnID string) {
	e.releaseSessionTurn(sessionID, turnID)
	if e.sessions != nil {
		// Soft-steer marks that were not claimed before the turn ended fall
		// back to the normal next-turn queue.
		if err := e.sessions.DemoteSteering(context.Background(), sessionID); err != nil {
			log.Printf("[steer] demote leftover steering session %s: %v", sessionID, err)
		}
		go e.sessions.DrainPendingQueue(context.Background(), sessionID)
	}
}

func historyHasUserGoal(history []Message, goal string) bool {
	for _, m := range history {
		if m.Role == RoleUser && m.Content == goal {
			return true
		}
	}
	return false
}

func (e *Engine) RecoverRunning(ctx context.Context) {
	runningTurns, err := e.turns.ListByStatus(ctx, domain.TurnRunning)
	if err != nil {
		log.Printf("[RecoverRunning] list running turns: %v", err)
		return
	}
	runningBySession := make(map[string][]domain.TurnLog)
	for _, t := range runningTurns {
		runningBySession[t.SessionID] = append(runningBySession[t.SessionID], t)
	}
	// If a crash left multiple parent turns running for one session, recover the
	// newest one and fail older conflicts instead of starting concurrent replay.
	sort.SliceStable(runningTurns, func(i, j int) bool {
		return runningTurns[i].ID > runningTurns[j].ID
	})

	// 1. Expire stale approvals and publish decided events so UI hides old buttons.
	pendingApprovals, err := e.approvals.ListByStatus(ctx, "pending")
	if err != nil {
		log.Printf("[RecoverRunning] list pending approvals: %v", err)
	} else if len(pendingApprovals) > 0 {
		log.Printf("[RecoverRunning] found %d stale pending approval(s), marking as expired", len(pendingApprovals))
		for _, a := range pendingApprovals {
			a.Status = "expired"
			if err := e.approvals.Update(ctx, a); err != nil {
				log.Printf("[RecoverRunning] update approval %s: %v", a.ID, err)
			}
			turnID := e.resolveApprovalTurnID(a, runningBySession)
			if a.SessionID != "" && turnID != "" {
				e.PublishPermissionDecided(a.SessionID, turnID, a.ID, false, "once")
			}
		}
	}

	// 2. Resume recoverable zombie turns from last complete tool pairs; fail the rest.
	//    Lifecycle: close open tools (JSONL + stream) → then LoadForRecovery / ResumeTurn.
	resumedSessions := make(map[string]bool)
	if len(runningTurns) > 0 {
		log.Printf("[RecoverRunning] found %d zombie running turn(s)", len(runningTurns))
	}
	for _, t := range runningTurns {
		// Nested tool-run turns (e.g. mid-flight delegate_agent) are not
		// parent session turns — close their open tools, fail them, and let
		// the parent closer emit tool.error / delegate.completed.
		if e.turnLog.IsNestedToolRun(t.ID) {
			log.Printf("[RecoverRunning] turn %s is nested tool run, marking as failed", t.ID)
			e.closeIncompleteToolPairs(t.SessionID, t.ID)
			if err := e.turns.UpdateStatus(ctx, t.ID, domain.TurnFailed); err != nil {
				log.Printf("[RecoverRunning] update turn %s status: %v", t.ID, err)
			}
			_ = e.turnLog.CreateNested(t.ID, t.SessionID, "", t.AgentID, t.Goal)
			e.turnLog.EndTurn(t.ID, domain.TurnFailed)
			e.publishTurnFailed(context.Background(), t.SessionID, t.ID, domain.TurnFailed, recoveryToolClosedReason)
			continue
		}

		// Materialize crash-orphaned tool failures before trim/resume so the
		// LLM and UI see the same failed pairs a live Execute error would produce.
		e.closeIncompleteToolPairs(t.SessionID, t.ID)

		// Recoverable only when JSONL exists (start goal and/or complete tool pairs).
		// DB Goal alone is not enough — injected zombies without work must stay failed.
		goal, entries := e.turnLog.LoadForRecovery(t.ID)
		if goal == "" && len(entries) == 0 {
			log.Printf("[RecoverRunning] turn %s not recoverable, marking as failed", t.ID)
			if err := e.turns.UpdateStatus(ctx, t.ID, domain.TurnFailed); err != nil {
				log.Printf("[RecoverRunning] update turn %s status: %v", t.ID, err)
			}
			_ = e.turnLog.Create(t.ID, t.SessionID, "", t.AgentID, t.Goal)
			e.turnLog.EndTurn(t.ID, domain.TurnFailed)
			continue
		}
		log.Printf("[RecoverRunning] auto-resuming turn %s (session %s) from %d tool pair entr(y/ies)", t.ID, t.SessionID, len(entries))
		resumedSessions[t.SessionID] = true
		if err := e.ResumeTurn(ctx, t.SessionID, t.ID); err != nil {
			log.Printf("[RecoverRunning] cannot resume turn %s: %v", t.ID, err)
			if errors.Is(err, port.ErrSessionTurnRunning) {
				_ = e.turns.UpdateStatus(ctx, t.ID, domain.TurnFailed)
				e.turnLog.EndTurn(t.ID, domain.TurnFailed)
			}
		}
	}

	// 3. Recover stuck sessions that were not auto-resumed.
	sessions, err := e.sessions.List(ctx)
	if err != nil {
		log.Printf("[RecoverRunning] list sessions: %v", err)
		return
	}
	for _, s := range sessions {
		if s.Status != domain.SessionStatusActive {
			continue
		}
		if resumedSessions[s.ID] {
			continue
		}
		turns, err := e.turns.ListBySession(ctx, s.ID)
		if err != nil {
			continue
		}
		hasRunning := false
		hasFailed := false
		for _, t := range turns {
			switch t.Status {
			case domain.TurnRunning:
				hasRunning = true
			case domain.TurnFailed, domain.TurnCancelled, domain.TurnTimeout:
				hasFailed = true
			}
		}
		if hasRunning {
			continue
		}
		status := domain.SessionStatusCompleted
		if hasFailed || len(turns) == 0 {
			status = domain.SessionStatusFailed
		}
		log.Printf("[RecoverRunning] session %s stuck in active with no running turns, marking as %s", s.ID, status)
		s.Status = status
		s.UpdatedAt = time.Now().UTC()
		_ = e.sessions.UpdateSession(ctx, s)
	}
}

// resolveApprovalTurnID finds a turn ID for publishing permission.decided after expiry.
func (e *Engine) resolveApprovalTurnID(a domain.Approval, runningBySession map[string][]domain.TurnLog) string {
	if a.TurnID != "" {
		return a.TurnID
	}
	if a.SessionID != "" {
		for _, ev := range e.stream.ListSince(a.SessionID, 0) {
			if ev.Type != domain.EventPermissionAsk {
				continue
			}
			var p domain.PermissionAskPayload
			if json.Unmarshal(ev.Payload, &p) != nil {
				continue
			}
			if p.ApprovalID == a.ID && ev.TurnID != "" {
				return ev.TurnID
			}
		}
		if turns := runningBySession[a.SessionID]; len(turns) == 1 {
			return turns[0].ID
		}
	}
	return ""
}

// recoveryToolClosedReason is written into synthetic tool_result / tool.error /
// delegate.completed payloads when a crash leaves a tool pair unfinished.
const recoveryToolClosedReason = "expired (process restarted)"

type recoveryDelegateMeta struct {
	agentID     string
	childTurnID string
}

type recoveryTurnStreamState struct {
	terminal          map[string]bool
	names             map[string]string
	inputs            map[string]map[string]any
	openFromStream    map[string]bool
	delegates         map[string]recoveryDelegateMeta
	delegateCompleted map[string]bool
	delegateChildDone map[string]bool // ChildTurnID — covers legacy completed without CallID
	turnTerminal      bool
}

func (e *Engine) scanTurnStreamState(sessionID, turnID string) recoveryTurnStreamState {
	st := recoveryTurnStreamState{
		terminal:          make(map[string]bool),
		names:             make(map[string]string),
		inputs:            make(map[string]map[string]any),
		openFromStream:    make(map[string]bool),
		delegates:         make(map[string]recoveryDelegateMeta),
		delegateCompleted: make(map[string]bool),
		delegateChildDone: make(map[string]bool),
	}
	if e.stream == nil || sessionID == "" || turnID == "" {
		return st
	}
	for _, ev := range e.stream.ListSince(sessionID, 0) {
		if ev.TurnID != turnID {
			continue
		}
		switch ev.Type {
		case domain.EventToolPending, domain.EventToolRunning:
			var p domain.ToolPart
			if json.Unmarshal(ev.Payload, &p) != nil || p.CallID == "" {
				continue
			}
			if !st.terminal[p.CallID] {
				st.openFromStream[p.CallID] = true
			}
			if p.Name != "" {
				st.names[p.CallID] = p.Name
			}
			if p.Input != nil {
				st.inputs[p.CallID] = p.Input
			}
		case domain.EventToolCompleted, domain.EventToolError:
			var p domain.ToolPart
			if json.Unmarshal(ev.Payload, &p) != nil || p.CallID == "" {
				continue
			}
			st.terminal[p.CallID] = true
			delete(st.openFromStream, p.CallID)
			if p.Name != "" {
				st.names[p.CallID] = p.Name
			}
		case domain.EventAskUserPending:
			var p domain.AskUserPayload
			if json.Unmarshal(ev.Payload, &p) != nil {
				continue
			}
			callID := p.CallID
			if callID == "" {
				callID = p.AskID
			}
			if callID == "" || st.terminal[callID] {
				continue
			}
			st.openFromStream[callID] = true
			st.names[callID] = "ask_user"
		case domain.EventDelegateStarted:
			var p domain.DelegateStartedPayload
			if json.Unmarshal(ev.Payload, &p) != nil || p.CallID == "" {
				continue
			}
			st.delegates[p.CallID] = recoveryDelegateMeta{
				agentID: p.AgentID, childTurnID: p.ChildTurnID,
			}
			if st.names[p.CallID] == "" {
				st.names[p.CallID] = "delegate_agent"
			}
		case domain.EventDelegateCompleted:
			var p domain.DelegateCompletedPayload
			if json.Unmarshal(ev.Payload, &p) != nil {
				continue
			}
			if p.CallID != "" {
				st.delegateCompleted[p.CallID] = true
			}
			if p.ChildTurnID != "" {
				st.delegateChildDone[p.ChildTurnID] = true
			}
		case domain.EventTurnFailed, domain.EventTurnEnded, domain.EventReport:
			st.turnTerminal = true
		}
	}
	return st
}

type recoveryPendingClose struct {
	callID     string
	name       string
	input      map[string]any
	needsJSONL bool
}

// closeIncompleteToolPairs finishes crash-orphaned tool calls the same way a
// normal Execute failure would: append tool_result to JSONL, publish tool.error,
// and for delegate_agent also settle the child turn + delegate.completed.
// Idempotent across multiple recoveries / ResumeTurn calls.
func (e *Engine) closeIncompleteToolPairs(sessionID, turnID string) {
	if sessionID == "" || turnID == "" || e.stream == nil || e.turnLog == nil {
		return
	}
	bg := context.Background()
	fromJSONL := e.turnLog.ListIncompleteToolCalls(turnID)
	st := e.scanTurnStreamState(sessionID, turnID)

	order := make([]string, 0, len(fromJSONL)+len(st.openFromStream)+len(st.delegates))
	byID := make(map[string]*recoveryPendingClose, len(fromJSONL)+len(st.openFromStream)+len(st.delegates))
	add := func(callID, name string, input map[string]any, needsJSONL bool) {
		if callID == "" {
			return
		}
		if existing, ok := byID[callID]; ok {
			if existing.name == "" && name != "" {
				existing.name = name
			}
			if existing.input == nil && input != nil {
				existing.input = input
			}
			existing.needsJSONL = existing.needsJSONL || needsJSONL
			return
		}
		if name == "" {
			name = st.names[callID]
		}
		if input == nil {
			input = st.inputs[callID]
		}
		byID[callID] = &recoveryPendingClose{
			callID: callID, name: name, input: input, needsJSONL: needsJSONL,
		}
		order = append(order, callID)
	}

	for _, c := range fromJSONL {
		add(c.CallID, c.Name, c.Input, true)
	}
	for callID := range st.openFromStream {
		add(callID, st.names[callID], st.inputs[callID], false)
	}
	for callID := range st.delegates {
		if st.terminal[callID] && st.delegateCompleted[callID] {
			continue
		}
		add(callID, "delegate_agent", st.inputs[callID], false)
	}
	if len(order) == 0 {
		return
	}

	needsAppend := false
	for _, id := range order {
		if byID[id].needsJSONL {
			needsAppend = true
			break
		}
	}
	if needsAppend {
		_ = e.turnLog.Create(turnID, sessionID, "", "", "")
	}

	for _, callID := range order {
		info := byID[callID]
		name := info.name
		if name == "" {
			name = "tool"
		}
		if info.needsJSONL {
			e.turnLog.Append(turnID, "tool_result", map[string]any{
				"call_id": callID, "name": name, "output": recoveryToolClosedReason,
			})
		}
		if !st.terminal[callID] {
			e.stream.Publish(bg, sessionID, turnID, domain.EventToolError, domain.ToolPart{
				CallID: callID, Name: name, Status: domain.ToolError,
				Error: recoveryToolClosedReason, Input: info.input,
			})
			st.terminal[callID] = true
		}

		meta, isDelegate := st.delegates[callID]
		if !isDelegate && name != "delegate_agent" {
			continue
		}
		if meta.agentID == "" {
			if agentID, _ := info.input["agent_id"].(string); agentID != "" {
				meta.agentID = agentID
			}
		}
		e.settleRecoveredDelegate(bg, sessionID, turnID, callID, meta, &st)
	}
}

func (e *Engine) settleRecoveredDelegate(
	ctx context.Context,
	sessionID, parentTurnID, callID string,
	meta recoveryDelegateMeta,
	st *recoveryTurnStreamState,
) {
	childTurnID := meta.childTurnID
	status := string(domain.TurnFailed)
	if childTurnID != "" {
		childDone := false
		if t, err := e.turns.Get(ctx, childTurnID); err == nil {
			switch t.Status {
			case domain.TurnRunning, "":
				_ = e.turns.UpdateStatus(ctx, childTurnID, domain.TurnFailed)
			default:
				// Already finished — keep its status; never rewrite a completed child
				// into failed just because the parent tool pair was orphaned.
				childDone = true
				status = string(t.Status)
			}
		}
		childState := e.scanTurnStreamState(sessionID, childTurnID)
		if childState.turnTerminal {
			childDone = true
		}
		if !childDone {
			if e.turnLog.IsNestedToolRun(childTurnID) {
				_ = e.turnLog.CreateNested(childTurnID, sessionID, "", meta.agentID, "")
				e.turnLog.EndTurn(childTurnID, domain.TurnFailed)
			}
			e.publishTurnFailed(ctx, sessionID, childTurnID, domain.TurnFailed, recoveryToolClosedReason)
		}
	}
	if st.delegateCompleted[callID] {
		return
	}
	// Legacy delegate.completed lacked CallID; ChildTurnID is enough to skip a
	// duplicate completed event for the same nested run.
	if childTurnID != "" && st.delegateChildDone[childTurnID] {
		st.delegateCompleted[callID] = true
		return
	}
	summary := recoveryToolClosedReason
	if status != string(domain.TurnFailed) {
		summary = ""
	}
	e.stream.Publish(ctx, sessionID, parentTurnID, domain.EventDelegateCompleted, domain.DelegateCompletedPayload{
		AgentID: meta.agentID, Status: status, Summary: summary,
		ChildTurnID: childTurnID, CallID: callID,
	})
	st.delegateCompleted[callID] = true
	if childTurnID != "" {
		st.delegateChildDone[childTurnID] = true
	}
}

func (e *Engine) ListTurns(sessionID string) []domain.TurnLog {
	turns, err := e.turns.ListBySession(context.Background(), sessionID)
	if err != nil {
		return nil
	}
	return turns
}

func (e *Engine) setupRegistry(s domain.Session, agent domain.Agent, planMode bool) (*tool.Registry, []domain.Skill) {
	workDir := e.resolveWorkDir(context.Background(), s.ProjectID)
	skills := e.resolveAgentSkills(agent, workDir)

	if e.readSkill != nil {
		e.readSkill.SetRoots(e.dataDir, workDir)
	}

	var reg *tool.Registry
	if agentHasDelegation(agent) {
		reg = e.buildTeamRegistry(agent, planMode)
	} else {
		reg = e.buildWorkerRegistry(agent, planMode)
	}
	return reg, skills
}

// spawnTurnRunner builds an isolated runner for one turn so concurrent sessions
// cannot race on Log / Registry / SkillList of the shared template runner.
func (e *Engine) spawnTurnRunner(turnID string, reg *tool.Registry, skills []domain.Skill, bindings []domain.ToolBinding) *TurnRunner {
	r := NewTurnRunner(e.llm, e.stream, e.turnRunner.Perm, reg, e.configStore)
	r.Approval = e
	r.SandboxStatus = e.sandboxStatus
	r.EffectiveIsolation = e.effectiveIsolation
	r.SessionAllowNetwork = e.sessionAllowsNetwork
	r.SessionAllowDomains = e.sessionAllowDomains
	r.GrantSessionDomains = e.grantSessionDomains
	r.GrantTurnDomains = e.grantTurnDomains
	r.ClearTurnDomains = e.clearTurnDomains
	r.FileChanges = e.turnRunner.FileChanges
	r.SkillList = skills
	r.ToolBindings = bindings
	r.Log = func(typ string, data map[string]any) {
		e.turnLog.Append(turnID, typ, data)
	}
	return r
}

func agentHasDelegation(agent domain.Agent) bool {
	return agent.CanDelegate
}

// delegatableAgents returns the agent list to inject into the system prompt.
// Only agents with delegation enabled receive it; the coordinator itself is excluded.
func (e *Engine) delegatableAgents(agent domain.Agent) []domain.Agent {
	if !agent.CanDelegate {
		return nil
	}
	all, _ := e.agents.List(context.Background())
	var result []domain.Agent
	for _, a := range all {
		if a.ID == agent.ID {
			continue
		}
		result = append(result, a)
	}
	return result
}

func homeDir() string {
	h, _ := os.UserHomeDir()
	return h
}

func (e *Engine) resolveAgentSkills(agent domain.Agent, workDir string) []domain.Skill {
	allSkills := service.ScanAllSkills(e.dataDir, workDir)
	for i := range allSkills {
		allSkills[i].PromptPath = builtin.SkillPathForPrompt(allSkills[i].Dir, e.dataDir, homeDir()+"/.agents", workDir)
	}

	if agent.Mode == domain.AgentModeSubagent {
		return service.BoundSkills(allSkills, agent)
	}

	bound := service.BoundSkills(allSkills, agent)
	orphan := service.OrphanSkills(allSkills, agent)
	return service.MergeSkillsByID(bound, orphan)
}

func (e *Engine) boundSkills(agent domain.Agent) []domain.Skill {
	all, _ := e.skills.List(context.Background())
	return service.BoundSkills(all, agent)
}

func (e *Engine) runTurn(ctx context.Context, sessionID, turnID, goal, modelID, projectID string, agent domain.Agent, reg *tool.Registry, skills []domain.Skill, attachments []domain.UserAttachment, extraSnapshotPaths []string, planMode bool) (domain.Report, error) {
	cfg := e.loadRunCfg(ctx)

	runner := e.spawnTurnRunner(turnID, reg, skills, agent.Tools)
	e.stream.Publish(ctx, sessionID, turnID, domain.EventTurnStarted, domain.TurnStartedPayload{
		TurnID: turnID, AgentID: agent.ID, Goal: goal,
	})
	e.stream.Publish(ctx, sessionID, turnID, domain.EventUserMessage, userMessagePayload(goal, attachments))

	e.turnLog.Create(turnID, sessionID, projectID, agent.ID, goal)
	_ = e.turns.Create(ctx, domain.TurnLog{ID: turnID, SessionID: sessionID, AgentID: agent.ID, Goal: goal, Status: domain.TurnRunning})

	// AI review: snapshot office-edit / Stage paths before any tool mutates disk.
	if e.preTurnSnapshot != nil {
		e.preTurnSnapshot(ctx, projectID, sessionID, turnID, goal, extraSnapshotPaths)
	}

	checkpoint := e.compactionMgr.Recover(ctx, sessionID)
	checkpointText := ""
	activeTodos := ""
	fileChanges := ""
	if checkpoint != nil {
		if checkpoint.Summary != "" {
			checkpointText = checkpoint.Summary
		}
		activeTodos = formatActiveTodos(checkpoint.Todos)
		fileChanges = formatFileChanges(checkpoint.FileChanges)
	}

	sys := buildSystemPrompt(agent.SystemPrompt, skills, e.delegatableAgents(agent), agent.CanDelegate, planMode, checkpointText, activeTodos, fileChanges, e.sandboxStatus(), e.environmentStatus())
	messages := []Message{
		{Role: RoleSystem, Content: sys},
	}

	if hits := e.knowledge.Search(agent.KnowledgeIDs, goal, cfg.knowledgeSearchTopK); len(hits) > 0 {
		content := ""
		for _, h := range hits {
			content += h + "\n"
		}
		messages = append(messages, Message{Role: RoleSystem, Content: content})
	}

	// Cross-turn history: full LLM messages from turn log (compaction bounds the window).
	messages = append(messages, e.loadRetainedHistory(sessionID, checkpoint)...)

	userMsg := userMessageFromAttachments(goal, attachments)
	e.turnLog.Append(turnID, "user", userMessageLogData(userMsg))
	messages = append(messages, userMsg)
	userIdx := len(messages) - 1

	workDir := e.resolveWorkDir(ctx, projectID)

	rep, turnMsgs, err := runner.Run(ctx, TurnContext{
		SessionID: sessionID,
		TurnID:    turnID,
		Agent:     agent,
		Model:     modelID,
		MaxSteps:  agent.Steps,
		WorkDir:   workDir,
		ProjectID: projectID,
		PlanMode:  planMode,
		Messages:  messages,
		Path:      []domain.TurnPathEntry{{TurnID: turnID, AgentID: agent.ID}},
		ClaimSteers: func() []Message {
			if e.sessions == nil {
				return nil
			}
			items, err := e.sessions.ClaimSteering(context.Background(), sessionID)
			if err != nil {
				log.Printf("[steer] claim session %s: %v", sessionID, err)
				return nil
			}
			out := make([]Message, 0, len(items))
			for _, it := range items {
				out = append(out, userMessageFromAttachments(it.Content, it.Attachments))
			}
			return out
		},
	})

	// History lives on disk; drop any in-memory session buffer after the turn.
	e.clearSessionTurnMessages(sessionID)
	_ = turnMsgs
	_ = userIdx

	e.afterTurn(sessionID, turnID, agent.ID, rep, err, nil, modelID)
	return rep, err
}

// clearSessionTurnMessages drops in-memory cross-turn history for a session.
// LLM history is reconstructed from turn logs on the next turn.
func (e *Engine) clearSessionTurnMessages(sessionID string) {
	e.mu.Lock()
	delete(e.turnMessages, sessionID)
	e.mu.Unlock()
}

func chatMessagesToRuntime(in []port.ChatMessage) []Message {
	out := make([]Message, 0, len(in))
	for _, m := range in {
		msg := Message{
			Role:       Role(m.Role),
			Content:    m.Content,
			ToolCallID: m.ToolCallID,
			Name:       m.Name,
		}
		if len(m.Parts) > 0 {
			parts := make([]ContentPart, len(m.Parts))
			for i, p := range m.Parts {
				parts[i] = ContentPart{Type: p.Type, MimeType: p.MimeType, Data: p.Data, Name: p.Name}
			}
			msg.Parts = parts
		}
		if len(m.ToolCalls) > 0 {
			tcs := make([]ToolCall, len(m.ToolCalls))
			for i, tc := range m.ToolCalls {
				tcs[i] = ToolCall{ID: tc.ID, Name: tc.Name, Arguments: tc.Arguments}
			}
			msg.ToolCalls = tcs
		}
		out = append(out, msg)
	}
	return out
}

func userMessageLogData(msg Message) map[string]any {
	data := map[string]any{"content": msg.Content}
	// v1: do not persist base64 image blobs in turn log.
	return data
}

// commitTurnMessages is retained for tests that simulate in-memory deltas.
// Production turns clear memory via clearSessionTurnMessages; history is on disk.
func (e *Engine) commitTurnMessages(sessionID string, prev []Message, turnMsgs []Message, userIdx int, runErr error) {
	if userIdx < 0 {
		userIdx = 0
	}
	if userIdx > len(turnMsgs) {
		userIdx = len(turnMsgs)
	}
	delta := turnMsgs[userIdx:]
	if runErr != nil {
		delta = salvagePairedTurnDelta(delta)
	}
	e.turnMessages[sessionID] = append(append([]Message(nil), prev...), delta...)
}

func (e *Engine) resolveWorkDir(ctx context.Context, projectID string) string {
	var dir string
	if projectID == "" {
		dir = e.dataDir
	} else {
		dir = e.projects.ResolveDir(ctx, projectID, e.dataDir)
	}
	// Ensure the working directory exists so tools can operate in it.
	_ = os.MkdirAll(dir, 0755)
	return dir
}

func (e *Engine) afterTurn(sessionID, turnID, agentID string, rep domain.Report, err error, messages []Message, model string) {
	// Evaluate compaction on every turn end, including failed/cancelled turns.
	// Sessions dominated by cancels would otherwise never advance the retain
	// cursor while their JSONL history keeps growing unbounded.
	defer func() {
		if e.compactionMgr == nil || e.turnLog == nil || e.turns == nil {
			return
		}
		turns, _ := e.turns.ListBySession(context.Background(), sessionID)
		e.maybeCompact(context.Background(), sessionID, turnID, len(turns), model, rep.MaxPromptTokens)
	}()

	// CancelTurn persists "cancelled" immediately for UI responsiveness. Do not
	// let a late-finishing goroutine resurrect the turn as completed/failed.
	if cur, getErr := e.turns.Get(context.Background(), turnID); getErr == nil && cur.Status == domain.TurnCancelled {
		if err != nil && errors.Is(err, context.Canceled) {
			e.updateSessionStatus(sessionID, domain.SessionStatusCompleted)
		}
		return
	}
	if err != nil {
		st := turnStatus(err, rep)
		summary := err.Error()
		sessionStatus := domain.SessionStatusFailed
		if st == domain.TurnCancelled {
			summary = "cancelled"
			// Intentional interrupt is not a hard failure for the session.
			sessionStatus = domain.SessionStatusCompleted
		}
		e.publishTurnFailed(context.Background(), sessionID, turnID, st, summary)
		e.updateSessionStatus(sessionID, sessionStatus)
		_ = e.turns.UpdateStatus(context.Background(), turnID, st)
		return
	}
	e.updateSessionStatus(sessionID, domain.SessionStatusCompleted)
	_ = e.turns.UpdateStatus(context.Background(), turnID, domain.TurnCompleted)
	e.stream.Publish(context.Background(), sessionID, turnID, domain.EventSessionCompleted, domain.SessionCompletedPayload{
		Summary: rep.Summary, Status: string(rep.Status),
	})
}

func (e *Engine) updateSessionStatus(sessionID string, status domain.SessionStatus) {
	if e.sessions == nil {
		return
	}
	s, err := e.sessions.Get(context.Background(), sessionID)
	if err != nil {
		return
	}
	s.Status = status
	s.UpdatedAt = time.Now().UTC()
	_ = e.sessions.UpdateSession(context.Background(), s)
}

func (e *Engine) maybeCompact(ctx context.Context, sessionID, turnID string, turnCount int, model string, maxPromptTokens int) {
	cp := e.compactionMgr.Recover(ctx, sessionID)
	retainFrom := ""
	retainSkip := 0
	if cp != nil {
		retainFrom = cp.RetainFromTurnID
		retainSkip = cp.RetainSkipMessages
	}

	type indexedMsg struct {
		msg       Message
		turnID    string
		idxInTurn int
	}
	var flat []indexedMsg
	for _, id := range e.turnLog.ListTurnIDs(sessionID) {
		if retainFrom != "" && id < retainFrom {
			continue
		}
		msgs := chatMessagesToRuntime(e.turnLog.LoadTurnMessages(id))
		start := 0
		if id == retainFrom && retainSkip > 0 {
			if retainSkip >= len(msgs) {
				continue
			}
			start = retainSkip
		}
		for i := start; i < len(msgs); i++ {
			flat = append(flat, indexedMsg{msg: msgs[i], turnID: id, idxInTurn: i})
		}
	}
	history := make([]Message, len(flat))
	for i := range flat {
		history[i] = flat[i].msg
	}
	tokenEstimate := estimateTokenCount(history)
	if !e.compactionMgr.ShouldCompact(sessionID, turnCount, tokenEstimate, maxPromptTokens, model) {
		return
	}
	cfg := e.compactionMgr.loadCfg(ctx)
	if cfg.cutTokens <= 0 || len(history) == 0 {
		return
	}
	keepStart := findKeepStart(history, cfg.cutTokens)
	if keepStart <= 0 {
		return
	}
	oldMessages := history[:keepStart]
	loc := flat[keepStart]
	newRetain := loc.turnID
	newSkip := loc.idxInTurn
	if newRetain == retainFrom && newSkip == retainSkip {
		return
	}
	if !e.compactionMgr.CompactToRetain(ctx, sessionID, turnID, oldMessages, history, turnCount, model, newRetain, newSkip, tokenEstimate) {
		return
	}
	e.clearSessionTurnMessages(sessionID)
}

// loadRetainedHistory loads compaction-bounded session messages.
// Window size is owned solely by maybeCompact (checkpoint retain cursor);
// do not re-truncate here — that path previously froze turns on huge history.
func (e *Engine) loadRetainedHistory(sessionID string, cp *domain.CompactionCheckpoint) []Message {
	retainFrom := ""
	skip := 0
	if cp != nil {
		retainFrom = cp.RetainFromTurnID
		skip = cp.RetainSkipMessages
	}
	return chatMessagesToRuntime(e.turnLog.LoadSessionMessages(sessionID, retainFrom, skip))
}

// alwaysOnBuiltinTools are mounted for every agent without requiring ToolBindings.
var alwaysOnBuiltinTools = []string{"read_skill"}

func (e *Engine) mountAlwaysOnBuiltins(reg *tool.Registry) {
	for _, id := range alwaysOnBuiltinTools {
		if h, ok := e.toolCatalog.Get(id); ok {
			reg.Register(h)
		}
	}
}

func (e *Engine) mountBuiltinTools(reg *tool.Registry, bindings []domain.ToolBinding) {
	for _, b := range bindings {
		if b.ToolID == "" || domain.IsCoreTool(b.ToolID) {
			// Core tools are wired in build*Registry; catalog stubs must not
			// overwrite them (ask_user OnAsk, KB scope, memory store, …).
			continue
		}
		if h, ok := e.toolCatalog.Get(b.ToolID); ok {
			reg.Register(h)
		}
	}
}

// checkDelegation enforces design constraints on the turn path:
// cycle = agent already on this path; depth = next child depth (len(path))
// against maxDelegationDepth (lead = 0). Parallel siblings share a parent
// path, so the same agent_id may fan out concurrently.
func checkDelegation(path []domain.TurnPathEntry, agentID string, maxDepth int) error {
	for _, frame := range path {
		if frame.AgentID == agentID {
			return fmt.Errorf("circular delegation: %s", agentID)
		}
	}
	// Lead is depth 0; len(path) is the depth of the next child turn.
	if len(path) > maxDepth {
		return fmt.Errorf("max delegation depth reached")
	}
	return nil
}

func appendTurnPath(path []domain.TurnPathEntry, turnID, agentID string) []domain.TurnPathEntry {
	next := make([]domain.TurnPathEntry, len(path)+1)
	copy(next, path)
	next[len(path)] = domain.TurnPathEntry{TurnID: turnID, AgentID: agentID}
	return next
}

func (e *Engine) buildTeamRegistry(agent domain.Agent, planMode bool) *tool.Registry {
	cfg := e.loadRunCfg(context.Background())
	delegator := &builtin.DelegateAgent{
		Stream: e.stream, Agents: e.agents,
		KnowledgeSearch: e.knowledge.Search,
		RunSubTurn: func(ctx context.Context, sessionID, modelID, parentTurnID, callID string, workerAgent domain.Agent, goal string, parentPath []domain.TurnPathEntry) (domain.Report, error) {
			childTurnID := newRuntimeID("turn")
			workDir := e.dataDir
			projectID := ""
			if s, err := e.sessions.Get(ctx, sessionID); err == nil {
				workDir = e.resolveWorkDir(ctx, s.ProjectID)
				projectID = s.ProjectID
			}
			if err := checkDelegation(parentPath, workerAgent.ID, cfg.teamMaxDelegationDepth); err != nil {
				return domain.Report{}, err
			}
			if workerAgent.ID == service.CodeGraphServerID {
				st := service.EnsureCodeGraphIndex(workDir)
				goal = service.CodeGraphIndexHint(st, workDir) + "\n\n" + goal
			}
			if workerAgent.ID == service.GitHubExpertID {
				mcpReady := e.githubMCPReady != nil && e.githubMCPReady(ctx)
				goal = service.GitHubAccessHint(mcpReady, service.ResolveGhBin(), service.ResolveGitBin()) + "\n\n" + goal
			}
			childPath := appendTurnPath(parentPath, childTurnID, workerAgent.ID)
			childCtx := TurnContext{
				SessionID: sessionID, TurnID: childTurnID,
				Agent: workerAgent, Model: modelID, MaxSteps: workerAgent.Steps,
				WorkDir: workDir, ProjectID: projectID, Path: childPath,
				PlanMode: planMode,
			}
			e.stream.Publish(ctx, sessionID, parentTurnID, domain.EventDelegateStarted, domain.DelegateStartedPayload{
				AgentID: workerAgent.ID, Goal: goal, ChildTurnID: childTurnID, CallID: callID,
			})

			// Isolated child runner so parallel delegate_agent calls do not
			// mutate the parent's Registry / SkillList / Log mid-flight.
			skills := e.resolveAgentSkills(workerAgent, workDir)
			var childReg *tool.Registry
			if agentHasDelegation(workerAgent) {
				childReg = e.buildTeamRegistry(workerAgent, planMode)
			} else {
				childReg = e.buildWorkerRegistry(workerAgent, planMode)
			}
			rs := &builtin.ReadSkill{}
			rs.SetRoots(e.dataDir, workDir)
			childReg.Register(rs)
			childRunner := e.spawnTurnRunner(childTurnID, childReg, skills, workerAgent.Tools)

			sys := buildSystemPrompt(workerAgent.SystemPrompt, skills, nil, workerAgent.CanDelegate, planMode, "", "", "", e.sandboxStatus(), e.environmentStatus())
			messages := []Message{
				{Role: RoleSystem, Content: sys},
			}
			if hits := e.knowledge.Search(workerAgent.KnowledgeIDs, goal, cfg.knowledgeSearchTopK); len(hits) > 0 {
				content := ""
				for _, h := range hits {
					content += h + "\n"
				}
				messages = append(messages, Message{Role: RoleSystem, Content: content})
			}
			messages = append(messages, Message{Role: RoleUser, Content: goal})
			childCtx.Messages = messages

			// Nested tool-run log for zip/debug only — not parent LLM history.
			e.turnLog.CreateNested(childTurnID, sessionID, projectID, workerAgent.ID, goal)
			_ = e.turns.Create(ctx, domain.TurnLog{ID: childTurnID, SessionID: sessionID, AgentID: workerAgent.ID, Goal: goal, Status: domain.TurnRunning})
			e.turnLog.Append(childTurnID, "user", map[string]any{"content": goal})

			e.stream.Publish(ctx, sessionID, childTurnID, domain.EventTurnStarted, domain.TurnStartedPayload{
				TurnID: childTurnID, AgentID: workerAgent.ID, Goal: goal,
			})
			e.stream.Publish(ctx, sessionID, childTurnID, domain.EventUserMessage, domain.UserMessagePayload{Content: goal})

			rep, _, err := childRunner.Run(ctx, childCtx)
			finalStatus := turnStatus(err, rep)
			e.turnLog.EndTurn(childTurnID, finalStatus)
			// Parent cancel leaves ctx cancelled; persist/publish with Background
			// so the child turn does not stay stuck as "running" in the UI.
			bg := context.Background()
			prevStatus := domain.TurnStatus("")
			if cur, getErr := e.turns.Get(bg, childTurnID); getErr == nil {
				prevStatus = cur.Status
			}
			_ = e.turns.UpdateStatus(bg, childTurnID, finalStatus)
			status := string(finalStatus)
			// CancelTurn already published turn.failed with status=cancelled for
			// eager UI updates — do not emit a second terminal event.
			if err != nil && prevStatus != domain.TurnCancelled {
				summary := err.Error()
				if finalStatus == domain.TurnCancelled {
					summary = "cancelled"
				}
				e.publishTurnFailed(bg, sessionID, childTurnID, finalStatus, summary)
			}
			e.stream.Publish(bg, sessionID, parentTurnID, domain.EventDelegateCompleted, domain.DelegateCompletedPayload{
				AgentID: workerAgent.ID, Status: status, Summary: rep.Summary,
				ChildTurnID: childTurnID, CallID: callID,
			})
			return rep, err
		},
	}
	handlers := []tool.Handler{
		&builtin.SearchKB{Knowledge: e.knowledge, KBIDs: agent.KnowledgeIDs},
		&builtin.ListKBDocs{Knowledge: e.knowledge, KBIDs: agent.KnowledgeIDs},
		&builtin.GetKBDoc{Knowledge: e.knowledge},
		&builtin.AskUser{
			Stream: e.stream,
			OnAsk:  e.waitAskUser,
		},
		&builtin.MemoryUpdate{Store: e.memories},
		&builtin.MemoryRead{Store: e.memories, TopK: cfg.memoryReadTopK},
		delegator,
	}
	handlers = append(handlers, e.coreTableTools()...)
	reg := tool.NewRegistry(handlers...)
	e.mountAlwaysOnBuiltins(reg)
	e.mountBuiltinTools(reg, agent.Tools)
	e.mountMCPForAgent(reg, agent)
	if planMode {
		reg.Filter(domain.PlanModeAllowedToolIDs)
	}
	return reg
}

func (e *Engine) buildWorkerRegistry(agent domain.Agent, planMode bool) *tool.Registry {
	cfg := e.loadRunCfg(context.Background())
	handlers := []tool.Handler{
		&builtin.SearchKB{Knowledge: e.knowledge, KBIDs: agent.KnowledgeIDs},
		&builtin.ListKBDocs{Knowledge: e.knowledge, KBIDs: agent.KnowledgeIDs},
		&builtin.GetKBDoc{Knowledge: e.knowledge},
		&builtin.AskUser{
			Stream: e.stream,
			OnAsk:  e.waitAskUser,
		},
		&builtin.MemoryUpdate{Store: e.memories},
		&builtin.MemoryRead{Store: e.memories, TopK: cfg.memoryReadTopK},
	}
	handlers = append(handlers, e.coreTableTools()...)
	reg := tool.NewRegistry(handlers...)
	e.mountAlwaysOnBuiltins(reg)
	e.mountBuiltinTools(reg, agent.Tools)
	e.mountMCPForAgent(reg, agent)
	if planMode {
		reg.Filter(domain.PlanModeAllowedToolIDs)
	}
	return reg
}

// mountMCPForAgent applies MCP policy:
// - Primary agent: all enabled MCP servers
// - Subagent: only agent.MCPServers (exact server ids)
func (e *Engine) mountMCPForAgent(reg *tool.Registry, agent domain.Agent) {
	reg.CopyMCPServersFrom(e.toolCatalog)
	if agent.Mode != domain.AgentModeSubagent {
		reg.MountAllMCP()
		return
	}
	reg.MountServers(agent.MCPServers)
}

func (e *Engine) waitAskUser(ctx context.Context, sessionID, turnID, callID, question string, options []string, defaultOpt string, formFields []domain.AskUserFormField) (string, error) {
	if evalModeEnabled() {
		return "", fmt.Errorf("ask_user is disabled in eval mode")
	}
	ch := make(chan string, 1)
	e.mu.Lock()
	e.askUserWait[callID] = ch
	e.mu.Unlock()
	defer func() {
		e.mu.Lock()
		if e.askUserWait[callID] == ch {
			delete(e.askUserWait, callID)
		}
		e.mu.Unlock()
	}()
	e.stream.Publish(ctx, sessionID, turnID, domain.EventAskUserPending, domain.AskUserPayload{
		AskID: callID, CallID: callID, Question: question, Options: options, DefaultOpt: defaultOpt, FormFields: formFields,
	})
	select {
	case answer := <-ch:
		return answer, nil
	case <-ctx.Done():
		return "", ctx.Err()
	}
}

func (e *Engine) StreamEvents(sessionID string, since int64) []domain.StreamEvent {
	return e.stream.ListSince(sessionID, since)
}
func (e *Engine) Subscribe(sessionID string) chan domain.StreamEvent {
	return e.stream.Subscribe(sessionID)
}
func (e *Engine) Unsubscribe(sessionID string, ch chan domain.StreamEvent) {
	e.stream.Unsubscribe(sessionID, ch)
}
func (e *Engine) ResolveApproval(id string, approved bool, scope string) {
	if scope == "" {
		scope = "once"
	}
	e.mu.Lock()
	ch := e.approvalWait[id]
	meta := e.approvalMeta[id]
	delete(e.approvalWait, id)
	delete(e.approvalMeta, id)
	if approved && scope == "session" && meta.SessionID != "" {
		st := e.sessionPerm[meta.SessionID]
		switch meta.Reason {
		case permission.ReasonNetwork:
			// Full egress escape — deny mode only.
			st.AllowNetwork = true
		case permission.ReasonNetworkDomain:
			if meta.Domain != "" {
				st.AllowedDomains = append(st.AllowedDomains, meta.Domain)
				// Hard grant is applied in gateToolCall via GrantSessionDomains
				// after WaitApproval returns with scope=session.
			}
		}
		e.sessionPerm[meta.SessionID] = st
	}
	e.mu.Unlock()
	if ch != nil {
		select {
		case ch <- ApprovalOutcome{Approved: approved, Scope: scope, Reason: meta.Reason}:
		default:
		}
	}
}

// PublishPermissionDecided records a durable stream event so reloads hide approval actions.
func (e *Engine) PublishPermissionDecided(sessionID, turnID, approvalID string, approved bool, scope string) {
	if sessionID == "" || approvalID == "" {
		return
	}
	if scope == "" {
		scope = "once"
	}
	e.stream.Publish(context.Background(), sessionID, turnID, domain.EventPermissionDecided, domain.PermissionDecidedPayload{
		ApprovalID: approvalID, Approved: approved, Scope: scope,
	})
}
func (e *Engine) WaitApproval(ctx context.Context, id string) (ApprovalOutcome, error) {
	e.mu.Lock()
	ch := e.approvalWait[id]
	meta := e.approvalMeta[id]
	e.mu.Unlock()
	if e.isAutoApprove() && permission.AutoApprovable(meta.Reason) {
		e.settleAbandonedApproval(id, "approved", true)
		return ApprovalOutcome{Approved: true, Scope: "once", Reason: meta.Reason}, nil
	}
	if ch == nil {
		return ApprovalOutcome{}, fmt.Errorf("approval not found")
	}
	select {
	case out := <-ch:
		return out, nil
	case <-ctx.Done():
		// Turn cancelled while waiting: without cleanup the waiter maps leak
		// and the DB row stays "pending" until the next process restart.
		e.settleAbandonedApproval(id, "expired", false)
		return ApprovalOutcome{}, ctx.Err()
	}
}

// settleAbandonedApproval finalizes an approval whose wait ended without a
// user decision (turn cancelled, or auto-approve resolved it): clears the
// in-memory waiter maps and, when the DB row is still pending, persists the
// terminal status and publishes permission.decided so reloads hide the buttons.
// Safe to race with ResolveApproval/DecideApproval — both sides are idempotent.
func (e *Engine) settleAbandonedApproval(id, status string, approved bool) {
	e.mu.Lock()
	delete(e.approvalWait, id)
	delete(e.approvalMeta, id)
	e.mu.Unlock()
	if e.approvals == nil {
		return
	}
	bg := context.Background()
	a, err := e.approvals.Get(bg, id)
	if err != nil || (a.Status != "" && a.Status != "pending") {
		return
	}
	a.Status = status
	if err := e.approvals.Update(bg, a); err != nil {
		log.Printf("[approval] settle abandoned %s as %s: %v", id, status, err)
		return
	}
	e.PublishPermissionDecided(a.SessionID, a.TurnID, id, approved, "once")
}
func (e *Engine) CreateApproval(sessionID, turnID, toolName, description, reason, hostDomain string) string {
	id := newRuntimeID("appr")
	ch := make(chan ApprovalOutcome, 1)
	e.mu.Lock()
	e.approvalWait[id] = ch
	e.approvalMeta[id] = approvalMeta{SessionID: sessionID, Reason: reason, Domain: hostDomain}
	e.mu.Unlock()
	_ = e.approvals.Create(context.Background(), domain.Approval{
		ID: id, SessionID: sessionID, TurnID: turnID, ToolName: toolName,
		Summary: description, Description: description, Status: "pending", CreatedAt: time.Now().UTC(),
	})
	return id
}

func (e *Engine) ResolveAskUser(askID, answer string) error {
	e.mu.Lock()
	ch := e.askUserWait[askID]
	delete(e.askUserWait, askID)
	e.mu.Unlock()
	if ch == nil {
		return fmt.Errorf("ask_user not found or already resolved: %s", askID)
	}
	select {
	case ch <- answer:
		return nil
	default:
		return fmt.Errorf("ask_user no longer waiting: %s", askID)
	}
}

func (e *Engine) buildTurnMessages(sessionID string, agent domain.Agent, goal string, checkpointText string, planMode bool) []Message {
	cfg := e.loadRunCfg(context.Background())
	var skills []domain.Skill
	if e.skills != nil {
		skills = e.resolveAgentSkills(agent, e.dataDir)
	}
	sys := buildSystemPrompt(agent.SystemPrompt, skills, e.delegatableAgents(agent), agent.CanDelegate, planMode, checkpointText, "", "", e.sandboxStatus(), e.environmentStatus())
	messages := []Message{
		{Role: RoleSystem, Content: sys},
	}

	if hits := e.knowledge.Search(agent.KnowledgeIDs, goal, cfg.knowledgeSearchTopK); len(hits) > 0 {
		content := ""
		for _, h := range hits {
			content += h + "\n"
		}
		messages = append(messages, Message{Role: RoleSystem, Content: content})
	}

	e.mu.Lock()
	prevMsgs := e.turnMessages[sessionID]
	e.mu.Unlock()
	messages = append(messages, prevMsgs...)

	messages = append(messages, Message{Role: RoleUser, Content: goal})
	return messages
}

func turnStatus(err error, rep domain.Report) domain.TurnStatus {
	if err != nil {
		if errors.Is(err, context.Canceled) {
			return domain.TurnCancelled
		}
		return domain.TurnFailed
	}
	switch rep.Status {
	case domain.ReportDone:
		return domain.TurnCompleted
	case domain.ReportFailed, domain.ReportBlocked:
		return domain.TurnFailed
	default:
		return domain.TurnCompleted
	}
}

// publishTurnFailed emits turn.failed with TurnEndedPayload so clients can
// read the durable turn status without inferring from ErrorPayload kind/message.
func (e *Engine) publishTurnFailed(ctx context.Context, sessionID, turnID string, status domain.TurnStatus, summary string) {
	if status == "" {
		status = domain.TurnFailed
	}
	if summary == "" {
		summary = string(status)
	}
	e.stream.Publish(ctx, sessionID, turnID, domain.EventTurnFailed, domain.TurnEndedPayload{
		TurnID: turnID, Status: string(status), Summary: summary,
	})
}
