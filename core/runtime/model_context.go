package runtime

import (
	"danmo-work/core/service"
)

// ModelConfigRegistry is re-exported from core/service to avoid import cycles.
// All implementation lives in core/service/model_config.go.
type ModelConfigRegistry = service.ModelConfigRegistry

// NewModelConfigRegistry creates a new registry (delegates to service package).
func NewModelConfigRegistry() *ModelConfigRegistry {
	return service.NewModelConfigRegistry()
}

// usableContextTokens is the prompt budget for compaction: context_window −
// max_output. Falls back to the registry default window when model is unknown.
// If max_output ≥ window, returns the full window so the gate still works.
func usableContextTokens(reg *ModelConfigRegistry, model string) int {
	if reg == nil {
		reg = NewModelConfigRegistry()
	}
	window := reg.ContextWindow(model)
	if window <= 0 {
		return 0
	}
	maxOut := reg.MaxOutputTokens(model)
	usable := window - maxOut
	if usable <= 0 {
		return window
	}
	return usable
}
