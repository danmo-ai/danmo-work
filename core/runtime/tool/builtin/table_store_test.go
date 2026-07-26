package builtin

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"danmo-work/core/domain"
	"danmo-work/core/store/tablestore"
)

func TestTableUpsertGetQueryDelete(t *testing.T) {
	st, err := tablestore.New(filepath.Join(t.TempDir(), "store.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	budget := NewTableTurnBudget()
	quotas := func() domain.ConfigTableSection {
		q := domain.DefaultTableSection()
		q.MaxRowsPerTable = 100
		return q
	}
	upd := &TableUpsert{Store: st, Quotas: quotas, Budget: budget}
	get := &TableGet{Store: st, Quotas: quotas}
	query := &TableQuery{Store: st, Quotas: quotas}
	del := &TableDelete{Store: st}
	list := &TableList{Store: st}

	ctx := context.Background()
	base := map[string]any{
		"__project_id": "proj-1",
		"__agent_id":   "default",
		"__turn_id":    "turn-1",
		"scope":        "project",
		"table":        "email_digests",
		"key":          "2026-07-26",
		"data": map[string]any{
			"date":    "2026-07-26",
			"summary": "hello",
		},
	}
	res, err := upd.Execute(ctx, cloneArgs(base))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(res.Content, `"ok":true`) {
		t.Fatalf("upsert result=%s", res.Content)
	}

	got, err := get.Execute(ctx, map[string]any{
		"__project_id": "proj-1",
		"scope":        "project",
		"table":        "email_digests",
		"key":          "2026-07-26",
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(got.Content, "hello") {
		t.Fatalf("get=%s", got.Content)
	}

	_, err = upd.Execute(ctx, map[string]any{
		"__project_id": "proj-1",
		"__turn_id":    "turn-1",
		"scope":        "project",
		"table":        "email_digests",
		"key":          "2026-07-26",
		"mode":         "merge",
		"data":         map[string]any{"count": 3},
	})
	if err != nil {
		t.Fatal(err)
	}
	got, _ = get.Execute(ctx, map[string]any{
		"__project_id": "proj-1",
		"scope":        "project",
		"table":        "email_digests",
		"key":          "2026-07-26",
	})
	if !strings.Contains(got.Content, "hello") || !strings.Contains(got.Content, "3") {
		t.Fatalf("merge get=%s", got.Content)
	}

	qres, err := query.Execute(ctx, map[string]any{
		"__project_id": "proj-1",
		"scope":        "project",
		"table":        "email_digests",
		"filter":       map[string]any{"date": "2026-07-26"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(qres.Content, `"count":1`) {
		t.Fatalf("query=%s", qres.Content)
	}

	lres, err := list.Execute(ctx, map[string]any{
		"__project_id": "proj-1",
		"__agent_id":   "default",
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(lres.Content, "email_digests") {
		t.Fatalf("list=%s", lres.Content)
	}

	if _, err := del.Execute(ctx, map[string]any{
		"__project_id": "proj-1",
		"scope":        "project",
		"table":        "email_digests",
		"key":          "2026-07-26",
	}); err != nil {
		t.Fatal(err)
	}
}

func TestTableUpsertRejectsMissingProject(t *testing.T) {
	st, err := tablestore.New(filepath.Join(t.TempDir(), "store.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	upd := &TableUpsert{Store: st, Budget: NewTableTurnBudget()}
	_, err = upd.Execute(context.Background(), map[string]any{
		"scope": "project",
		"table": "t",
		"key":   "k",
		"data":  map[string]any{"x": 1},
	})
	if err == nil || !strings.Contains(err.Error(), "project") {
		t.Fatalf("expected project scope error, got %v", err)
	}
}

func TestTableUpsertTurnQuota(t *testing.T) {
	st, err := tablestore.New(filepath.Join(t.TempDir(), "store.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	upd := &TableUpsert{
		Store:  st,
		Budget: NewTableTurnBudget(),
		Quotas: func() domain.ConfigTableSection {
			q := domain.DefaultTableSection()
			q.MaxRowsPerTurn = 2
			return q
		},
	}
	for i := 0; i < 2; i++ {
		_, err := upd.Execute(context.Background(), map[string]any{
			"__project_id": "p",
			"__turn_id":    "t1",
			"scope":        "project",
			"table":        "t",
			"key":          string(rune('a' + i)),
			"data":         map[string]any{"i": i},
		})
		if err != nil {
			t.Fatalf("upsert %d: %v", i, err)
		}
	}
	_, err = upd.Execute(context.Background(), map[string]any{
		"__project_id": "p",
		"__turn_id":    "t1",
		"scope":        "project",
		"table":        "t",
		"key":          "c",
		"data":         map[string]any{"i": 2},
	})
	if err == nil || !strings.Contains(err.Error(), "max_rows_per_turn") {
		t.Fatalf("expected turn quota error, got %v", err)
	}
}

func TestTableUpsertTableQuota(t *testing.T) {
	st, err := tablestore.New(filepath.Join(t.TempDir(), "store.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	upd := &TableUpsert{
		Store:  st,
		Budget: NewTableTurnBudget(),
		Quotas: func() domain.ConfigTableSection {
			q := domain.DefaultTableSection()
			q.MaxRowsPerTable = 1
			q.MaxRowsPerTurn = 100
			return q
		},
	}
	_, err = upd.Execute(context.Background(), map[string]any{
		"__project_id": "p",
		"__turn_id":    "t",
		"scope":        "project",
		"table":        "t",
		"key":          "a",
		"data":         map[string]any{"x": 1},
	})
	if err != nil {
		t.Fatal(err)
	}
	_, err = upd.Execute(context.Background(), map[string]any{
		"__project_id": "p",
		"__turn_id":    "t",
		"scope":        "project",
		"table":        "t",
		"key":          "b",
		"data":         map[string]any{"x": 2},
	})
	if err == nil || !strings.Contains(err.Error(), "max_rows_per_table") {
		t.Fatalf("expected table quota error, got %v", err)
	}
}

func cloneArgs(m map[string]any) map[string]any {
	out := make(map[string]any, len(m))
	for k, v := range m {
		out[k] = v
	}
	return out
}
