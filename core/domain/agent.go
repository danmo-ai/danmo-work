package domain

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
	KnowledgeIDs []string      `json:"knowledgeIds"`
	CanDelegate  bool          `json:"canDelegate"`
	// InheritAmbient controls the Ambient capability layer (filesystem skills +
	// all enabled MCP servers). nil = default by Mode: primary true, subagent false.
	InheritAmbient *bool `json:"inheritAmbient,omitempty"`
	// Builtin is computed (not a DB column): true when an embedded template exists.
	Builtin bool `json:"builtin,omitempty"`
	// MarketSource is computed from YAML frontmatter stored in SystemPrompt
	// (metadata.market.source); not a DB column.
	MarketSource string `json:"marketSource,omitempty"`
}

// MCPServerAll is a ToolBinding.MCPServer wildcard: mount every enabled MCP server.
const MCPServerAll = "*"

// ToolBinding attaches capabilities to an agent.
//
// Builtin tools: set ToolID (e.g. "read_file").
// MCP: bind by server — set MCPServer to a server id, or MCPServerAll ("*") for
// every enabled server. Per-tool enable/disable lives on the MCP server config,
// not on the agent (no per-tool agent bindings).
type ToolBinding struct {
	ToolID    string    `json:"toolId"`
	MCPServer string    `json:"mcpServer"`
	RiskLevel RiskLevel `json:"riskLevel"`
}

// InheritsAmbient reports whether this agent receives Ambient capabilities
// (filesystem skills + enabled MCP servers not listed in Tools).
func (a Agent) InheritsAmbient() bool {
	if a.InheritAmbient != nil {
		return *a.InheritAmbient
	}
	return a.Mode != AgentModeSubagent
}
