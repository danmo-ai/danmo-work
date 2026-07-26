package tool

import (
	"context"
	"testing"

	"danmo-work/core/domain"
)

type stubHandler struct {
	name string
}

func (h *stubHandler) Name() string                   { return h.name }
func (h *stubHandler) Schema() domain.ToolSchema      { return domain.ToolSchema{Name: h.name} }
func (h *stubHandler) RiskLevel() domain.RiskLevel    { return domain.RiskLow }
func (h *stubHandler) Describe(map[string]any) string { return h.name }
func (h *stubHandler) Execute(context.Context, map[string]any) (domain.ToolResult, error) {
	return domain.ToolResult{}, nil
}

func TestMountServersExactIDs(t *testing.T) {
	r := NewRegistry()
	r.RegisterServer("github", &stubHandler{name: "mcp_github_list"})
	r.RegisterServer("notion", &stubHandler{name: "mcp_notion_query"})

	r.MountServers([]string{"github"})
	if _, ok := r.Get("mcp_github_list"); !ok {
		t.Fatal("expected github tools mounted")
	}
	if _, ok := r.Get("mcp_notion_query"); ok {
		t.Fatal("notion should not be mounted")
	}
}
