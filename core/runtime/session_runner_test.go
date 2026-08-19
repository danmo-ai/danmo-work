package runtime

import (
	"context"
	"encoding/json"
	"errors"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"danmo-work/core/domain"
	"danmo-work/core/port"
	"danmo-work/core/runtime/tool"
	"danmo-work/core/runtime/tool/builtin"
	"danmo-work/core/service"
	sqlitestore "danmo-work/core/store/sqlite"
	"danmo-work/core/store/turnlog"
)

func TestMountBuiltinToolsSkipsCoreAskUser(t *testing.T) {
	wired := &builtin.AskUser{
		OnAsk: func(ctx context.Context, sessionID, turnID, callID, question string, options []string, defaultOpt string, formFields []domain.AskUserFormField) (string, error) {
			return "ok", nil
		},
	}
	catalog := tool.NewRegistry(&builtin.AskUser{}, &builtin.ReadFile{})
	engine := &Engine{toolCatalog: catalog}
	reg := tool.NewRegistry(wired)
	engine.mountBuiltinTools(reg, []domain.ToolBinding{
		{ToolID: "ask_user", RiskLevel: domain.RiskLow},
		{ToolID: "read_file", RiskLevel: domain.RiskLow},
	})

	h, ok := reg.Get("ask_user")
	if !ok {
		t.Fatal("ask_user missing")
	}
	got, ok := h.(*builtin.AskUser)
	if !ok || got.OnAsk == nil {
		t.Fatal("catalog ask_user stub must not overwrite wired OnAsk")
	}
	if _, ok := reg.Get("read_file"); !ok {
		t.Fatal("bound read_file should still mount")
	}
}

type memStreamRepo struct {
	mu     sync.Mutex
	events []domain.StreamEvent
}

func (r *memStreamRepo) Save(_ context.Context, ev domain.StreamEvent) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.events = append(r.events, ev)
	return nil
}

func (r *memStreamRepo) ListBySession(_ context.Context, sessionID string, since int64) ([]domain.StreamEvent, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	var out []domain.StreamEvent
	for _, ev := range r.events {
		if ev.SessionID == sessionID && ev.Seq > since {
			out = append(out, ev)
		}
	}
	return out, nil
}

func (r *memStreamRepo) DeleteBySession(_ context.Context, sessionID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	kept := r.events[:0]
	for _, ev := range r.events {
		if ev.SessionID != sessionID {
			kept = append(kept, ev)
		}
	}
	r.events = kept
	return nil
}

func (r *memStreamRepo) MaxSeq() int64 {
	r.mu.Lock()
	defer r.mu.Unlock()
	if len(r.events) == 0 {
		return 0
	}
	return r.events[len(r.events)-1].Seq
}

type memApprovalRepo struct {
	mu   sync.Mutex
	byID map[string]domain.Approval
}

func newMemApprovalRepo() *memApprovalRepo {
	return &memApprovalRepo{byID: make(map[string]domain.Approval)}
}

func (r *memApprovalRepo) Create(_ context.Context, a domain.Approval) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.byID[a.ID] = a
	return nil
}

func (r *memApprovalRepo) Get(_ context.Context, id string) (domain.Approval, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	a, ok := r.byID[id]
	if !ok {
		return domain.Approval{}, errors.New("approval not found")
	}
	return a, nil
}

func (r *memApprovalRepo) Update(_ context.Context, a domain.Approval) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.byID[a.ID] = a
	return nil
}

func (r *memApprovalRepo) ListByStatus(_ context.Context, status string) ([]domain.Approval, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	var out []domain.Approval
	for _, a := range r.byID {
		if a.Status == status {
			out = append(out, a)
		}
	}
	return out, nil
}

