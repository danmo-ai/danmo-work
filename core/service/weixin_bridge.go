package service

import (
	"context"
	"encoding/base64"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	"danmo-work/core/adapter/weixin/ilink"
	"danmo-work/core/domain"
	"danmo-work/core/port"

	qrcode "github.com/skip2/go-qrcode"
)

type WeixinBridge struct {
	client   *ilink.Client
	store    port.Repository
	sessions *SessionManager
	projects *ProjectManager
	config   *ConfigManager
	ingress  port.ChannelIngress

	mu       sync.Mutex
	running  bool
	cancel   context.CancelFunc
	wg       sync.WaitGroup
	logins   map[string]*weixinLoginSession
	cfgCache domain.ConfigWeixinChannel
}

type weixinLoginSession struct {
	SessionKey string
	QRCode     string
	QRCodeURL  string
	ProjectID  string
	StartedAt  time.Time
	BaseURL    string
}

func NewWeixinBridge(store port.Repository, sessions *SessionManager, projects *ProjectManager, config *ConfigManager, ingress port.ChannelIngress) *WeixinBridge {
	b := &WeixinBridge{
		client:   ilink.NewClient(),
		store:    store,
		sessions: sessions,
		projects: projects,
		config:   config,
		ingress:  ingress,
		logins:   make(map[string]*weixinLoginSession),
	}
	if ingress != nil {
		ingress.RegisterEndpoint(NewWeixinEndpoint(b))
	}
	return b
}

func (b *WeixinBridge) Type() port.ChannelType { return port.ChannelWeixin }

// SetClient replaces the iLink HTTP client (tests).
func (b *WeixinBridge) SetClient(c *ilink.Client) {
	if c != nil {
		b.client = c
	}
}

func (b *WeixinBridge) IsRunning() bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.running
}

func (b *WeixinBridge) loadChannelCfg(ctx context.Context) (domain.ConfigWeixinChannel, error) {
	cfg, err := b.config.Get(ctx)
	if err != nil {
		return domain.ConfigWeixinChannel{}, err
	}
	return cfg.Channels.Weixin, nil
}

const weixinMigrateMetaKey = "weixin_account_project_v1"

// MigrateAccountProjectsOnce copies deprecated channels.weixin.default_project_id
// onto accounts missing project_id, then clears the config field (idempotent).
func (b *WeixinBridge) MigrateAccountProjectsOnce(ctx context.Context) error {
	if _, ok, err := b.store.AppMeta().Get(ctx, weixinMigrateMetaKey); err != nil {
		return err
	} else if ok {
		return nil
	}
	cfg, err := b.config.Get(ctx)
	if err != nil {
		return err
	}
	legacy := strings.TrimSpace(cfg.Channels.Weixin.DefaultProjectID)
	if legacy != "" {
		accounts, err := b.store.WeixinAccounts().List(ctx)
		if err != nil {
			return err
		}
		for _, a := range accounts {
			if strings.TrimSpace(a.ProjectID) != "" {
				continue
			}
			if err := b.store.WeixinAccounts().UpdateProjectID(ctx, a.AccountID, legacy); err != nil {
				return err
			}
		}
		wx := cfg.Channels.Weixin
		wx.DefaultProjectID = ""
		sec := cfg.Channels
		sec.Weixin = wx
		if _, err := b.config.Update(ctx, domain.UpdateConfigFileRequest{Channels: &sec}); err != nil {
			return err
		}
		log.Printf("[weixin] migrated default_project_id=%s onto accounts", legacy)
	}
	return b.store.AppMeta().Set(ctx, weixinMigrateMetaKey, "1")
}

// SyncFromConfig starts or stops the bridge based on config.
func (b *WeixinBridge) SyncFromConfig(ctx context.Context) error {
	_ = b.MigrateAccountProjectsOnce(ctx)
	cfg, err := b.loadChannelCfg(ctx)
	if err != nil {
		return err
	}
	b.mu.Lock()
	b.cfgCache = cfg
	b.mu.Unlock()
	if cfg.Enabled {
		if cfg.DefaultAgentID == "" {
			b.Stop()
			return fmt.Errorf("channels.weixin.default_agent_id required when enabled")
		}
		if strings.TrimSpace(cfg.DefaultModelID) == "" || !strings.Contains(cfg.DefaultModelID, "/") {
			b.Stop()
			return fmt.Errorf("channels.weixin.default_model_id required when enabled (provider/model)")
		}
		// Always restart so newly logged-in accounts get a monitor goroutine.
		b.Stop()
		return b.Start(ctx)
	}
	b.Stop()
	return nil
}

