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
	dataDir       string
	turnMessages  map[string][]Message
	mu            sync.Mutex
	approvalWait  map[string]chan ApprovalOutcome
	approvalMeta  map[string]approvalMeta
	sessionPerm   map[string]sessionPermState
	askUserWait   map[string]chan string
	cancel        map[string]context.CancelFunc
	activeTurns   map[string]string // session ID -> in-flight turn ID
	agentChain    []string
	readSkill     *builtin.ReadSkill
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

// SetMCPCaller wires the MCP tool executor used by catalog handlers.
func (e *Engine) SetMCPCaller(c port.MCPCaller) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.mcpCaller = c
}

// ReplaceMCPServer implements port.MCPToolSync.
func (e *Engine) ReplaceMCPServer(serverID string, tools []domain.MCPToolBinding) {
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
	e.toolCatalog.ReplaceServer(serverID, handlers...)
}

// RemoveMCPServer implements port.MCPToolSync.
func (e *Engine) RemoveMCPServer(serverID string) {
	e.toolCatalog.RemoveServer(serverID)
}

func (e *Engine) StartSession(ctx context.Context, s domain.Session, attachments []domain.UserAttachment) {
	turnID := fmt.Sprintf("turn-%d", time.Now().UnixNano())
	atts := append([]domain.UserAttachment(nil), attachments...)
	if err := e.reserveSessionTurn(s.ID, turnID); err != nil {
		e.stream.Publish(ctx, s.ID, turnID, domain.EventTurnFailed, domain.ErrorPayload{
			Message: err.Error(), Kind: "conflict",
		})
		return
	}
	go func() {
		defer e.releaseSessionTurn(s.ID, turnID)
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
			e.stream.Publish(ctx, s.ID, "", domain.EventTurnFailed, map[string]string{"error": err.Error()})
			return
		}
		agentPtr := *agent

		reg := e.setupRegistry(s, agentPtr)
		rep, err := e.runTurn(turnCtx, s.ID, turnID, s.Content, s.ModelID, s.ProjectID, agentPtr, reg, atts)
		e.turnLog.EndTurn(turnID, turnStatus(err, rep))
	}()
}

func (e *Engine) StartTurn(ctx context.Context, sessionID, userInput, agentID, modelID string, attachments []domain.UserAttachment) (string, error) {
	turnID := fmt.Sprintf("turn-%d", time.Now().UnixNano())
	if err := e.reserveSessionTurn(sessionID, turnID); err != nil {
		return "", err
	}
	atts := append([]domain.UserAttachment(nil), attachments...)
	// Reset session status to active so UI shows "运行中"
	e.updateSessionStatus(sessionID, domain.SessionStatusActive)
	go func() {
		defer e.releaseSessionTurn(sessionID, turnID)
		s, err := e.sessions.Get(ctx, sessionID)
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
		agent, err := e.agents.Get(ctx, targetAgentID)
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

		reg := e.setupRegistry(s, agentPtr)
		rep, err := e.runTurn(turnCtx, sessionID, turnID, userInput, targetModelID, s.ProjectID, agentPtr, reg, atts)
		e.turnLog.EndTurn(turnID, turnStatus(err, rep))
	}()
	return turnID, nil
}

