package service

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"danmo-work/core/domain"
	"danmo-work/core/port"
	sqlitestore "danmo-work/core/store/sqlite"
)

type pendingStubEngine struct {
	active  string
	started []string
	err     error
}

func (e *pendingStubEngine) StartSession(context.Context, domain.Session, []domain.UserAttachment) {
}
func (e *pendingStubEngine) StartTurn(_ context.Context, _, userInput, _, _ string, _ []domain.UserAttachment) (string, error) {
	if e.err != nil {
		return "", e.err
	}
	e.started = append(e.started, userInput)
	return "turn-1", nil
}
func (e *pendingStubEngine) ResumeTurn(context.Context, string, string) error { return nil }
func (e *pendingStubEngine) CancelTurn(context.Context, string)               {}
func (e *pendingStubEngine) ActiveTurnID(string) string                       { return e.active }
func (e *pendingStubEngine) ListTurns(string) []domain.TurnLog                { return nil }
func (e *pendingStubEngine) StreamEvents(string, int64) []domain.StreamEvent  { return nil }
func (e *pendingStubEngine) Subscribe(string) chan domain.StreamEvent         { return nil }
func (e *pendingStubEngine) Unsubscribe(string, chan domain.StreamEvent)      {}
func (e *pendingStubEngine) ResolveApproval(string, bool, string)             {}
func (e *pendingStubEngine) PublishPermissionDecided(string, string, string, bool, string) {
}
func (e *pendingStubEngine) ResolveAskUser(string, string) error { return nil }
func (e *pendingStubEngine) RevokeSessionNetworkGrants(string)   {}

var _ port.Engine = (*pendingStubEngine)(nil)

func setupPendingSession(t *testing.T) (*SessionManager, *sqlitestore.Store, *pendingStubEngine, domain.Session) {
	t.Helper()
	dir := t.TempDir()
	st, err := sqlitestore.New(filepath.Join(dir, "work.db"))
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	now := time.Now().UTC()
	sess := domain.Session{
		ID: "sess-1", Title: "q", AgentID: "agent", ProjectID: "proj",
		Status: domain.SessionStatusActive, CreatedAt: now, UpdatedAt: now,
	}
	if err := st.Sessions().Create(ctx, sess); err != nil {
		t.Fatal(err)
	}
	eng := &pendingStubEngine{}
	return NewSessionManager(st, eng, nil), st, eng, sess
}

func insertQueued(t *testing.T, st *sqlitestore.Store, sessionID, id, content string) {
	t.Helper()
	now := time.Now().UTC()
	if err := st.PendingMessages().Create(context.Background(), domain.PendingMessage{
		ID: id, SessionID: sessionID, Content: content, Position: 1,
		Status: domain.PendingQueued, CreatedAt: now, UpdatedAt: now,
	}); err != nil {
		t.Fatal(err)
	}
}

func TestDrainPendingQueueDeletesAfterStartTurn(t *testing.T) {
	m, st, eng, sess := setupPendingSession(t)
	insertQueued(t, st, sess.ID, "p1", "next please")

	m.DrainPendingQueue(context.Background(), sess.ID)

	if len(eng.started) != 1 || eng.started[0] != "next please" {
		t.Fatalf("StartTurn calls: %v", eng.started)
	}
	list, err := m.ListPending(context.Background(), sess.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 0 {
		t.Fatalf("queue should be empty after drain, got %+v", list)
	}
}

func TestSteerPendingIdleRemovesFromQueue(t *testing.T) {
	m, st, eng, sess := setupPendingSession(t)
	insertQueued(t, st, sess.ID, "p1", "send now")

	if err := m.SteerPending(context.Background(), sess.ID, "p1"); err != nil {
		t.Fatal(err)
	}
	if len(eng.started) != 1 || eng.started[0] != "send now" {
		t.Fatalf("StartTurn calls: %v", eng.started)
	}
	list, err := m.ListPending(context.Background(), sess.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 0 {
		t.Fatalf("idle steer should drain and delete, got %+v", list)
	}
}

func TestSteerPendingClaimRemovesFromQueue(t *testing.T) {
	m, st, eng, sess := setupPendingSession(t)
	eng.active = "turn-running"
	insertQueued(t, st, sess.ID, "p1", "inject into turn")

	if err := m.SteerPending(context.Background(), sess.ID, "p1"); err != nil {
		t.Fatal(err)
	}
	list, err := m.ListPending(context.Background(), sess.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 1 || list[0].Status != domain.PendingSteering {
		t.Fatalf("armed steer should stay until claim: %+v", list)
	}

	claimed, err := m.ClaimSteering(context.Background(), sess.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(claimed) != 1 || claimed[0].Content != "inject into turn" {
		t.Fatalf("claim: %+v", claimed)
	}
	list, err = m.ListPending(context.Background(), sess.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 0 {
		t.Fatalf("claimed steer must leave the pending queue, got %+v", list)
	}
}
