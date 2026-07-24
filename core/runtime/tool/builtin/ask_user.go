package builtin

import (
	"context"
	"encoding/json"

	"danmo-work/core/domain"
	"danmo-work/core/port"
)

type AskUser struct {
	Stream port.EventStream
	OnAsk  func(ctx context.Context, sessionID, turnID, callID, question string, options []string, defaultOpt string, formFields []domain.AskUserFormField) (string, error)
}

func (h *AskUser) Name() string                { return "ask_user" }
func (h *AskUser) RiskLevel() domain.RiskLevel { return domain.RiskLow }
func (h *AskUser) Describe(args map[string]any) string {
	question, _ := args["question"].(string)
	if len(question) > 80 {
		question = question[:80] + "..."
	}
	return question
}
func (h *AskUser) Schema() domain.ToolSchema {
	return domain.ToolSchema{
		Name: "ask_user",
		Description: "Ask the human user a question and block until they reply. REQUIRED for clarification, decisions, or missing inputs — never ask in a plain assistant message (that ends the turn without waiting).\n\n" +
			"## Modes\n" +
			"1. **Simple question**: just provide `question`. User types a free-text reply.\n" +
			"2. **Choice**: provide `question` + `options`. User picks one or types custom text.\n" +
			"   Set `defaultOption` to pre-select one.\n" +
			"3. **Form**: provide `question` + `form_fields`. User fills a structured form.\n" +
			"   Supported field types: text, number, select, boolean.\n\n" +
			"The user's answer is returned as the tool result.",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"question": map[string]any{
					"type":        "string",
					"description": "The question to ask the user. Be clear and specific.",
				},
				"options": map[string]any{
					"type": "array",
					"items": map[string]any{
						"type": "string",
					},
					"description": "Predefined choices for the user to pick from. User can also type a custom reply.",
				},
				"defaultOption": map[string]any{
					"type":        "string",
					"description": "The default selected option (must match one of the options).",
				},
				"form_fields": map[string]any{
					"type": "array",
					"items": map[string]any{
						"type": "object",
						"properties": map[string]any{
							"name": map[string]any{
								"type":        "string",
								"description": "Field key name, used in the result JSON.",
							},
							"label": map[string]any{
								"type":        "string",
								"description": "Human-readable label displayed to the user.",
							},
							"type": map[string]any{
								"type":        "string",
								"enum":        []string{"text", "number", "select", "boolean"},
								"description": "Input type. 'select' requires 'options'.",
							},
							"required": map[string]any{
								"type":        "boolean",
								"description": "Whether this field is required. Default false.",
							},
							"default": map[string]any{
								"description": "Default value for the field.",
							},
							"options": map[string]any{
								"type": "array",
								"items": map[string]any{
									"type": "string",
								},
								"description": "Choices for 'select' type fields.",
							},
							"placeholder": map[string]any{
								"type":        "string",
								"description": "Placeholder text for text/number fields.",
							},
						},
						"required": []string{"name", "label", "type"},
					},
					"description": "Structured form fields for the user to fill in. When provided, options are ignored.",
				},
			},
			"required": []string{"question"},
		},
	}
}

func (h *AskUser) Execute(ctx context.Context, input map[string]any) (domain.ToolResult, error) {
	question, _ := input["question"].(string)
	if question == "" {
		return domain.ToolResult{Content: "ask_user: question is required"}, nil
	}

	sessionID, _ := input["__session_id"].(string)
	turnID, _ := input["__turn_id"].(string)
	callID, _ := input["__call_id"].(string)

	var options []string
	if raw, ok := input["options"].([]any); ok {
		for _, o := range raw {
			if s, ok2 := o.(string); ok2 {
				options = append(options, s)
			}
		}
	}

	defaultOpt, _ := input["defaultOption"].(string)

	var formFields []domain.AskUserFormField
	if raw, ok := input["form_fields"].([]any); ok {
		for _, item := range raw {
			b, err := json.Marshal(item)
			if err != nil {
				continue
			}
			var f domain.AskUserFormField
			if err := json.Unmarshal(b, &f); err != nil {
				continue
			}
			if f.Name != "" && f.Label != "" && f.Type != "" {
				formFields = append(formFields, f)
			}
		}
	}

	if h.OnAsk == nil {
		return domain.ToolResult{Content: "ask_user: not connected to user input channel"}, nil
	}

	answer, err := h.OnAsk(ctx, sessionID, turnID, callID, question, options, defaultOpt, formFields)
	if err != nil {
		return domain.ToolResult{}, err
	}

	return domain.ToolResult{Content: answer}, nil
}
