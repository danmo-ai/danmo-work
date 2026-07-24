package service_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"danmo-work/core/adapter/config"
	"danmo-work/core/adapter/weixin/ilink"
	"danmo-work/core/domain"
	"danmo-work/core/service"
	sqlitestore "danmo-work/core/store/sqlite"
)

func TestWeixinBindingRepoPeerSession(t *testing.T) {
	dir := t.TempDir()
	st, err := sqlitestore.New(filepath.Join(dir, "t.db"))
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	acc := domain.WeixinAccount{AccountID: "bot1", Token: "tok", BaseURL: ilink.DefaultBaseURL, CreatedAt: time.Now().UTC(), UpdatedAt: time.Now().UTC()}
	if err := st.WeixinAccounts().Upsert(ctx, acc); err != nil {
		t.Fatal(err)
	}
	b := domain.WeixinBinding{AccountID: "bot1", PeerUserID: "peer-a", SessionID: "session-xyz", ContextToken: "ct"}
	if err := st.WeixinBindings().Upsert(ctx, b); err != nil {
		t.Fatal(err)
	}
	got, err := st.WeixinBindings().GetByPeer(ctx, "bot1", "peer-a")
	if err != nil {
		t.Fatal(err)
	}
	if got.SessionID != "session-xyz" || got.ContextToken != "ct" {
		t.Fatalf("%+v", got)
	}
	n, err := st.WeixinBindings().Count(ctx)
	if err != nil || n != 1 {
		t.Fatalf("count=%d err=%v", n, err)
	}
}

func TestMigrateAccountProjectsOnce(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "t.db")
	cfgPath := filepath.Join(dir, "config.yaml")
	st, err := sqlitestore.New(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	loader := config.NewLoader(cfgPath)
	cm := service.NewConfigManager(loader)
	pm := service.NewProjectManager(st, dir)
	bridge := testWeixinBridge(st, pm, cm)

	proj, err := pm.Create(context.Background(), domain.CreateProjectRequest{Name: "Biz"})
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	if err := st.WeixinAccounts().Upsert(context.Background(), domain.WeixinAccount{
		AccountID: "bot-mig", Token: "tok", BaseURL: ilink.DefaultBaseURL, CreatedAt: now, UpdatedAt: now,
	}); err != nil {
		t.Fatal(err)
	}
	cfg, err := cm.Get(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	wx := cfg.Channels.Weixin
	wx.DefaultProjectID = proj.ID
	sec := cfg.Channels
	sec.Weixin = wx
	if _, err := cm.Update(context.Background(), domain.UpdateConfigFileRequest{Channels: &sec}); err != nil {
		t.Fatal(err)
	}

	if err := bridge.MigrateAccountProjectsOnce(context.Background()); err != nil {
		t.Fatal(err)
	}
	acc, err := st.WeixinAccounts().Get(context.Background(), "bot-mig")
	if err != nil {
		t.Fatal(err)
	}
	if acc.ProjectID != proj.ID {
		t.Fatalf("projectId=%q want %q", acc.ProjectID, proj.ID)
	}
	cfg, err = cm.Get(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Channels.Weixin.DefaultProjectID != "" {
		t.Fatalf("default_project_id still set: %q", cfg.Channels.Weixin.DefaultProjectID)
	}
	// idempotent
	if err := bridge.MigrateAccountProjectsOnce(context.Background()); err != nil {
		t.Fatal(err)
	}
}

func TestSetAccountProject(t *testing.T) {
	dir := t.TempDir()
	st, err := sqlitestore.New(filepath.Join(dir, "t.db"))
	if err != nil {
		t.Fatal(err)
	}
	loader := config.NewLoader(filepath.Join(dir, "config.yaml"))
	cm := service.NewConfigManager(loader)
	pm := service.NewProjectManager(st, dir)
	bridge := testWeixinBridge(st, pm, cm)

	proj, err := pm.Create(context.Background(), domain.CreateProjectRequest{Name: "A"})
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	if err := st.WeixinAccounts().Upsert(context.Background(), domain.WeixinAccount{
		AccountID: "bot1", Token: "tok", BaseURL: ilink.DefaultBaseURL, CreatedAt: now, UpdatedAt: now,
	}); err != nil {
		t.Fatal(err)
	}
	acc, err := bridge.SetAccountProject(context.Background(), "bot1", proj.ID)
	if err != nil {
		t.Fatal(err)
	}
	if acc.ProjectID != proj.ID {
		t.Fatalf("%+v", acc)
	}
	acc, err = bridge.SetAccountProject(context.Background(), "bot1", "")
	if err != nil {
		t.Fatal(err)
	}
	if acc.ProjectID != "" {
		t.Fatalf("expected unbound, got %+v", acc)
	}
}

func TestWeixinLoginConfirmed(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.Contains(r.URL.Path, "get_bot_qrcode"):
			_ = json.NewEncoder(w).Encode(map[string]string{
				"qrcode":             "qr1",
				"qrcode_img_content": "https://liteapp.weixin.qq.com/q/demo?qrcode=abc",
			})
		case strings.Contains(r.URL.Path, "get_qrcode_status"):
			_ = json.NewEncoder(w).Encode(map[string]any{
				"status":        "confirmed",
				"bot_token":     "token-1",
				"ilink_bot_id":  "bot@im.bot",
				"ilink_user_id": "user-1",
				"baseurl":       r.Host,
			})
		default:
			w.WriteHeader(404)
		}
	}))
	defer srv.Close()

	dir := t.TempDir()
	st, err := sqlitestore.New(filepath.Join(dir, "t.db"))
	if err != nil {
		t.Fatal(err)
	}
	loader := config.NewLoader(filepath.Join(dir, "config.yaml"))
	cm := service.NewConfigManager(loader)
	pm := service.NewProjectManager(st, dir)
	bridge := testWeixinBridge(st, pm, cm)
	client := ilink.NewClient()
	client.HTTP = srv.Client()
	client.BaseURL = srv.URL
	bridge.SetClient(client)

	proj, err := pm.Create(context.Background(), domain.CreateProjectRequest{Name: "Biz"})
	if err != nil {
		t.Fatal(err)
	}

	if _, err := bridge.StartLogin(context.Background(), ""); err == nil {
		t.Fatal("expected error without projectId")
	}

	start, err := bridge.StartLogin(context.Background(), proj.ID)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(start.QRCodeURL, "data:image/png;base64,") {
		t.Fatalf("expected generated png data url, got %q", start.QRCodeURL[:min(40, len(start.QRCodeURL))])
	}
	wait, err := bridge.WaitLogin(context.Background(), start.SessionKey, "", 5000)
	if err != nil {
		t.Fatal(err)
	}
	if !wait.Connected || wait.AccountID == "" {
		t.Fatalf("%+v", wait)
	}
	if wait.ProjectID != proj.ID {
		t.Fatalf("wait projectId=%q want %q", wait.ProjectID, proj.ID)
	}
	acc, err := st.WeixinAccounts().Get(context.Background(), wait.AccountID)
	if err != nil {
		t.Fatal(err)
	}
	if acc.Token != "token-1" {
		t.Fatalf("token=%q", acc.Token)
	}
	if acc.ProjectID != proj.ID {
		t.Fatalf("account projectId=%q want %q", acc.ProjectID, proj.ID)
	}
}
