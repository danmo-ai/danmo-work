package sqlite

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"danmo-work/core/domain"
	"danmo-work/core/port"
)

func newSplitStores(t *testing.T) (*Store, *Store) {
	t.Helper()
	dir := t.TempDir()
	work, err := New(filepath.Join(dir, "work.db"))
	if err != nil {
		t.Fatalf("open work: %v", err)
	}
	hist, err := NewHistory(filepath.Join(dir, "history.db"))
	if err != nil {
		t.Fatalf("open history: %v", err)
	}
	work.SetHistory(hist)
	return work, hist
}

func countRows(t *testing.T, s *Store, model any) int64 {
	t.Helper()
	var n int64
	if err := s.db.Model(model).Count(&n).Error; err != nil {
		t.Fatalf("count: %v", err)
	}
	return n
}

// Stream events and turn entries must land in history.db while turn metadata
// stays in work.db.
func TestHistorySplitRouting(t *testing.T) {
	ctx := context.Background()
	work, hist := newSplitStores(t)

	if err := work.StreamEvents().Save(ctx, domain.StreamEvent{SessionID: "s1", Seq: 1, Type: "x"}); err != nil {
		t.Fatalf("save event: %v", err)
	}
	tl := work.TurnLogs()
	if err := tl.UpsertTurnMeta(ctx, domain.TurnLog{ID: "t1", SessionID: "s1", Status: domain.TurnRunning}); err != nil {
		t.Fatalf("upsert meta: %v", err)
	}
	if err := tl.AppendEntry(ctx, port.TurnLogEntryRecord{TurnID: "t1", Seq: 1, Type: "user", Data: map[string]any{"content": "hi"}}); err != nil {
		t.Fatalf("append entry: %v", err)
	}

	if n := countRows(t, hist, &streamEventModel{}); n != 1 {
		t.Fatalf("history stream_events = %d, want 1", n)
	}
	if n := countRows(t, hist, &turnLogEntryModel{}); n != 1 {
		t.Fatalf("history turn_log_entries = %d, want 1", n)
	}
	if n := countRows(t, work, &streamEventModel{}); n != 0 {
		t.Fatalf("work stream_events = %d, want 0", n)
	}
	if n := countRows(t, work, &turnModel{}); n != 1 {
		t.Fatalf("work turns = %d, want 1", n)
	}
	// Reads route through the same plane.
	evs, err := work.StreamEvents().ListBySession(ctx, "s1", 0)
	if err != nil || len(evs) != 1 {
		t.Fatalf("ListBySession = %v, %v", evs, err)
	}
	entries, err := tl.ListEntries(ctx, "t1")
	if err != nil || len(entries) != 1 {
		t.Fatalf("ListEntries = %v, %v", entries, err)
	}
}

// The one-time split migration moves existing bulk rows out of work.db and is
// idempotent via the app_meta marker.
func TestMigrateHistoryTables(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	work, err := New(filepath.Join(dir, "work.db"))
	if err != nil {
		t.Fatalf("open work: %v", err)
	}

	// Seed legacy rows directly in work.db (pre-split layout).
	for i := 1; i <= 5; i++ {
		m := streamEventModel{SessionID: "s1", Seq: int64(i), Type: "x", CreatedAt: time.Now()}
		if err := work.db.Create(&m).Error; err != nil {
			t.Fatal(err)
		}
	}
	for i := 1; i <= 3; i++ {
		m := turnLogEntryModel{TurnID: "t1", Seq: i, Type: "user", Data: `{"content":"hi"}`}
		if err := work.db.Create(&m).Error; err != nil {
			t.Fatal(err)
		}
	}

	hist, err := NewHistory(filepath.Join(dir, "history.db"))
	if err != nil {
		t.Fatalf("open history: %v", err)
	}
	work.SetHistory(hist)

	if err := MigrateHistoryTables(ctx, work, hist); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	if n := countRows(t, hist, &streamEventModel{}); n != 5 {
		t.Fatalf("history stream_events = %d, want 5", n)
	}
	if n := countRows(t, hist, &turnLogEntryModel{}); n != 3 {
		t.Fatalf("history turn_log_entries = %d, want 3", n)
	}
	if n := countRows(t, work, &streamEventModel{}); n != 0 {
		t.Fatalf("work stream_events = %d, want 0", n)
	}

	// Second run is a no-op even if new rows appear in work.db afterwards
	// (marker set): seed one stray row and re-run.
	stray := streamEventModel{SessionID: "s2", Seq: 99, Type: "x"}
	if err := work.db.Create(&stray).Error; err != nil {
		t.Fatal(err)
	}
	if err := MigrateHistoryTables(ctx, work, hist); err != nil {
		t.Fatalf("migrate 2nd: %v", err)
	}
	if n := countRows(t, hist, &streamEventModel{}); n != 5 {
		t.Fatalf("history stream_events after 2nd run = %d, want 5", n)
	}
}

