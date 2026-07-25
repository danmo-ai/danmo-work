package qq

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
)

const (
	tokenURL   = "https://bots.qq.com/app/getAppAccessToken"
	apiBase    = "https://api.sgroup.qq.com"
	defaultUA  = "DanmoWork/qqbot"
)

// Adapter talks to QQ Bot OpenAPI (token + REST sends). Inbound is Gateway WS.
type Adapter struct {
	mu     sync.Mutex
	cfg    domain.ConfigQQChannel
	client *http.Client
	token  string
	expiry time.Time
}

func NewAdapter(cfg domain.ConfigQQChannel) *Adapter {
	return &Adapter{
		cfg:    cfg,
		client: &http.Client{Timeout: 30 * time.Second},
	}
}

func (a *Adapter) UpdateConfig(cfg domain.ConfigQQChannel) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.cfg = cfg
	a.token = ""
	a.expiry = time.Time{}
}

func (a *Adapter) config() domain.ConfigQQChannel {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.cfg
}

// NativeC2CStreamEnabled reports whether C2C stream_messages should be used (default true).
func (a *Adapter) NativeC2CStreamEnabled() bool {
	return a.config().QQNativeC2CStreamEnabled()
}

func (a *Adapter) AccountID() string {
	cfg := a.config()
	if cfg.AppID != "" {
		return cfg.AppID
	}
	return "qq-default"
}

func (a *Adapter) accessToken(ctx context.Context) (string, error) {
	a.mu.Lock()
	if a.token != "" && time.Now().Before(a.expiry) {
		tok := a.token
		a.mu.Unlock()
		return tok, nil
	}
	cfg := a.cfg
	a.mu.Unlock()
	if cfg.AppID == "" || cfg.ClientSecret == "" {
		return "", fmt.Errorf("qq: appId/clientSecret required")
	}
	payload, _ := json.Marshal(map[string]string{
		"appId":        cfg.AppID,
		"clientSecret": cfg.ClientSecret,
	})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, tokenURL, bytes.NewReader(payload))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := a.client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	var out struct {
		AccessToken string `json:"access_token"`
		ExpiresIn   any    `json:"expires_in"`
		Message     string `json:"message"`
		Code        int    `json:"code"`
	}
	if err := json.Unmarshal(body, &out); err != nil {
		return "", err
	}
	if out.AccessToken == "" {
		return "", fmt.Errorf("qq token: code=%d msg=%s body=%s", out.Code, out.Message, string(body))
	}
	exp := 7200
	switch v := out.ExpiresIn.(type) {
	case float64:
		exp = int(v)
	case string:
		fmt.Sscanf(v, "%d", &exp)
	}
	if exp <= 0 {
		exp = 7200
	}
	a.mu.Lock()
	a.token = out.AccessToken
	a.expiry = time.Now().Add(time.Duration(exp-60) * time.Second)
	a.mu.Unlock()
	return out.AccessToken, nil
}

func (a *Adapter) authHeader(ctx context.Context) (string, error) {
	tok, err := a.accessToken(ctx)
	if err != nil {
		return "", err
	}
	return "QQBot " + tok, nil
}

func (a *Adapter) GatewayURL(ctx context.Context) (string, error) {
	auth, err := a.authHeader(ctx)
	if err != nil {
		return "", err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, apiBase+"/gateway", nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", auth)
	resp, err := a.client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	var out struct {
		URL string `json:"url"`
	}
	if err := json.Unmarshal(body, &out); err != nil {
		return "", err
	}
	if strings.TrimSpace(out.URL) == "" {
		return "", fmt.Errorf("qq gateway: empty url: %s", string(body))
	}
	return out.URL, nil
}

func (a *Adapter) doJSON(ctx context.Context, method, path string, payload any) ([]byte, error) {
	auth, err := a.authHeader(ctx)
	if err != nil {
		return nil, err
	}
	var reader io.Reader
	if payload != nil {
		b, _ := json.Marshal(payload)
		reader = bytes.NewReader(b)
	}
	req, err := http.NewRequestWithContext(ctx, method, apiBase+path, reader)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", auth)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", defaultUA)
	resp, err := a.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 300 {
		return body, fmt.Errorf("qq api %s %s: HTTP %d: %s", method, path, resp.StatusCode, string(body))
	}
	return body, nil
}

type sendResult struct {
	MessageID string
	StreamID  string
}

func (a *Adapter) SendC2CMessage(ctx context.Context, openID string, body map[string]any) (sendResult, error) {
	openID = strings.TrimSpace(openID)
	if openID == "" {
		return sendResult{}, fmt.Errorf("qq send: openid required")
	}
	raw, err := a.doJSON(ctx, http.MethodPost, "/v2/users/"+openID+"/messages", body)
	if err != nil {
		return sendResult{}, err
	}
	var out struct {
		ID string `json:"id"`
	}
	_ = json.Unmarshal(raw, &out)
	return sendResult{MessageID: out.ID}, nil
}

func (a *Adapter) SendGroupMessage(ctx context.Context, groupOpenID string, body map[string]any) (sendResult, error) {
	groupOpenID = strings.TrimSpace(groupOpenID)
	if groupOpenID == "" {
		return sendResult{}, fmt.Errorf("qq send: group openid required")
	}
	raw, err := a.doJSON(ctx, http.MethodPost, "/v2/groups/"+groupOpenID+"/messages", body)
	if err != nil {
		return sendResult{}, err
	}
	var out struct {
		ID string `json:"id"`
	}
	_ = json.Unmarshal(raw, &out)
	return sendResult{MessageID: out.ID}, nil
}

func (a *Adapter) StreamC2C(ctx context.Context, openID string, body map[string]any) (sendResult, error) {
	openID = strings.TrimSpace(openID)
	if openID == "" {
		return sendResult{}, fmt.Errorf("qq stream: openid required")
	}
	raw, err := a.doJSON(ctx, http.MethodPost, "/v2/users/"+openID+"/stream_messages", body)
	if err != nil {
		return sendResult{}, err
	}
	var out struct {
		ID string `json:"id"`
	}
	_ = json.Unmarshal(raw, &out)
	return sendResult{MessageID: out.ID, StreamID: out.ID}, nil
}

func (a *Adapter) AckInteraction(ctx context.Context, interactionID string) error {
	interactionID = strings.TrimSpace(interactionID)
	if interactionID == "" {
		return nil
	}
	_, err := a.doJSON(ctx, http.MethodPut, "/interactions/"+interactionID, map[string]any{"code": 0})
	return err
}