// Regression: a turn cancelled while waiting for approval must clean up the
// in-memory waiter maps and expire the DB row instead of leaking both until
// the next process restart.
func TestWaitApprovalCancelSettlesAbandonedApproval(t *testing.T) {
	repo := newMemApprovalRepo()
	e := &Engine{
		approvals:    service.NewApprovalManager(repo),
		stream:       NewStreamEventManager(&memStreamRepo{}),
		approvalWait: make(map[string]chan ApprovalOutcome),
		approvalMeta: make(map[string]approvalMeta),
	}

	id, err := e.CreateApproval("sess-appr", "turn-appr", "exec_shell", "rm -rf x", "high_risk", "")
	if err != nil {
		t.Fatalf("create approval: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := e.WaitApproval(ctx, id); !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context.Canceled, got %v", err)
	}

	e.mu.Lock()
	_, waitLeak := e.approvalWait[id]
	_, metaLeak := e.approvalMeta[id]
	e.mu.Unlock()
	if waitLeak || metaLeak {
		t.Fatal("approvalWait/approvalMeta must be cleaned up after cancelled wait")
	}

	a, err := repo.Get(context.Background(), id)
	if err != nil {
		t.Fatalf("get approval: %v", err)
	}
	if a.Status != "expired" {
		t.Fatalf("approval status: want expired, got %q", a.Status)
	}

	var decided bool
	for _, ev := range e.stream.ListSince("sess-appr", 0) {
		if ev.Type != domain.EventPermissionDecided {
			continue
		}
		var p domain.PermissionDecidedPayload
		if json.Unmarshal(ev.Payload, &p) == nil && p.ApprovalID == id && !p.Approved {
			decided = true
		}
	}
	if !decided {
		t.Fatal("expected permission.decided(approved=false) after abandoned wait")
	}

	// A later user click must be a no-op on the already-expired row.
	e.ResolveApproval(id, true, "once")
	a, _ = repo.Get(context.Background(), id)
	if a.Status != "expired" {
		t.Fatalf("late resolve must not resurrect approval, got %q", a.Status)
	}
}

type failCreateApprovalRepo struct {
	*memApprovalRepo
}

func (r *failCreateApprovalRepo) Create(context.Context, domain.Approval) error {
	return errors.New("db locked")
}

// Regression: a failed approval INSERT must surface as an error and clean up
// the waiter maps. Previously the error was swallowed — the UI showed the ask
// buttons but DecideApproval could never find the row, so the turn hung in
// WaitApproval until cancelled.
func TestCreateApprovalPersistFailureCleansWaiters(t *testing.T) {
	repo := &failCreateApprovalRepo{memApprovalRepo: newMemApprovalRepo()}
	e := &Engine{
		approvals:    service.NewApprovalManager(repo),
		stream:       NewStreamEventManager(&memStreamRepo{}),
		approvalWait: make(map[string]chan ApprovalOutcome),
		approvalMeta: make(map[string]approvalMeta),
	}

	id, err := e.CreateApproval("sess-x", "turn-x", "exec_shell", "rm -rf x", "high_risk", "")
	if err == nil {
		t.Fatal("expected error when approval row cannot be persisted")
	}
	if id != "" {
		t.Fatalf("expected empty id on persist failure, got %q", id)
	}

	e.mu.Lock()
	nWait, nMeta := len(e.approvalWait), len(e.approvalMeta)
	e.mu.Unlock()
	if nWait != 0 || nMeta != 0 {
		t.Fatalf("waiter maps must be cleaned up on persist failure: wait=%d meta=%d", nWait, nMeta)
	}
}

func TestNewRuntimeIDUniqueUnderConcurrency(t *testing.T) {
	const n = 2000
	ids := make(chan string, n)
	var wg sync.WaitGroup
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			ids <- newRuntimeID("turn")
		}()
	}
	wg.Wait()
	close(ids)
	seen := make(map[string]bool, n)
	for id := range ids {
		if seen[id] {
			t.Fatalf("duplicate id generated: %s", id)
		}
		seen[id] = true
		if !strings.HasPrefix(id, "turn-") {
			t.Fatalf("unexpected id format: %s", id)
		}
	}
}

func TestReserveSessionTurnPreventsConcurrentStartAndResume(t *testing.T) {
	engine := &Engine{}
	if err := engine.reserveSessionTurn("session-1", "turn-1"); err != nil {
		t.Fatalf("reserve first turn: %v", err)
	}
	if err := engine.reserveSessionTurn("session-1", "turn-2"); !errors.Is(err, port.ErrSessionTurnRunning) {
		t.Fatalf("second turn should conflict, got %v", err)
	}

	// A stale goroutine must not release another turn's reservation.
	engine.releaseSessionTurn("session-1", "turn-2")
	if err := engine.reserveSessionTurn("session-1", "turn-3"); !errors.Is(err, port.ErrSessionTurnRunning) {
		t.Fatalf("stale release cleared active turn: %v", err)
	}

	engine.releaseSessionTurn("session-1", "turn-1")
	if err := engine.reserveSessionTurn("session-1", "turn-3"); err != nil {
		t.Fatalf("reserve after release: %v", err)
	}
	if err := engine.reserveSessionTurn("session-2", "turn-4"); err != nil {
		t.Fatalf("different session should run independently: %v", err)
	}
}

