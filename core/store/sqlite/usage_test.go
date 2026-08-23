package sqlite

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"danmo-work/core/domain"
)

func TestUsageRepoAddDeltaRollupAndSeries(t *testing.T) {
	st, err := New(filepath.Join(t.TempDir(), "usage.db"))
	if err != nil {
		t.Fatal(err)
	}
	repo := st.Usage()
	ctx := context.Background()
	day1 := time.Date(2026, 8, 1, 10, 0, 0, 0, time.UTC)
	day2 := time.Date(2026, 8, 2, 10, 0, 0, 0, time.UTC)

	d1 := domain.UsageDelta{PromptTokens: 100, CompletionTokens: 20, CacheReadTokens: 40, CacheCreationTokens: 5, Model: "m1", AgentID: "agent-a"}
	if err := repo.AddDelta(ctx, "turn-1", "sess-1", "proj-1", d1, day1); err != nil {
		t.Fatal(err)
	}
	d2 := domain.UsageDelta{PromptTokens: 50, CompletionTokens: 10, TotalTokens: 0, Model: "m1", AgentID: "agent-a"}
	if err := repo.AddDelta(ctx, "turn-1", "sess-1", "proj-1", d2, day2); err != nil {
		t.Fatal(err)
	}
	d3 := domain.UsageDelta{PromptTokens: 200, CompletionTokens: 30, Model: "m2", AgentID: "agent-b"}
	if err := repo.AddDelta(ctx, "turn-2", "sess-1", "proj-1", d3, day2); err != nil {
		t.Fatal(err)
	}

	turn, err := repo.Get(ctx, domain.UsageGrainTurn, "turn-1")
	if err != nil {
		t.Fatal(err)
	}
	if turn.PromptTokens != 150 || turn.CompletionTokens != 30 || turn.TotalTokens != 180 || turn.CallCount != 2 {
		t.Fatalf("turn-1 rollup: %+v", turn)
	}
	if turn.CacheReadTokens != 40 || turn.CacheCreationTokens != 5 {
		t.Fatalf("turn-1 cache: %+v", turn)
	}
	if !turn.UpdatedAt.Equal(day2) {
		t.Fatalf("turn updated_at want day2, got %v", turn.UpdatedAt)
	}

	sess, err := repo.Get(ctx, domain.UsageGrainSession, "sess-1")
	if err != nil {
		t.Fatal(err)
	}
	if sess.TotalTokens != 410 || sess.CallCount != 3 {
		t.Fatalf("session rollup: %+v", sess)
	}

	proj, err := repo.Get(ctx, domain.UsageGrainProject, "proj-1")
	if err != nil {
		t.Fatal(err)
	}
	if proj.TotalTokens != 410 {
		t.Fatalf("project rollup: %+v", proj)
	}

	bd, err := repo.SummarizeProject(ctx, "proj-1")
	if err != nil {
		t.Fatal(err)
	}
	if bd.Summary.TotalTokens != 410 {
		t.Fatalf("summary: %+v", bd.Summary)
	}
	if bd.Summary.TurnCount != 2 {
		t.Fatalf("turn count: %+v", bd.Summary)
	}
	if bd.Summary.AvgTurnTokens != 205 {
		t.Fatalf("avg turn tokens: %+v", bd.Summary)
	}
	scope, err := repo.SummarizeScope(ctx, "proj-1", "")
	if err != nil {
		t.Fatal(err)
	}
	if scope.TurnCount != 2 || scope.AvgTurnTokens != 205 {
		t.Fatalf("SummarizeScope: %+v", scope)
	}
	if len(bd.Models) < 2 {
		t.Fatalf("expected >=2 model rows, got %d", len(bd.Models))
	}
	if len(bd.Agents) < 2 {
		t.Fatalf("expected >=2 agent rows, got %d", len(bd.Agents))
	}

	// Entire session cumulative lands on last updated day (day2).
	points, err := repo.Series(ctx, domain.UsageSeriesFilter{
		Period:    domain.UsagePeriodDay,
		ProjectID: "proj-1",
		Grain:     domain.UsageGrainSession,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(points) != 1 {
		t.Fatalf("expected 1 day bucket (last update), got %+v", points)
	}
	if !points[0].PeriodStart.Equal(time.Date(2026, 8, 2, 0, 0, 0, 0, time.UTC)) {
		t.Fatalf("bucket start: %v", points[0].PeriodStart)
	}
	if points[0].TotalTokens != 410 {
		t.Fatalf("bucket total: %+v", points[0])
	}

	modelPts, err := repo.Series(ctx, domain.UsageSeriesFilter{
		Period:    domain.UsagePeriodDay,
		ProjectID: "proj-1",
		Grain:     domain.UsageGrainModel,
		Model:     "m1",
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(modelPts) != 1 || modelPts[0].TotalTokens != 180 {
		t.Fatalf("model m1 series: %+v", modelPts)
	}
}

func TestUsageRepoNullCacheCoalesce(t *testing.T) {
	st, err := New(filepath.Join(t.TempDir(), "usage-null.db"))
	if err != nil {
		t.Fatal(err)
	}
	repo := st.Usage()
	ctx := context.Background()

	d0 := domain.UsageDelta{PromptTokens: 1000, CompletionTokens: 50, Model: "m1", AgentID: "a1"}
	if err := repo.AddDelta(ctx, "turn-a", "sess-a", "proj-a", d0, time.Now().UTC()); err != nil {
		t.Fatal(err)
	}
	// Simulate legacy row: cache columns stayed NULL after schema add.
	if err := st.DB().Exec(`UPDATE llm_usage_rollups SET cache_read_tokens = NULL, cache_creation_tokens = NULL WHERE grain = ? AND ref_id = ?`,
		string(domain.UsageGrainModel), "m1").Error; err != nil {
		t.Fatal(err)
	}

	d1 := domain.UsageDelta{PromptTokens: 200, CompletionTokens: 10, CacheReadTokens: 80, CacheCreationTokens: 5, Model: "m1", AgentID: "a1"}
	if err := repo.AddDelta(ctx, "turn-b", "sess-a", "proj-a", d1, time.Now().UTC()); err != nil {
		t.Fatal(err)
	}

	global, err := repo.Get(ctx, domain.UsageGrainModel, "m1")
	if err != nil {
		t.Fatal(err)
	}
	if global.CacheReadTokens != 80 || global.CacheCreationTokens != 5 {
		t.Fatalf("global model cache after NULL coalesce: %+v", global)
	}
}

func TestUsageSinkViaStreamManager(t *testing.T) {
	st, err := New(filepath.Join(t.TempDir(), "usage-sink.db"))
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	_ = st.Sessions().Create(ctx, domain.Session{
		ID: "s1", ProjectID: "p1", Title: "t", Status: domain.SessionStatusActive,
		CreatedAt: time.Now().UTC(), UpdatedAt: time.Now().UTC(),
	})

	// Import cycle: test sink in runtime package instead — keep sqlite-only here.
	delta := domain.UsageDelta{PromptTokens: 10, CompletionTokens: 5, Model: "gpt", AgentID: "default"}
	if err := st.Usage().AddDelta(ctx, "t1", "s1", "p1", delta, time.Now().UTC()); err != nil {
		t.Fatal(err)
	}
	has, err := st.Usage().HasGrain(ctx, domain.UsageGrainSession, "s1")
	if err != nil || !has {
		t.Fatalf("has session grain: %v %v", has, err)
	}
}
