package turnlog

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"danmo-work/core/domain"
)

func TestCheckpointStoreLoadSelectsNewestTurnOnCountTie(t *testing.T) {
	root := t.TempDir()
	projector := func(projectID string) string { return filepath.Join(root, projectID) }
	dir := filepath.Join(projector("proj-a"), "sessions", "sess-1")
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}

	writeCheckpointFile(t, dir, domain.CompactionCheckpoint{
		SessionID: "sess-1", TurnID: "turn-100", TurnCount: 4, Summary: "older",
	})
	writeCheckpointFile(t, dir, domain.CompactionCheckpoint{
		SessionID: "sess-1", TurnID: "turn-200", TurnCount: 4, Summary: "newer",
	})

	store := NewCheckpointStore(projector)
	store.RegisterSession("sess-1", "proj-a")
	cp, err := store.Load("sess-1")
	if err != nil {
		t.Fatal(err)
	}
	if cp == nil || cp.TurnID != "turn-200" || cp.Summary != "newer" {
		t.Fatalf("expected newest tied checkpoint, got %+v", cp)
	}
}

func TestCheckpointStoreLoadIgnoresDifferentSession(t *testing.T) {
	root := t.TempDir()
	projector := func(projectID string) string { return filepath.Join(root, projectID) }
	dir := filepath.Join(projector("proj-a"), "sessions", "sess-1")
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}

	writeCheckpointFile(t, dir, domain.CompactionCheckpoint{
		SessionID: "sess-1", TurnID: "turn-100", TurnCount: 2, Summary: "valid",
	})
	writeCheckpointFile(t, dir, domain.CompactionCheckpoint{
		SessionID: "other", TurnID: "turn-999", TurnCount: 99, Summary: "wrong",
	})

	store := NewCheckpointStore(projector)
	store.RegisterSession("sess-1", "proj-a")
	cp, err := store.Load("sess-1")
	if err != nil {
		t.Fatal(err)
	}
	if cp == nil || cp.Summary != "valid" {
		t.Fatalf("expected matching-session checkpoint, got %+v", cp)
	}
}

func writeCheckpointFile(t *testing.T, dir string, cp domain.CompactionCheckpoint) {
	t.Helper()
	data, err := json.Marshal(cp)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "checkpoint_"+cp.TurnID+".json")
	if err := os.WriteFile(path, data, 0644); err != nil {
		t.Fatal(err)
	}
}