// contains is a shared test helper (compaction_test.go, file_changes_test.go).
func contains(s, substr string) bool {
	return strings.Contains(s, substr)
}

func TestCheckDelegation_AllowsParallelSameAgent(t *testing.T) {
	// Sibling fan-out shares the lead turn path; same agent_id is fine.
	parent := []domain.TurnPathEntry{{TurnID: "turn-lead", AgentID: "team"}}
	if err := checkDelegation(parent, "researcher", 3); err != nil {
		t.Fatalf("first parallel delegate: %v", err)
	}
	if err := checkDelegation(parent, "researcher", 3); err != nil {
		t.Fatalf("second parallel delegate should not look circular: %v", err)
	}
}

func TestCheckDelegation_DetectsCycleOnPath(t *testing.T) {
	path := []domain.TurnPathEntry{
		{TurnID: "turn-lead", AgentID: "team"},
		{TurnID: "turn-child", AgentID: "researcher"},
	}
	err := checkDelegation(path, "researcher", 3)
	if err == nil || !strings.Contains(err.Error(), "circular delegation: researcher") {
		t.Fatalf("expected circular delegation error, got %v", err)
	}
}

func TestCheckDelegation_MaxDepth(t *testing.T) {
	// Lead depth 0 + 3 workers ⇒ path len 4; next child would be depth 4.
	path := []domain.TurnPathEntry{
		{TurnID: "t0", AgentID: "lead"},
		{TurnID: "t1", AgentID: "a"},
		{TurnID: "t2", AgentID: "b"},
		{TurnID: "t3", AgentID: "c"},
	}
	err := checkDelegation(path, "d", 3)
	if err == nil || !strings.Contains(err.Error(), "max delegation depth") {
		t.Fatalf("expected max depth error, got %v", err)
	}
	// At depth 2 (lead+a+b), next child depth 3 is still allowed.
	shallow := path[:3]
	if err := checkDelegation(shallow, "c", 3); err != nil {
		t.Fatalf("depth 3 of max 3 should be allowed: %v", err)
	}
}

func TestAppendTurnPath(t *testing.T) {
	parent := []domain.TurnPathEntry{{TurnID: "t0", AgentID: "team"}}
	child := appendTurnPath(parent, "t1", "researcher")
	if len(child) != 2 || child[1].TurnID != "t1" || child[1].AgentID != "researcher" {
		t.Fatalf("unexpected child path: %+v", child)
	}
	if len(parent) != 1 {
		t.Fatalf("append must not mutate parent, got %+v", parent)
	}
}

type memTurnRepo struct {
	mu    sync.Mutex
	turns map[string]domain.TurnLog
}

func (r *memTurnRepo) Create(_ context.Context, t domain.TurnLog) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.turns == nil {
		r.turns = map[string]domain.TurnLog{}
	}
	r.turns[t.ID] = t
	return nil
}

func (r *memTurnRepo) UpdateStatus(_ context.Context, id string, status domain.TurnStatus) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	t, ok := r.turns[id]
	if !ok {
		return errors.New("not found")
	}
	t.Status = status
	r.turns[id] = t
	return nil
}

func (r *memTurnRepo) Get(_ context.Context, id string) (domain.TurnLog, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	t, ok := r.turns[id]
	if !ok {
		return domain.TurnLog{}, errors.New("not found")
	}
	return t, nil
}

func (r *memTurnRepo) ListBySession(_ context.Context, sessionID string) ([]domain.TurnLog, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]domain.TurnLog, 0)
	for _, t := range r.turns {
		if t.SessionID == sessionID {
			out = append(out, t)
		}
	}
	return out, nil
}

func (r *memTurnRepo) ListByStatus(_ context.Context, status domain.TurnStatus) ([]domain.TurnLog, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]domain.TurnLog, 0)
	for _, t := range r.turns {
		if t.Status == status {
			out = append(out, t)
		}
	}
	return out, nil
}

type noopStream struct{}

func (noopStream) Publish(_ context.Context, sessionID, turnID, typ string, _ any) domain.StreamEvent {
	return domain.StreamEvent{SessionID: sessionID, TurnID: turnID, Type: typ}
}
func (noopStream) Subscribe(string) chan domain.StreamEvent     { return nil }
func (noopStream) Unsubscribe(string, chan domain.StreamEvent)  {}
func (noopStream) ListSince(string, int64) []domain.StreamEvent { return nil }

type memStream struct {
	mu     sync.Mutex
	seq    int64
	events []domain.StreamEvent
}

