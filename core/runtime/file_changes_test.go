package runtime

import (
	"context"
	"strconv"
	"testing"

	"danmo-work/core/adapter/llm"
	"danmo-work/core/domain"
	"danmo-work/core/store/turnlog"
)

func TestMergeFileChangesLastOpWins(t *testing.T) {
	prev := []domain.CompactionFileChange{
		{Path: "a.go", Op: domain.FileChangeCreate, Tools: []string{"write"}, TurnIDs: []string{"t1"}},
	}
	delta := []domain.FileChangeRecord{
		{Seq: 2, Path: "a.go", Op: domain.FileChangeUpdate, Tool: "edit", TurnID: "t2"},
		{Seq: 3, Path: "b.go", Op: domain.FileChangeCreate, Tool: "write", TurnID: "t2"},
	}
	got := mergeFileChanges(prev, delta)
	if len(got) != 2 {
		t.Fatalf("len=%d want 2: %+v", len(got), got)
	}
	byPath := map[string]domain.CompactionFileChange{}
	for _, c := range got {
		byPath[c.Path] = c
	}
	if byPath["a.go"].Op != domain.FileChangeUpdate {
		t.Fatalf("a.go op=%s", byPath["a.go"].Op)
	}
	if !containsString(byPath["a.go"].Tools, "write") || !containsString(byPath["a.go"].Tools, "edit") {
		t.Fatalf("a.go tools=%v", byPath["a.go"].Tools)
	}
	if byPath["b.go"].Op != domain.FileChangeCreate {
		t.Fatalf("b.go op=%s", byPath["b.go"].Op)
	}
}

func TestMergeFileChangesCreateThenDeleteDrops(t *testing.T) {
	delta := []domain.FileChangeRecord{
		{Seq: 1, Path: "tmp.md", Op: domain.FileChangeCreate, Tool: "write", TurnID: "t1"},
		{Seq: 2, Path: "tmp.md", Op: domain.FileChangeDelete, Tool: "apply_patch", TurnID: "t1"},
	}
	got := mergeFileChanges(nil, delta)
	if len(got) != 0 {
		t.Fatalf("expected create→delete drop, got %+v", got)
	}
}

func TestMergeFileChangesInheritsAndKeepsDelete(t *testing.T) {
	prev := []domain.CompactionFileChange{
		{Path: "keep.go", Op: domain.FileChangeUpdate, Tools: []string{"edit"}, TurnIDs: []string{"t1"}},
	}
	// Existing file deleted — keep delete marker.
	delta := []domain.FileChangeRecord{
		{Seq: 5, Path: "keep.go", Op: domain.FileChangeDelete, Tool: "apply_patch", TurnID: "t3"},
	}
	got := mergeFileChanges(prev, delta)
	if len(got) != 1 || got[0].Op != domain.FileChangeDelete {
		t.Fatalf("want delete marker, got %+v", got)
	}
}

func TestMergeFileChangesPathCap(t *testing.T) {
	prev := make([]domain.CompactionFileChange, 0, maxCompactionFileChanges+10)
	for i := 0; i < maxCompactionFileChanges+10; i++ {
		prev = append(prev, domain.CompactionFileChange{
			Path: "file-" + strconv.Itoa(i) + ".go",
			Op:   domain.FileChangeUpdate,
		})
	}
	got := mergeFileChanges(prev, nil)
	if len(got) != maxCompactionFileChanges {
		t.Fatalf("len=%d want %d", len(got), maxCompactionFileChanges)
	}
	wantLast := "file-" + strconv.Itoa(maxCompactionFileChanges+9) + ".go"
	if got[len(got)-1].Path != wantLast {
		t.Fatalf("last path=%s want %s", got[len(got)-1].Path, wantLast)
	}
}

