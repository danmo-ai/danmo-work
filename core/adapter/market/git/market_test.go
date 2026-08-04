package git

import (
	"context"
	"os"
	"path/filepath"
	"strings"
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

func TestAssertZipArchive(t *testing.T) {
	if err := assertZipArchive([]byte("PK\x03\x04xxxx"), "http://example/a.zip"); err != nil {
		t.Fatal(err)
	}
	err := assertZipArchive([]byte("<!DOCTYPE html><html>"), "http://example/a.zip")
	if err == nil || !strings.Contains(err.Error(), "HTML") {
		t.Fatalf("want HTML error, got %v", err)
	}
	err = assertZipArchive([]byte(`{"status":404}`), "http://example/a.zip")
	if err == nil || !strings.Contains(err.Error(), "not a zip") {
		t.Fatalf("want not-a-zip error, got %v", err)
	}
}

func TestGiteeArchiveDownloadIsZip(t *testing.T) {
	if testing.Short() {
		t.Skip("network")
	}
	m := New(domain.MarketSource{
		ID:       "official-gitee",
		Kind:     "git",
		Platform: "gitee",
		Repo:     "https://gitee.com/danmo-ai/dq-market",
		Ref:      "main",
		Enabled:  true,
	})
	url, err := m.archiveURL("main")
	if err != nil {
		t.Fatal(err)
	}
	data, err := m.httpGet(context.Background(), url)
	if err != nil {
		t.Fatal(err)
	}
	if err := assertZipArchive(data, url); err != nil {
		t.Fatal(err)
	}
}
