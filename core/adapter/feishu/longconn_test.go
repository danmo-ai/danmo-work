package feishu_test

import (
	"testing"

	"danmo-work/core/adapter/feishu"
	"danmo-work/core/port"

	larkim "github.com/larksuite/oapi-sdk-go/v3/service/im/v1"
)

func TestOpenAPIBase(t *testing.T) {
	if got := feishu.OpenAPIBase(""); got != "https://open.feishu.cn/open-apis" {
		t.Fatalf("feishu default: %s", got)
	}
	if got := feishu.OpenAPIBase("lark"); got != "https://open.larksuite.com/open-apis" {
		t.Fatalf("lark: %s", got)
	}
}

func strPtr(s string) *string { return &s }

func TestInboundFromP2Message(t *testing.T) {
	event := &larkim.P2MessageReceiveV1{
		Event: &larkim.P2MessageReceiveV1Data{
			Message: &larkim.EventMessage{
				MessageId:   strPtr("om_1"),
				ChatId:      strPtr("oc_chat"),
				MessageType: strPtr("text"),
				Content:     strPtr(`{"text":"hi ws"}`),
			},
			Sender: &larkim.EventSender{
				SenderId: &larkim.UserId{
					OpenId: strPtr("ou_user"),
				},
			},
		},
	}
	msg := feishu.InboundFromP2Message("cli_x", event)
	if msg == nil {
		t.Fatal("nil msg")
	}
	if msg.Type != port.ChannelFeishu || msg.AccountID != "cli_x" {
		t.Fatalf("%+v", msg)
	}
	if msg.Text != "hi ws" || msg.PeerID != "ou_user" || msg.ChatID != "oc_chat" {
		t.Fatalf("%+v", msg)
	}
	if msg.Meta["receive_id"] != "oc_chat" {
		t.Fatalf("meta=%v", msg.Meta)
	}
}

func TestInboundFromP2MessageNonText(t *testing.T) {
	event := &larkim.P2MessageReceiveV1{
		Event: &larkim.P2MessageReceiveV1Data{
			Message: &larkim.EventMessage{
				MessageType: strPtr("image"),
				Content:     strPtr(`{"image_key":"x"}`),
				ChatId:      strPtr("oc_chat"),
			},
			Sender: &larkim.EventSender{
				SenderId: &larkim.UserId{OpenId: strPtr("ou_user")},
			},
		},
	}
	if msg := feishu.InboundFromP2Message("cli_x", event); msg != nil {
		t.Fatalf("expected nil, got %+v", msg)
	}
}
