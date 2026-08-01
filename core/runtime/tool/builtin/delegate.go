package builtin

import (
	"context"
	"encoding/json"
	"fmt"

	"danmo-work/core/domain"
	"danmo-work/core/port"
	"danmo-work/core/service"
)

type DelegateAgent struct {
	Stream          port.EventStream
	Agents          *service.AgentManager
	KnowledgeSearch func(kbIDs []string, query string, k int) []string
	RunSubTurn      func(ctx context.Context, sessionID, modelID, parentTurnID string, agent domain.Agent, goal string, parentPath []domain.TurnPathEntry) (domain.Report, error)
}

func (h *DelegateAgent) Name() string                { return "delegate_agent" }
func (h *DelegateAgent) RiskLevel() domain.RiskLevel { return domain.RiskLow }
func (h *DelegateAgent) Describe(args map[string]any) string {
	agentID, _ := args["agent_id"].(string)
	goal, _ := args["goal"].(string)
	if len(goal) > 80 {
		goal = goal[:80] + "..."
	}
	return agentID + ": " + goal
}
func (h *DelegateAgent) Schema() domain.ToolSchema {
	return domain.ToolSchema{
		Name: "delegate_agent",
		Description: "Delegate a task to a subagent and get back a structured report.\n\n" +
			"- agent_id: the ID of the agent to delegate to (see <available_agents> in your system prompt).\n" +
			"- goal: a clear, specific description of what the subagent should accomplish.\n" +
			"- context: optional additional background information.\n" +
			"- Assign complete subtasks, not single actions — let subagents decide how to use their tools.\n" +
			"- Launch multiple subagents in parallel when their work is independent.\n" +
			"- Read subagent reports before deciding the next step.",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"agent_id": map[string]any{"type": "string"},
				"goal":     map[string]any{"type": "string"},
				"context":  map[string]any{"type": "string"},
			},
			"required": []string{"agent_id", "goal"},
		},
	}
}

func (h *DelegateAgent) Execute(ctx context.Context, input map[string]any) (domain.ToolResult, error) {
	agentID, _ := input["agent_id"].(string)
	goal, _ := input["goal"].(string)
	if extra, ok := input["context"].(string); ok && extra != "" {
		goal = goal + "\nContext: " + extra
	}
	sessionID, _ := input["__session_id"].(string)
	modelID, _ := input["__model_id"].(string)
	parentTurnID, _ := input["__turn_id"].(string)
	parentPath := turnPathFromInput(input, parentTurnID)

	agent, err := h.Agents.Get(ctx, agentID)
	if err != nil {
		return domain.ToolResult{}, fmt.Errorf("unknown agent %q", agentID)
	}

	report, err := h.RunSubTurn(ctx, sessionID, modelID, parentTurnID, *agent, goal, parentPath)
	if err != nil {
		return domain.ToolResult{}, err
	}
	report.WorkerID = agent.ID
	report.WorkerName = agent.Name
	report.TraceID = parentTurnID

	raw, _ := json.Marshal(report)
	return domain.ToolResult{Content: formatSessionResult(report), Meta: map[string]any{"report": string(raw)}}, nil
}

func turnPathFromInput(input map[string]any, parentTurnID string) []domain.TurnPathEntry {
	if path := coerceTurnPath(input["__turn_path"]); len(path) > 0 {
		return path
	}
	agentID, _ := input["__agent_id"].(string)
	if parentTurnID == "" && agentID == "" {
		return nil
	}
	return []domain.TurnPathEntry{{TurnID: parentTurnID, AgentID: agentID}}
}

func coerceTurnPath(raw any) []domain.TurnPathEntry {
	switch v := raw.(type) {
	case []domain.TurnPathEntry:
		out := make([]domain.TurnPathEntry, len(v))
		copy(out, v)
		return out
	case []any:
		out := make([]domain.TurnPathEntry, 0, len(v))
		for _, item := range v {
			switch e := item.(type) {
			case domain.TurnPathEntry:
				out = append(out, e)
			case map[string]any:
				turnID, _ := e["turnId"].(string)
				if turnID == "" {
					turnID, _ = e["TurnID"].(string)
				}
				agentID, _ := e["agentId"].(string)
				if agentID == "" {
					agentID, _ = e["AgentID"].(string)
				}
				if turnID != "" || agentID != "" {
					out = append(out, domain.TurnPathEntry{TurnID: turnID, AgentID: agentID})
				}
			}
		}
		return out
	default:
		return nil
	}
}

func formatSessionResult(report domain.Report) string {
	b, _ := json.Marshal(report)
	return fmt.Sprintf("<session_result>%s</session_result>", string(b))
}
