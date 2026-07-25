package feishu

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"

	"danmo-work/core/domain"
	"danmo-work/core/port"

	lark "github.com/larksuite/oapi-sdk-go/v3"
	larkcore "github.com/larksuite/oapi-sdk-go/v3/core"
	"github.com/larksuite/oapi-sdk-go/v3/event/dispatcher"
	"github.com/larksuite/oapi-sdk-go/v3/event/dispatcher/callback"
	larkim "github.com/larksuite/oapi-sdk-go/v3/service/im/v1"
	larkws "github.com/larksuite/oapi-sdk-go/v3/ws"
)

// InboundHandler is called for each normalized inbound Feishu message.
type InboundHandler func(ctx context.Context, msg port.InboundMessage) error

// CardActionHandler handles card.action.trigger callbacks (dw|… token already extracted).
// formValue carries schema 2.0 form field values when the user submitted a form.
type CardActionHandler func(ctx context.Context, msg port.InboundMessage, token string, formValue map[string]any) error

// LongConn runs the Feishu outbound WebSocket event client (no public URL).
type LongConn struct {
	cfg      domain.ConfigFeishuChannel
	onMsg    InboundHandler
	onCard   CardActionHandler
	account  string
}

func NewLongConn(cfg domain.ConfigFeishuChannel, onMsg InboundHandler) *LongConn {
	acc := strings.TrimSpace(cfg.AppID)
	if acc == "" {
		acc = "feishu-default"
	}
	return &LongConn{cfg: cfg, onMsg: onMsg, account: acc}
}

// WithCardAction sets the interactive card callback handler.
func (lc *LongConn) WithCardAction(h CardActionHandler) *LongConn {
	lc.onCard = h
	return lc
}

// OpenAPIBase returns the Open API host for the configured domain.
func OpenAPIBase(domainHint string) string {
	if domainIsLark(domainHint) {
		return lark.LarkBaseUrl + "/open-apis"
	}
	return lark.FeishuBaseUrl + "/open-apis"
}

// Run blocks until ctx is cancelled or the client exits with error.
func (lc *LongConn) Run(ctx context.Context) error {
	cfg := lc.cfg
	if cfg.AppID == "" || cfg.AppSecret == "" {
		return fmt.Errorf("feishu websocket: appId/appSecret required")
	}
	handler := dispatcher.NewEventDispatcher("", "").
		OnP2MessageReceiveV1(func(ctx context.Context, event *larkim.P2MessageReceiveV1) error {
			msg := InboundFromP2Message(lc.account, event)
			if msg == nil || lc.onMsg == nil {
				return nil
			}
			if err := lc.onMsg(ctx, *msg); err != nil {
				log.Printf("[feishu] inbound handler: %v", err)
			}
			return nil
		}).
		OnP2CardActionTrigger(func(ctx context.Context, event *callback.CardActionTriggerEvent) (*callback.CardActionTriggerResponse, error) {
			if lc.onCard == nil || event == nil || event.Event == nil {
				return &callback.CardActionTriggerResponse{}, nil
			}
			msg := InboundFromCardAction(lc.account, event)
			token := ""
			var formValue map[string]any
			if event.Event.Action != nil {
				token = CallbackTokenFromActionValue(event.Event.Action.Value)
				if event.Event.Action.FormValue != nil {
					formValue = event.Event.Action.FormValue
				}
			}
			if err := lc.onCard(ctx, msg, token, formValue); err != nil {
				log.Printf("[feishu] card action: %v", err)
				return &callback.CardActionTriggerResponse{
					Toast: &callback.Toast{Type: "error", Content: "处理失败"},
				}, nil
			}
			return &callback.CardActionTriggerResponse{
				Toast: &callback.Toast{Type: "info", Content: "已处理"},
			}, nil
		})

	opts := []larkws.ClientOption{
		larkws.WithEventHandler(handler),
		larkws.WithLogLevel(larkcore.LogLevelInfo),
	}
	if domainIsLark(cfg.Domain) {
		opts = append(opts, larkws.WithDomain(lark.LarkBaseUrl))
	}

	cli := larkws.NewClient(cfg.AppID, cfg.AppSecret, opts...)
	log.Printf("[feishu] websocket starting app=%s domain=%s", cfg.AppID, feishuDomainLabel(cfg.Domain))
	return cli.Start(ctx)
}

