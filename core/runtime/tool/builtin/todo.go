package builtin

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"danmo-work/core/domain"
)

type TodoWrite struct{}

func (h *TodoWrite) Name() string                { return "todowrite" }
func (h *TodoWrite) RiskLevel() domain.RiskLevel { return domain.RiskLow }
func (h *TodoWrite) Describe(args map[string]any) string {
	return "todowrite"
}
func (h *TodoWrite) Schema() domain.ToolSchema {
	return domain.ToolSchema{
		Name: "todowrite",
		Description: "Create and maintain a structured task list for the current session.\n\n" +
			"When to use: the task requires 3+ distinct steps, or the work benefits from planning.\n" +
			"Skip when: single straightforward task or purely conversational request.\n\n" +
			"States: pending, in_progress (exactly one at a time), completed, cancelled.\n" +
			"Priority levels: high, medium, low.\n\n" +
			"Mark completed only after work is actually done, including verification.\n" +
			"Update status in real time; don't batch completions.",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"todos": map[string]any{
					"type": "array",
					"items": map[string]any{
						"type": "object",
						"properties": map[string]any{
							"content":  map[string]any{"type": "string", "description": "Brief description of the task"},
							"status":   map[string]any{"type": "string", "description": "Current status: pending, in_progress, completed, cancelled"},
							"priority": map[string]any{"type": "string", "description": "Priority level: high, medium, low"},
						},
						"required": []string{"content", "status", "priority"},
					},
					"description": "The updated todo list",
				},
			},
			"required": []string{"todos"},
		},
	}
}

func (h *TodoWrite) Execute(_ context.Context, input map[string]any) (domain.ToolResult, error) {
	todos, ok := input["todos"]
	if !ok {
		return domain.ToolResult{}, fmt.Errorf("todos is required")
	}

	list, ok := todos.([]any)
	if !ok {
		return domain.ToolResult{}, fmt.Errorf("todos must be an array")
	}

	type item struct {
		Content  string `json:"content"`
		Status   string `json:"status"`
		Priority string `json:"priority"`
	}

	completed := 0
	pending := 0
	inProgress := 0
	cancelled := 0

	var buf strings.Builder
	buf.WriteString("Todo list:\n")
	for i, t := range list {
		m, ok := t.(map[string]any)
		if !ok {
			continue
		}
		it := item{
			Content:  strVal(m, "content"),
			Status:   strVal(m, "status"),
			Priority: strVal(m, "priority"),
		}
		if it.Content == "" {
			continue
		}

		icon := "  "
		switch it.Status {
		case "completed":
			icon = "[x]"
			completed++
		case "in_progress":
			icon = "[>]"
			inProgress++
		case "cancelled":
			icon = "[-]"
			cancelled++
		default:
			icon = "[ ]"
			pending++
		}

		priTag := ""
		switch it.Priority {
		case "high":
			priTag = " 🔴"
		case "medium":
			priTag = " 🟡"
		case "low":
			priTag = " 🟢"
		}

		buf.WriteString(fmt.Sprintf("  %d. %s %s%s\n", i+1, icon, it.Content, priTag))
	}

	buf.WriteString(fmt.Sprintf("\nSummary: %d total | %d pending | %d in-progress | %d completed | %d cancelled",
		len(list), pending, inProgress, completed, cancelled))

	raw, _ := json.Marshal(list)
	return domain.ToolResult{Content: buf.String(), Meta: map[string]any{"todos": string(raw)}}, nil
}

func strVal(m map[string]any, key string) string {
	v, _ := m[key].(string)
	return v
}