func TestFormatFileChangesAndSystemPrompt(t *testing.T) {
	changes := []domain.CompactionFileChange{
		{Path: "core/foo.go", Op: domain.FileChangeUpdate, Tools: []string{"edit", "write"}, TurnIDs: []string{"t3", "t5"}},
		{Path: "docs/note.md", Op: domain.FileChangeCreate, Tools: []string{"write"}, TurnIDs: []string{"t4"}},
	}
	block := formatFileChanges(changes)
	if !contains(block, "<session-file-changes>") || !contains(block, "core/foo.go") {
		t.Fatalf("formatFileChanges=%q", block)
	}
	sys := buildSystemPrompt("persona", nil, nil, false, "", "", block, domain.SandboxStatus{}, domain.EnvironmentStatus{})
	if !contains(sys, "<session-file-changes>") || !contains(sys, "update core/foo.go") {
		t.Fatalf("expected file changes in prompt, got %q", sys)
	}
}

func TestFileChangeRecordsFromWriteAndPatchMeta(t *testing.T) {
	writeRecs := fileChangeRecordsFromResult("turn-1", "c1", "write", map[string]any{"path": "a.go"}, domain.ToolResult{
		Meta: map[string]any{"path": "a.go", "op": "create", "bytes_written": 12},
	})
	if len(writeRecs) != 1 || writeRecs[0].Op != domain.FileChangeCreate || writeRecs[0].Path != "a.go" {
		t.Fatalf("write recs=%+v", writeRecs)
	}

	// Directory write ignored.
	dirRecs := fileChangeRecordsFromResult("turn-1", "c2", "write", nil, domain.ToolResult{
		Meta: map[string]any{"path": "dir", "op": "create", "write_type": "directory"},
	})
	if len(dirRecs) != 0 {
		t.Fatalf("directory write should be skipped: %+v", dirRecs)
	}

	// No-op write (identical content) ignored.
	noopRecs := fileChangeRecordsFromResult("turn-1", "c3", "write", map[string]any{"path": "a.go"}, domain.ToolResult{
		Meta: map[string]any{"path": "a.go", "op": "noop", "changed": false, "bytes_written": 0},
	})
	if len(noopRecs) != 0 {
		t.Fatalf("no-op write should be skipped: %+v", noopRecs)
	}

	patchRecs := fileChangeRecordsFromResult("turn-2", "c3", "apply_patch", nil, domain.ToolResult{
		Meta: map[string]any{
			"file_changes": []map[string]any{
				{"path": "one.go", "op": "update", "bytes_written": 10},
				{"path": "two.go", "op": "create", "bytes_written": 5},
				{"path": "gone.go", "op": "delete", "bytes_written": 0},
			},
		},
	})
	if len(patchRecs) != 3 {
		t.Fatalf("patch recs=%+v", patchRecs)
	}
	if patchRecs[2].Op != domain.FileChangeDelete || patchRecs[2].Path != "gone.go" {
		t.Fatalf("delete rec=%+v", patchRecs[2])
	}
}

