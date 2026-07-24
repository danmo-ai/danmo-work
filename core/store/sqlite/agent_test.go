package sqlite

import (
	"context"
	"path/filepath"
	"testing"

	"danmo-work/core/domain"
)

func TestAgentRepoUpsertPersistsZeroSteps(t *testing.T) {
	st, err := New(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	repo := st.Agents()
	ctx := context.Background()

	if err := repo.Upsert(ctx, domain.Agent{
		ID: "default", Name: "Default", Mode: domain.AgentModePrimary,
		Steps: 15, CanDelegate: true,
	}); err != nil {
		t.Fatal(err)
	}

	if err := repo.Upsert(ctx, domain.Agent{
		ID: "default", Name: "Default", Mode: domain.AgentModePrimary,
		Steps: 0, CanDelegate: false,
	}); err != nil {
		t.Fatal(err)
	}

	got, err := repo.Get(ctx, "default")
	if err != nil {
		t.Fatal(err)
	}
	if got.Steps != 0 {
		t.Fatalf("steps: got %d want 0 (follow global default)", got.Steps)
	}
	if got.CanDelegate {
		t.Fatal("can_delegate: got true want false")
	}
}

func TestMigrateClearsLegacyAgentSteps(t *testing.T) {
	st, err := New(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()

	if err := st.Agents().Upsert(ctx, domain.Agent{
		ID: "default", Name: "Default", Mode: domain.AgentModePrimary, Steps: 0,
	}); err != nil {
		t.Fatal(err)
	}
	// Simulate pre-migration DB: wipe meta and restore legacy caps.
	if err := st.db.Exec("DELETE FROM app_meta WHERE key = ?", "agent_steps_follow_global_v1").Error; err != nil {
		t.Fatal(err)
	}
	if err := st.db.Exec("UPDATE agents SET steps = 15").Error; err != nil {
		t.Fatal(err)
	}
	if err := st.migrate(); err != nil {
		t.Fatal(err)
	}
	got, err := st.Agents().Get(ctx, "default")
	if err != nil {
		t.Fatal(err)
	}
	if got.Steps != 0 {
		t.Fatalf("after migrate steps: got %d want 0", got.Steps)
	}
}
