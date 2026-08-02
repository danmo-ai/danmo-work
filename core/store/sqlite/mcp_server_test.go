package sqlite

import (
	"context"
	"path/filepath"
	"testing"

	"danmo-work/core/domain"
)

func TestMCPServerAmbientMountFalsePersists(t *testing.T) {
	st, err := New(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	repo := st.MCPServers()
	ctx := context.Background()

	srv := domain.MCPServer{
		ID:           "danmo-make",
		Name:         "Danmo Make",
		Transport:    "streamable-http",
		URL:          "http://127.0.0.1:7800/mcp",
		Auth:         domain.MCPAuthNone,
		CatalogID:    "danmo-make",
		Enabled:      true,
		AmbientMount: false,
		ToolTimeout:  900,
		Status:       "disconnected",
		EnabledTools: []string{"*"},
	}
	if err := repo.Upsert(ctx, srv); err != nil {
		t.Fatal(err)
	}
	got, err := repo.Get(ctx, "danmo-make")
	if err != nil {
		t.Fatal(err)
	}
	if got.AmbientMount {
		t.Fatal("AmbientMount should stay false after create")
	}

	srv.Name = "Danmo Make (updated)"
	srv.AmbientMount = false
	if err := repo.Upsert(ctx, srv); err != nil {
		t.Fatal(err)
	}
	got, err = repo.Get(ctx, "danmo-make")
	if err != nil {
		t.Fatal(err)
	}
	if got.AmbientMount {
		t.Fatal("AmbientMount should stay false after update")
	}
	if got.Name != "Danmo Make (updated)" {
		t.Fatalf("name=%q", got.Name)
	}
}
