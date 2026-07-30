package turnlog

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSnapshotStoreRoundTrip(t *testing.T) {
	root := t.TempDir()
	st := NewSnapshotStore(func(projectID string) string {
		return filepath.Join(root, projectID)
	})
	content := []byte("hello\nworld\n")
	meta, err := st.Save("p1", "s1", "t1", "docs/a.md", content)
	if err != nil {
		t.Fatal(err)
	}
	if meta.Hash == "" || !meta.HasContent {
		t.Fatalf("bad meta: %+v", meta)
	}
	got, meta2, err := st.ReadContent("p1", "s1", "t1", "docs/a.md")
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(content) {
		t.Fatalf("content mismatch: %q", got)
	}
	if meta2.Path != "docs/a.md" {
		t.Fatalf("path: %s", meta2.Path)
	}
	list, err := st.ListTurnPaths("p1", "s1", "t1")
	if err != nil || len(list) != 1 {
		t.Fatalf("list: %v %v", list, err)
	}
}

func TestSnapshotStoreTooLargeHashOnly(t *testing.T) {
	root := t.TempDir()
	st := NewSnapshotStore(func(projectID string) string {
		return filepath.Join(root, projectID)
	})
	big := make([]byte, MaxSnapshotBytes+1)
	for i := range big {
		big[i] = 'a'
	}
	meta, err := st.Save("p1", "s1", "t1", "big.bin", big)
	if err != nil {
		t.Fatal(err)
	}
	if meta.HasContent {
		t.Fatal("expected hash-only")
	}
	_, _, err = st.ReadContent("p1", "s1", "t1", "big.bin")
	if err != ErrSnapshotNoContent {
		t.Fatalf("want ErrSnapshotNoContent, got %v", err)
	}
	// meta file must exist
	dir := st.snapDir("p1", "s1", "t1")
	entries, _ := os.ReadDir(dir)
	if len(entries) == 0 {
		t.Fatal("expected meta on disk")
	}
}
