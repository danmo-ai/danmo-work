package llm

import (
	"fmt"

	"danmo-work/core/port"
)

// anthropicUserContent builds Anthropic Messages API content for a user/assistant text+image message.
func anthropicUserContent(m port.ChatMessage) any {
	if len(m.Parts) == 0 {
		return m.Content
	}
	var contents []map[string]any
	if m.Content != "" {
		contents = append(contents, map[string]any{"type": "text", "text": m.Content})
	}
	for _, p := range m.Parts {
		if p.Type != "image" || p.Data == "" {
			continue
		}
		mime := p.MimeType
		if mime == "" {
			mime = "image/png"
		}
		contents = append(contents, map[string]any{
			"type": "image",
			"source": map[string]any{
				"type":       "base64",
				"media_type": mime,
				"data":       p.Data,
			},
		})
	}
	if len(contents) == 0 {
		return m.Content
	}
	return contents
}

// openaiToolOutput builds a function_call_output value for the Responses API,
// including image parts from tool results.
func openaiToolOutput(m port.ChatMessage) any {
	if len(m.Parts) == 0 {
		return m.Content
	}
	var contents []map[string]any
	if m.Content != "" {
		contents = append(contents, map[string]any{"type": "input_text", "text": m.Content})
	}
	for _, p := range m.Parts {
		if p.Type != "image" || p.Data == "" {
			continue
		}
		mime := p.MimeType
		if mime == "" {
			mime = "image/png"
		}
		contents = append(contents, map[string]any{
			"type": "input_image",
			"image_url": map[string]any{
				"url": fmt.Sprintf("data:%s;base64,%s", mime, p.Data),
			},
		})
	}
	if len(contents) == 0 {
		return m.Content
	}
	return contents
}

// openaiUserContent builds OpenAI-compatible multimodal content (image_url data URLs).
func openaiUserContent(m port.ChatMessage) any {
	if len(m.Parts) == 0 {
		return m.Content
	}
	var contents []map[string]any
	if m.Content != "" {
		contents = append(contents, map[string]any{"type": "text", "text": m.Content})
	}
	for _, p := range m.Parts {
		if p.Type != "image" || p.Data == "" {
			continue
		}
		mime := p.MimeType
		if mime == "" {
			mime = "image/png"
		}
		contents = append(contents, map[string]any{
			"type": "image_url",
			"image_url": map[string]any{
				"url": fmt.Sprintf("data:%s;base64,%s", mime, p.Data),
			},
		})
	}
	if len(contents) == 0 {
		return m.Content
	}
	return contents
}
