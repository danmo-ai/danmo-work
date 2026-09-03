package service

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"danmo-work/core/domain"
)

// LoadHooksConfig parses a plugin's hooks.json (Codex-style lifecycle hooks).
func LoadHooksConfig(path string) (domain.HooksConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return domain.HooksConfig{}, err
	}
	var cfg domain.HooksConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		return domain.HooksConfig{}, fmt.Errorf("hooks.json parse: %w", err)
	}
	for event, groups := range cfg.Hooks {
		switch event {
		case domain.HookEventUserPromptSubmit, domain.HookEventSubagentStart:
		default:
			return domain.HooksConfig{}, fmt.Errorf("hooks.json: unsupported event %q (v1: userPromptSubmit | subagentStart)", event)
		}
		for gi, g := range groups {
			for hi, h := range g.Hooks {
				if h.Type != "command" {
					return domain.HooksConfig{}, fmt.Errorf("hooks.json: %s[%d].hooks[%d]: unsupported handler type %q (v1: command)", event, gi, hi, h.Type)
				}
				if strings.TrimSpace(h.Command) == "" {
					return domain.HooksConfig{}, fmt.Errorf("hooks.json: %s[%d].hooks[%d]: empty command", event, gi, hi)
				}
			}
		}
	}
	return cfg, nil
}

// resolveHooks flattens a HooksConfig into ResolvedHook entries bound to the plugin root.
func resolveHooks(pluginName, pluginRoot string, cfg domain.HooksConfig) []domain.ResolvedHook {
	var out []domain.ResolvedHook
	for event, groups := range cfg.Hooks {
		for _, g := range groups {
			for _, h := range g.Hooks {
				out = append(out, domain.ResolvedHook{
					PluginName: pluginName,
					PluginRoot: pluginRoot,
					Event:      event,
					Matcher:    g.Matcher,
					Command:    h.Command,
					TimeoutSec: h.TimeoutSec,
				})
			}
		}
	}
	return out
}

// Hooks returns the resolved builtin-plugin hooks registered at Init.
func (pm *PluginManager) Hooks() []domain.ResolvedHook {
	pm.mu.RLock()
	defer pm.mu.RUnlock()
	out := make([]domain.ResolvedHook, len(pm.hooks))
	copy(out, pm.hooks)
	return out
}

// HooksForAgent filters resolved hooks by event and agent-id matcher.
func HooksForAgent(hooks []domain.ResolvedHook, event, agentID string) []domain.ResolvedHook {
	var out []domain.ResolvedHook
	for _, h := range hooks {
		if h.Event != event {
			continue
		}
		if domain.HookMatcherMatches(h.Matcher, agentID) {
			out = append(out, h)
		}
	}
	return out
}
