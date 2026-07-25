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
	header, _ := card["header"].(map[string]any)
	if header["template"] != "green" {
		t.Fatalf("expected green template for 授权, got %v", header["template"])
	}
}

func TestBuildProgressCardWithActions(t *testing.T) {
	card := BuildProgressCard("等待授权", "agent text", []string{"✓ read"}, []port.OutboundAction{
		{ID: "dw|p|apr1|once", Label: "允许一次"},
		{ID: "dw|p|apr1|deny", Label: "拒绝"},
	})
	body, _ := card["body"].(map[string]any)
	els, _ := body["elements"].([]any)
	if len(els) < 2 {
		t.Fatalf("expected markdown + buttons, got %d", len(els))
	}
	header, _ := card["header"].(map[string]any)
	if header["template"] != "orange" {
		t.Fatalf("template=%v", header["template"])
	}
}

func TestCallbackTokenFromActionValue(t *testing.T) {
	tok := CallbackTokenFromActionValue(map[string]any{"dw": "dw|a|x|y"})
	if tok != "dw|a|x|y" {
		t.Fatalf("got %q", tok)
	}
}
