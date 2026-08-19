package turnlog

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"danmo-work/core/domain"
)

func TestFileChangeStoreAppendAndLoadAfter(t *testing.T) {
	st := newTestSQLStore(t)
	store := NewFileChangeStore(st.FileChanges())

	seq1, err := store.Append("sess", "proj", domain.FileChangeRecord{
		TurnID: "t1", Tool: "write", Path: "a.go", Op: domain.FileChangeCreate,
		Diff: strings.Repeat("x", 5000),
	})
	if err != nil {
		t.Fatal(err)
	}
	if seq1 != 1 {
		t.Fatalf("seq1=%d", seq1)
	}
	seq2, err := store.Append("sess", "proj", domain.FileChangeRecord{
		TurnID: "t1", Tool: "edit", Path: "a.go", Op: domain.FileChangeUpdate,
	})
	if err != nil {
		t.Fatal(err)
	}
	if seq2 != 2 {
		t.Fatalf("seq2=%d", seq2)
	}

	all, err := store.LoadAfter("sess", 0)
	if err != nil || len(all) != 2 {
		t.Fatalf("LoadAfter(0)=%+v err=%v", all, err)
	}
	if len(all[0].Diff) > maxFileChangeDiffBytes+40 {
		t.Fatalf("diff not truncated: %d", len(all[0].Diff))
	}
	if !strings.Contains(all[0].Diff, "truncated") {
		t.Fatalf("expected truncation marker, got %q", all[0].Diff[len(all[0].Diff)-40:])
	}

	delta, err := store.LoadAfter("sess", 1)
	if err != nil || len(delta) != 1 || delta[0].Seq != 2 {
		t.Fatalf("LoadAfter(1)=%+v err=%v", delta, err)
	}

	// Restart: a new store instance over the same DB continues the seq.
	store2 := NewFileChangeStore(st.FileChanges())
	seq3, err := store2.Append("sess", "proj", domain.FileChangeRecord{
		TurnID: "t2", Tool: "write", Path: "b.go", Op: domain.FileChangeCreate,
	})
	if err != nil {
		t.Fatal(err)
	}
	if seq3 != 3 {
		t.Fatalf("seq3=%d want 3 after restart", seq3)
	}

	// Sessions are isolated.
	if other, _ := store2.LoadAfter("other", 0); len(other) != 0 {
		t.Fatalf("other session leaked: %+v", other)
	}
}

// Legacy import preserves original seq values so checkpoint FileChangeLogSeq
// cursors stay valid, and skips sessions that already have DB rows.
func TestImportLegacyFileChangesPreservesSeq(t *testing.T) {
	ctx := context.Background()
	st := newTestSQLStore(t)
	tmp := t.TempDir()
	sessionDir := filepath.Join(tmp, "proj-a", "sessions", "sess-1")
	if err := os.MkdirAll(sessionDir, 0o755); err != nil {
		t.Fatal(err)
	}

	var lines []string
	for _, rec := range []domain.FileChangeRecord{
		{Seq: 3, TurnID: "t1", Tool: "write", Path: "a.go", Op: domain.FileChangeCreate},
		{Seq: 7, TurnID: "t2", Tool: "edit", Path: "a.go", Op: domain.FileChangeUpdate},
	} {
		b, _ := json.Marshal(rec)
		lines = append(lines, string(b))
	}
	if err := os.WriteFile(filepath.Join(sessionDir, "file_changes.jsonl"), []byte(strings.Join(lines, "\n")+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	n, err := ImportLegacyArtifacts(ctx, st.Checkpoints(), st.FileChanges(), func(pid string) string { return filepath.Join(tmp, pid) })
	if err != nil {
		t.Fatalf("import: %v", err)
	}
	if n != 1 {
		t.Fatalf("imported = %d, want 1", n)
	}

	recs, err := st.FileChanges().ListAfter(ctx, "sess-1", 0)
	if err != nil || len(recs) != 2 {
		t.Fatalf("recs = %+v, %v", recs, err)
	}
	if recs[0].Seq != 3 || recs[1].Seq != 7 {
		t.Fatalf("seqs not preserved: %d, %d", recs[0].Seq, recs[1].Seq)
	}
	// Cursor semantics survive: afterSeq=3 returns only the second record.
	delta, _ := st.FileChanges().ListAfter(ctx, "sess-1", 3)
	if len(delta) != 1 || delta[0].Seq != 7 {
		t.Fatalf("cursor broken: %+v", delta)
	}

	// New appends continue after the preserved max seq.
	seq, err := NewFileChangeStore(st.FileChanges()).Append("sess-1", "", domain.FileChangeRecord{TurnID: "t3", Tool: "write", Path: "b.go", Op: domain.FileChangeCreate})
	if err != nil {
		t.Fatal(err)
	}
	if seq != 8 {
		t.Fatalf("next seq = %d, want 8", seq)
	}

	// Re-import is a no-op for sessions with rows.
	if _, err := ImportLegacyArtifacts(ctx, st.Checkpoints(), st.FileChanges(), func(pid string) string { return filepath.Join(tmp, pid) }); err != nil {
		t.Fatal(err)
	}
	recs, _ = st.FileChanges().ListAfter(ctx, "sess-1", 0)
	if len(recs) != 3 {
		t.Fatalf("re-import duplicated rows: %d", len(recs))
	}
}
