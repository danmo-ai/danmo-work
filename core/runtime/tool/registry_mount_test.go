package tool

import (
	"context"
	"strings"
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

func TestMountAllMCPSortsServerIDs(t *testing.T) {
	r := NewRegistry()
	r.RegisterServer("zeta", &stubHandler{name: "mcp_zeta_a"})
	r.RegisterServer("alpha", &stubHandler{name: "mcp_alpha_a"})
	r.MountAllMCP()
	schemas := r.Schemas()
	if len(schemas) < 2 {
		t.Fatalf("schemas=%+v", schemas)
	}
	var names []string
	for _, s := range schemas {
		if strings.HasPrefix(s.Name, "mcp_") {
			names = append(names, s.Name)
		}
	}
	if len(names) != 2 || names[0] != "mcp_alpha_a" || names[1] != "mcp_zeta_a" {
		t.Fatalf("MCP tools should mount in server-id order, got %v", names)
	}
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

func TestMountAllMCPSkipsBoundOnly(t *testing.T) {
	r := NewRegistry()
	r.RegisterServerAmbient("github", true, &stubHandler{name: "mcp_github_list"})
	r.RegisterServerAmbient("creative", false, &stubHandler{name: "mcp_creative_gen"})

	r.MountAllMCP()
	if _, ok := r.Get("mcp_github_list"); !ok {
		t.Fatal("ambient connector should mount")
	}
	if _, ok := r.Get("mcp_creative_gen"); ok {
		t.Fatal("bound-only connector must not mount via MountAllMCP")
	}

	r2 := NewRegistry()
	r2.CopyMCPServersFrom(r)
	r2.MountServers([]string{"creative"})
	if _, ok := r2.Get("mcp_creative_gen"); !ok {
		t.Fatal("bound-only connector must mount via MountServers")
	}
}

func TestFilterKeepsOnlyAllowedTools(t *testing.T) {
	r := NewRegistry()
	r.RegisterServer("github", &stubHandler{name: "mcp_github_list"}, &stubHandler{name: "mcp_github_write"})
	r.Register(&stubHandler{name: "read_file"})
	r.Register(&stubHandler{name: "write"})
	r.MountAllMCP()

	allowed := map[string]struct{}{"read_file": {}, "mcp_github_list": {}}
	r.Filter(allowed)

	if _, ok := r.Get("read_file"); !ok {
		t.Fatal("expected read_file to remain")
	}
	if _, ok := r.Get("mcp_github_list"); !ok {
		t.Fatal("expected allowed MCP tool to remain")
	}
	if _, ok := r.Get("write"); ok {
		t.Fatal("write should be filtered out")
	}
	if _, ok := r.Get("mcp_github_write"); ok {
		t.Fatal("non-allowed MCP tool should be filtered out")
	}
	if len(r.Schemas()) != 2 {
		t.Fatalf("expected 2 schemas, got %d", len(r.Schemas()))
	}
}
