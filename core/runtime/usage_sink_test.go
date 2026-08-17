package runtime

import (
	"context"
	"encoding/json"
	"path/filepath"
	"testing"
	"time"

	"danmo-work/core/domain"
	"danmo-work/core/store/sqlite"
)

func TestUsageSinkOnLLMUsageEvent(t *testing.T) {
	st, err := sqlite.New(filepath.Join(t.TempDir(), "sink.db"))
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	_ = st.Sessions().Create(ctx, domain.Session{
		ID: "sess-a", ProjectID: "proj-a", Title: "x", Status: domain.SessionStatusActive,
		CreatedAt: time.Now().UTC(), UpdatedAt: time.Now().UTC(),
	})

	stream := NewStreamEventManager(st.StreamEvents())
	stream.SetUsageSink(NewUsageSink(st.Usage(), st.Sessions()))

	stream.Publish(ctx, "sess-a", "turn-a", domain.EventLLMUsage, domain.LLMUsagePayload{
		PromptTokens: 40, CompletionTokens: 10, TotalTokens: 50,
		CacheReadTokens: 12, CacheCreationTokens: 3,
		Model: "deepseek-v4", AgentID: "default",
	})

	sess, err := st.Usage().Get(ctx, domain.UsageGrainSession, "sess-a")
	if err != nil {
		t.Fatal(err)
	}
	if sess.TotalTokens != 50 || sess.CallCount != 1 {
		t.Fatalf("session: %+v", sess)
	}
	if sess.CacheReadTokens != 12 || sess.CacheCreationTokens != 3 {
		t.Fatalf("session cache: %+v", sess)
	}
	models, err := st.Usage().ListByProject(ctx, "proj-a", domain.UsageGrainModel)
	if err != nil {
		t.Fatal(err)
	}
	if len(models) != 1 || models[0].Model != "deepseek-v4" || models[0].TotalTokens != 50 {
		t.Fatalf("models: %+v", models)
	}
}

func TestBackfillUsageFromStreamEvents(t *testing.T) {
	st, err := sqlite.New(filepath.Join(t.TempDir(), "backfill.db"))
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	_ = st.Sessions().Create(ctx, domain.Session{
		ID: "sess-b", ProjectID: "proj-b", Title: "y", Status: domain.SessionStatusActive,
		CreatedAt: time.Now().UTC(), UpdatedAt: time.Now().UTC(),
	})
	payload, _ := json.Marshal(domain.LLMUsagePayload{PromptTokens: 7, CompletionTokens: 3, TotalTokens: 10, Model: "m"})
	_ = st.StreamEvents().Save(ctx, domain.StreamEvent{
		Seq: 1, Type: domain.EventLLMUsage, SessionID: "sess-b", TurnID: "turn-b",
		Payload: payload, CreatedAt: time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC),
	})
	payload2, _ := json.Marshal(domain.LLMUsagePayload{PromptTokens: 1, CompletionTokens: 1, TotalTokens: 2, Model: "m"})
	_ = st.StreamEvents().Save(ctx, domain.StreamEvent{
		Seq: 2, Type: domain.EventLLMUsage, SessionID: "sess-b", TurnID: "turn-b",
		Payload: payload2, CreatedAt: time.Date(2026, 7, 2, 0, 0, 0, 0, time.UTC),
	})

	BackfillUsageFromStreamEvents(ctx, st.Usage(), st.Sessions(), st.StreamEvents())

	sess, err := st.Usage().Get(ctx, domain.UsageGrainSession, "sess-b")
	if err != nil {
		t.Fatal(err)
	}
	if sess.TotalTokens != 12 || sess.CallCount != 2 {
		t.Fatalf("backfill session: %+v", sess)
	}

	// Second run should no-op (HasGrain).
	BackfillUsageFromStreamEvents(ctx, st.Usage(), st.Sessions(), st.StreamEvents())
	sess2, _ := st.Usage().Get(ctx, domain.UsageGrainSession, "sess-b")
	if sess2.TotalTokens != 12 {
		t.Fatalf("backfill duplicated: %+v", sess2)
	}
}
