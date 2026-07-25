package feishu

import (
	"fmt"
	"strings"

	"danmo-work/core/domain"
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
			"template": headerTemplate(title),
			"title":    map[string]any{"tag": "plain_text", "content": title},
		},
		"body": map[string]any{
			"elements": elements,
		},
	}
}

func headerTemplate(title string) string {
	lower := strings.ToLower(strings.TrimSpace(title))
	switch {
	case strings.Contains(lower, "失败") || strings.Contains(lower, "出错") || strings.Contains(lower, "error") || strings.Contains(lower, "fail"):
		return "red"
	case strings.Contains(lower, "等待") || strings.Contains(lower, "审批") || strings.Contains(lower, "确认"):
		return "orange"
	case strings.Contains(lower, "完成") || strings.Contains(lower, "done") || strings.Contains(lower, "授权"):
		return "green"
	default:
		return "blue"
	}
}

// BuildProgressCard builds a progress / status interactive card.
// actions may attach approval / ask buttons on the same card (Phase A same-card prefer).
func BuildProgressCard(headline, textBody string, toolLines []string, actions []port.OutboundAction) map[string]any {
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
	return BuildInteractiveCard(headline, body, actions)
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

// BuildAskFormCard builds a schema 2.0 form card for ask_user formFields.
func BuildAskFormCard(title, question string, fields []domain.AskUserFormField, submitToken string) map[string]any {
	title = strings.TrimSpace(title)
	if title == "" {
		title = "需要你的确认"
	}
	formChildren := []any{}
	if q := strings.TrimSpace(question); q != "" {
		formChildren = append(formChildren, map[string]any{
			"tag":     "markdown",
			"content": q,
		})
	}
	for _, f := range fields {
		name := strings.TrimSpace(f.Name)
		if name == "" {
			continue
		}
		label := strings.TrimSpace(f.Label)
		if label == "" {
			label = name
		}
		formChildren = append(formChildren, map[string]any{
			"tag":     "markdown",
			"content": "**" + label + "**" + requiredMark(f.Required),
		})
		switch strings.ToLower(strings.TrimSpace(f.Type)) {
		case "select":
			opts := make([]any, 0, len(f.Options))
			for _, o := range f.Options {
				opts = append(opts, map[string]any{
					"text":  map[string]any{"tag": "plain_text", "content": o},
					"value": o,
				})
			}
			formChildren = append(formChildren, map[string]any{
				"tag":         "select_static",
				"name":        name,
				"required":    f.Required,
				"placeholder": map[string]any{"tag": "plain_text", "content": pickPlaceholder(f)},
				"options":     opts,
			})
		case "boolean":
			formChildren = append(formChildren, map[string]any{
				"tag":         "select_static",
				"name":        name,
				"required":    f.Required,
				"placeholder": map[string]any{"tag": "plain_text", "content": "是 / 否"},
				"options": []any{
					map[string]any{"text": map[string]any{"tag": "plain_text", "content": "是"}, "value": "是"},
					map[string]any{"text": map[string]any{"tag": "plain_text", "content": "否"}, "value": "否"},
				},
			})
		default: // text, number
			inputType := "text"
			if strings.EqualFold(f.Type, "number") {
				inputType = "number"
			}
			el := map[string]any{
				"tag":         "input",
				"name":        name,
				"required":    f.Required,
				"input_type":  inputType,
				"placeholder": map[string]any{"tag": "plain_text", "content": pickPlaceholder(f)},
			}
			if f.Default != nil {
				el["default_value"] = fmt.Sprint(f.Default)
			}
			formChildren = append(formChildren, el)
		}
	}
	if submitToken == "" {
		submitToken = "submit"
	}
	formChildren = append(formChildren, map[string]any{
		"tag":  "button",
		"type": "primary",
		"text": map[string]any{"tag": "plain_text", "content": "提交"},
		"behaviors": []any{
			map[string]any{
				"type":  "callback",
				"value": map[string]any{"dw": submitToken},
			},
		},
		"form_action_type": "submit",
		"name":             "submit",
	})
	return map[string]any{
		"schema": "2.0",
		"header": map[string]any{
			"template": "blue",
			"title":    map[string]any{"tag": "plain_text", "content": title},
		},
		"body": map[string]any{
			"elements": []any{
				map[string]any{
					"tag":      "form",
					"name":     "ask_form",
					"elements": formChildren,
				},
			},
		},
	}
}

func pickPlaceholder(f domain.AskUserFormField) string {
	if ph := strings.TrimSpace(f.Placeholder); ph != "" {
		return ph
	}
	if f.Default != nil {
		return fmt.Sprint(f.Default)
	}
	return "请输入"
}

func requiredMark(required bool) string {
	if required {
		return "（必填）"
	}
	return ""
}
