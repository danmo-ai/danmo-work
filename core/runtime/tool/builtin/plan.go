package builtin

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"danmo-work/core/domain"
)

// planTodoItem is the structured todo attached to create_plan / update_plan.
type planTodoItem struct {
	ID      string `json:"id"`
	Content string `json:"content"`
	Status  string `json:"status"`
}

func planTodoSchema() map[string]any {
	return map[string]any{
		"type": "array",
		"items": map[string]any{
			"type": "object",
			"properties": map[string]any{
				"id":      map[string]any{"type": "string", "description": "Stable todo id (e.g. todo-1)"},
				"content": map[string]any{"type": "string", "description": "Brief description of the task"},
				"status":  map[string]any{"type": "string", "description": "pending | in_progress | completed | cancelled"},
			},
			"required": []string{"id", "content", "status"},
		},
		"description": "Bite-sized implementation todos derived from the plan",
	}
}

func parsePlanTodos(raw any) ([]planTodoItem, error) {
	if raw == nil {
		return nil, nil
	}
	list, ok := raw.([]any)
	if !ok {
		return nil, fmt.Errorf("todos must be an array")
	}
	out := make([]planTodoItem, 0, len(list))
	for _, t := range list {
		m, ok := t.(map[string]any)
		if !ok {
			continue
		}
		it := planTodoItem{
			ID:      strVal(m, "id"),
			Content: strVal(m, "content"),
			Status:  strVal(m, "status"),
		}
		if it.Content == "" {
			continue
		}
		if it.ID == "" {
			it.ID = fmt.Sprintf("todo-%d", len(out)+1)
		}
		switch it.Status {
		case "pending", "in_progress", "completed", "cancelled":
		case "":
			it.Status = "pending"
		default:
			it.Status = "pending"
		}
		out = append(out, it)
	}
	return out, nil
}

func formatPlanResult(action, name, overview, plan string, todos []planTodoItem) (domain.ToolResult, error) {
	var buf strings.Builder
	if name != "" {
		buf.WriteString(fmt.Sprintf("Plan (%s): %s\n", action, name))
	} else {
		buf.WriteString(fmt.Sprintf("Plan (%s)\n", action))
	}
	if overview != "" {
		buf.WriteString("Overview: ")
		buf.WriteString(overview)
		buf.WriteString("\n")
	}
	if plan != "" {
		buf.WriteString("\n")
		buf.WriteString(plan)
		buf.WriteString("\n")
	}
	if len(todos) > 0 {
		buf.WriteString("\nTodos:\n")
		completed, pending, inProgress, cancelled := 0, 0, 0, 0
		for i, it := range todos {
			icon := "[ ]"
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
				pending++
			}
			buf.WriteString(fmt.Sprintf("  %d. %s %s (%s)\n", i+1, icon, it.Content, it.ID))
		}
		buf.WriteString(fmt.Sprintf("\nSummary: %d total | %d pending | %d in-progress | %d completed | %d cancelled",
			len(todos), pending, inProgress, completed, cancelled))
	}

	meta := map[string]any{
		"action": action,
		"name":   name,
		"plan":   plan,
	}
	if overview != "" {
		meta["overview"] = overview
	}
	if len(todos) > 0 {
		raw, _ := json.Marshal(todos)
		meta["todos"] = string(raw)
	}
	return domain.ToolResult{Content: buf.String(), Meta: meta}, nil
}

// CreatePlan publishes a structured implementation plan for the Plan panel.
// The UI reads tool input from the stream; calling this tool (not only writing
// the plan in the assistant message) is what surfaces the Execute button.
type CreatePlan struct{}

