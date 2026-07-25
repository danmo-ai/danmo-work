package feishu

import (
	"strings"

	"danmo-work/core/port"
)

// BuildInteractiveCard builds a Feishu schema 2.0 interactive card JSON object.
func BuildInteractiveCard(title, body string, actions []port.OutboundAction) map[string]any {
	title = strings.TrimSpace(title)
	if title == "" {
		title = "Danmo Work"
	}
	body = strings.TrimSpace(body)
	elements := []any{}
	if body != "" {
		elements = append(elements, map[string]any{
			"tag":     "markdown",
			"content": body,
		})
	}
	if len(actions) > 0 {
		cols := make([]any, 0, len(actions))
		for _, act := range actions {
			label := strings.TrimSpace(act.Label)
			if label == "" {
				label = act.ID
			}
			cb := strings.TrimSpace(act.ID)
			if cb == "" {
				cb = label
			}
			btnType := "default"
			lower := strings.ToLower(label)
			if strings.Contains(lower, "拒绝") || strings.Contains(lower, "deny") {
				btnType = "danger"
			} else if strings.Contains(lower, "允许") || strings.Contains(lower, "allow") {
				btnType = "primary"
			}
			cols = append(cols, map[string]any{
				"tag":   "column",
				"width": "weighted",
				"weight": 1,
				"elements": []any{
					map[string]any{
						"tag":  "button",
						"type": btnType,
						"text": map[string]any{"tag": "plain_text", "content": label},
						"behaviors": []any{
							map[string]any{
								"type": "callback",
								"value": map[string]any{
									"dw": cb,
								},
							},
						},
					},
				},
			})
		}
		// Feishu column_set is happier with ≤3 columns per row.
		for i := 0; i < len(cols); i += 3 {
			end := i + 3
			if end > len(cols) {
				end = len(cols)
			}
			elements = append(elements, map[string]any{
				"tag":       "column_set",
				"flex_mode": "flow",
				"columns":   cols[i:end],
			})
		}
	}
	return map[string]any{
		"schema": "2.0",
		"header": map[string]any{
			"template": "blue",
			"title":    map[string]any{"tag": "plain_text", "content": title},
		},
		"body": map[string]any{
			"elements": elements,
		},
	}
}

// BuildProgressCard builds a progress / status interactive card (no buttons).
func BuildProgressCard(headline, textBody string, toolLines []string) map[string]any {
	var b strings.Builder
	if h := strings.TrimSpace(headline); h != "" {
		b.WriteString("**")
		b.WriteString(h)
		b.WriteString("**\n\n")
	}
	if len(toolLines) > 0 {
		b.WriteString(strings.Join(toolLines, "\n"))
		b.WriteString("\n\n")
	}
	if t := strings.TrimSpace(textBody); t != "" {
		b.WriteString(t)
	}
	body := strings.TrimSpace(b.String())
	if body == "" {
		body = "正在处理…"
	}
	return BuildInteractiveCard(headline, body, nil)
}

// CallbackTokenFromActionValue extracts our dw|… token from a card action value map.
func CallbackTokenFromActionValue(value map[string]any) string {
	if value == nil {
		return ""
	}
	if v, ok := value["dw"].(string); ok {
		return strings.TrimSpace(v)
	}
	// Tolerate flat stringish maps from older cards.
	if v, ok := value["action"].(string); ok {
		return strings.TrimSpace(v)
	}
	return ""
}
