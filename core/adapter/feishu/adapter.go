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

// RichProgressEnabled reports whether interactive progress cards are enabled (default true).
func (a *Adapter) RichProgressEnabled() bool {
	return a.config().FeishuRichProgressEnabled()
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

// DownloadMessageResource downloads an image/file resource attached to a message.
// resourceType is "image" or "file".
func (a *Adapter) DownloadMessageResource(ctx context.Context, messageID, fileKey, resourceType string) ([]byte, error) {
	messageID = strings.TrimSpace(messageID)
	fileKey = strings.TrimSpace(fileKey)
	resourceType = strings.TrimSpace(resourceType)
	if messageID == "" || fileKey == "" {
		return nil, fmt.Errorf("feishu download: message_id and file_key required")
	}
	if resourceType == "" {
		resourceType = "file"
	}
	token, err := a.tenantToken(ctx)
	if err != nil {
		return nil, err
	}
	url := fmt.Sprintf("%s/im/v1/messages/%s/resources/%s?type=%s",
		OpenAPIBase(a.config().Domain), messageID, fileKey, resourceType)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := a.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 32<<20))
	if resp.StatusCode >= 300 {
		return nil, fmt.Errorf("feishu download: HTTP %d: %s", resp.StatusCode, string(body))
	}
	return body, nil
}

// EnrichInboundMedia downloads Feishu image/file keys into local channel media paths.
func (a *Adapter) EnrichInboundMedia(ctx context.Context, msg *port.InboundMessage) error {
	if a == nil || msg == nil || len(msg.Media) == 0 {
		return nil
	}
	for i := range msg.Media {
		m := &msg.Media[i]
		if m.Path != "" || m.Key == "" || msg.MessageID == "" {
			continue
		}
		resType := "file"
		if m.Kind == "image" {
			resType = "image"
		}
		data, err := a.DownloadMessageResource(ctx, msg.MessageID, m.Key, resType)
		if err != nil {
			return err
		}
		// Save via service helper would create import cycle; write here with same layout.
		path, err := saveFeishuMedia(msg.AccountID, msg.MessageID, m.Name, data)
		if err != nil {
			return err
		}
		m.Path = path
	}
	return nil
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
