package feishu

import (
	"testing"

	"danmo-work/core/domain"
)

func TestBuildAskFormCard(t *testing.T) {
	card := BuildAskFormCard("确认", "请填写", []domain.AskUserFormField{
		{Name: "title", Label: "标题", Type: "text", Required: true},
		{Name: "ok", Label: "同意", Type: "boolean"},
		{Name: "color", Label: "颜色", Type: "select", Options: []string{"红", "蓝"}},
	}, "dw|a|ask1|form")
	if card["schema"] != "2.0" {
		t.Fatalf("schema=%v", card["schema"])
	}
	body, _ := card["body"].(map[string]any)
	els, _ := body["elements"].([]any)
	if len(els) == 0 {
		t.Fatal("no elements")
	}
}
