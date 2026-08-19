package turnlog

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"danmo-work/core/domain"
)

func writeLegacyFile(t *testing.T, dir, name, content string) {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestImportLegacyJSONL(t *testing.T) {
	root := t.TempDir()
	projector := func(projectID string) string {
		if projectID == "" {
			return root
		}
		return filepath.Join(root, projectID)
	}

	sessDir := filepath.Join(root, "proj-a", "sessions", "sess-1")
	writeLegacyFile(t, sessDir, "turn-1.jsonl",
		`{"seq":1,"type":"start","data":{"agent_id":"agent-1","goal":"say hi"}}`+"\n"+
			`{"seq":2,"type":"user","data":{"content":"hi"}}`+"\n"+
			`{"seq":3,"type":"assistant","data":{"content":"hello"}}`+"\n"+
			`{"seq":4,"type":"end","data":{"status":"completed"}}`+"\n")
	// Crashed turn: no end entry — must import as failed, never a recovery candidate.
	writeLegacyFile(t, sessDir, "turn-2.jsonl",
		`{"seq":1,"type":"start","data":{"agent_id":"agent-1","goal":"crash"}}`+"\n"+
			`{"seq":2,"type":"user","data":{"content":"crash"}}`+"\n")
	// Nested tool run under tool_runs/.
	writeLegacyFile(t, filepath.Join(sessDir, "tool_runs"), "turn-child.jsonl",
		`{"seq":1,"type":"start","data":{"agent_id":"worker","goal":"sub"}}`+"\n"+
			`{"seq":2,"type":"user","data":{"content":"sub"}}`+"\n"+
			`{"seq":3,"type":"end","data":{"status":"completed"}}`+"\n")

	repo := newTestRepo(t)
	ctx := context.Background()
	n, err := ImportLegacyJSONL(ctx, repo, projector)
	if err != nil {
		t.Fatal(err)
	}
	if n != 3 {
		t.Fatalf("want 3 imported turns, got %d", n)
	}

	meta, ok, err := repo.GetTurnMeta(ctx, "turn-1")
	if err != nil || !ok {
		t.Fatalf("turn-1 meta: ok=%v err=%v", ok, err)
	}
	if meta.SessionID != "sess-1" || meta.ProjectID != "proj-a" || meta.AgentID != "agent-1" ||
		meta.Goal != "say hi" || meta.Status != domain.TurnCompleted || meta.Nested {
		t.Fatalf("turn-1 meta: %+v", meta)
	}

	crashed, ok, _ := repo.GetTurnMeta(ctx, "turn-2")
	if !ok || crashed.Status != domain.TurnFailed {
		t.Fatalf("crashed turn must import as failed: ok=%v %+v", ok, crashed)
	}

	child, ok, _ := repo.GetTurnMeta(ctx, "turn-child")
	if !ok || !child.Nested {
		t.Fatalf("nested turn must carry nested flag: ok=%v %+v", ok, child)
	}

	s := NewTurnLogStore(repo)
	msgs := s.LoadSessionMessages("sess-1", "", 0)
	if len(msgs) != 3 {
		t.Fatalf("want 3 replayed msgs (turn-2 user included, nested excluded), got %d %+v", len(msgs), msgs)
	}
	if msgs[0].Content != "hi" || msgs[1].Content != "hello" || msgs[2].Content != "crash" {
		t.Fatalf("replayed: %+v", msgs)
	}

	// Idempotent: a second run must not duplicate anything.
	n2, err := ImportLegacyJSONL(ctx, repo, projector)
	if err != nil {
		t.Fatal(err)
	}
	if n2 != 0 {
		t.Fatalf("second import must be a no-op, imported %d", n2)
	}
	if again := s.LoadSessionMessages("sess-1", "", 0); len(again) != 3 {
		t.Fatalf("idempotency broken: %d msgs", len(again))
	}
}

func TestImportLegacyJSONLKeepsExistingTurnStatus(t *testing.T) {
	root := t.TempDir()
	projector := func(projectID string) string {
		if projectID == "" {
			return root
		}
		return filepath.Join(root, projectID)
	}
	sessDir := filepath.Join(root, "proj-a", "sessions", "sess-1")
	// File says completed, but the turns row (engine dual-write era) says cancelled.
	writeLegacyFile(t, sessDir, "turn-1.jsonl",
		`{"seq":1,"type":"start","data":{"agent_id":"a","goal":"g"}}`+"\n"+
			`{"seq":2,"type":"user","data":{"content":"hi"}}`+"\n"+
			`{"seq":3,"type":"end","data":{"status":"completed"}}`+"\n")

	repo := newTestRepo(t)
	ctx := context.Background()
	if err := repo.UpsertTurnMeta(ctx, domain.TurnLog{
		ID: "turn-1", SessionID: "sess-1", ProjectID: "proj-a",
		AgentID: "a", Goal: "g", Status: domain.TurnCancelled,
	}); err != nil {
		t.Fatal(err)
	}

	if _, err := ImportLegacyJSONL(ctx, repo, projector); err != nil {
		t.Fatal(err)
	}
	meta, _, _ := repo.GetTurnMeta(ctx, "turn-1")
	if meta.Status != domain.TurnCancelled {
		t.Fatalf("DB status is authoritative, got %s", meta.Status)
	}
	entries, err := repo.ListEntries(ctx, "turn-1")
	if err != nil || len(entries) != 1 {
		t.Fatalf("entries still imported for existing row: %v %+v", err, entries)
	}
}
