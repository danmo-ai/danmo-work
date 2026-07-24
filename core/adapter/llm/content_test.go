package llm

import (
	"testing"

	"danmo-work/core/port"
)

func TestAnthropicUserContent_WithImage(t *testing.T) {
	out := anthropicUserContent(port.ChatMessage{
		Role:    "user",
		Content: "what is this?",
		Parts: []port.ChatContentPart{{
			Type: "image", MimeType: "image/png", Data: "abc",
		}},
	})
	arr, ok := out.([]map[string]any)
	if !ok || len(arr) != 2 {
		t.Fatalf("expected 2 content blocks, got %#v", out)
	}
	if arr[0]["type"] != "text" {
		t.Fatalf("first block: %#v", arr[0])
	}
	if arr[1]["type"] != "image" {
		t.Fatalf("second block: %#v", arr[1])
	}
	src, _ := arr[1]["source"].(map[string]any)
	if src["data"] != "abc" || src["media_type"] != "image/png" {
		t.Fatalf("source: %#v", src)
	}
}

func TestOpenAIUserContent_WithImage(t *testing.T) {
	out := openaiUserContent(port.ChatMessage{
		Role:    "user",
		Content: "look",
		Parts: []port.ChatContentPart{{
			Type: "image", MimeType: "image/jpeg", Data: "xyz",
		}},
	})
	arr, ok := out.([]map[string]any)
	if !ok || len(arr) != 2 {
		t.Fatalf("expected 2 content blocks, got %#v", out)
	}
	img, _ := arr[1]["image_url"].(map[string]any)
	if img["url"] != "data:image/jpeg;base64,xyz" {
		t.Fatalf("url: %#v", img)
	}
}
