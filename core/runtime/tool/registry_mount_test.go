package tool

import (
	"context"
	"testing"

	"danmo-work/core/domain"
)

type stubHandler struct {
	name string
}

func (h *stubHandler) Name() string                         { return h.name }
func (h *stubHandler) Schema() domain.ToolSchema            { return domain.ToolSchema{Name: h.name} }
func (h *stubHandler) RiskLevel() domain.RiskLevel          { return domain.RiskLow }
func (h *stubHandler) Describe(map[string]any) string       { return h.name }
func (h *stubHandler) Execute(context.Context, map[string]any) (domain.ToolResult, error) {
	return domain.ToolResult{}, nil
}

func TestMountFromBindingsServerAndWildcard(t *testing.T) {
	r := NewRegistry()
	r.RegisterServer("github", &stubHandler{name: "mcp_github_list"})
	r.RegisterServer("notion", &stubHandler{name: "mcp_notion_query"})

	r.MountFromBindings([]domain.ToolBinding{{MCPServer: "github"}})
	if _, ok := r.Get("mcp_github_list"); !ok {
		t.Fatal("expected github tools mounted")
	}
	if _, ok := r.Get("mcp_notion_query"); ok {
		t.Fatal("notion should not be mounted yet")
	}

	r2 := NewRegistry()
	r2.RegisterServer("github", &stubHandler{name: "mcp_github_list"})
	r2.RegisterServer("notion", &stubHandler{name: "mcp_notion_query"})
	r2.MountFromBindings([]domain.ToolBinding{{MCPServer: domain.MCPServerAll}})
	if _, ok := r2.Get("mcp_github_list"); !ok {
		t.Fatal("wildcard should mount github")
	}
	if _, ok := r2.Get("mcp_notion_query"); !ok {
		t.Fatal("wildcard should mount notion")
	}
}
