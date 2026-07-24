package git

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"danmo-work/core/domain"
)

func TestLocalFetchCatalogAndPackage(t *testing.T) {
	root := filepath.Clean(filepath.Join("..", "..", "..", "..", "dq-market"))
	if _, err := os.Stat(filepath.Join(root, "catalog", "index.json")); err != nil {
		t.Skip("dq-market sibling repo not found")
	}
	abs, _ := filepath.Abs(root)
	m := New(domain.MarketSource{
		ID:       "local",
		Kind:     "git",
		Platform: "local",
		Repo:     abs,
		Ref:      "main",
		Enabled:  true,
	})
	ctx := context.Background()
	cat, err := m.FetchCatalog(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(cat.Items) < 2 {
		t.Fatalf("catalog items: %d", len(cat.Items))
	}
	var skillItem domain.MarketItem
	for _, it := range cat.Items {
		if it.ID == "meeting-notes" {
			skillItem = it
			break
		}
	}
	if skillItem.ID == "" {
		t.Fatal("meeting-notes not in catalog")
	}
	dir, cleanup, err := m.FetchPackage(ctx, skillItem, "main")
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()
	if _, err := os.Stat(filepath.Join(dir, "SKILL.md")); err != nil {
		t.Fatal(err)
	}
}
