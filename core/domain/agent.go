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
// InheritAmbient is unused (replaced by AgentMode) but kept for backward compat.
// Primary agents get all skills/MCP; subagents get only bound.
	InheritAmbient *bool `json:"inheritAmbient,omitempty"`
	// Source tracks the origin: builtin, market, or user.
	Source string `json:"source,omitempty"`
	Builtin bool `json:"builtin"`
	// MarketSource is the market source id when installed from the marketplace.
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

// PlanModeAllowedToolIDs is the built-in read-only tool whitelist used when
// plan mode is enabled. In plan mode, the agent's available tools are the
// intersection of its own tool bindings and this set.
var PlanModeAllowedToolIDs = map[string]struct{}{
	// Codebase exploration
	"read_file":  {},
	"read_image": {},
	"grep":       {},
	"glob":       {},
	// External research
	"web_search": {},
	"web_fetch":  {},
	// Clarification / delegation (delegated sub-agents also inherit plan mode)
	"ask_user":       {},
	"delegate_agent": {},
	// Skills / memory read
	"read_skill":  {},
	"memory_read": {},
	// Knowledge base read
	"search_kb":    {},
	"list_kb_docs": {},
	"get_kb_doc":   {},
	// Table store read
	"table_get":    {},
	"table_query":  {},
	"table_list":   {},
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
// Bound builtins only. Agents that bind edit always get apply_patch too
// (same risk as edit when newly added).
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
	seenTools := make(map[string]struct{}, len(a.Tools))
	var editRisk RiskLevel
	hasEdit, hasApplyPatch := false, false
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
		if _, ok := seenTools[tid]; ok {
			continue
		}
		seenTools[tid] = struct{}{}
		outTools = append(outTools, ToolBinding{ToolID: tid, RiskLevel: t.RiskLevel})
		switch tid {
		case "edit":
			hasEdit = true
			editRisk = t.RiskLevel
		case "apply_patch":
			hasApplyPatch = true
		}
	}
	if hasEdit && !hasApplyPatch {
		risk := editRisk
		if risk == "" {
			risk = RiskMedium
		}
		outTools = append(outTools, ToolBinding{ToolID: "apply_patch", RiskLevel: risk})
	}
	a.MCPServers = outServers
	a.Tools = outTools
}
