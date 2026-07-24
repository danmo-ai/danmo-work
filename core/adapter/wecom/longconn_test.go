package wecom

import (
	"encoding/json"
	"testing"

	"danmo-work/core/port"
)

func TestInboundFromCallbackText(t *testing.T) {
	raw := []byte(`{
		"cmd": "aibot_msg_callback",
		"headers": {"req_id": "req-1"},
		"body": {
			"msgid": "m1",
			"aibotid": "bot-x",
			"chattype": "single",
			"from": {"userid": "u1"},
			"msgtype": "text",
			"text": {"content": "@Bot hello wecom"}
		}
	}`)
	var env envelope
	if err := json.Unmarshal(raw, &env); err != nil {
		t.Fatal(err)
	}
	msg := InboundFromCallback("bot-x", env)
	if msg == nil {
		t.Fatal("nil")
	}
	if msg.Type != port.ChannelWecom || msg.PeerID != "u1" || msg.Text != "hello wecom" {
		t.Fatalf("%+v", msg)
	}
	if msg.Meta["req_id"] != "req-1" {
		t.Fatalf("meta=%v", msg.Meta)
	}
}

func TestInboundFromCallbackGroup(t *testing.T) {
	raw := []byte(`{
		"cmd": "aibot_msg_callback",
		"headers": {"req_id": "req-2"},
		"body": {
			"msgid": "m2",
			"aibotid": "bot-x",
			"chatid": "wrxxx",
			"chattype": "group",
			"from": {"userid": "u2"},
			"msgtype": "text",
			"text": {"content": "hi"}
		}
	}`)
	var env envelope
	if err := json.Unmarshal(raw, &env); err != nil {
		t.Fatal(err)
	}
	msg := InboundFromCallback("bot-x", env)
	if msg == nil || msg.PeerID != "wrxxx" {
		t.Fatalf("%+v", msg)
	}
}

func TestInboundFromCallbackNonText(t *testing.T) {
	raw := []byte(`{
		"cmd": "aibot_msg_callback",
		"headers": {"req_id": "req-3"},
		"body": {"msgtype": "image", "from": {"userid": "u1"}}
	}`)
	var env envelope
	_ = json.Unmarshal(raw, &env)
	if msg := InboundFromCallback("bot-x", env); msg != nil {
		t.Fatalf("expected nil: %+v", msg)
	}
}

func TestStripAtMention(t *testing.T) {
	if got := stripAtMention("@Bot hello"); got != "hello" {
		t.Fatalf("%q", got)
	}
	if got := stripAtMention("plain"); got != "plain" {
		t.Fatalf("%q", got)
	}
}
