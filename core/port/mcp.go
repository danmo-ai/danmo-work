package port

import (
	"context"

	"danmo-work/core/domain"
)

// MCPToolInfo is a tool discovered from an MCP server, including JSON Schema.
type MCPToolInfo struct {
	Name        string
	Description string
	InputSchema map[string]any
}

// MCPSession is a live connection to one MCP server (stdio process or HTTP).
type MCPSession interface {
	ListTools(ctx context.Context) ([]MCPToolInfo, error)
	CallTool(ctx context.Context, name string, args map[string]any) (string, error)
	Close() error
}

// MCPDialer opens MCP sessions from server configuration.
type MCPDialer interface {
	Dial(ctx context.Context, srv domain.MCPServer) (MCPSession, error)
}

// MCPCaller invokes a tool on a managed MCP server by id.
type MCPCaller interface {
	CallTool(ctx context.Context, serverID, toolName string, args map[string]any) (string, error)
}

// MCPToolSync receives rebuilt tool handlers when MCP servers change.
// Implemented by the runtime Engine (wired in bootstrap).
type MCPToolSync interface {
	// ReplaceMCPServer syncs tools; ambientMount=false skips MountAllMCP (bound-only).
	ReplaceMCPServer(serverID string, tools []domain.MCPToolBinding, ambientMount bool)
	RemoveMCPServer(serverID string)
}
