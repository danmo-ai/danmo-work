package domain

type ToolSchema struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	Parameters  map[string]any `json:"parameters"`
	RiskLevel   RiskLevel      `json:"riskLevel"`
}

type ToolResult struct {
	Content string         `json:"content"`
	Meta    map[string]any `json:"meta,omitempty"`
	// Parts carries multimodal content (images) a tool returns to the model.
	Parts []ToolResultPart `json:"parts,omitempty"`
}

// ToolResultPart is a multimodal content block from a tool result.
type ToolResultPart struct {
	Type     string `json:"type"` // "image"
	MimeType string `json:"mimeType,omitempty"`
	Data     string `json:"data,omitempty"` // raw base64
}