func (s *memStream) Publish(_ context.Context, sessionID, turnID, typ string, payload any) domain.StreamEvent {
	raw, _ := json.Marshal(payload)
	s.mu.Lock()
	defer s.mu.Unlock()
	s.seq++
	ev := domain.StreamEvent{
		Seq: s.seq, Type: typ, SessionID: sessionID, TurnID: turnID,
		Payload: raw,
	}
	s.events = append(s.events, ev)
	return ev
}
func (s *memStream) Subscribe(string) chan domain.StreamEvent    { return nil }
func (s *memStream) Unsubscribe(string, chan domain.StreamEvent) {}
func (s *memStream) ListSince(sessionID string, since int64) []domain.StreamEvent {
	s.mu.Lock()
	defer s.mu.Unlock()
	var out []domain.StreamEvent
	for _, ev := range s.events {
		if ev.SessionID == sessionID && ev.Seq > since {
			out = append(out, ev)
		}
	}
	return out
}

func TestCloseIncompleteToolPairsSettlesDelegateAndAskUser(t *testing.T) {
	st, err := sqlitestore.New(filepath.Join(t.TempDir(), "work.db"))
	if err != nil {
		t.Fatal(err)
	}
	tls := turnlog.NewTurnLogStore(st.TurnLogs())
	turnLogs := service.NewTurnLogManager(tls)
	stream := &memStream{}
	repo := &memTurnRepo{turns: map[string]domain.TurnLog{
		"turn-parent": {
			ID: "turn-parent", SessionID: "sess-1", AgentID: "team",
			Status: domain.TurnRunning, Goal: "research",
		},
		"turn-child": {
			ID: "turn-child", SessionID: "sess-1", AgentID: "researcher",
			Status: domain.TurnRunning, Goal: "gather",
		},
	}}
	engine := &Engine{
		turns:   service.NewTurnManager(repo),
		turnLog: turnLogs,
		stream:  stream,
	}

	if err := turnLogs.Create("turn-parent", "sess-1", "proj-a", "team", "research"); err != nil {
		t.Fatal(err)
	}
	turnLogs.Append("turn-parent", "assistant", map[string]any{
		"tool_calls": []any{
			map[string]any{
				"id": "dlg-1", "name": "delegate_agent",
				"arguments": map[string]any{"agent_id": "researcher", "goal": "gather"},
			},
			map[string]any{
				"id": "ask-1", "name": "ask_user",
				"arguments": map[string]any{"question": "continue?"},
			},
		},
	})
	if err := turnLogs.CreateNested("turn-child", "sess-1", "proj-a", "researcher", "gather"); err != nil {
		t.Fatal(err)
	}

	ctx := context.Background()
	stream.Publish(ctx, "sess-1", "turn-parent", domain.EventToolRunning, domain.ToolPart{
		CallID: "dlg-1", Name: "delegate_agent", Status: domain.ToolRunning,
		Input: map[string]any{"agent_id": "researcher", "goal": "gather"},
	})
	stream.Publish(ctx, "sess-1", "turn-parent", domain.EventDelegateStarted, domain.DelegateStartedPayload{
		AgentID: "researcher", Goal: "gather", ChildTurnID: "turn-child", CallID: "dlg-1",
	})
	stream.Publish(ctx, "sess-1", "turn-parent", domain.EventToolRunning, domain.ToolPart{
		CallID: "ask-1", Name: "ask_user", Status: domain.ToolRunning,
	})
	stream.Publish(ctx, "sess-1", "turn-parent", domain.EventAskUserPending, domain.AskUserPayload{
		AskID: "ask-1", CallID: "ask-1", Question: "continue?",
	})
	stream.Publish(ctx, "sess-1", "turn-child", domain.EventTurnStarted, domain.TurnStartedPayload{
		TurnID: "turn-child", AgentID: "researcher", Goal: "gather",
	})

	engine.closeIncompleteToolPairs("sess-1", "turn-parent")

	if left := turnLogs.ListIncompleteToolCalls("turn-parent"); len(left) != 0 {
		t.Fatalf("JSONL still incomplete: %+v", left)
	}
	_, entries := turnLogs.LoadForRecovery("turn-parent")
	gotResults := map[string]string{}
	for _, e := range entries {
		if e["type"] != "tool_result" {
			continue
		}
		data, _ := e["data"].(map[string]any)
		gotResults[data["call_id"].(string)] = data["output"].(string)
	}
	if gotResults["dlg-1"] != recoveryToolClosedReason || gotResults["ask-1"] != recoveryToolClosedReason {
		t.Fatalf("expected synthetic tool_results, got %+v", gotResults)
	}

	child, err := repo.Get(ctx, "turn-child")
	if err != nil {
		t.Fatal(err)
	}
	if child.Status != domain.TurnFailed {
		t.Fatalf("child status: want failed, got %s", child.Status)
	}

	var sawDlgErr, sawAskErr, sawDelegateDone, sawChildFailed bool
	for _, ev := range stream.ListSince("sess-1", 0) {
		switch ev.Type {
		case domain.EventToolError:
			var p domain.ToolPart
			_ = json.Unmarshal(ev.Payload, &p)
			if p.CallID == "dlg-1" {
				sawDlgErr = true
			}
			if p.CallID == "ask-1" {
				sawAskErr = true
			}
		case domain.EventDelegateCompleted:
			var p domain.DelegateCompletedPayload
			_ = json.Unmarshal(ev.Payload, &p)
			if p.CallID == "dlg-1" && p.ChildTurnID == "turn-child" && p.Status == string(domain.TurnFailed) {
				sawDelegateDone = true
			}
		case domain.EventTurnFailed:
			if ev.TurnID == "turn-child" {
				sawChildFailed = true
			}
		}
	}
	if !sawDlgErr || !sawAskErr || !sawDelegateDone || !sawChildFailed {
		t.Fatalf("missing settle events: dlgErr=%v askErr=%v delegateDone=%v childFailed=%v",
			sawDlgErr, sawAskErr, sawDelegateDone, sawChildFailed)
	}

	// Idempotent: second close must not duplicate terminal stream events.
	before := len(stream.ListSince("sess-1", 0))
	engine.closeIncompleteToolPairs("sess-1", "turn-parent")
	after := len(stream.ListSince("sess-1", 0))
	if after != before {
		t.Fatalf("second close published %d extra events", after-before)
	}
}

