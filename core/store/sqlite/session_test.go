package sqlite_test

import (
	"context"
	"path/filepath"
	"testing"

	"danmo-work/core/domain"
	"danmo-work/core/store/sqlite"
)

func TestSessionUpdatePersistsZeroValueFields(t *testing.T) {
	dir := t.TempDir()
	st, err := sqlite.New(filepath.Join(dir, "work.db"))
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	repo := st.Sessions()

	s := domain.Session{
		ID: "s1", Title: "t", ProjectID: "p1", AgentID: "a1", ModelID: "m1",
		PlanMode: true, Content: "hello", Status: domain.SessionStatusActive,
	}
	if err := repo.Create(ctx, s); err != nil {
		t.Fatal(err)
	}

	got, err := repo.Get(ctx, "s1")
	if err != nil {
		t.Fatal(err)
	}
	got.PlanMode = false
	got.ProjectID = ""
	if err := repo.Update(ctx, got); err != nil {
		t.Fatal(err)
	}

	after, err := repo.Get(ctx, "s1")
	if err != nil {
		t.Fatal(err)
	}
	if after.PlanMode {
		t.Fatal("plan_mode=false was not persisted")
	}
	if after.ProjectID != "" {
		t.Fatalf("project_id clear was not persisted: %q", after.ProjectID)
	}
	if after.Title != "t" || after.Content != "hello" || after.Status != domain.SessionStatusActive {
		t.Fatalf("unexpected session after update: %+v", after)
	}
}
