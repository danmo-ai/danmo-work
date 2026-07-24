package llm

import (
	"context"
	"fmt"
	"strings"

	"danmo-work/core/service"
	"danmo-work/core/domain"
	"danmo-work/core/port"
)

type DefaultLLMProviderClient struct {
	mgr     *service.LLMConfigManager
	modelCfg *service.ModelConfigRegistry
}

func NewDefaultLLMProvider(mgr *service.LLMConfigManager, modelCfg *service.ModelConfigRegistry) port.LLMProvider {
	return &DefaultLLMProviderClient{mgr: mgr, modelCfg: modelCfg}
}

func (c *DefaultLLMProviderClient) Chat(ctx context.Context, req port.LLMChatRequest) (port.LLMChatResponse, error) {
	providerName, modelName, effort := ParseModelID(req.Model)
	if providerName == "" {
		return port.LLMChatResponse{}, fmt.Errorf("model not specified or invalid format (expected provider/model)")
	}

	cfg, mn, err := c.mgr.ResolveProvider(ctx, providerName+"/"+modelName)
	if err != nil {
		return port.LLMChatResponse{}, err
	}
	if mn == "" {
		mn = modelName
	}
	req.Model = mn

	modelID := providerName + "/" + modelName

	// Non-vision models must not receive image_url / image parts (providers
	// like DeepSeek reject them with a 400). Strip before provider dispatch.
	if c.modelCfg == nil || !c.modelCfg.SupportsVision(modelID) {
		req.Messages = stripImageParts(req.Messages)
	}

	// Load gen params from registry, merge with caller-provided params.
	if c.modelCfg != nil {
		registryParams := c.modelCfg.GenParams(modelID)
		if registryParams != nil {
			if req.GenParams == nil {
				req.GenParams = registryParams
			} else {
				// Merge: registry values only fill in missing caller-provided values.
				gp := req.GenParams
				rp := registryParams
				if gp.MaxTokens == 0 {
					gp.MaxTokens = rp.MaxTokens
				}
				if gp.Temperature == 0 {
					gp.Temperature = rp.Temperature
				}
				if gp.TopP == 0 {
					gp.TopP = rp.TopP
				}
				if gp.FrequencyPenalty == 0 {
					gp.FrequencyPenalty = rp.FrequencyPenalty
				}
				if gp.PresencePenalty == 0 {
					gp.PresencePenalty = rp.PresencePenalty
				}
				if len(gp.Stop) == 0 {
					gp.Stop = rp.Stop
				}
				if gp.ThinkingMode == "" {
					gp.ThinkingMode = rp.ThinkingMode
				}
				if len(gp.EffortBudgetTokens) == 0 {
					gp.EffortBudgetTokens = rp.EffortBudgetTokens
				}
			}
		}
	}

	var effortCfg *EffortConfig
	if req.GenParams != nil {
		effortCfg = &EffortConfig{
			ThinkingMode:       req.GenParams.ThinkingMode,
			EffortBudgetTokens: req.GenParams.EffortBudgetTokens,
		}
	}

	switch cfg.Provider {
	case domain.LLMProviderAnthropic:
		return NewAnthropicProvider(cfg.BaseURL, cfg.APIKey).Chat(ctx, req, effort, effortCfg)
	case domain.LLMProviderMock:
		return NewMock().Chat(ctx, req)
	default:
		return NewHTTPProvider(cfg.BaseURL, cfg.APIKey).Chat(ctx, req, effort)
	}
}

func ParseModelID(modelID string) (providerName, modelName, effort string) {
	parts := strings.SplitN(modelID, "/", 3)
	if len(parts) >= 1 {
		providerName = parts[0]
	}
	if len(parts) >= 2 {
		modelName = parts[1]
	}
	if len(parts) >= 3 {
		effort = parts[2]
	}
	return
}

// stripImageParts returns a shallow copy of messages with multimodal Parts cleared.
func stripImageParts(msgs []port.ChatMessage) []port.ChatMessage {
	out := make([]port.ChatMessage, len(msgs))
	for i, m := range msgs {
		out[i] = m
		if len(m.Parts) > 0 {
			out[i].Parts = nil
		}
	}
	return out
}
