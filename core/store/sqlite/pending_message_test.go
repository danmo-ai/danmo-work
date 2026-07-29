package sqlite_test

import (
	"context"
	"path/filepath"
	"testing"

	"danmo-work/core/domain"
	"danmo-work/core/store/sqlite"
)

func TestPendingMessageQueueOrderAndPop(t *testing.T) {
	dir := t.TempDir()
	st, err := sqlite.New(filepath.Join(dir, "work.db"))
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	repo := st.PendingMessages()

	a := domain.PendingMessage{
		ID: "p1", SessionID: "s1", Content: "first", Position: 1, Status: domain.PendingQueued,
	}
	b := domain.PendingMessage{
		ID: "p2", SessionID: "s1", Content: "second", Position: 2, Status: domain.PendingQueued,
	}
	if err := repo.Create(ctx, a); err != nil {
		t.Fatal(err)
	}
	if err := repo.Create(ctx, b); err != nil {
		t.Fatal(err)
	}

	list, err := repo.ListBySession(ctx, "s1")
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 2 || list[0].ID != "p1" || list[1].ID != "p2" {
		t.Fatalf("unexpected list: %+v", list)
	}

	if err := repo.Reorder(ctx, "s1", []string{"p2", "p1"}); err != nil {
		t.Fatal(err)
	}
	list, err = repo.ListBySession(ctx, "s1")
	if err != nil {
		t.Fatal(err)
	}
	if list[0].ID != "p2" || list[1].ID != "p1" {
		t.Fatalf("reorder failed: %+v", list)
	}

	front, ok, err := repo.PopFront(ctx, "s1")
	if err != nil || !ok {
		t.Fatalf("pop: ok=%v err=%v", ok, err)
	}
	if front.ID != "p2" || front.Status != domain.PendingSending {
		t.Fatalf("unexpected front: %+v", front)
	}
	list, err = repo.ListBySession(ctx, "s1")
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 1 || list[0].ID != "p1" {
		t.Fatalf("after pop list: %+v", list)
	}
}
