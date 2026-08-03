package tool

import (
	"context"
	"testing"

	"danmo-work/core/domain"
	"danmo-work/core/service"
)

func TestMCPHandlerStripsInternalArgsAndInjectsProjectPath(t *testing.T) {
	var gotArgs map[string]any
	h := &MCPHandler{
		ServerID:    service.CodeGraphServerID,
		ServerName:  "CodeGraph",
		ToolName:    "explore",
		ExposedName: "mcp_codegraph_explore",
		Call: func(ctx context.Context, serverID, toolName string, args map[string]any) (string, error) {
			gotArgs = args
			return "ok", nil
		},
	}
	_, err := h.Execute(context.Background(), map[string]any{
		"query":        "Greet",
		"__work_dir":   "/proj/app",
		"__session_id": "s1",
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := gotArgs["__work_dir"]; ok {
		t.Fatal("internal __work_dir must not be sent to MCP")
	}
	if gotArgs["projectPath"] != "/proj/app" {
		t.Fatalf("projectPath=%v", gotArgs["projectPath"])
	}
	if gotArgs["query"] != "Greet" {
		t.Fatalf("query=%v", gotArgs["query"])
	}
}

func TestMCPHandlerPreservesExplicitProjectPath(t *testing.T) {
	var gotArgs map[string]any
	h := &MCPHandler{
		ServerID: service.CodeGraphServerID,
		Call: func(ctx context.Context, serverID, toolName string, args map[string]any) (string, error) {
			gotArgs = args
			return "ok", nil
		},
	}
	_, err := h.Execute(context.Background(), map[string]any{
		"projectPath": "/explicit",
		"__work_dir":  "/other",
	})
	if err != nil {
		t.Fatal(err)
	}
	if gotArgs["projectPath"] != "/explicit" {
		t.Fatalf("projectPath=%v", gotArgs["projectPath"])
	}
}

func TestMCPHandlerNoInjectForOtherServers(t *testing.T) {
	var gotArgs map[string]any
	h := NewMCPHandler(domain.MCPToolBinding{
		ServerID: "other", ServerName: "Other", ToolName: "ping", ExposedName: "mcp_other_ping",
	}, func(ctx context.Context, serverID, toolName string, args map[string]any) (string, error) {
		gotArgs = args
		return "ok", nil
	})
	_, err := h.Execute(context.Background(), map[string]any{"__work_dir": "/proj"})
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := gotArgs["projectPath"]; ok {
		t.Fatal("should not inject projectPath for non-codegraph servers")
	}
}
