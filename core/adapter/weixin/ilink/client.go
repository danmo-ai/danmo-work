package ilink

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// Client talks to the Weixin iLink Bot HTTP API.
type Client struct {
	HTTP       *http.Client
	BaseURL    string // defaults to DefaultBaseURL
	AppID      string
	AppVersion string // semver for iLink-App-ClientVersion + base_info.channel_version
	BotAgent   string
}

func NewClient() *Client {
	return &Client{
		HTTP:       &http.Client{},
		BaseURL:    DefaultBaseURL,
		AppID:      DefaultAppID,
		AppVersion: DefaultChannelVer,
		BotAgent:   DefaultBotAgent,
	}
}

func BuildClientVersion(version string) int {
	parts := strings.Split(version, ".")
	parse := func(i int) int {
		if i >= len(parts) {
			return 0
		}
		n, _ := strconv.Atoi(parts[i])
		if n < 0 {
			n = 0
		}
		return n & 0xff
	}
	return (parse(0) << 16) | (parse(1) << 8) | parse(2)
}

func (c *Client) baseInfo() BaseInfo {
	ver := c.AppVersion
	if ver == "" {
		ver = DefaultChannelVer
	}
	agent := c.BotAgent
	if agent == "" {
		agent = DefaultBotAgent
	}
	return BaseInfo{ChannelVersion: ver, BotAgent: agent}
}

func (c *Client) commonHeaders() http.Header {
	h := make(http.Header)
	appID := c.AppID
	if appID == "" {
		appID = DefaultAppID
	}
	ver := c.AppVersion
	if ver == "" {
		ver = DefaultChannelVer
	}
	h.Set("iLink-App-Id", appID)
	h.Set("iLink-App-ClientVersion", strconv.Itoa(BuildClientVersion(ver)))
	return h
}

func (c *Client) authHeaders(token string) http.Header {
	h := c.commonHeaders()
	h.Set("Content-Type", "application/json")
	h.Set("AuthorizationType", "ilink_bot_token")
	h.Set("X-WECHAT-UIN", randomWechatUIN())
	if t := strings.TrimSpace(token); t != "" {
		h.Set("Authorization", "Bearer "+t)
	}
	return h
}

func randomWechatUIN() string {
	var buf [4]byte
	_, _ = rand.Read(buf[:])
	n := binary.BigEndian.Uint32(buf[:])
	return base64.StdEncoding.EncodeToString([]byte(strconv.FormatUint(uint64(n), 10)))
}

func ensureBaseURL(base string) string {
	base = strings.TrimSpace(base)
	if base == "" {
		base = DefaultBaseURL
	}
	if !strings.HasSuffix(base, "/") {
		base += "/"
	}
	return base
}

func (c *Client) doJSON(ctx context.Context, method, base, endpoint string, headers http.Header, body any, timeout time.Duration) ([]byte, error) {
	u, err := url.Parse(ensureBaseURL(base) + strings.TrimPrefix(endpoint, "/"))
	if err != nil {
		return nil, err
	}
	var rdr io.Reader
	if body != nil {
		raw, err := json.Marshal(body)
		if err != nil {
			return nil, err
		}
		rdr = bytes.NewReader(raw)
	}
	reqCtx := ctx
	var cancel context.CancelFunc
	if timeout > 0 {
		reqCtx, cancel = context.WithTimeout(ctx, timeout)
		defer cancel()
	}
	req, err := http.NewRequestWithContext(reqCtx, method, u.String(), rdr)
	if err != nil {
		return nil, err
	}
	for k, vals := range headers {
		for _, v := range vals {
			req.Header.Add(k, v)
		}
	}
	httpClient := c.HTTP
	if httpClient == nil {
		httpClient = http.DefaultClient
	}
	res, err := httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	data, err := io.ReadAll(res.Body)
	if err != nil {
		return nil, err
	}
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		msg := strings.TrimSpace(string(data))
		if msg == "" {
			msg = res.Status
		}
		return nil, fmt.Errorf("ilink %s %s: %s", method, endpoint, msg)
	}
	return data, nil
}

func (c *Client) apiBase() string {
	if strings.TrimSpace(c.BaseURL) != "" {
		return c.BaseURL
	}
	return DefaultBaseURL
}

