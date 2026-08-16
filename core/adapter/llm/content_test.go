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

func TestAnthropicToolResult_WithImage(t *testing.T) {
	out := anthropicUserContent(port.ChatMessage{
		Role:       "tool",
		ToolCallID: "call_1",
		Content:    "Read image",
		Parts: []port.ChatContentPart{{
			Type: "image", MimeType: "image/png", Data: "abc",
		}},
	})
	arr, ok := out.([]map[string]any)
	if !ok || len(arr) != 2 {
		t.Fatalf("expected 2 content blocks, got %#v", out)
	}
	if arr[0]["type"] != "text" || arr[1]["type"] != "image" {
		t.Fatalf("unexpected blocks: %#v", arr)
	}
}

func TestOpenAIToolOutput_WithImage(t *testing.T) {
	out := openaiToolOutput(port.ChatMessage{
		Role:       "tool",
		ToolCallID: "call_1",
		Content:    "Read image",
		Parts: []port.ChatContentPart{{
			Type: "image", MimeType: "image/gif", Data: "gifdata",
		}},
	})
	arr, ok := out.([]map[string]any)
	if !ok || len(arr) != 2 {
		t.Fatalf("expected 2 content blocks, got %#v", out)
	}
	if arr[0]["type"] != "input_text" {
		t.Fatalf("first block: %#v", arr[0])
	}
	img, _ := arr[1]["image_url"].(map[string]any)
	if img["url"] != "data:image/gif;base64,gifdata" {
		t.Fatalf("url: %#v", img)
	}
}

func TestOpenAIToolOutput_TextOnly(t *testing.T) {
	out := openaiToolOutput(port.ChatMessage{
		Role:       "tool",
		ToolCallID: "call_1",
		Content:    "done",
	})
	if out != "done" {
		t.Fatalf("expected plain string, got %#v", out)
	}
}
