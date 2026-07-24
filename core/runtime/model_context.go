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
