package turnlog

import (
	"path/filepath"
	"strings"
	"testing"

	"danmo-work/core/domain"
)

func TestFileChangeStoreAppendAndLoadAfter(t *testing.T) {
	tmp := t.TempDir()
	store := NewFileChangeStore(func(pid string) string { return filepath.Join(tmp, pid) })

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

	// Restart: new store instance should continue seq.
	store2 := NewFileChangeStore(func(pid string) string { return filepath.Join(tmp, pid) })
	store2.RegisterSession("sess", "proj")
	seq3, err := store2.Append("sess", "proj", domain.FileChangeRecord{
		TurnID: "t2", Tool: "write", Path: "b.go", Op: domain.FileChangeCreate,
	})
	if err != nil {
		t.Fatal(err)
	}
	if seq3 != 3 {
		t.Fatalf("seq3=%d want 3 after restart", seq3)
	}
}
