package runtime

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"danmo-work/core/domain"
	"danmo-work/core/service"
	"danmo-work/core/store/turnlog"
)

func setupContinueFromEngine(t *testing.T) (*Engine, *memTurnRepo, *memStream, *service.TurnLogManager) {
	t.Helper()
	root := t.TempDir()
	tls := turnlog.NewTurnLogStore(func(projectID string) string {
		if projectID == "" {
			return root
		}
		return filepath.Join(root, projectID)
	})
	turnLogs := service.NewTurnLogManager(tls)
	stream := &memStream{}
	repo := &memTurnRepo{turns: map[string]domain.TurnLog{}}
	engine := &Engine{
		turns:   service.NewTurnManager(repo),
		turnLog: turnLogs,
		stream:  stream,
	}
	return engine, repo, stream, turnLogs
}

func seedParentChild(t *testing.T, engine *Engine, repo *memTurnRepo, stream *memStream, turnLogs *service.TurnLogManager, parentStatus, childStatus domain.TurnStatus) {
	t.Helper()
	ctx := context.Background()
	_ = repo.Create(ctx, domain.TurnLog{
		ID: "turn-parent", SessionID: "sess-1", AgentID: "team",
		Status: parentStatus, Goal: "lead",
	})
	_ = repo.Create(ctx, domain.TurnLog{
		ID: "turn-child", SessionID: "sess-1", AgentID: "researcher",
		Status: childStatus, Goal: "gather",
	})
	if err := turnLogs.Create("turn-parent", "sess-1", "proj-a", "team", "lead"); err != nil {
		t.Fatal(err)
	}
	if err := turnLogs.CreateNested("turn-child", "sess-1", "proj-a", "researcher", "gather"); err != nil {
		t.Fatal(err)
	}
	turnLogs.Append("turn-child", "user", map[string]any{"content": "gather facts"})
	turnLogs.Append("turn-child", "assistant", map[string]any{"content": "here are facts"})
	turnLogs.EndTurn("turn-child", domain.TurnCompleted)
	stream.Publish(ctx, "sess-1", "turn-parent", domain.EventDelegateStarted, domain.DelegateStartedPayload{
		AgentID: "researcher", Goal: "gather", ChildTurnID: "turn-child", CallID: "dlg-1",
	})
	_ = engine
}

func TestValidateContinueFromRequiresNestedAndTerminalParentChild(t *testing.T) {
	engine, repo, stream, turnLogs := setupContinueFromEngine(t)
	ctx := context.Background()

	seedParentChild(t, engine, repo, stream, turnLogs, domain.TurnCompleted, domain.TurnCompleted)
	if _, err := engine.validateContinueFrom(ctx, "sess-1", "turn-child"); err != nil {
		t.Fatalf("expected ok, got %v", err)
	}

	_ = repo.UpdateStatus(ctx, "turn-child", domain.TurnRunning)
	if _, err := engine.validateContinueFrom(ctx, "sess-1", "turn-child"); err == nil || !strings.Contains(err.Error(), "sub-turn") {
		t.Fatalf("want sub-turn still running error, got %v", err)
	}

	_ = repo.UpdateStatus(ctx, "turn-child", domain.TurnCompleted)
	_ = repo.UpdateStatus(ctx, "turn-parent", domain.TurnRunning)
	if _, err := engine.validateContinueFrom(ctx, "sess-1", "turn-child"); err == nil || !strings.Contains(err.Error(), "parent turn") {
		t.Fatalf("want parent still running error, got %v", err)
	}

	_ = repo.UpdateStatus(ctx, "turn-parent", domain.TurnCompleted)
	if _, err := engine.validateContinueFrom(ctx, "sess-1", "turn-parent"); err == nil || !strings.Contains(err.Error(), "nested") {
		t.Fatalf("want nested error, got %v", err)
	}

	if _, err := engine.validateContinueFrom(ctx, "sess-other", "turn-child"); err == nil {
		t.Fatal("expected session mismatch error")
	}
}

func TestContinueSubTurnReopensSameNestedLog(t *testing.T) {
	engine, repo, stream, turnLogs := setupContinueFromEngine(t)
	seedParentChild(t, engine, repo, stream, turnLogs, domain.TurnCompleted, domain.TurnCompleted)

	before := turnLogs.LoadTurnMessages("turn-child")
	if len(before) < 2 {
		t.Fatalf("expected prior child history, got %+v", before)
	}

	// Reopen nested log the same way continueSubTurn does.
	if err := turnLogs.CreateNested("turn-child", "sess-1", "proj-a", "researcher", "follow up"); err != nil {
		t.Fatal(err)
	}
	_ = repo.UpdateStatus(context.Background(), "turn-child", domain.TurnRunning)
	turnLogs.Append("turn-child", "user", map[string]any{"content": "follow up please"})

	after := turnLogs.LoadTurnMessages("turn-child")
	if len(after) != len(before)+1 {
		t.Fatalf("want prior history + new user, before=%d after=%d %+v", len(before), len(after), after)
	}
	if after[len(after)-1].Content != "follow up please" {
		t.Fatalf("last message: %+v", after[len(after)-1])
	}
	if !turnLogs.IsNestedToolRun("turn-child") {
		t.Fatal("continued turn must remain nested under tool_runs/")
	}
	// Still not a session root history turn.
	for _, id := range turnLogs.ListTurnIDs("sess-1") {
		if id == "turn-child" {
			t.Fatal("nested child must not appear in ListTurnIDs")
		}
	}
	_ = stream
	_ = engine
}
