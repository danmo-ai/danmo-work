package builtin

import (
	"context"
	"fmt"
	"time"

	"danmo-work/core/domain"
)

const maxSleepSeconds = 300

// Sleep pauses execution for a specified duration. Useful for rate limiting,
// polling, and backoff delays. Context-aware: interrupted if session is cancelled.
type Sleep struct{}

func (h *Sleep) Name() string                { return "sleep" }
func (h *Sleep) RiskLevel() domain.RiskLevel { return domain.RiskLow }
func (h *Sleep) Describe(args map[string]any) string {
	var s int
	switch v := args["seconds"].(type) {
	case float64:
		s = int(v)
	case int:
		s = v
	}
	return fmt.Sprintf("%d s", s)
}
func (h *Sleep) Schema() domain.ToolSchema {
	return domain.ToolSchema{
		Name: "sleep",
		Description: "Pause execution for a specified number of seconds. Use this to:\n" +
			"- Wait between API calls to respect rate limits\n" +
			"- Poll for async operation completion (e.g. build, deploy, external task)\n" +
			"- Add backoff delay after errors before retrying\n\n" +
			"Constraints:\n" +
			"- Maximum: 300 seconds (5 minutes). Longer values will be truncated.\n" +
			"- Interruptible: if the session is cancelled, sleep stops immediately.\n" +
			"- Do NOT use sleep as a substitute for proper async handling.",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"seconds": map[string]any{
					"type":        "integer",
					"description": "Number of seconds to sleep (1-300)",
				},
			},
			"required": []string{"seconds"},
		},
	}
}

func (h *Sleep) Execute(ctx context.Context, input map[string]any) (domain.ToolResult, error) {
	var seconds int
	switch v := input["seconds"].(type) {
	case float64:
		seconds = int(v)
	case int:
		seconds = v
	default:
		return domain.ToolResult{}, fmt.Errorf("seconds is required and must be an integer")
	}
	if seconds < 1 {
		seconds = 1
	}
	if seconds > maxSleepSeconds {
		seconds = maxSleepSeconds
	}

	start := time.Now()
	timer := time.NewTimer(time.Duration(seconds) * time.Second)
	defer timer.Stop()

	select {
	case <-timer.C:
		return domain.ToolResult{
			Content: fmt.Sprintf("Slept for %d second(s).", seconds),
		}, nil
	case <-ctx.Done():
		elapsed := int(time.Since(start).Seconds())
		return domain.ToolResult{
			Content: fmt.Sprintf("Sleep interrupted after %d second(s): session cancelled.", elapsed),
		}, ctx.Err()
	}
}
