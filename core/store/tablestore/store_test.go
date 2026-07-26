package tablestore

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"danmo-work/core/domain"
)

func TestUpsertGetQueryDelete(t *testing.T) {
	st, err := New(filepath.Join(t.TempDir(), "store.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	ctx := context.Background()

	row, err := st.Upsert(ctx, domain.TableRow{
		Scope:   domain.TableScopeProject,
		ScopeID: "proj-a",
		Table:   "email_digests",
		Key:     "2026-07-26",
		Data: map[string]any{
			"date":    "2026-07-26",
			"count":   float64(42),
			"summary": "busy day",
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if row.Key != "2026-07-26" || row.Data["summary"] != "busy day" {
		t.Fatalf("unexpected row: %+v", row)
	}

	got, err := st.Get(ctx, domain.TableScopeProject, "proj-a", "email_digests", "2026-07-26")
	if err != nil {
		t.Fatal(err)
	}
	if got.Data["count"] != float64(42) {
		t.Fatalf("count=%v", got.Data["count"])
	}

	// Upsert same key updates
	_, err = st.Upsert(ctx, domain.TableRow{
		Scope: domain.TableScopeProject, ScopeID: "proj-a",
		Table: "email_digests", Key: "2026-07-26",
		Data: map[string]any{"date": "2026-07-26", "count": float64(43), "summary": "updated"},
	})
	if err != nil {
		t.Fatal(err)
	}
	got, _ = st.Get(ctx, domain.TableScopeProject, "proj-a", "email_digests", "2026-07-26")
	if got.Data["summary"] != "updated" {
		t.Fatalf("expected update, got %+v", got.Data)
	}
	if got.CreatedAt.After(row.CreatedAt.Add(time.Second)) {
		t.Fatalf("created_at changed unexpectedly: %v -> %v", row.CreatedAt, got.CreatedAt)
	}

	_, _ = st.Upsert(ctx, domain.TableRow{
		Scope: domain.TableScopeProject, ScopeID: "proj-a",
		Table: "email_digests", Key: "2026-07-25",
		Data: map[string]any{"date": "2026-07-25", "count": float64(10)},
	})
	_, _ = st.Upsert(ctx, domain.TableRow{
		Scope: domain.TableScopeProject, ScopeID: "proj-b",
		Table: "email_digests", Key: "2026-07-26",
		Data: map[string]any{"date": "2026-07-26", "count": float64(1)},
	})

	hits, err := st.Query(ctx, domain.TableQuery{
		Scope: domain.TableScopeProject, ScopeID: "proj-a",
		Table:  "email_digests",
		Filter: map[string]any{"date": "2026-07-26"},
		Limit:  10,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(hits) != 1 || hits[0].Key != "2026-07-26" {
		t.Fatalf("filter hits=%+v", hits)
	}

	n, err := st.CountTable(ctx, domain.TableScopeProject, "proj-a", "email_digests")
	if err != nil || n != 2 {
		t.Fatalf("count=%d err=%v", n, err)
	}

	tables, err := st.ListTables(ctx, []domain.TableScopeRef{
		{Scope: domain.TableScopeProject, ScopeID: "proj-a"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(tables) != 1 || tables[0].Count != 2 {
		t.Fatalf("tables=%+v", tables)
	}

	if err := st.Delete(ctx, domain.TableScopeProject, "proj-a", "email_digests", "2026-07-25"); err != nil {
		t.Fatal(err)
	}
	n, _ = st.CountTable(ctx, domain.TableScopeProject, "proj-a", "email_digests")
	if n != 1 {
		t.Fatalf("after delete count=%d", n)
	}
}

func TestRejectBadTableName(t *testing.T) {
	st, err := New(filepath.Join(t.TempDir(), "store.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	_, err = st.Upsert(context.Background(), domain.TableRow{
		Scope: domain.TableScopeUser, ScopeID: "default",
		Table: "Email-Digests", Key: "1", Data: map[string]any{"x": 1},
	})
	if err == nil {
		t.Fatal("expected invalid table name error")
	}
}

func TestScopeIsolation(t *testing.T) {
	st, err := New(filepath.Join(t.TempDir(), "store.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	ctx := context.Background()
	_, err = st.Upsert(ctx, domain.TableRow{
		Scope: domain.TableScopeProject, ScopeID: "a",
		Table: "t", Key: "k", Data: map[string]any{"v": 1},
	})
	if err != nil {
		t.Fatal(err)
	}
	_, err = st.Get(ctx, domain.TableScopeProject, "b", "t", "k")
	if err == nil {
		t.Fatal("expected not found across projects")
	}
}