// Deleting a session cascades turn rows, entries, and events across planes.
func TestDeleteSessionHistoryCascade(t *testing.T) {
	ctx := context.Background()
	work, hist := newSplitStores(t)
	tl := work.TurnLogs()

	for _, turn := range []string{"t1", "t2"} {
		if err := tl.UpsertTurnMeta(ctx, domain.TurnLog{ID: turn, SessionID: "s1", Status: domain.TurnCompleted}); err != nil {
			t.Fatal(err)
		}
		if err := tl.AppendEntry(ctx, port.TurnLogEntryRecord{TurnID: turn, Seq: 1, Type: "user", Data: map[string]any{"c": 1}}); err != nil {
			t.Fatal(err)
		}
	}
	if err := tl.UpsertTurnMeta(ctx, domain.TurnLog{ID: "t3", SessionID: "s2", Status: domain.TurnCompleted}); err != nil {
		t.Fatal(err)
	}
	if err := work.StreamEvents().Save(ctx, domain.StreamEvent{SessionID: "s1", Seq: 1, Type: "x"}); err != nil {
		t.Fatal(err)
	}
	if err := work.StreamEvents().Save(ctx, domain.StreamEvent{SessionID: "s2", Seq: 2, Type: "x"}); err != nil {
		t.Fatal(err)
	}

	if err := tl.DeleteSessionHistory(ctx, "s1"); err != nil {
		t.Fatalf("cascade: %v", err)
	}
	if err := work.StreamEvents().DeleteBySession(ctx, "s1"); err != nil {
		t.Fatalf("events cascade: %v", err)
	}

	if n := countRows(t, work, &turnModel{}); n != 1 {
		t.Fatalf("turns = %d, want 1 (s2 only)", n)
	}
	if n := countRows(t, hist, &turnLogEntryModel{}); n != 0 {
		t.Fatalf("entries = %d, want 0", n)
	}
	if n := countRows(t, hist, &streamEventModel{}); n != 1 {
		t.Fatalf("events = %d, want 1 (s2 only)", n)
	}
}

// Retention: orphaned history is always pruned; stale sessions lose bulk rows
// but keep turn metadata.
func TestPruneHistory(t *testing.T) {
	ctx := context.Background()
	work, hist := newSplitStores(t)
	tl := work.TurnLogs()

	old := time.Now().UTC().Add(-72 * time.Hour)
	fresh := time.Now().UTC()
	if err := work.Sessions().Create(ctx, domain.Session{ID: "stale", CreatedAt: old, UpdatedAt: old}); err != nil {
		t.Fatal(err)
	}
	if err := work.Sessions().Create(ctx, domain.Session{ID: "live", CreatedAt: fresh, UpdatedAt: fresh}); err != nil {
		t.Fatal(err)
	}

	seed := func(session, turn string, seq int64) {
		t.Helper()
		if err := tl.UpsertTurnMeta(ctx, domain.TurnLog{ID: turn, SessionID: session, Status: domain.TurnCompleted}); err != nil {
			t.Fatal(err)
		}
		if err := tl.AppendEntry(ctx, port.TurnLogEntryRecord{TurnID: turn, Seq: 1, Type: "user", Data: map[string]any{"c": 1}}); err != nil {
			t.Fatal(err)
		}
		if err := work.StreamEvents().Save(ctx, domain.StreamEvent{SessionID: session, Seq: seq, Type: "x"}); err != nil {
			t.Fatal(err)
		}
		if err := work.Checkpoints().Save(ctx, domain.CompactionCheckpoint{SessionID: session, TurnID: turn, Summary: "s"}); err != nil {
			t.Fatal(err)
		}
		if _, err := work.FileChanges().Append(ctx, session, domain.FileChangeRecord{TurnID: turn, Tool: "write", Path: "a.go", Op: domain.FileChangeCreate}); err != nil {
			t.Fatal(err)
		}
	}
	seed("stale", "t-stale", 1)
	seed("live", "t-live", 2)
	seed("ghost", "t-ghost", 3) // no session row → orphan

	stats, err := work.PruneHistory(ctx, 24*time.Hour)
	if err != nil {
		t.Fatalf("prune: %v", err)
	}
	if stats.OrphanSessions != 1 {
		t.Fatalf("orphans = %d, want 1", stats.OrphanSessions)
	}
	if stats.AgedSessions != 1 {
		t.Fatalf("aged = %d, want 1", stats.AgedSessions)
	}

	// Ghost: everything gone including turn rows. Stale: turn rows kept,
	// bulk rows gone. Live: untouched.
	var turnIDs []string
	if err := work.db.Model(&turnModel{}).Order("id").Pluck("id", &turnIDs).Error; err != nil {
		t.Fatal(err)
	}
	if len(turnIDs) != 2 || turnIDs[0] != "t-live" || turnIDs[1] != "t-stale" {
		t.Fatalf("turns = %v, want [t-live t-stale]", turnIDs)
	}
	entries, err := tl.ListEntries(ctx, "t-live")
	if err != nil || len(entries) != 1 {
		t.Fatalf("live entries = %v, %v", entries, err)
	}
	if entries, _ := tl.ListEntries(ctx, "t-stale"); len(entries) != 0 {
		t.Fatalf("stale entries not pruned: %v", entries)
	}
	if n := countRows(t, hist, &streamEventModel{}); n != 1 {
		t.Fatalf("events = %d, want 1 (live only)", n)
	}
	if n := countRows(t, hist, &fileChangeModel{}); n != 1 {
		t.Fatalf("file changes = %d, want 1 (live only)", n)
	}
	// Checkpoints: ghost's row dropped (orphan), stale's kept (metadata
	// survives age pruning), live untouched.
	if cp, _ := work.Checkpoints().Get(ctx, "ghost"); cp != nil {
		t.Fatalf("ghost checkpoint not dropped: %+v", cp)
	}
	if cp, _ := work.Checkpoints().Get(ctx, "stale"); cp == nil {
		t.Fatal("stale checkpoint should survive age pruning")
	}
	if cp, _ := work.Checkpoints().Get(ctx, "live"); cp == nil {
		t.Fatal("live checkpoint should be untouched")
	}

	// maxAge=0 disables age pruning: live session stays even after re-run.
	if _, err := work.PruneHistory(ctx, 0); err != nil {
		t.Fatal(err)
	}
	if entries, _ := tl.ListEntries(ctx, "t-live"); len(entries) != 1 {
		t.Fatalf("live entries pruned with maxAge=0: %v", entries)
	}
}
