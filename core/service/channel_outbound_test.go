package service

import (
	"strings"
	"testing"

	"danmo-work/core/port"
)

func TestFormatAskTextAndResolve(t *testing.T) {
	ask := port.AskPrompt{
		AskID:      "a1",
		Question:   "选一个？",
		Options:    []string{"苹果", "香蕉"},
		DefaultOpt: "苹果",
	}
	text := formatAskText(ask)
	for _, part := range []string{"选一个？", "1. 苹果", "2. 香蕉", "默认：苹果"} {
		if !strings.Contains(text, part) {
			t.Fatalf("ask text missing %q in %q", part, text)
		}
	}
	if got := resolveAskAnswer("2", ask.Options); got != "香蕉" {
		t.Fatalf("index resolve: got %q", got)
	}
	if got := resolveAskAnswer("苹果", ask.Options); got != "苹果" {
		t.Fatalf("label resolve: got %q", got)
	}
	if got := resolveAskAnswer("其他", ask.Options); got != "其他" {
		t.Fatalf("free text: got %q", got)
	}
}

func TestPreferOutboundKind(t *testing.T) {
	rich := port.ChannelCapabilities{RichCards: true}
	plain := port.ChannelCapabilities{}
	if preferOutboundKind(rich, port.OutboundKindCard) != port.OutboundKindCard {
		t.Fatal("rich should keep card")
	}
	if preferOutboundKind(plain, port.OutboundKindCard) != port.OutboundKindText {
		t.Fatal("plain should degrade card to text")
	}
	if preferOutboundKind(plain, port.OutboundKindMarkdown) != port.OutboundKindText {
		t.Fatal("plain should degrade markdown to text")
	}
}

func TestEndpointCapabilitiesDiffer(t *testing.T) {
	fs := NewFeishuEndpoint(nil)
	wx := NewWeixinEndpoint(nil)
	wc := NewWecomEndpoint()
	if !fs.Capabilities().RichCards {
		t.Fatal("feishu should support rich cards")
	}
	if wx.Capabilities().RichCards || wc.Capabilities().RichCards {
		t.Fatal("weixin/wecom should not claim rich cards")
	}
	if !fs.Capabilities().ProgressiveStream || !wx.Capabilities().ProgressiveStream || !wc.Capabilities().ProgressiveStream {
		t.Fatal("all three should support progressive stream (native or emulated)")
	}
	if !fs.Capabilities().InteractiveAsk || !wx.Capabilities().InteractiveAsk || !wc.Capabilities().InteractiveAsk {
		t.Fatal("all three should support interactive ask (card or text menu)")
	}
}
