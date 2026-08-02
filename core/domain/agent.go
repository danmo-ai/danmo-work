package domain

import "strings"

type AgentMode string

const (
	AgentModePrimary  AgentMode = "primary"
	AgentModeSubagent AgentMode = "subagent"
)

type RiskLevel string

const (
	RiskLow      RiskLevel = "low"
	RiskMedium   RiskLevel = "medium"
	RiskHigh     RiskLevel = "high"
	RiskExternal RiskLevel = "external" // side effects off-machine (MCP, outbound APIs)
)

// PermissionMode presets map to Gate behaviour (OpenWorker-aligned).
type PermissionMode string

const (
	PermModeDiscuss     PermissionMode = "discuss"     // read-only: deny write/exec/external
	PermModePlan        PermissionMode = "plan"        // read-only planning
	PermModeInteractive PermissionMode = "interactive" // ask before high/external (default)
	PermModeAuto        PermissionMode = "auto"        // allow within sandbox; still ask dangerous
)

type Agent struct {
	ID           string    `json:"id"`
	Name         string    `json:"name"`
	Description  string    `json:"description"`
	Persona      string    `json:"persona"`
	Mode         AgentMode `json:"mode"`
	SystemPrompt string    `json:"systemPrompt"`
	// Steps is the max tool/LLM steps per turn. 0 means follow
	// runtime.turn.max_steps_default.
	Steps        int           `json:"steps"`
	SkillIDs     []string      `json:"skillIds"`
	Tools        []ToolBinding `json:"tools"`
	// MCPServers lists MCP server ids this agent may use when InheritAmbient
	// is false. Ambient on → all enabled servers (this list ignored for mount).
	MCPServers   []string `json:"mcpServers,omitempty"`
	KnowledgeIDs []string `json:"knowledgeIds"`
	CanDelegate  bool     `json:"canDelegate"`
	// InheritAmbient controls the Ambient capability layer (filesystem skills +
	// all enabled MCP servers). nil = default by Mode: primary true, subagent false.
	InheritAmbient *bool `json:"inheritAmbient,omitempty"`
	// Builtin is computed (not a DB column): true when an embedded template exists.
	Builtin bool `json:"builtin,omitempty"`
	// MarketSource is computed from YAML frontmatter stored in SystemPrompt
	// (metadata.market.source); not a DB column.
	MarketSource string `json:"marketSource,omitempty"`
}

// ToolBinding attaches a builtin tool to an agent (set ToolID, e.g. "read_file").
// MCP is bound via Agent.MCPServers, not ToolBinding.
//
// Deprecated: MCPServer was used before mcpServers was split out; still read on
// load for migration, then stripped by NormalizeAgentBindings.
type ToolBinding struct {
	ToolID    string    `json:"toolId"`
	MCPServer string    `json:"mcpServer,omitempty"` // deprecated; migrate → Agent.MCPServers
	RiskLevel RiskLevel `json:"riskLevel"`
}

// InheritsAmbient reports whether this agent receives Ambient capabilities
// (filesystem skills + all enabled MCP servers).
func (a Agent) InheritsAmbient() bool {
	if a.InheritAmbient != nil {
		return *a.InheritAmbient
	}
	return a.Mode != AgentModeSubagent
}

// coreToolIDs are engine-mounted for every agent (Core layer). Binding them in
// tools[] is redundant and harmful: mountBuiltinTools would replace the wired
// handlers with catalog stubs (e.g. ask_user with OnAsk=nil).
var coreToolIDs = map[string]struct{}{
	"ask_user": {}, "read_skill": {}, "delegate_agent": {},
	"memory_update": {}, "memory_read": {},
	"search_kb": {}, "list_kb_docs": {}, "get_kb_doc": {},
	"table_upsert": {}, "table_get": {}, "table_query": {}, "table_delete": {}, "table_list": {},
}

// IsCoreTool reports whether toolID is a Core-layer builtin (not Agent-bound).
func IsCoreTool(toolID string) bool {
	_, ok := coreToolIDs[strings.TrimSpace(toolID)]
	return ok
}

// NormalizeAgentBindings moves legacy tools[].mcpServer entries into
// MCPServers, drops wildcards/empties/Core tools, and keeps tools[] as
// Bound builtins only.
func NormalizeAgentBindings(a *Agent) {
	if a == nil {
		return
	}
	seen := make(map[string]struct{}, len(a.MCPServers))
	outServers := make([]string, 0, len(a.MCPServers))
	for _, id := range a.MCPServers {
		id = strings.TrimSpace(id)
		if id == "" || id == "*" {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		outServers = append(outServers, id)
	}
	outTools := make([]ToolBinding, 0, len(a.Tools))
	for _, t := range a.Tools {
		if mcp := strings.TrimSpace(t.MCPServer); mcp != "" && mcp != "*" {
			if _, ok := seen[mcp]; !ok {
				seen[mcp] = struct{}{}
				outServers = append(outServers, mcp)
			}
			continue
		}
		tid := strings.TrimSpace(t.ToolID)
		if tid == "" || IsCoreTool(tid) {
			continue
		}
		outTools = append(outTools, ToolBinding{ToolID: tid, RiskLevel: t.RiskLevel})
	}
	a.MCPServers = outServers
	a.Tools = outTools
}
