package wecom

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"

	"danmo-work/core/domain"
	"danmo-work/core/port"

	"github.com/gorilla/websocket"
)

const defaultWSURL = "wss://openws.work.weixin.qq.com"

// InboundHandler is called for each normalized inbound WeCom text message.
// The LongConn already sent a stream placeholder; the handler should call
// ReplyStreamFinish with the final content (same streamID / reqID in msg.Meta).
type InboundHandler func(ctx context.Context, msg port.InboundMessage) error

// LongConn is the WeCom AI Bot outbound WebSocket client.
type LongConn struct {
	cfg     domain.ConfigWecomChannel
	onMsg   InboundHandler
	account string

	mu   sync.Mutex
	conn *websocket.Conn
}

func NewLongConn(cfg domain.ConfigWecomChannel, onMsg InboundHandler) *LongConn {
	acc := strings.TrimSpace(cfg.BotID)
	if acc == "" {
		acc = "wecom-default"
	}
	return &LongConn{cfg: cfg, onMsg: onMsg, account: acc}
}

func (lc *LongConn) wsURL() string {
	if u := strings.TrimSpace(lc.cfg.WSURL); u != "" {
		return u
	}
	return defaultWSURL
}

// Run blocks until ctx is cancelled or the connection fails after subscribe.
func (lc *LongConn) Run(ctx context.Context) error {
	cfg := lc.cfg
	if cfg.BotID == "" || cfg.Secret == "" {
		return fmt.Errorf("wecom websocket: botId/secret required")
	}
	dialer := websocket.Dialer{HandshakeTimeout: 15 * time.Second}
	conn, _, err := dialer.DialContext(ctx, lc.wsURL(), http.Header{})
	if err != nil {
		return fmt.Errorf("wecom dial: %w", err)
	}
	lc.mu.Lock()
	lc.conn = conn
	lc.mu.Unlock()
	defer func() {
		lc.mu.Lock()
		if lc.conn != nil {
			_ = lc.conn.Close()
			lc.conn = nil
		}
		lc.mu.Unlock()
	}()

	reqID := newReqID()
	if err := lc.writeJSON(frame{
		Cmd:     "aibot_subscribe",
		Headers: frameHeaders{ReqID: reqID},
		Body: map[string]string{
			"bot_id": cfg.BotID,
			"secret": cfg.Secret,
		},
	}); err != nil {
		return err
	}
	_ = conn.SetReadDeadline(time.Now().Add(20 * time.Second))
	var subResp ackFrame
	if err := conn.ReadJSON(&subResp); err != nil {
		return fmt.Errorf("wecom subscribe read: %w", err)
	}
	if subResp.ErrCode != 0 {
		return fmt.Errorf("wecom subscribe: code=%d msg=%s", subResp.ErrCode, subResp.ErrMsg)
	}
	_ = conn.SetReadDeadline(time.Time{})
	log.Printf("[wecom] websocket subscribed bot=%s", cfg.BotID)

	pingStop := make(chan struct{})
	defer close(pingStop)
	go lc.heartbeat(ctx, pingStop)

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		_ = conn.SetReadDeadline(time.Now().Add(90 * time.Second))
		var raw json.RawMessage
		if err := conn.ReadJSON(&raw); err != nil {
			return fmt.Errorf("wecom read: %w", err)
		}
		var env envelope
		if err := json.Unmarshal(raw, &env); err != nil {
			continue
		}
		switch env.Cmd {
		case "aibot_msg_callback":
			msg := InboundFromCallback(lc.account, env)
			if msg == nil {
				continue
			}
			streamID := newReqID()
			msg.Meta["stream_id"] = streamID
			// Must respond within ~5s: send placeholder first.
			_ = lc.ReplyStream(msg.Meta["req_id"], streamID, "正在处理…", false)
			if lc.onMsg != nil {
				m := *msg
				go func() {
					defer func() {
						if r := recover(); r != nil {
							log.Printf("[wecom] inbound panic: %v", r)
						}
					}()
					if err := lc.onMsg(context.Background(), m); err != nil {
						log.Printf("[wecom] inbound handler: %v", err)
					}
				}()
			}
		case "aibot_event_callback", "":
			// Ignore events / ack frames for v1.
		default:
			// pong / other acks
		}
	}
}