func (b *WeixinBridge) Start(ctx context.Context) error {
	b.mu.Lock()
	if b.running {
		b.mu.Unlock()
		return nil
	}
	runCtx, cancel := context.WithCancel(context.Background())
	b.cancel = cancel
	b.running = true
	b.mu.Unlock()

	accounts, err := b.store.WeixinAccounts().List(ctx)
	if err != nil {
		b.Stop()
		return err
	}
	for _, acc := range accounts {
		if strings.TrimSpace(acc.Token) == "" {
			continue
		}
		acc := acc
		b.wg.Add(1)
		go func() {
			defer b.wg.Done()
			b.monitorAccount(runCtx, acc)
		}()
	}
	log.Printf("[weixin] bridge started with %d account(s)", len(accounts))
	return nil
}

func (b *WeixinBridge) Stop() {
	b.mu.Lock()
	if !b.running {
		b.mu.Unlock()
		return
	}
	if b.cancel != nil {
		b.cancel()
	}
	b.running = false
	b.mu.Unlock()
	b.wg.Wait()
	log.Printf("[weixin] bridge stopped")
}

func (b *WeixinBridge) refreshCfg(ctx context.Context) {
	if cfg, err := b.loadChannelCfg(ctx); err == nil {
		b.mu.Lock()
		b.cfgCache = cfg
		b.mu.Unlock()
	}
}

func (b *WeixinBridge) toILinkAccount(a domain.WeixinAccount) ilink.Account {
	base := a.BaseURL
	if base == "" {
		base = ilink.DefaultBaseURL
	}
	return ilink.Account{
		AccountID: a.AccountID,
		Token:     a.Token,
		BaseURL:   base,
		UserID:    a.UserID,
	}
}

func (b *WeixinBridge) monitorAccount(ctx context.Context, acc domain.WeixinAccount) {
	ilAcc := b.toILinkAccount(acc)
	_ = b.client.NotifyStart(ctx, ilAcc)
	defer func() { _ = b.client.NotifyStop(context.Background(), ilAcc) }()

	syncBuf := acc.SyncBuf
	timeout := 35 * time.Second
	failures := 0
	for {
		if ctx.Err() != nil {
			return
		}
		b.refreshCfg(ctx)
		resp, err := b.client.GetUpdates(ctx, ilAcc, syncBuf, timeout)
		if ctx.Err() != nil {
			return
		}
		if err != nil {
			failures++
			delay := 2 * time.Second
			if failures >= 3 {
				delay = 30 * time.Second
				failures = 0
			}
			log.Printf("[weixin] getupdates account=%s err=%v", acc.AccountID, err)
			select {
			case <-ctx.Done():
				return
			case <-time.After(delay):
			}
			continue
		}
		failures = 0
		if resp.LongPollingTimeoutMs > 0 {
			timeout = time.Duration(resp.LongPollingTimeoutMs) * time.Millisecond
		}
		if resp.Ret != 0 || resp.ErrCode != 0 {
			failures++
			select {
			case <-ctx.Done():
				return
			case <-time.After(2 * time.Second):
			}
			continue
		}
		if resp.GetUpdatesBuf != "" {
			syncBuf = resp.GetUpdatesBuf
			_ = b.store.WeixinAccounts().UpdateSyncBuf(ctx, acc.AccountID, syncBuf)
		}
		for _, msg := range resp.Msgs {
			if ctx.Err() != nil {
				return
			}
			if msg.MessageType == ilink.MessageTypeBot {
				continue
			}
			peer := strings.TrimSpace(msg.FromUserID)
			if peer == "" {
				continue
			}
			text := ilink.TextFromMessage(msg)
			media := b.downloadInboundMedia(ctx, acc.AccountID, fmt.Sprintf("%v", msg.MessageID), msg)
			if text == "" && len(media) == 0 {
				_ = b.client.SendText(ctx, ilAcc, peer, "未能识别该消息内容（可发送文字、图片或文件）。", msg.ContextToken, "")
				continue
			}
			meta := map[string]string{}
			if msg.ContextToken != "" {
				meta["context_token"] = msg.ContextToken
				_ = b.store.WeixinBindings().UpdateContextToken(ctx, acc.AccountID, peer, msg.ContextToken)
			}
			inbound := port.InboundMessage{
				Type:      port.ChannelWeixin,
				AccountID: acc.AccountID,
				PeerID:    peer,
				ChatID:    peer,
				Text:      text,
				MessageID: fmt.Sprintf("%v", msg.MessageID),
				Meta:      meta,
				Media:     media,
			}
			reply, err := b.ingress.HandleInbound(ctx, inbound)
			if err != nil {
				log.Printf("[weixin] handle inbound peer=%s: %v", peer, err)
				reply = "处理消息时出错：" + err.Error()
			}
			// Ingress delivers via WeixinEndpoint when registered; reply is a fallback.
			if strings.TrimSpace(reply) == "" {
				continue
			}
			ctxTok := msg.ContextToken
			if binding, berr := b.store.WeixinBindings().GetByPeer(ctx, acc.AccountID, peer); berr == nil && binding.ContextToken != "" {
				ctxTok = binding.ContextToken
			}
			if err := b.client.SendText(ctx, ilAcc, peer, reply, ctxTok, ""); err != nil {
				log.Printf("[weixin] send reply peer=%s: %v", peer, err)
			}
		}
	}
}

