package service

import (
	"context"
	"path/filepath"
	"testing"

	"danmo-work/core/domain"
	"danmo-work/core/port"
	sqlitestore "danmo-work/core/store/sqlite"
)

func TestWeixinPeerStorePersistsProjectMeta(t *testing.T) {
	dir := t.TempDir()
	st, err := sqlitestore.New(filepath.Join(dir, "t.db"))
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	if err := st.WeixinAccounts().Upsert(ctx, domain.WeixinAccount{
		AccountID: "bot1",
		Token:     "tok",
		ProjectID: "default-proj",
	}); err != nil {
		t.Fatal(err)
	}
	peers := NewWeixinPeerStore(st)
	if err := peers.UpsertBinding(ctx, port.ChannelWeixin, "bot1", "peer1", "sess1", map[string]string{
		"context_token": "ct1",
		"project_id":    "peer-proj",
	}); err != nil {
		t.Fatal(err)
	}
	sid, meta, err := peers.GetBinding(ctx, port.ChannelWeixin, "bot1", "peer1")
	if err != nil {
		t.Fatal(err)
	}
	if sid != "sess1" || meta["project_id"] != "peer-proj" || meta["context_token"] != "ct1" {
		t.Fatalf("sid=%s meta=%v", sid, meta)
	}
	// Default account project remains; peer meta overrides via resolvePeerProject.
	def, err := peers.GetProjectID(ctx, port.ChannelWeixin, "bot1")
	if err != nil || def != "default-proj" {
		t.Fatalf("default=%q err=%v", def, err)
	}
	if err := peers.UpdateBindingMeta(ctx, port.ChannelWeixin, "bot1", "peer1", map[string]string{
		"project_id": "peer-proj-2",
	}); err != nil {
		t.Fatal(err)
	}
	_, meta, err = peers.GetBinding(ctx, port.ChannelWeixin, "bot1", "peer1")
	if err != nil {
		t.Fatal(err)
	}
	if meta["project_id"] != "peer-proj-2" || meta["context_token"] != "ct1" {
		t.Fatalf("after update meta=%v", meta)
	}
}
