package tool

import (
	"context"
	"fmt"
	"strings"

	"danmo-work/core/domain"
	"danmo-work/core/service"
)

// MCPCallFunc invokes an MCP tool by server id and original tool name.
type MCPCallFunc func(ctx context.Context, serverID, toolName string, args map[string]any) (string, error)

// MCPHandler adapts a remote MCP tool into the local Handler interface.
type MCPHandler struct {
	ServerID    string
	ServerName  string
	ToolName    string
	ExposedName string
	Description string
	Parameters  map[string]any
	Risk        domain.RiskLevel
	Call        MCPCallFunc
}

func NewMCPHandler(b domain.MCPToolBinding, call MCPCallFunc) *MCPHandler {
	params := b.InputSchema
	if params == nil {
		params = map[string]any{"type": "object", "properties": map[string]any{}}
	}
	risk := b.RiskLevel
	if risk == "" {
		risk = domain.RiskExternal
	}
	desc := b.Description
	if desc == "" {
		desc = fmt.Sprintf("MCP tool %s from %s", b.ToolName, b.ServerName)
	} else {
		desc = fmt.Sprintf("[MCP:%s] %s", b.ServerName, desc)
	}
	return &MCPHandler{
		ServerID:    b.ServerID,
		ServerName:  b.ServerName,
		ToolName:    b.ToolName,
		ExposedName: b.ExposedName,
		Description: desc,
		Parameters:  params,
		Risk:        risk,
		Call:        call,
	}
}

func (h *MCPHandler) Name() string { return h.ExposedName }

func (h *MCPHandler) RiskLevel() domain.RiskLevel { return h.Risk }

func (h *MCPHandler) Schema() domain.ToolSchema {
	return domain.ToolSchema{
		Name:        h.ExposedName,
		Description: h.Description,
		Parameters:  h.Parameters,
		RiskLevel:   h.Risk,
	}
}

func (h *MCPHandler) Describe(args map[string]any) string {
	if len(args) == 0 {
		return h.ExposedName
	}
	keys := make([]string, 0, len(args))
	for k := range args {
		if strings.HasPrefix(k, "__") {
			continue
		}
		keys = append(keys, k)
	}
	if len(keys) > 4 {
		keys = keys[:4]
	}
	return fmt.Sprintf("%s(%s)", h.ExposedName, strings.Join(keys, ", "))
}

func (h *MCPHandler) Execute(ctx context.Context, input map[string]any) (domain.ToolResult, error) {
	if h.Call == nil {
		return domain.ToolResult{}, fmt.Errorf("mcp caller not configured")
	}
	args := sanitizeMCPArgs(input)
	injectCodeGraphProjectPath(h.ServerID, args, input)
	out, err := h.Call(ctx, h.ServerID, h.ToolName, args)
	if err != nil {
		return domain.ToolResult{}, err
	}
	return domain.ToolResult{
		Content: out,
		Meta: map[string]any{
			"mcp_server": h.ServerID,
			"mcp_tool":   h.ToolName,
		},
	}, nil
}

// sanitizeMCPArgs drops runtime-injected __* keys before calling the remote server.
func sanitizeMCPArgs(input map[string]any) map[string]any {
	out := make(map[string]any, len(input))
	for k, v := range input {
		if strings.HasPrefix(k, "__") {
			continue
		}
		out[k] = v
	}
	return out
}

func injectCodeGraphProjectPath(serverID string, args, rawInput map[string]any) {
	if serverID != service.CodeGraphServerID {
		return
	}
	if p, _ := args["projectPath"].(string); strings.TrimSpace(p) != "" {
		return
	}
	workDir, _ := rawInput["__work_dir"].(string)
	workDir = strings.TrimSpace(workDir)
	if workDir == "" {
		return
	}
	args["projectPath"] = workDir
}