func domainIsLark(d string) bool {
	d = strings.ToLower(strings.TrimSpace(d))
	return d == "lark" || d == "larksuite" || strings.Contains(d, "larksuite.com")
}

func feishuDomainLabel(d string) string {
	if domainIsLark(d) {
		return "lark"
	}
	return "feishu"
}

// InboundFromP2Message converts a Feishu SDK message-receive event into InboundMessage.
func InboundFromP2Message(accountID string, event *larkim.P2MessageReceiveV1) *port.InboundMessage {
	if event == nil || event.Event == nil || event.Event.Message == nil {
		return nil
	}
	m := event.Event.Message
	msgType := ""
	if m.MessageType != nil {
		msgType = *m.MessageType
	}
	content := ""
	if m.Content != nil {
		content = *m.Content
	}
	text := extractTextContent(content, msgType)
	media := extractMediaDescriptors(content, msgType)
	if text == "" && len(media) == 0 {
		return nil
	}
	peer := ""
	if event.Event.Sender != nil && event.Event.Sender.SenderId != nil {
		if event.Event.Sender.SenderId.OpenId != nil {
			peer = *event.Event.Sender.SenderId.OpenId
		} else if event.Event.Sender.SenderId.UserId != nil {
			peer = *event.Event.Sender.SenderId.UserId
		}
	}
	chatID := ""
	if m.ChatId != nil {
		chatID = *m.ChatId
	}
	messageID := ""
	if m.MessageId != nil {
		messageID = *m.MessageId
	}
	threadID := ""
	if m.RootId != nil {
		threadID = *m.RootId
	} else if m.ParentId != nil {
		threadID = *m.ParentId
	}
	if peer == "" && chatID == "" {
		return nil
	}
	if peer == "" {
		peer = chatID
	}
	return &port.InboundMessage{
		Type:      port.ChannelFeishu,
		AccountID: accountID,
		PeerID:    peer,
		ChatID:    chatID,
		ThreadID:  threadID,
		Text:      text,
		MessageID: messageID,
		Media:     media,
		Meta: map[string]string{
			"chat_id":      chatID,
			"message_id":   messageID,
			"receive_id":   chatID,
			"receive_type": "chat_id",
		},
	}
}

func extractMediaDescriptors(contentJSON, msgType string) []port.InboundMedia {
	msgType = strings.ToLower(strings.TrimSpace(msgType))
	switch msgType {
	case "image":
		var c struct {
			ImageKey string `json:"image_key"`
		}
		if json.Unmarshal([]byte(contentJSON), &c) != nil || c.ImageKey == "" {
			return nil
		}
		return []port.InboundMedia{{Kind: "image", Key: c.ImageKey, Name: c.ImageKey + ".png", MimeType: "image/png"}}
	case "file", "audio", "media":
		var c struct {
			FileKey  string `json:"file_key"`
			FileName string `json:"file_name"`
		}
		if json.Unmarshal([]byte(contentJSON), &c) != nil || c.FileKey == "" {
			return nil
		}
		kind := "file"
		if msgType == "audio" {
			kind = "audio"
		}
		name := c.FileName
		if name == "" {
			name = c.FileKey
		}
		return []port.InboundMedia{{Kind: kind, Key: c.FileKey, Name: name}}
	default:
		return nil
	}
}

// InboundFromCardAction builds a peer context from a card action trigger.
func InboundFromCardAction(accountID string, event *callback.CardActionTriggerEvent) port.InboundMessage {
	peer := ""
	chatID := ""
	messageID := ""
	if event != nil && event.Event != nil {
		if event.Event.Operator != nil && event.Event.Operator.OpenID != "" {
			peer = event.Event.Operator.OpenID
		}
		if event.Event.Context != nil {
			chatID = event.Event.Context.OpenChatID
			messageID = event.Event.Context.OpenMessageID
		}
	}
	if peer == "" {
		peer = chatID
	}
	return port.InboundMessage{
		Type:      port.ChannelFeishu,
		AccountID: accountID,
		PeerID:    peer,
		ChatID:    chatID,
		MessageID: messageID,
		Meta: map[string]string{
			"chat_id":      chatID,
			"message_id":   messageID,
			"receive_id":   chatID,
			"receive_type": "chat_id",
		},
	}
}

// DebugJSON is used in tests.
func DebugJSON(v any) string {
	b, _ := json.Marshal(v)
	return string(b)
}