// downloadInboundMedia pulls CDN attachments into data/channels/weixin/...
func (b *WeixinBridge) downloadInboundMedia(ctx context.Context, accountID, messageID string, msg ilink.Message) []port.InboundMedia {
	refs := ilink.CollectMediaRefs(msg)
	if len(refs) == 0 || b.client == nil {
		return nil
	}
	out := make([]port.InboundMedia, 0, len(refs))
	for _, ref := range refs {
		data, err := b.client.DownloadMediaRef(ctx, ref)
		if err != nil {
			log.Printf("[weixin] media download kind=%s name=%s: %v", ref.Kind, ref.Name, err)
			continue
		}
		path, err := SaveChannelMedia(port.ChannelWeixin, accountID, messageID, ref.Name, data)
		if err != nil {
			log.Printf("[weixin] media save kind=%s: %v", ref.Kind, err)
			continue
		}
		out = append(out, port.InboundMedia{
			Name:     ref.Name,
			Path:     path,
			MimeType: ref.MimeType,
			Kind:     ref.Kind,
			Key:      ref.Media.EncryptQueryParam,
		})
	}
	return out
}

// activateAfterLogin restarts monitors when the channel is already fully configured.
func (b *WeixinBridge) activateAfterLogin(ctx context.Context) string {
	cfg, err := b.config.Get(ctx)
	if err != nil {
		return "已添加微信账号。请在设置中选择 Agent 与模型并启用通道。"
	}
	wx := cfg.Channels.Weixin
	if wx.Enabled && wx.DefaultAgentID != "" && strings.Contains(wx.DefaultModelID, "/") {
		if err := b.SyncFromConfig(ctx); err != nil {
			return "已添加微信账号，但启动 Bridge 失败：" + err.Error()
		}
		return ""
	}
	return "已添加微信账号。请在设置 → 微信中选择默认 Agent 与模型并启用通道。"
}

// --- Login API helpers ---

func (b *WeixinBridge) StartLogin(ctx context.Context, projectID string) (domain.WeixinLoginStartResult, error) {
	projectID = strings.TrimSpace(projectID)
	if projectID == "" {
		return domain.WeixinLoginStartResult{}, fmt.Errorf("请先选择要关联的项目")
	}
	if b.projects == nil {
		return domain.WeixinLoginStartResult{}, fmt.Errorf("项目服务不可用")
	}
	if _, err := b.projects.Get(ctx, projectID); err != nil {
		return domain.WeixinLoginStartResult{}, fmt.Errorf("项目不存在或无法读取（%s）: %w", projectID, err)
	}
	accounts, _ := b.store.WeixinAccounts().List(ctx)
	tokens := make([]string, 0, len(accounts))
	for i := len(accounts) - 1; i >= 0 && len(tokens) < 10; i-- {
		if t := strings.TrimSpace(accounts[i].Token); t != "" {
			tokens = append(tokens, t)
		}
	}
	if b.client == nil {
		return domain.WeixinLoginStartResult{}, fmt.Errorf("微信客户端未初始化")
	}
	qr, err := b.client.GetBotQRCode(ctx, ilink.DefaultBotType, tokens)
	if err != nil {
		return domain.WeixinLoginStartResult{}, err
	}
	if qr.QRCode == "" || qr.QRCodeImgContent == "" {
		return domain.WeixinLoginStartResult{}, fmt.Errorf("服务器未返回二维码")
	}
	qrImage := qr.QRCodeImgContent
	if !strings.HasPrefix(qrImage, "data:image/") {
		payload := qr.QRCodeImgContent
		if payload == "" {
			payload = qr.QRCode
		}
		png, err := qrcode.Encode(payload, qrcode.Medium, 256)
		if err != nil {
			return domain.WeixinLoginStartResult{}, fmt.Errorf("生成二维码图片失败: %w", err)
		}
		qrImage = "data:image/png;base64," + base64.StdEncoding.EncodeToString(png)
	}
	sessionKey := fmt.Sprintf("wxlogin-%d", time.Now().UnixNano())
	base := ilink.DefaultBaseURL
	if b.client != nil && strings.TrimSpace(b.client.BaseURL) != "" {
		base = b.client.BaseURL
	}
	b.mu.Lock()
	b.logins[sessionKey] = &weixinLoginSession{
		SessionKey: sessionKey,
		QRCode:     qr.QRCode,
		QRCodeURL:  qr.QRCodeImgContent,
		ProjectID:  projectID,
		StartedAt:  time.Now(),
		BaseURL:    base,
	}
	b.mu.Unlock()
	return domain.WeixinLoginStartResult{
		SessionKey: sessionKey,
		QRCodeURL:  qrImage,
	}, nil
}