func (c *Client) GetBotQRCode(ctx context.Context, botType string, localTokens []string) (QRCodeResponse, error) {
	if botType == "" {
		botType = DefaultBotType
	}
	endpoint := "ilink/bot/get_bot_qrcode?bot_type=" + url.QueryEscape(botType)
	// Official iLink login uses GET. Some deployments also accept POST with
	// local_token_list; try GET first so a POST-only 404 "not found" cannot
	// block QR generation.
	raw, err := c.doJSON(ctx, http.MethodGet, c.apiBase(), endpoint, c.commonHeaders(), nil, 15*time.Second)
	if err != nil {
		body := map[string]any{"local_token_list": localTokens}
		if localTokens == nil {
			body["local_token_list"] = []string{}
		}
		raw, err = c.doJSON(ctx, http.MethodPost, c.apiBase(), endpoint, c.authHeaders(""), body, 15*time.Second)
		if err != nil {
			return QRCodeResponse{}, fmt.Errorf("获取微信二维码失败: %w", err)
		}
	}
	var out QRCodeResponse
	if err := json.Unmarshal(raw, &out); err != nil {
		return QRCodeResponse{}, fmt.Errorf("解析微信二维码响应失败: %w", err)
	}
	if out.QRCode == "" && out.QRCodeImgContent == "" {
		return QRCodeResponse{}, fmt.Errorf("微信服务器未返回二维码")
	}
	return out, nil
}

func (c *Client) GetQRCodeStatus(ctx context.Context, baseURL, qrcode, verifyCode string) (QRStatusResponse, error) {
	endpoint := "ilink/bot/get_qrcode_status?qrcode=" + url.QueryEscape(qrcode)
	if verifyCode != "" {
		endpoint += "&verify_code=" + url.QueryEscape(verifyCode)
	}
	raw, err := c.doJSON(ctx, http.MethodGet, baseURL, endpoint, c.commonHeaders(), nil, 35*time.Second)
	if err != nil {
		return QRStatusResponse{}, err
	}
	var out QRStatusResponse
	if err := json.Unmarshal(raw, &out); err != nil {
		return QRStatusResponse{}, err
	}
	return out, nil
}

func (c *Client) GetUpdates(ctx context.Context, account Account, getUpdatesBuf string, timeout time.Duration) (GetUpdatesResp, error) {
	if timeout <= 0 {
		timeout = 35 * time.Second
	}
	body := map[string]any{
		"get_updates_buf": getUpdatesBuf,
		"base_info":       c.baseInfo(),
	}
	raw, err := c.doJSON(ctx, http.MethodPost, account.BaseURL, "ilink/bot/getupdates", c.authHeaders(account.Token), body, timeout)
	if err != nil {
		if ctx.Err() != nil {
			return GetUpdatesResp{}, err
		}
		// Long-poll client timeout → empty success for retry.
		if strings.Contains(err.Error(), "context deadline exceeded") || strings.Contains(err.Error(), "Client.Timeout") {
			return GetUpdatesResp{Ret: 0, Msgs: nil, GetUpdatesBuf: getUpdatesBuf}, nil
		}
		return GetUpdatesResp{}, err
	}
	var out GetUpdatesResp
	if err := json.Unmarshal(raw, &out); err != nil {
		return GetUpdatesResp{}, err
	}
	return out, nil
}

func (c *Client) SendText(ctx context.Context, account Account, to, text, contextToken, clientID string) error {
	if clientID == "" {
		clientID = "dq-weixin-" + strconv.FormatInt(time.Now().UnixNano(), 10)
	}
	req := SendMessageReq{
		Msg: Message{
			FromUserID:   "",
			ToUserID:     to,
			ClientID:     clientID,
			MessageType:  MessageTypeBot,
			MessageState: MessageStateFinish,
			ItemList: []MessageItem{{
				Type:     MessageItemText,
				TextItem: &TextItem{Text: text},
			}},
			ContextToken: contextToken,
		},
		BaseInfo: c.baseInfo(),
	}
	_, err := c.doJSON(ctx, http.MethodPost, account.BaseURL, "ilink/bot/sendmessage", c.authHeaders(account.Token), req, 15*time.Second)
	return err
}

func (c *Client) NotifyStart(ctx context.Context, account Account) error {
	body := map[string]any{"base_info": c.baseInfo()}
	_, err := c.doJSON(ctx, http.MethodPost, account.BaseURL, "ilink/bot/msg/notifystart", c.authHeaders(account.Token), body, 10*time.Second)
	return err
}

func (c *Client) NotifyStop(ctx context.Context, account Account) error {
	body := map[string]any{"base_info": c.baseInfo()}
	_, err := c.doJSON(ctx, http.MethodPost, account.BaseURL, "ilink/bot/msg/notifystop", c.authHeaders(account.Token), body, 10*time.Second)
	return err
}

// TextFromMessage extracts plain text (or voice transcription) from a message.
func TextFromMessage(msg Message) string {
	for _, item := range msg.ItemList {
		if item.Type == MessageItemText && item.TextItem != nil {
			if t := strings.TrimSpace(item.TextItem.Text); t != "" {
				return t
			}
		}
		if item.Type == MessageItemVoice && item.VoiceItem != nil {
			if t := strings.TrimSpace(item.VoiceItem.Text); t != "" {
				return t
			}
		}
	}
	return ""
}

// NormalizeAccountID makes filesystem / DB safe ids from ilink_bot_id.
func NormalizeAccountID(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	replacer := strings.NewReplacer("@", "_", "/", "_", "\\", "_", ":", "_")
	return replacer.Replace(raw)
}
