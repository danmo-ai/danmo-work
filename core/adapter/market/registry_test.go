package market

import (
	"testing"

	"danmo-work/core/domain"
)

func TestRegistryDispatchesGitAndClawhub(t *testing.T) {
	r := NewRegistry([]domain.MarketSource{
		{ID: "g1", Kind: "git", Enabled: true, Repo: "/tmp/x", Platform: "local"},
		{ID: "c1", Kind: "clawhub", Enabled: true, Repo: "https://clawhub.ai"},
		{ID: "t1", Kind: "techleads", Enabled: true, Repo: "@tech-leads-club/skills-catalog"},
		{ID: "off", Kind: "clawhub", Enabled: false, Repo: "https://clawhub.ai"},
		{ID: "bad", Kind: "http", Enabled: true, Repo: "https://example.com"},
	})
	list := r.List()
	if len(list) != 3 {
		t.Fatalf("list len = %d", len(list))
	}
	g, ok := r.Get("g1")
	if !ok || g.Kind() != "git" {
		t.Fatalf("git adapter missing")
	}
	c, ok := r.Get("c1")
	if !ok || c.Kind() != "clawhub" {
		t.Fatalf("clawhub adapter missing")
	}
	tl, ok := r.Get("t1")
	if !ok || tl.Kind() != "techleads" {
		t.Fatalf("techleads adapter missing")
	}
	if _, ok := r.Get("off"); ok {
		t.Fatal("disabled source should be absent")
	}
	if _, ok := r.Get("bad"); ok {
		t.Fatal("unknown kind should be absent")
	}
}