func (b *WeixinBridge) WaitLogin(ctx context.Context, sessionKey, verifyCode string, timeoutMs int) (domain.WeixinLoginWaitResult, error) {
	b.mu.Lock()
	login := b.logins[sessionKey]
	b.mu.Unlock()
	if login == nil {
		return domain.WeixinLoginWaitResult{Connected: false, Message: "当前没有进行中的登录，请先发起登录。"}, nil
	}
	if time.Since(login.StartedAt) > 5*time.Minute {
		b.mu.Lock()
		delete(b.logins, sessionKey)
		b.mu.Unlock()
		return domain.WeixinLoginWaitResult{Connected: false, Message: "二维码已过期，请重新生成。"}, nil
	}
	if timeoutMs <= 0 {
		timeoutMs = 120_000
	}
	deadline := time.Now().Add(time.Duration(timeoutMs) * time.Millisecond)
	baseURL := login.BaseURL
	if baseURL == "" {
		baseURL = ilink.DefaultBaseURL
	}
	for time.Now().Before(deadline) {
		if ctx.Err() != nil {
			return domain.WeixinLoginWaitResult{}, ctx.Err()
		}
		status, err := b.client.GetQRCodeStatus(ctx, baseURL, login.QRCode, verifyCode)
		if err != nil {
			time.Sleep(time.Second)
			continue
		}
		switch status.Status {
		case "wait", "scaned":
			// keep polling
		case "need_verifycode":
			return domain.WeixinLoginWaitResult{
				Connected:       false,
				NeedsVerifyCode: true,
				Message:         "请输入手机微信显示的验证码。",
			}, nil
		case "expired":
			b.mu.Lock()
			delete(b.logins, sessionKey)
			b.mu.Unlock()
			return domain.WeixinLoginWaitResult{Connected: false, Message: "二维码已过期，请重新生成。"}, nil
		case "verify_code_blocked":
			b.mu.Lock()
			delete(b.logins, sessionKey)
			b.mu.Unlock()
			return domain.WeixinLoginWaitResult{Connected: false, Message: "多次输入错误，连接流程已停止。"}, nil
		case "binded_redirect":
			b.mu.Lock()
			delete(b.logins, sessionKey)
			b.mu.Unlock()
			return domain.WeixinLoginWaitResult{
				Connected:        true,
				AlreadyConnected: true,
				Message:          "已连接过此应用，无需重复连接。",
			}, nil
		case "scaned_but_redirect":
			if status.RedirectHost != "" {
				baseURL = "https://" + status.RedirectHost
				b.mu.Lock()
				if l := b.logins[sessionKey]; l != nil {
					l.BaseURL = baseURL
				}
				b.mu.Unlock()
			}
		case "confirmed":
			accountID := ilink.NormalizeAccountID(status.ILinkBotID)
			token := strings.TrimSpace(status.BotToken)
			if accountID == "" || token == "" {
				b.mu.Lock()
				delete(b.logins, sessionKey)
				b.mu.Unlock()
				return domain.WeixinLoginWaitResult{Connected: false, Message: "登录失败：服务器未返回完整账号信息。"}, nil
			}
			base := status.BaseURL
			if base == "" {
				base = ilink.DefaultBaseURL
			}
			projectID := strings.TrimSpace(login.ProjectID)
			now := time.Now().UTC()
			acc := domain.WeixinAccount{
				AccountID: accountID,
				Token:     token,
				BaseURL:   base,
				UserID:    status.ILinkUserID,
				ProjectID: projectID,
				CreatedAt: now,
				UpdatedAt: now,
			}
			if existing, err := b.store.WeixinAccounts().Get(ctx, accountID); err == nil {
				acc.SyncBuf = existing.SyncBuf
				acc.CreatedAt = existing.CreatedAt
				if projectID == "" {
					acc.ProjectID = existing.ProjectID
				}
			}
			if err := b.store.WeixinAccounts().Upsert(ctx, acc); err != nil {
				return domain.WeixinLoginWaitResult{}, err
			}
			b.mu.Lock()
			delete(b.logins, sessionKey)
			b.mu.Unlock()
			msg := b.activateAfterLogin(ctx)
			if msg == "" {
				msg = "已添加微信账号并绑定项目。可在微信中给机器人发消息。"
			}
			return domain.WeixinLoginWaitResult{
				Connected: true,
				AccountID: accountID,
				UserID:    status.ILinkUserID,
				ProjectID: acc.ProjectID,
				Message:   msg,
			}, nil
		}
		time.Sleep(time.Second)
	}
	return domain.WeixinLoginWaitResult{Connected: false, Message: "等待扫码超时，请重试。"}, nil
}