func (lc *LongConn) heartbeat(ctx context.Context, stop <-chan struct{}) {
	t := time.NewTicker(30 * time.Second)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-stop:
			return
		case <-t.C:
			_ = lc.writeJSON(frame{
				Cmd:     "ping",
				Headers: frameHeaders{ReqID: newReqID()},
			})
		}
	}
}

// ReplyStream sends aibot_respond_msg (stream). content is full replacement text.
func (lc *LongConn) ReplyStream(reqID, streamID, content string, finish bool) error {
	if strings.TrimSpace(reqID) == "" || strings.TrimSpace(streamID) == "" {
		return fmt.Errorf("wecom reply: req_id and stream_id required")
	}
	return lc.writeJSON(frame{
		Cmd:     "aibot_respond_msg",
		Headers: frameHeaders{ReqID: reqID},
		Body: map[string]any{
			"msgtype": "stream",
			"stream": map[string]any{
				"id":      streamID,
				"finish":  finish,
				"content": content,
			},
		},
	})
}

func (lc *LongConn) writeJSON(v any) error {
	lc.mu.Lock()
	defer lc.mu.Unlock()
	if lc.conn == nil {
		return fmt.Errorf("wecom: not connected")
	}
	_ = lc.conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
	return lc.conn.WriteJSON(v)
}

type frame struct {
	Cmd     string       `json:"cmd,omitempty"`
	Headers frameHeaders `json:"headers"`
	Body    any          `json:"body,omitempty"`
}

type frameHeaders struct {
	ReqID string `json:"req_id"`
}

type ackFrame struct {
	Headers frameHeaders `json:"headers"`
	ErrCode int          `json:"errcode"`
	ErrMsg  string       `json:"errmsg"`
}

type envelope struct {
	Cmd     string          `json:"cmd"`
	Headers frameHeaders    `json:"headers"`
	Body    json.RawMessage `json:"body"`
}

type callbackBody struct {
	MsgID    string `json:"msgid"`
	AIBotID  string `json:"aibotid"`
	ChatID   string `json:"chatid"`
	ChatType string `json:"chattype"`
	From     struct {
		UserID string `json:"userid"`
	} `json:"from"`
	MsgType string `json:"msgtype"`
	Text    struct {
		Content string `json:"content"`
	} `json:"text"`
	Voice struct {
		Content string `json:"content"`
	} `json:"voice"`
}

// InboundFromCallback converts an aibot_msg_callback envelope into InboundMessage.
func InboundFromCallback(accountID string, env envelope) *port.InboundMessage {
	if env.Cmd != "aibot_msg_callback" {
		return nil
	}
	var body callbackBody
	if err := json.Unmarshal(env.Body, &body); err != nil {
		return nil
	}
	text := ""
	switch body.MsgType {
	case "text":
		text = strings.TrimSpace(body.Text.Content)
		text = stripAtMention(text)
	case "voice":
		text = strings.TrimSpace(body.Voice.Content)
	default:
		return nil
	}
	if text == "" {
		return nil
	}
	peer := strings.TrimSpace(body.From.UserID)
	chatID := strings.TrimSpace(body.ChatID)
	if body.ChatType == "group" && chatID != "" {
		peer = chatID
	}
	if peer == "" {
		peer = chatID
	}
	if peer == "" {
		return nil
	}
	acc := accountID
	if body.AIBotID != "" {
		acc = body.AIBotID
	}
	return &port.InboundMessage{
		Type:      port.ChannelWecom,
		AccountID: acc,
		PeerID:    peer,
		ChatID:    chatID,
		Text:      text,
		MessageID: body.MsgID,
		Meta: map[string]string{
			"req_id":    env.Headers.ReqID,
			"chat_id":   chatID,
			"chat_type": body.ChatType,
			"userid":    body.From.UserID,
			"msgid":     body.MsgID,
		},
	}
}

func stripAtMention(s string) string {
	// "@Name content" or "@Name content" (WeCom may use special spaces)
	if !strings.HasPrefix(s, "@") {
		return s
	}
	parts := strings.Fields(s)
	if len(parts) <= 1 {
		return ""
	}
	return strings.TrimSpace(strings.Join(parts[1:], " "))
}

func newReqID() string {
	return fmt.Sprintf("%d-%d", time.Now().UnixNano(), time.Now().Unix()%1000)
}
