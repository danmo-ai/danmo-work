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

func TestPendingMessageClaimAndDemoteSteering(t *testing.T) {
	dir := t.TempDir()
	st, err := sqlite.New(filepath.Join(dir, "work.db"))
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	repo := st.PendingMessages()

	queued := domain.PendingMessage{
		ID: "q1", SessionID: "s1", Content: "later", Position: 2, Status: domain.PendingQueued,
	}
	steer := domain.PendingMessage{
		ID: "s1msg", SessionID: "s1", Content: "nudge", Position: 1, Status: domain.PendingSteering,
	}
	if err := repo.Create(ctx, queued); err != nil {
		t.Fatal(err)
	}
	if err := repo.Create(ctx, steer); err != nil {
		t.Fatal(err)
	}

	// PopFront must skip steering rows.
	front, ok, err := repo.PopFront(ctx, "s1")
	if err != nil || !ok {
		t.Fatalf("pop queued: ok=%v err=%v", ok, err)
	}
	if front.ID != "q1" {
		t.Fatalf("popped steering instead of queued: %+v", front)
	}
	_ = repo.Update(ctx, domain.PendingMessage{
		ID: "q1", SessionID: "s1", Content: "later", Position: 2, Status: domain.PendingQueued,
	})

	claimed, err := repo.ClaimSteering(ctx, "s1")
	if err != nil {
		t.Fatal(err)
	}
	if len(claimed) != 1 || claimed[0].ID != "s1msg" || claimed[0].Content != "nudge" {
		t.Fatalf("claim: %+v", claimed)
	}
	list, err := repo.ListBySession(ctx, "s1")
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 1 || list[0].ID != "q1" {
		t.Fatalf("after claim list: %+v", list)
	}

	again, err := repo.ClaimSteering(ctx, "s1")
	if err != nil || len(again) != 0 {
		t.Fatalf("second claim should be empty: %+v err=%v", again, err)
	}

	leftover := domain.PendingMessage{
		ID: "s2msg", SessionID: "s1", Content: "missed", Position: 1, Status: domain.PendingSteering,
	}
	if err := repo.Create(ctx, leftover); err != nil {
		t.Fatal(err)
	}
	if err := repo.DemoteSteering(ctx, "s1"); err != nil {
		t.Fatal(err)
	}
	list, err = repo.ListBySession(ctx, "s1")
	if err != nil {
		t.Fatal(err)
	}
	var found bool
	for _, m := range list {
		if m.ID == "s2msg" {
			found = true
			if m.Status != domain.PendingQueued {
				t.Fatalf("demote status: %+v", m)
			}
		}
	}
	if !found {
		t.Fatalf("demoted message missing: %+v", list)
	}
}