func (e *Engine) CancelTurn(ctx context.Context, turnID string) {
	e.mu.Lock()
	cancel, ok := e.cancel[turnID]
	e.mu.Unlock()
	if ok {
		cancel()
		return
	}

	// Child/delegate turns share the parent context and are not registered in
	// e.cancel. Cancelling by child ID (or healing a zombie "running" child after
	// the parent already ended) must still clear DB status so the UI leaves
	// the Composer "running" state.
	t, err := e.turns.Get(context.Background(), turnID)
	if err != nil || t.Status != domain.TurnRunning {
		return
	}
	turns, _ := e.turns.ListBySession(context.Background(), t.SessionID)
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
	}
	_ = e.turns.UpdateStatus(context.Background(), turnID, domain.TurnCancelled)
	e.stream.Publish(context.Background(), t.SessionID, turnID, domain.EventTurnFailed, domain.ErrorPayload{
		Message: "cancelled", Kind: "cancelled",
	})
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
		defer e.releaseSessionTurn(sessionID, turnID)
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

		reg := e.setupRegistry(s, agentPtr)
		e.stream.Publish(turnCtx, sessionID, turnID, domain.EventTurnStarted, domain.TurnStartedPayload{
			TurnID: turnID, AgentID: agentPtr.ID, Goal: goal,
		})
		e.stream.Publish(turnCtx, sessionID, turnID, domain.EventUserMessage, domain.UserMessagePayload{Content: goal})
		// Create reopens existing JSONL for append (no duplicate start) when present.
		_ = e.turnLog.Create(turnID, sessionID, s.ProjectID, agentPtr.ID, goal)
		_ = e.turns.Create(turnCtx, domain.TurnLog{ID: turnID, SessionID: sessionID, AgentID: agentPtr.ID, Goal: goal, Status: domain.TurnRunning})
		_ = e.turns.UpdateStatus(turnCtx, turnID, domain.TurnRunning)
		e.turnRunner.Log = func(typ string, data map[string]any) {
			e.turnLog.Append(turnID, typ, data)
		}

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

		sys := buildSystemPrompt(agentPtr.SystemPrompt, e.turnRunner.SkillList, e.delegatableAgents(agentPtr), agentPtr.CanDelegate, checkpointText, activeTodos, fileChanges, e.sandboxStatus())
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
		if !historyHasUserGoal(history, goal) {
			e.turnLog.Append(turnID, "user", map[string]any{"content": goal})
			messages = append(messages, Message{Role: RoleUser, Content: goal})
		}

		workDir := e.resolveWorkDir(turnCtx, s.ProjectID)
		e.turnRunner.Registry = reg
		rep, _, err := e.turnRunner.Run(turnCtx, TurnContext{
			SessionID: sessionID, TurnID: turnID, Agent: agentPtr,
			Model: s.ModelID, MaxSteps: agentPtr.Steps, WorkDir: workDir, ProjectID: s.ProjectID, Messages: messages,
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
	resumedSessions := make(map[string]bool)
	if len(runningTurns) > 0 {
		log.Printf("[RecoverRunning] found %d zombie running turn(s)", len(runningTurns))
	}
	for _, t := range runningTurns {
		e.expireOrphanAskUsers(t.SessionID, t.ID)

		// Nested tool-run turns (e.g. mid-flight delegate_agent) are not
		// parent session turns — fail them like any unfinished tool.
		if e.turnLog.IsNestedToolRun(t.ID) {
			log.Printf("[RecoverRunning] turn %s is nested tool run, marking as failed", t.ID)
			if err := e.turns.UpdateStatus(ctx, t.ID, domain.TurnFailed); err != nil {
				log.Printf("[RecoverRunning] update turn %s status: %v", t.ID, err)
			}
			_ = e.turnLog.CreateNested(t.ID, t.SessionID, "", t.AgentID, t.Goal)
			e.turnLog.EndTurn(t.ID, domain.TurnFailed)
			continue
		}

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

// expireOrphanAskUsers publishes tool.error for ask_user calls left pending across restart.
func (e *Engine) expireOrphanAskUsers(sessionID, turnID string) {
	if sessionID == "" || turnID == "" || e.stream == nil {
		return
	}
	pending := make(map[string]bool)
	for _, ev := range e.stream.ListSince(sessionID, 0) {
		if ev.TurnID != turnID {
			continue
		}
		switch ev.Type {
		case domain.EventAskUserPending:
			var p domain.AskUserPayload
			if json.Unmarshal(ev.Payload, &p) != nil {
				continue
			}
			callID := p.CallID
			if callID == "" {
				callID = p.AskID
			}
			if callID != "" {
				pending[callID] = true
			}
		case domain.EventToolCompleted, domain.EventToolError:
			var p domain.ToolPart
			if json.Unmarshal(ev.Payload, &p) != nil {
				continue
			}
			if p.CallID != "" {
				delete(pending, p.CallID)
			}
		}
	}
	for callID := range pending {
		e.stream.Publish(context.Background(), sessionID, turnID, domain.EventToolError, domain.ToolPart{
			CallID: callID, Name: "ask_user", Status: domain.ToolError,
			Error: "expired (process restarted)",
		})
	}
}

func (e *Engine) ListTurns(sessionID string) []domain.TurnLog {
	turns, err := e.turns.ListBySession(context.Background(), sessionID)
	if err != nil {
		return nil
	}
	return turns
}

func (e *Engine) setupRegistry(s domain.Session, agent domain.Agent) *tool.Registry {
	workDir := e.resolveWorkDir(context.Background(), s.ProjectID)
	skills := e.resolveAgentSkills(agent, workDir)
	e.turnRunner.SkillList = skills
	e.turnRunner.ToolBindings = agent.Tools

	var reg *tool.Registry
	if agentHasDelegation(agent) {
		reg = e.buildTeamRegistry(agent)
	} else {
		reg = e.buildWorkerRegistry(agent)
	}
	return reg
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

// resolveAgentSkills returns Agent-bound DB skills, optionally merged with
// Ambient filesystem skills when agent.InheritsAmbient() (default: primary yes,
// subagent no). Filesystem skills are not written to the database.
func (e *Engine) resolveAgentSkills(agent domain.Agent, workDir string) []domain.Skill {
	bound := e.boundDBSkills(agent)
	if !agent.InheritsAmbient() {
		// Do not clear the shared ReadSkill FS overlay. Parent turns (and
		// parallel sibling tool calls) may still need it; subagent registries
		// mount an isolated ReadSkill without that overlay.
		return bound
	}
	fsSkills, fsFiles := service.ScanFilesystemSkills(workDir)
	e.setTurnFSSkills(fsSkills, fsFiles)
	return service.MergeSkillsByID(bound, fsSkills)
}

func (e *Engine) boundDBSkills(agent domain.Agent) []domain.Skill {
	all, _ := e.skills.List(context.Background())
	return service.BoundDBSkills(all, agent)
}

func (e *Engine) setTurnFSSkills(skills []domain.Skill, files map[string][]domain.SkillFile) {
	byID := make(map[string]domain.Skill, len(skills))
	for _, sk := range skills {
		byID[sk.ID] = sk
	}
	if e.readSkill != nil {
		e.readSkill.SetTurnFS(byID, files)
	}
}

func (e *Engine) runTurn(ctx context.Context, sessionID, turnID, goal, modelID, projectID string, agent domain.Agent, reg *tool.Registry, attachments []domain.UserAttachment) (domain.Report, error) {
	cfg := e.loadRunCfg(ctx)

	e.turnRunner.Registry = reg
	e.stream.Publish(ctx, sessionID, turnID, domain.EventTurnStarted, domain.TurnStartedPayload{
		TurnID: turnID, AgentID: agent.ID, Goal: goal,
	})
	e.stream.Publish(ctx, sessionID, turnID, domain.EventUserMessage, userMessagePayload(goal, attachments))

	e.turnLog.Create(turnID, sessionID, projectID, agent.ID, goal)
	_ = e.turns.Create(ctx, domain.TurnLog{ID: turnID, SessionID: sessionID, AgentID: agent.ID, Goal: goal, Status: domain.TurnRunning})
	e.turnRunner.Log = func(typ string, data map[string]any) {
		e.turnLog.Append(turnID, typ, data)
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

	sys := buildSystemPrompt(agent.SystemPrompt, e.turnRunner.SkillList, e.delegatableAgents(agent), agent.CanDelegate, checkpointText, activeTodos, fileChanges, e.sandboxStatus())
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

	rep, turnMsgs, err := e.turnRunner.Run(ctx, TurnContext{
		SessionID: sessionID,
		TurnID:    turnID,
		Agent:     agent,
		Model:     modelID,
		MaxSteps:  agent.Steps,
		WorkDir:   workDir,
		ProjectID: projectID,
		Messages:  messages,
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
	if err != nil {
		kind := "turn"
		msg := err.Error()
		sessionStatus := domain.SessionStatusFailed
		if errors.Is(err, context.Canceled) {
			kind = "cancelled"
			msg = "cancelled"
			// Intentional interrupt is not a hard failure for the session.
			sessionStatus = domain.SessionStatusCompleted
		}
		e.stream.Publish(context.Background(), sessionID, turnID, domain.EventTurnFailed, domain.ErrorPayload{
			Message: msg, Kind: kind,
		})
		e.updateSessionStatus(sessionID, sessionStatus)
		_ = e.turns.UpdateStatus(context.Background(), turnID, turnStatus(err, rep))
		return
	}
	e.updateSessionStatus(sessionID, domain.SessionStatusCompleted)
	_ = e.turns.UpdateStatus(context.Background(), turnID, domain.TurnCompleted)
	e.stream.Publish(context.Background(), sessionID, turnID, domain.EventSessionCompleted, domain.SessionCompletedPayload{
		Summary: rep.Summary, Status: string(rep.Status),
	})
	turns, _ := e.turns.ListBySession(context.Background(), sessionID)
	e.maybeCompact(context.Background(), sessionID, turnID, len(turns), model, rep.MaxPromptTokens)
}

func (e *Engine) updateSessionStatus(sessionID string, status domain.SessionStatus) {
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

// loadRetainedHistory loads compaction-bounded session messages and shrinks
// oversized tool results so the retain window stays near cutTokens.
func (e *Engine) loadRetainedHistory(sessionID string, cp *domain.CompactionCheckpoint) []Message {
	retainFrom := ""
	skip := 0
	if cp != nil {
		retainFrom = cp.RetainFromTurnID
		skip = cp.RetainSkipMessages
	}
	msgs := chatMessagesToRuntime(e.turnLog.LoadSessionMessages(sessionID, retainFrom, skip))
	cfg := e.compactionMgr.loadCfg(context.Background())
	if cfg.cutTokens > 0 {
		msgs = truncateToolResultsToBudget(msgs, cfg.cutTokens)
	}
	return msgs
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
		if b.ToolID == "" {
			continue
		}
		if h, ok := e.toolCatalog.Get(b.ToolID); ok {
			reg.Register(h)
		}
	}
}

func (e *Engine) buildTeamRegistry(agent domain.Agent) *tool.Registry {
	cfg := e.loadRunCfg(context.Background())
	delegator := &builtin.DelegateAgent{
		Stream: e.stream, Agents: e.agents,
		KnowledgeSearch: e.knowledge.Search,
		RunSubTurn: func(ctx context.Context, sessionID, modelID, parentTurnID string, workerAgent domain.Agent, goal string) (domain.Report, error) {
			childTurnID := fmt.Sprintf("turn-%d", time.Now().UnixNano())
			workDir := e.dataDir
			projectID := ""
			if s, err := e.sessions.Get(ctx, sessionID); err == nil {
				workDir = e.resolveWorkDir(ctx, s.ProjectID)
				projectID = s.ProjectID
			}
			childCtx := TurnContext{
				SessionID: sessionID, TurnID: childTurnID,
				Agent: workerAgent, Model: modelID, MaxSteps: workerAgent.Steps,
				WorkDir: workDir, ProjectID: projectID,
			}
			e.stream.Publish(ctx, sessionID, parentTurnID, domain.EventDelegateStarted, domain.DelegateStartedPayload{
				AgentID: workerAgent.ID, Goal: goal, ChildTurnID: childTurnID,
			})

			e.mu.Lock()
			for _, id := range e.agentChain {
				if id == workerAgent.ID {
					e.mu.Unlock()
					return domain.Report{}, fmt.Errorf("circular delegation: %s", workerAgent.ID)
				}
			}
			if len(e.agentChain) >= cfg.teamMaxDelegationDepth {
				e.mu.Unlock()
				return domain.Report{}, fmt.Errorf("max delegation depth reached")
			}
			e.agentChain = append(e.agentChain, workerAgent.ID)
			e.mu.Unlock()
			defer func() {
				e.mu.Lock()
				for i := len(e.agentChain) - 1; i >= 0; i-- {
					if e.agentChain[i] == workerAgent.ID {
						e.agentChain = append(e.agentChain[:i], e.agentChain[i+1:]...)
						break
					}
				}
				e.mu.Unlock()
			}()

			// Isolated child runner so parallel delegate_agent calls do not
			// mutate the parent's Registry / SkillList / Log mid-flight.
			skills := e.resolveAgentSkills(workerAgent, workDir)
			var childReg *tool.Registry
			if agentHasDelegation(workerAgent) {
				childReg = e.buildTeamRegistry(workerAgent)
			} else {
				childReg = e.buildWorkerRegistry(workerAgent)
			}
			childReg.Register(&builtin.ReadSkill{Skills: e.skills})

			childRunner := NewTurnRunner(e.llm, e.stream, e.turnRunner.Perm, childReg, e.configStore)
			childRunner.Approval = e
			childRunner.SandboxStatus = e.sandboxStatus
			childRunner.EffectiveIsolation = e.effectiveIsolation
			childRunner.SessionAllowNetwork = e.sessionAllowsNetwork
			childRunner.SessionAllowDomains = e.sessionAllowDomains
			childRunner.GrantSessionDomains = e.grantSessionDomains
			childRunner.GrantTurnDomains = e.grantTurnDomains
			childRunner.ClearTurnDomains = e.clearTurnDomains
			childRunner.FileChanges = e.turnRunner.FileChanges
			childRunner.SkillList = skills
			childRunner.ToolBindings = workerAgent.Tools
			childRunner.Log = func(typ string, data map[string]any) {
				e.turnLog.Append(childTurnID, typ, data)
			}

			sys := buildSystemPrompt(workerAgent.SystemPrompt, skills, nil, workerAgent.CanDelegate, "", "", "", e.sandboxStatus())
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
			_ = e.turns.UpdateStatus(bg, childTurnID, finalStatus)
			status := string(finalStatus)
			if err != nil {
				kind := "turn"
				msg := err.Error()
				if errors.Is(err, context.Canceled) {
					kind = "cancelled"
					msg = "cancelled"
				}
				e.stream.Publish(bg, sessionID, childTurnID, domain.EventTurnFailed, domain.ErrorPayload{
					Message: msg, Kind: kind,
				})
			}
			e.stream.Publish(bg, sessionID, parentTurnID, domain.EventDelegateCompleted, domain.DelegateCompletedPayload{
				AgentID: workerAgent.ID, Status: status, Summary: rep.Summary,
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
	return reg
}

func (e *Engine) buildWorkerRegistry(agent domain.Agent) *tool.Registry {
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
	return reg
}

// mountMCPForAgent applies the Ambient / Bound MCP policy:
// - InheritAmbient (default primary): all enabled MCP servers
// - otherwise: only agent.MCPServers (exact server ids)
func (e *Engine) mountMCPForAgent(reg *tool.Registry, agent domain.Agent) {
	reg.CopyMCPServersFrom(e.toolCatalog)
	if agent.InheritsAmbient() {
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
		return ApprovalOutcome{Approved: true, Scope: "once", Reason: meta.Reason}, nil
	}
	if ch == nil {
		return ApprovalOutcome{}, fmt.Errorf("approval not found")
	}
	select {
	case out := <-ch:
		return out, nil
	case <-ctx.Done():
		return ApprovalOutcome{}, ctx.Err()
	}
}
func (e *Engine) CreateApproval(sessionID, turnID, toolName, description, reason, hostDomain string) string {
	id := fmt.Sprintf("appr-%d", time.Now().UnixNano())
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

func (e *Engine) buildTurnMessages(sessionID string, agent domain.Agent, goal string, checkpointText string) []Message {
	cfg := e.loadRunCfg(context.Background())
	sys := buildSystemPrompt(agent.SystemPrompt, e.turnRunner.SkillList, e.delegatableAgents(agent), agent.CanDelegate, checkpointText, "", "", e.sandboxStatus())
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
