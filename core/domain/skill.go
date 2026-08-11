package domain

type Skill struct {
	ID            string            `json:"id"`
	Name          string            `json:"name"`
	Description   string            `json:"description"`
	License       string            `json:"license,omitempty"`
	Compatibility string            `json:"compatibility,omitempty"`
	Metadata      map[string]string `json:"metadata,omitempty"`
	AllowedTools  string            `json:"allowedTools,omitempty"`
	Keywords      []string          `json:"keywords"`
	ToolIDs       []string          `json:"toolIds"`
	SystemHint    string            `json:"systemHint"`
	Body          string            `json:"body"`
	SourcePath    string            `json:"sourcePath,omitempty"`
	// Dir is the absolute path to the skill directory on disk.
	Dir string `json:"dir,omitempty"`
	// PromptPath is the path shown to the LLM in <available_skills> (placeholder form).
	PromptPath string `json:"-"`
	// ProjectID is set when the skill comes from a project-level directory.
	ProjectID string `json:"projectId,omitempty"`
	// Source tracks the origin: builtin, market, or user.
	Source string `json:"source,omitempty"`
	// Builtin is true when Source == "builtin" (read-time computed, kept for frontend compatibility).
	Builtin bool `json:"builtin"`
	// MarketSource is the market source id when installed from the marketplace.
	MarketSource string `json:"marketSource,omitempty"`
}