func TestCompactionMergesFileChangesIncrementally(t *testing.T) {
	mock := llm.NewMock().
		AddText("summary-1").
		AddText("summary-2")
	stream := NewStreamEventManager(nil)
	artifacts := newArtifactTestStore(t)
	cpStore := turnlog.NewCheckpointStore(artifacts.Checkpoints())
	fcStore := turnlog.NewFileChangeStore(artifacts.FileChanges())
	mgr := NewCompactionManager(mock, stream, testCompactionConfig(true, 2, 2, 128000, 50), cpStore, nil)
	mgr.SetFileChangeJournal(fcStore)

	sessionID := "s-fc"
	projectID := "proj"
	if _, err := fcStore.Append(sessionID, projectID, domain.FileChangeRecord{
		TurnID: "t1", CallID: "c1", Tool: "write", Path: "a.go", Op: domain.FileChangeCreate,
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := fcStore.Append(sessionID, projectID, domain.FileChangeRecord{
		TurnID: "t1", CallID: "c2", Tool: "edit", Path: "a.go", Op: domain.FileChangeUpdate,
	}); err != nil {
		t.Fatal(err)
	}

	pad := string(make([]byte, 200))
	messages := []Message{
		{Role: RoleSystem, Content: "sys"},
		{Role: RoleUser, Content: "long user message that should exceed cut " + pad},
		{Role: RoleAssistant, Content: "working " + pad},
		{Role: RoleUser, Content: "more long content " + pad},
		{Role: RoleAssistant, Content: "reply " + pad},
	}
	if cut := mgr.Compact(context.Background(), sessionID, "turn-1", messages, 2, "mock/test"); cut <= 0 {
		t.Fatal("first compact failed")
	}
	cp := mgr.Recover(context.Background(), sessionID)
	if cp == nil || len(cp.FileChanges) != 1 || cp.FileChanges[0].Path != "a.go" {
		t.Fatalf("first FileChanges=%+v", cp)
	}
	if cp.FileChangeLogSeq != 2 {
		t.Fatalf("FileChangeLogSeq=%d want 2", cp.FileChangeLogSeq)
	}
	if !containsString(cp.FileChanges[0].Tools, "write") || !containsString(cp.FileChanges[0].Tools, "edit") {
		t.Fatalf("tools=%v", cp.FileChanges[0].Tools)
	}

	// Incremental: only new journal rows after seq=2.
	if _, err := fcStore.Append(sessionID, projectID, domain.FileChangeRecord{
		TurnID: "t2", CallID: "c3", Tool: "write", Path: "b.go", Op: domain.FileChangeCreate,
	}); err != nil {
		t.Fatal(err)
	}
	if cut := mgr.Compact(context.Background(), sessionID, "turn-2", messages, 4, "mock/test"); cut <= 0 {
		t.Fatal("second compact failed")
	}
	cp = mgr.Recover(context.Background(), sessionID)
	if cp == nil || len(cp.FileChanges) != 2 {
		t.Fatalf("second FileChanges=%+v", cp)
	}
	if cp.FileChangeLogSeq != 3 {
		t.Fatalf("FileChangeLogSeq=%d want 3", cp.FileChangeLogSeq)
	}
	byPath := map[string]domain.CompactionFileChange{}
	for _, c := range cp.FileChanges {
		byPath[c.Path] = c
	}
	if byPath["a.go"].Op != domain.FileChangeUpdate || byPath["b.go"].Op != domain.FileChangeCreate {
		t.Fatalf("merged=%+v", cp.FileChanges)
	}
}

func TestCompactionInheritsFileChangesWhenNoDelta(t *testing.T) {
	mock := llm.NewMock().
		AddText("first").
		AddText("second")
	stream := NewStreamEventManager(nil)
	artifacts := newArtifactTestStore(t)
	cpStore := turnlog.NewCheckpointStore(artifacts.Checkpoints())
	fcStore := turnlog.NewFileChangeStore(artifacts.FileChanges())
	mgr := NewCompactionManager(mock, stream, testCompactionConfig(true, 2, 2, 128000, 50), cpStore, nil)
	mgr.SetFileChangeJournal(fcStore)

	sessionID := "s-inherit-fc"
	if _, err := fcStore.Append(sessionID, "proj", domain.FileChangeRecord{
		TurnID: "t1", Tool: "write", Path: "keep.go", Op: domain.FileChangeCreate,
	}); err != nil {
		t.Fatal(err)
	}
	pad := string(make([]byte, 200))
	msgs := []Message{
		{Role: RoleSystem, Content: "sys"},
		{Role: RoleUser, Content: "long user message that should exceed cut " + pad},
		{Role: RoleAssistant, Content: "done " + pad},
		{Role: RoleUser, Content: "more long content " + pad},
		{Role: RoleAssistant, Content: "reply " + pad},
	}
	if mgr.Compact(context.Background(), sessionID, "turn-1", msgs, 2, "mock/test") <= 0 {
		t.Fatal("first compact failed")
	}
	if mgr.Compact(context.Background(), sessionID, "turn-2", msgs, 4, "mock/test") <= 0 {
		t.Fatal("second compact failed")
	}
	cp := mgr.Recover(context.Background(), sessionID)
	if cp == nil || len(cp.FileChanges) != 1 || cp.FileChanges[0].Path != "keep.go" {
		t.Fatalf("expected inherited file changes, got %+v", cp)
	}
	if cp.FileChangeLogSeq != 1 {
		t.Fatalf("seq=%d want 1", cp.FileChangeLogSeq)
	}
}
