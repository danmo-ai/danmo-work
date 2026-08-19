package turnlog

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"danmo-work/core/domain"
)

func TestCheckpointStoreSaveLoadRoundtrip(t *testing.T) {
	st := newTestSQLStore(t)
	store := NewCheckpointStore(st.Checkpoints())

	cp := &domain.CompactionCheckpoint{
		SessionID: "sess-1", TurnID: "turn-2", Summary: "sum",
		Todos:     []domain.CompactionTodoItem{{Content: "do x", Status: "pending"}},
		TurnCount: 3, RetainFromTurnID: "turn-2",
	}
	if err := store.Save("sess-1", cp); err != nil {
		t.Fatalf("save: %v", err)
	}
	// Only the latest survives (upsert by session).
	cp2 := &domain.CompactionCheckpoint{SessionID: "sess-1", TurnID: "turn-5", Summary: "newer", TurnCount: 5}
	if err := store.Save("sess-1", cp2); err != nil {
		t.Fatalf("save 2: %v", err)
	}

	// Fresh store over the same DB = process restart.
	store2 := NewCheckpointStore(st.Checkpoints())
	got, err := store2.Load("sess-1")
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if got == nil || got.TurnID != "turn-5" || got.Summary != "newer" {
		t.Fatalf("got %+v, want turn-5/newer", got)
	}

	missing, err := store2.Load("sess-none")
	if err != nil || missing != nil {
		t.Fatalf("missing session: got %+v, %v", missing, err)
	}
}

func writeLegacyCheckpoint(t *testing.T, dir string, cp domain.CompactionCheckpoint) {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	data, _ := json.Marshal(cp)
	if err := os.WriteFile(filepath.Join(dir, "checkpoint_"+cp.TurnID+".json"), data, 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestImportLegacyCheckpointSelectsNewestTurnOnCountTie(t *testing.T) {
	ctx := context.Background()
	st := newTestSQLStore(t)
	tmp := t.TempDir()
	sessionDir := filepath.Join(tmp, "proj-a", "sessions", "sess-1")

	writeLegacyCheckpoint(t, sessionDir, domain.CompactionCheckpoint{
		SessionID: "sess-1", TurnID: "turn-a", Summary: "old", TurnCount: 4,
	})
	writeLegacyCheckpoint(t, sessionDir, domain.CompactionCheckpoint{
		SessionID: "sess-1", TurnID: "turn-b", Summary: "new", TurnCount: 4,
	})
	// A different session's file in the same dir must be ignored.
	other, _ := json.Marshal(domain.CompactionCheckpoint{SessionID: "sess-2", TurnID: "turn-z", Summary: "alien", TurnCount: 99})
	if err := os.WriteFile(filepath.Join(sessionDir, "checkpoint_turn-z.json"), other, 0o644); err != nil {
		t.Fatal(err)
	}

	n, err := ImportLegacyArtifacts(ctx, st.Checkpoints(), st.FileChanges(), func(pid string) string { return filepath.Join(tmp, pid) })
	if err != nil {
		t.Fatalf("import: %v", err)
	}
	if n != 1 {
		t.Fatalf("imported = %d, want 1", n)
	}

	got, err := st.Checkpoints().Get(ctx, "sess-1")
	if err != nil || got == nil {
		t.Fatalf("get: %+v, %v", got, err)
	}
	if got.TurnID != "turn-b" || got.Summary != "new" {
		t.Fatalf("imported wrong checkpoint: %+v", got)
	}

	// Idempotent: an existing DB row wins over files on re-run.
	writeLegacyCheckpoint(t, sessionDir, domain.CompactionCheckpoint{
		SessionID: "sess-1", TurnID: "turn-c", Summary: "later-file", TurnCount: 9,
	})
	if _, err := ImportLegacyArtifacts(ctx, st.Checkpoints(), st.FileChanges(), func(pid string) string { return filepath.Join(tmp, pid) }); err != nil {
		t.Fatal(err)
	}
	got, _ = st.Checkpoints().Get(ctx, "sess-1")
	if got.TurnID != "turn-b" {
		t.Fatalf("re-import overwrote DB row: %+v", got)
	}
}