func (b *WeixinBridge) SetAccountProject(ctx context.Context, accountID, projectID string) (domain.WeixinAccount, error) {
	accountID = strings.TrimSpace(accountID)
	if accountID == "" {
		return domain.WeixinAccount{}, fmt.Errorf("accountId required")
	}
	if _, err := b.store.WeixinAccounts().Get(ctx, accountID); err != nil {
		return domain.WeixinAccount{}, err
	}
	projectID = strings.TrimSpace(projectID)
	if projectID != "" {
		if _, err := b.projects.Get(ctx, projectID); err != nil {
			return domain.WeixinAccount{}, fmt.Errorf("项目不存在：%s", projectID)
		}
	}
	if err := b.store.WeixinAccounts().UpdateProjectID(ctx, accountID, projectID); err != nil {
		return domain.WeixinAccount{}, err
	}
	acc, err := b.store.WeixinAccounts().Get(ctx, accountID)
	if err != nil {
		return domain.WeixinAccount{}, err
	}
	acc.Token = ""
	return acc, nil
}

func (b *WeixinBridge) Logout(ctx context.Context, accountID string) error {
	stopOne := func(a domain.WeixinAccount) {
		if strings.TrimSpace(a.Token) != "" {
			_ = b.client.NotifyStop(ctx, b.toILinkAccount(a))
		}
		_ = b.store.WeixinBindings().DeleteByAccount(ctx, a.AccountID)
		_ = b.store.WeixinAccounts().Delete(ctx, a.AccountID)
	}
	if accountID == "" {
		accounts, err := b.store.WeixinAccounts().List(ctx)
		if err != nil {
			return err
		}
		for _, a := range accounts {
			stopOne(a)
		}
	} else {
		a, err := b.store.WeixinAccounts().Get(ctx, accountID)
		if err != nil {
			return err
		}
		stopOne(a)
	}
	wasRunning := b.IsRunning()
	b.Stop()
	if wasRunning {
		cfg, _ := b.loadChannelCfg(ctx)
		if cfg.Enabled {
			_ = b.Start(ctx)
		}
	}
	return nil
}

func (b *WeixinBridge) Status(ctx context.Context) (domain.WeixinStatus, error) {
	_ = b.MigrateAccountProjectsOnce(ctx)
	cfg, err := b.loadChannelCfg(ctx)
	if err != nil {
		return domain.WeixinStatus{}, err
	}
	accounts, err := b.store.WeixinAccounts().List(ctx)
	if err != nil {
		return domain.WeixinStatus{}, err
	}
	safe := make([]domain.WeixinAccount, 0, len(accounts))
	for _, a := range accounts {
		a.Token = ""
		safe = append(safe, a)
	}
	n, _ := b.store.WeixinBindings().Count(ctx)
	return domain.WeixinStatus{
		Enabled:        cfg.Enabled,
		Running:        b.IsRunning(),
		DefaultAgentID: cfg.DefaultAgentID,
		DefaultModelID: cfg.DefaultModelID,
		AutoApprove:    cfg.AutoApprove,
		Accounts:       safe,
		BindingCount:   n,
	}, nil
}

func (b *WeixinBridge) ListBindings(ctx context.Context) ([]domain.WeixinBinding, error) {
	return b.store.WeixinBindings().List(ctx)
}
