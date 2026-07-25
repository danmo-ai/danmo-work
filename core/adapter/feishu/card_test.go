package feishu

import (
	"testing"

	"danmo-work/core/port"
)

func TestBuildInteractiveCard(t *testing.T) {
	card := BuildInteractiveCard("工具授权", "允许执行？", []port.OutboundAction{
		{ID: "dw|p|a|once", Label: "允许一次"},
		{ID: "dw|p|a|deny", Label: "拒绝"},
	})
	if card["schema"] != "2.0" {
		t.Fatalf("schema=%v", card["schema"])
	}
	body, _ := card["body"].(map[string]any)
	els, _ := body["elements"].([]any)
	if len(els) < 2 {
		t.Fatalf("elements=%d", len(els))
	}
}

func TestCallbackTokenFromActionValue(t *testing.T) {
	tok := CallbackTokenFromActionValue(map[string]any{"dw": "dw|a|x|y"})
	if tok != "dw|a|x|y" {
		t.Fatalf("got %q", tok)
	}
}
