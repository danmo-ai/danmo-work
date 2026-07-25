package qq

import (
	"strings"

	"danmo-work/core/port"
)

// BuildKeyboard builds a QQ markdown keyboard from outbound actions.
func BuildKeyboard(actions []port.OutboundAction) map[string]any {
	if len(actions) == 0 {
		return nil
	}
	rows := []any{}
	row := []any{}
	flush := func() {
		if len(row) == 0 {
			return
		}
		rows = append(rows, map[string]any{"buttons": row})
		row = nil
	}
	for _, act := range actions {
		label := strings.TrimSpace(act.Label)
		if label == "" {
			label = act.ID
		}
		data := strings.TrimSpace(act.ID)
		if data == "" {
			data = label
		}
		style := 1
		lower := strings.ToLower(label)
		if strings.Contains(lower, "拒绝") || strings.Contains(lower, "deny") {
			style = 0
		}
		row = append(row, map[string]any{
			"id": truncateID(data, 40),
			"render_data": map[string]any{
				"label":         truncateID(label, 30),
				"visited_label": truncateID(label, 30),
				"style":         style,
			},
			"action": map[string]any{
				"type": 1,
				"permission": map[string]any{
					"type": 2,
				},
				"data":            data,
				"unsupport_tips": "请升级 QQ 客户端",
			},
		})
		if len(row) >= 3 {
			flush()
		}
		if len(rows) >= 5 {
			break
		}
	}
	flush()
	if len(rows) == 0 {
		return nil
	}
	return map[string]any{
		"content": map[string]any{
			"rows": rows,
		},
	}
}

func truncateID(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n])
}
