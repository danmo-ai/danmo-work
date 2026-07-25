package feishu

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"danmo-work/core/domain"
	"danmo-work/core/port"
)

// Adapter sends Feishu Open API replies (inbound is WebSocket LongConn).
type Adapter struct {
	mu     sync.Mutex
	cfg    domain.ConfigFeishuChannel
	client *http.Client
	token  string
	expiry time.Time
}

func NewAdapter(cfg domain.ConfigFeishuChannel) *Adapter {
	return &Adapter{
		cfg:    cfg,
		client: &http.Client{Timeout: 30 * time.Second},
	}
}

func (a *Adapter) Type() port.ChannelType { return port.ChannelFeishu }

func (a *Adapter) UpdateConfig(cfg domain.ConfigFeishuChannel) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.cfg = cfg
	a.token = ""
	a.expiry = time.Time{}
}

func (a *Adapter) config() domain.ConfigFeishuChannel {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.cfg
}

func (a *Adapter) AccountID() string {
	cfg := a.config()
	if cfg.AppID != "" {
		return cfg.AppID
	}
	return "feishu-default"
}

func extractTextContent(contentJSON, msgType string) string {
	if msgType != "" && msgType != "text" {
		return ""
	}
	var c struct {
		Text string `json:"text"`
	}
	if json.Unmarshal([]byte(contentJSON), &c) == nil {
		return strings.TrimSpace(c.Text)
	}
	return strings.TrimSpace(contentJSON)
}

func (a *Adapter) SendReply(ctx context.Context, in *port.InboundMessage, reply port.OutboundReply) error {
	_, err := a.SendTextMessage(ctx, in, reply.Content)
	return err
}

func (a *Adapter) tenantToken(ctx context.Context) (string, error) {
	a.mu.Lock()
	if a.token != "" && time.Now().Before(a.expiry) {
		tok := a.token
		a.mu.Unlock()
		return tok, nil
	}
	cfg := a.cfg
	a.mu.Unlock()

	if cfg.AppID == "" || cfg.AppSecret == "" {
		return "", fmt.Errorf("feishu: appId/appSecret required")
	}
	payload, _ := json.Marshal(map[string]string{
		"app_id":     cfg.AppID,
		"app_secret": cfg.AppSecret,
	})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, OpenAPIBase(cfg.Domain)+"/auth/v3/tenant_access_token/internal", bytes.NewReader(payload))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json; charset=utf-8")
	resp, err := a.client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	var out struct {
		Code              int    `json:"code"`
		Msg               string `json:"msg"`
		TenantAccessToken string `json:"tenant_access_token"`
		Expire            int    `json:"expire"`
	}
	if err := json.Unmarshal(body, &out); err != nil {
		return "", err
	}
	if out.Code != 0 || out.TenantAccessToken == "" {
		return "", fmt.Errorf("feishu token: code=%d msg=%s", out.Code, out.Msg)
	}
	a.mu.Lock()
	a.token = out.TenantAccessToken
	exp := out.Expire
	if exp <= 0 {
		exp = 7200
	}
	a.expiry = time.Now().Add(time.Duration(exp-60) * time.Second)
	a.mu.Unlock()
	return out.TenantAccessToken, nil
}

func mustJSON(v any) []byte {
	b, _ := json.Marshal(v)
	return b
}
