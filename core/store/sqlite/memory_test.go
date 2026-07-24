package sqlite

import (
	"context"
	"path/filepath"
	"testing"

	"danmo-work/core/domain"
)

func TestMemoryRepoUpsertAndScopeIsolation(t *testing.T) {
	st, err := New(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	repo := st.Memories()
	ctx := context.Background()

	if _, err := repo.Upsert(ctx, domain.Memory{
		Scope: domain.MemoryScopeUser, ScopeID: domain.MemoryUserScopeID,
		Key: "lang", Content: "prefer Chinese",
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.Upsert(ctx, domain.Memory{
		Scope: domain.MemoryScopeProject, ScopeID: "proj-a",
		Key: "stack", Content: "use Go",
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.Upsert(ctx, domain.Memory{
		Scope: domain.MemoryScopeProject, ScopeID: "proj-b",
		Key: "stack", Content: "use Rust",
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.Upsert(ctx, domain.Memory{
		Scope: domain.MemoryScopeAgent, ScopeID: "default",
		Key: "style", Content: "be concise",
	}); err != nil {
		t.Fatal(err)
	}

	// Upsert same key updates content
	saved, err := repo.Upsert(ctx, domain.Memory{
		Scope: domain.MemoryScopeUser, ScopeID: domain.MemoryUserScopeID,
		Key: "lang", Content: "prefer English",
	})
	if err != nil {
		t.Fatal(err)
	}
	got, err := repo.GetByKey(ctx, domain.MemoryScopeUser, domain.MemoryUserScopeID, "lang")
	if err != nil {
		t.Fatal(err)
	}
	if got.Content != "prefer English" {
		t.Fatalf("upsert did not update: %q", got.Content)
	}
	if got.ID != saved.ID {
		t.Fatalf("id changed on upsert: %s vs %s", got.ID, saved.ID)
	}

	// Project A cannot see Project B
	hits, err := repo.Search(ctx, domain.MemoryQuery{
		Scopes: []domain.MemoryScopeRef{
			{Scope: domain.MemoryScopeUser, ScopeID: domain.MemoryUserScopeID},
			{Scope: domain.MemoryScopeProject, ScopeID: "proj-a"},
			{Scope: domain.MemoryScopeAgent, ScopeID: "default"},
		},
		TopK: 20,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(hits) != 3 {
		t.Fatalf("expected 3 visible memories, got %d", len(hits))
	}
	for _, m := range hits {
		if m.Scope == domain.MemoryScopeProject && m.ScopeID == "proj-b" {
			t.Fatal("proj-b memory leaked into proj-a visibility")
		}
	}
}

func TestMemoryRepoKeywordSearch(t *testing.T) {
	st, err := New(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	repo := st.Memories()
	ctx := context.Background()

	_, _ = repo.Upsert(ctx, domain.Memory{
		Scope: domain.MemoryScopeUser, ScopeID: domain.MemoryUserScopeID,
		Key: "preferred_language", Content: "Reply in Chinese by default",
	})
	_, _ = repo.Upsert(ctx, domain.Memory{
		Scope: domain.MemoryScopeUser, ScopeID: domain.MemoryUserScopeID,
		Key: "theme", Content: "dark mode preferred",
	})

	hits, err := repo.Search(ctx, domain.MemoryQuery{
		Scopes: []domain.MemoryScopeRef{{Scope: domain.MemoryScopeUser, ScopeID: domain.MemoryUserScopeID}},
		Query:  "Chinese",
		TopK:   10,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(hits) != 1 || hits[0].Key != "preferred_language" {
		t.Fatalf("keyword search failed: %+v", hits)
	}

	byKey, err := repo.Search(ctx, domain.MemoryQuery{
		Scopes: []domain.MemoryScopeRef{{Scope: domain.MemoryScopeUser, ScopeID: domain.MemoryUserScopeID}},
		Key:    "theme",
		TopK:   10,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(byKey) != 1 || byKey[0].Content != "dark mode preferred" {
		t.Fatalf("key search failed: %+v", byKey)
	}
}
