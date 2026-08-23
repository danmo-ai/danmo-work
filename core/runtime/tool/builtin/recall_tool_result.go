package builtin

import (
	"context"
	"fmt"
	"strings"

	"danmo-work/core/domain"
	"danmo-work/core/port"
)

// ToolResultRecaller reads durable tool_result entries from the turn log.
type ToolResultRecaller interface {
	RecallToolResult(turnID, callID string) (port.RecalledToolResult, bool)
	RecallToolResultInSession(sessionID, callID string) (port.RecalledToolResult, bool)
}

// RecallToolResult fetches full tool output from the durable turn log when
// in-memory compaction has pruned or truncated the LLM-visible copy.
type RecallToolResult struct {
	Store ToolResultRecaller
}

func (h *RecallToolResult) Name() string                { return "recall_tool_result" }
func (h *RecallToolResult) RiskLevel() domain.RiskLevel { return domain.RiskLow }

func (h *RecallToolResult) Describe(args map[string]any) string {
	callID := strVal(args, "call_id")
	if callID != "" {
		return "recall_tool_result " + callID
	}
	return "recall_tool_result"
}

func (h *RecallToolResult) Schema() domain.ToolSchema {
	return domain.ToolSchema{
		Name: "recall_tool_result",
		Description: "Recall the full durable output of a prior tool call from the turn log.\n\n" +
			"Use when compaction pruned or truncated a tool result in the LLM context but you still " +
			"need the original text. Does not inject into context automatically — returns the text " +
			"as this tool's result.\n\n" +
			"Provide call_id from the original tool invocation. turn_id is optional (defaults to the " +
			"current turn); when not found, searches earlier non-nested turns in the session.\n\n" +
			"Limitation: output is capped at runtime.tools.max_output_chars at execute time; " +
			"ingest-truncated results cannot be recovered beyond that cap.",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"call_id": map[string]any{
					"type":        "string",
					"description": "Tool call id to recall (required)",
				},
				"turn_id": map[string]any{
					"type":        "string",
					"description": "Turn id containing the tool result (optional; defaults to current turn)",
				},
			},
			"required": []string{"call_id"},
		},
	}
}

func (h *RecallToolResult) Execute(ctx context.Context, input map[string]any) (domain.ToolResult, error) {
	_ = ctx
	if h.Store == nil {
		return domain.ToolResult{}, fmt.Errorf("turn log is not configured")
	}
	callID := strings.TrimSpace(strVal(input, "call_id"))
	if callID == "" {
		return domain.ToolResult{}, fmt.Errorf("call_id is required")
	}
	turnID := strings.TrimSpace(strVal(input, "turn_id"))
	if turnID == "" {
		turnID = strings.TrimSpace(strVal(input, "__turn_id"))
	}
	sessionID := strings.TrimSpace(strVal(input, "__session_id"))

	var recalled port.RecalledToolResult
	var ok bool
	if turnID != "" {
		recalled, ok = h.Store.RecallToolResult(turnID, callID)
	}
	if !ok && sessionID != "" {
		recalled, ok = h.Store.RecallToolResultInSession(sessionID, callID)
	}
	if !ok {
		return domain.ToolResult{}, fmt.Errorf("tool result not found for call_id %q", callID)
	}

	var header strings.Builder
	fmt.Fprintf(&header, "Recalled tool result [%s] call_id=%s turn=%s", recalled.ToolName, recalled.CallID, recalled.TurnID)
	if recalled.IngestTruncated {
		header.WriteString(" (ingest-truncated at execute time; full durable text below)")
	}
	header.WriteString("\n\n")

	return domain.ToolResult{
		Content: header.String() + recalled.Output,
		Meta: map[string]any{
			"call_id":          recalled.CallID,
			"turn_id":          recalled.TurnID,
			"tool_name":        recalled.ToolName,
			"ingest_truncated": recalled.IngestTruncated,
		},
	}, nil
}
