package tool

import (
	"context"

	"danmo-work/core/domain"
)

type Handler interface {
	Name() string
	Schema() domain.ToolSchema
	RiskLevel() domain.RiskLevel
	Describe(args map[string]any) string
	Execute(ctx context.Context, input map[string]any) (domain.ToolResult, error)
}
