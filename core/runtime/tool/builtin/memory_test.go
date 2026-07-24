package builtin

import (
	"context"
	"strings"
	"testing"
	"time"

	"danmo-work/core/domain"
)

type memMemoryStore struct {
	byKey map[string]domain.Memory
}

func newMemMemoryStore() *memMemoryStore {
	return &memMemoryStore{byKey: make(map[string]domain.Memory)}
}

func memKey(scope domain.MemoryScope, scopeID, key string) string {
	return string(scope) + "|" + scopeID + "|" + key
}

func (s *memMemoryStore) Upsert(_ context.Context, m domain.Memory) (domain.Memory, error) {
	k := memKey(m.Scope, m.ScopeID, m.Key)
	if existing, ok := s.byKey[k]; ok {
		m.ID = existing.ID
	} else if m.ID == "" {
		m.ID = "mem-test"
	}
	m.UpdatedAt = time.Now().UTC()
	s.byKey[k] = m
	return m, nil
}

func (s *memMemoryStore) GetByKey(_ context.Context, scope domain.MemoryScope, scopeID, key string) (domain.Memory, error) {
	m, ok := s.byKey[memKey(scope, scopeID, key)]
	if !ok {
		return domain.Memory{}, errNotFound
	}
	return m, nil
}

func (s *memMemoryStore) Search(_ context.Context, q domain.MemoryQuery) ([]domain.Memory, error) {
	allowed := map[string]struct{}{}
	for _, ref := range q.Scopes {
		allowed[string(ref.Scope)+"|"+ref.ScopeID] = struct{}{}
	}
	var out []domain.Memory
	for _, m := range s.byKey {
		if _, ok := allowed[string(m.Scope)+"|"+m.ScopeID]; !ok {
			continue
		}
		if q.Scope != "" && m.Scope != q.Scope {
			continue
		}
		if q.Key != "" && m.Key != q.Key {
			continue
		}
		if q.Query != "" {
			hay := strings.ToLower(m.Key + " " + m.Content)
			if !strings.Contains(hay, strings.ToLower(q.Query)) {
				continue
			}
		}
		out = append(out, m)
	}
	if q.TopK > 0 && len(out) > q.TopK {
		out = out[:q.TopK]
	}
	return out, nil
}

type notFoundError struct{}

func (notFoundError) Error() string { return "not found" }

var errNotFound = notFoundError{}

func TestMemoryUpdateAndRead(t *testing.T) {
	store := newMemMemoryStore()
	upd := &MemoryUpdate{Store: store}
	rd := &MemoryRead{Store: store, TopK: 10}
	ctx := context.Background()

	args := map[string]any{
		"scope":        "project",
		"key":          "api_style",
		"content":      "prefer REST over GraphQL",
		"__project_id": "proj-1",
		"__agent_id":   "default",
	}
	res, err := upd.Execute(ctx, args)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(res.Content, "api_style") {
		t.Fatalf("unexpected update result: %s", res.Content)
	}

	// Append mode
	args["mode"] = "append"
	args["content"] = "use OpenAPI"
	if _, err := upd.Execute(ctx, args); err != nil {
		t.Fatal(err)
	}
	got, err := store.GetByKey(ctx, domain.MemoryScopeProject, "proj-1", "api_style")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(got.Content, "prefer REST") || !strings.Contains(got.Content, "use OpenAPI") {
		t.Fatalf("append failed: %q", got.Content)
	}

	readRes, err := rd.Execute(ctx, map[string]any{
		"query":        "OpenAPI",
		"__project_id": "proj-1",
		"__agent_id":   "default",
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(readRes.Content, "api_style") {
		t.Fatalf("read missed memory: %s", readRes.Content)
	}
}

func TestMemoryUpdateRejectsMissingProject(t *testing.T) {
	upd := &MemoryUpdate{Store: newMemMemoryStore()}
	_, err := upd.Execute(context.Background(), map[string]any{
		"scope":   "project",
		"key":     "x",
		"content": "y",
	})
	if err == nil {
		t.Fatal("expected error for missing project id")
	}
}

func TestMemoryUpdateUserScope(t *testing.T) {
	store := newMemMemoryStore()
	upd := &MemoryUpdate{Store: store}
	_, err := upd.Execute(context.Background(), map[string]any{
		"scope":   "user",
		"key":     "lang",
		"content": "Chinese",
	})
	if err != nil {
		t.Fatal(err)
	}
	got, err := store.GetByKey(context.Background(), domain.MemoryScopeUser, domain.MemoryUserScopeID, "lang")
	if err != nil {
		t.Fatal(err)
	}
	if got.Content != "Chinese" {
		t.Fatalf("got %q", got.Content)
	}
}

func TestMemoryReadScopeIsolation(t *testing.T) {
	store := newMemMemoryStore()
	ctx := context.Background()
	_, _ = store.Upsert(ctx, domain.Memory{
		Scope: domain.MemoryScopeProject, ScopeID: "proj-a", Key: "k", Content: "A only",
	})
	_, _ = store.Upsert(ctx, domain.Memory{
		Scope: domain.MemoryScopeProject, ScopeID: "proj-b", Key: "k", Content: "B only",
	})

	rd := &MemoryRead{Store: store, TopK: 10}
	res, err := rd.Execute(ctx, map[string]any{
		"__project_id": "proj-a",
		"__agent_id":   "default",
	})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(res.Content, "B only") {
		t.Fatalf("scope leak: %s", res.Content)
	}
	if !strings.Contains(res.Content, "A only") {
		t.Fatalf("missing visible memory: %s", res.Content)
	}
}
