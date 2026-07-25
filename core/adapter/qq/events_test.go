package qq

import (
	"encoding/json"
	"testing"

	"danmo-work/core/port"
)

func TestNormalizeC2C(t *testing.T) {
	raw := json.RawMessage(`{"id":"m1","content":"hello","author":{"user_openid":"u1"}}`)
	msg, inter, _ := NormalizeDispatch("app1", "C2C_MESSAGE_CREATE", raw)
	if inter != nil || msg == nil {
		t.Fatal("expected inbound only")
	}
	if msg.Type != port.ChannelQQ || msg.PeerID != "u1" || msg.Text != "hello" {
		t.Fatalf("got %+v", msg)
	}
}

func TestNormalizeInteractionResolvedTypo(t *testing.T) {
	raw := json.RawMessage(`{"id":"i1","chat_type":2,"user_openid":"u1","data":{"resoloved":{"button_data":"dw|p|apr1|once","button_id":"b1"}}}`)
	msg, inter, id := NormalizeDispatch("app1", "INTERACTION_CREATE", raw)
	if msg != nil || inter == nil || id != "i1" {
		t.Fatalf("msg=%v inter=%v id=%s", msg, inter, id)
	}
	if inter.Raw != "dw|p|apr1|once" || inter.PeerID != "u1" {
		t.Fatalf("got %+v", inter)
	}
}

func TestBuildKeyboard(t *testing.T) {
	kb := BuildKeyboard([]port.OutboundAction{
		{ID: "dw|p|a|once", Label: "允许一次"},
		{ID: "dw|p|a|deny", Label: "拒绝"},
	})
	if kb == nil {
		t.Fatal("expected keyboard")
	}
}