func (h *CreatePlan) Name() string                { return "create_plan" }
func (h *CreatePlan) RiskLevel() domain.RiskLevel { return domain.RiskLow }
func (h *CreatePlan) Describe(args map[string]any) string {
	name := strVal(args, "name")
	if name == "" {
		return "create_plan"
	}
	if len(name) > 80 {
		name = name[:80] + "..."
	}
	return name
}
func (h *CreatePlan) Schema() domain.ToolSchema {
	return domain.ToolSchema{
		Name: "create_plan",
		Description: "Publish an implementation plan for the Plan panel (review + Execute → composer).\n\n" +
			"When to use: PLAN MODE after research, when the user asked for a plan before coding, or when you need a reviewable handoff before edits.\n" +
			"Not a progress tracker: while implementing in normal mode, use todowrite for live checklists. create_plan is the plan artifact, not step-by-step execution status.\n\n" +
			"After research, call with full markdown `plan` (+ optional `todos` derived from tasks). The user clicks Execute to send it to the composer.\n" +
			"Do NOT claim files were written. Prefer this over dumping the plan only as a final assistant message.\n" +
			"To revise after feedback, call update_plan.",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"name": map[string]any{
					"type":        "string",
					"description": "Short plan title (e.g. feature name)",
				},
				"overview": map[string]any{
					"type":        "string",
					"description": "One-sentence summary of the approach",
				},
				"plan": map[string]any{
					"type":        "string",
					"description": "Full markdown plan: goal, architecture, tasks with files/steps/done-when",
				},
				"todos": planTodoSchema(),
			},
			"required": []string{"name", "plan"},
		},
	}
}

func (h *CreatePlan) Execute(_ context.Context, input map[string]any) (domain.ToolResult, error) {
	name := strings.TrimSpace(strVal(input, "name"))
	plan := strings.TrimSpace(strVal(input, "plan"))
	overview := strings.TrimSpace(strVal(input, "overview"))
	if name == "" {
		return domain.ToolResult{}, fmt.Errorf("name is required")
	}
	if plan == "" {
		return domain.ToolResult{}, fmt.Errorf("plan is required")
	}
	todos, err := parsePlanTodos(input["todos"])
	if err != nil {
		return domain.ToolResult{}, err
	}
	return formatPlanResult("created", name, overview, plan, todos)
}

// UpdatePlan revises the plan shown in the Plan panel (replace semantics for
// provided fields; the UI merges against the latest create_plan / update_plan).
type UpdatePlan struct{}

func (h *UpdatePlan) Name() string                { return "update_plan" }
func (h *UpdatePlan) RiskLevel() domain.RiskLevel { return domain.RiskLow }
func (h *UpdatePlan) Describe(args map[string]any) string {
	name := strVal(args, "name")
	if name == "" {
		return "update_plan"
	}
	if len(name) > 80 {
		name = name[:80] + "..."
	}
	return name
}
func (h *UpdatePlan) Schema() domain.ToolSchema {
	return domain.ToolSchema{
		Name: "update_plan",
		Description: "Revise the implementation plan in the Plan panel after user feedback or new findings.\n\n" +
			"First version: create_plan. Revisions: this tool. Prefer sending the full updated `plan` markdown.\n" +
			"Not for live execution progress — use todowrite while implementing in normal mode.\n" +
			"Does not start execution; the user clicks Execute.",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"name": map[string]any{
					"type":        "string",
					"description": "Updated short plan title",
				},
				"overview": map[string]any{
					"type":        "string",
					"description": "Updated one-sentence summary",
				},
				"plan": map[string]any{
					"type":        "string",
					"description": "Full updated markdown plan (preferred on every revision)",
				},
				"todos": planTodoSchema(),
			},
		},
	}
}

func (h *UpdatePlan) Execute(_ context.Context, input map[string]any) (domain.ToolResult, error) {
	name := strings.TrimSpace(strVal(input, "name"))
	plan := strings.TrimSpace(strVal(input, "plan"))
	overview := strings.TrimSpace(strVal(input, "overview"))
	todos, err := parsePlanTodos(input["todos"])
	if err != nil {
		return domain.ToolResult{}, err
	}
	if name == "" && overview == "" && plan == "" && len(todos) == 0 {
		return domain.ToolResult{}, fmt.Errorf("provide at least one of name, overview, plan, or todos")
	}
	return formatPlanResult("updated", name, overview, plan, todos)
}
