package feishu

import (
	"testing"

	"danmo-work/core/port"
)

func TestLooksLikeMarkdown(t *testing.T) {
	if !looksLikeMarkdown("line1\nline2") {
		t.Fatal("multiline should look like markdown")
	}
	if looksLikeMarkdown("plain") {
		t.Fatal("plain should not")
	}
}

func TestResolveReceiveDefaults(t *testing.T) {
	a := &Adapter{}
	in := &port.InboundMessage{PeerID: "ou_x", ChatID: "oc_y", Meta: map[string]string{
		"receive_id":   "oc_y",
		"receive_type": "chat_id",
	}}
	id, typ := a.resolveReceive(in)
	if id != "oc_y" || typ != "chat_id" {
		t.Fatalf("got %s %s", id, typ)
	}
	in2 := &port.InboundMessage{PeerID: "ou_only"}
	id, typ = a.resolveReceive(in2)
	if id != "ou_only" || typ != "open_id" {
		t.Fatalf("peer fallback: %s %s", id, typ)
	}
}