func TestCancelTurnEagerlyClearsRunningStatus(t *testing.T) {
	repo := &memTurnRepo{turns: map[string]domain.TurnLog{
		"turn-1": {
			ID:        "turn-1",
			SessionID: "session-1",
			Status:    domain.TurnRunning,
			Goal:      "stuck",
		},
		"turn-child": {
			ID:        "turn-child",
			SessionID: "session-1",
			Status:    domain.TurnRunning,
			Goal:      "delegate",
		},
	}}
	stream := &memStream{}
	engine := &Engine{
		turns:  service.NewTurnManager(repo),
		stream: stream,
		cancel: map[string]context.CancelFunc{},
	}
	// Register a cancel func as if the turn goroutine is alive in this process.
	// Old bug: CancelTurn called cancel() and returned without updating DB.
	_, cancel := context.WithCancel(context.Background())
	engine.cancel["turn-1"] = cancel
	if err := engine.reserveSessionTurn("session-1", "turn-1"); err != nil {
		t.Fatalf("reserve: %v", err)
	}

	engine.CancelTurn(context.Background(), "turn-1")

	got, err := repo.Get(context.Background(), "turn-1")
	if err != nil {
		t.Fatalf("get turn: %v", err)
	}
	if got.Status != domain.TurnCancelled {
		t.Fatalf("expected cancelled after CancelTurn, got %s", got.Status)
	}
	child, err := repo.Get(context.Background(), "turn-child")
	if err != nil {
		t.Fatalf("get child: %v", err)
	}
	if child.Status != domain.TurnCancelled {
		t.Fatalf("expected child cancelled after CancelTurn, got %s", child.Status)
	}
	if id := engine.ActiveTurnID("session-1"); id != "" {
		t.Fatalf("expected activeTurns cleared after CancelTurn, still %q", id)
	}

	saw := map[string]string{}
	for _, ev := range stream.ListSince("session-1", 0) {
		if ev.Type != domain.EventTurnFailed {
			continue
		}
		var p domain.TurnEndedPayload
		if err := json.Unmarshal(ev.Payload, &p); err != nil {
			t.Fatalf("decode turn.failed: %v", err)
		}
		saw[ev.TurnID] = p.Status
	}
	if saw["turn-1"] != string(domain.TurnCancelled) {
		t.Fatalf("turn-1 stream status: want cancelled, got %q (events=%v)", saw["turn-1"], saw)
	}
	if saw["turn-child"] != string(domain.TurnCancelled) {
		t.Fatalf("turn-child stream status: want cancelled, got %q (events=%v)", saw["turn-child"], saw)
	}
}
