package domain

import (
	"fmt"
	"regexp"
	"strings"
)

// PluginManifest represents a plugin.json manifest (Agent Plugins spec §5.2).
type PluginManifest struct {
	Schema      string        `json:"$schema"`
	Name        string        `json:"name"`
	Version     string        `json:"version,omitempty"`
	Description string        `json:"description,omitempty"`
	Author      *PluginAuthor `json:"author,omitempty"`
	Homepage    string        `json:"homepage,omitempty"`
	Repository  string        `json:"repository,omitempty"`
	License     string        `json:"license,omitempty"`
	Keywords    []string      `json:"keywords,omitempty"`
	Extensions  map[string]any `json:"extensions,omitempty"`
}

type PluginAuthor struct {
	Name  string `json:"name,omitempty"`
	Email string `json:"email,omitempty"`
	URL   string `json:"url,omitempty"`
}

// PluginComponents lists the component IDs provided by a plugin.
type PluginComponents struct {
	Skills     []string `json:"skills,omitempty"`
	Experts    []string `json:"experts,omitempty"`
	MCP        []string `json:"mcp,omitempty"`
	Knowledge  []string `json:"knowledge,omitempty"`
}

// PluginInstalled records one installed plugin for installed.json and API responses.
type PluginInstalled struct {
	Name         string           `json:"name"`
	Version      string           `json:"version"`
	Description  string           `json:"description,omitempty"`
	Author       *PluginAuthor    `json:"author,omitempty"`
	Homepage     string           `json:"homepage,omitempty"`
	Repository   string           `json:"repository,omitempty"`
	License      string           `json:"license,omitempty"`
	Keywords     []string         `json:"keywords,omitempty"`
	RootPath     string           `json:"rootPath"`
	MarketSource string           `json:"marketSource,omitempty"`
	InstalledAt  string           `json:"installedAt"`
	Components   PluginComponents `json:"components"`
}

// PluginInstalledManifest is the persisted installed.json (minimal on-disk format).
type PluginInstalledManifest struct {
	Plugins map[string]PluginInstalled `json:"plugins"`
}

// DanmoWorkExtensionKey is the extension namespace for danmo-work-specific plugin components.
const DanmoWorkExtensionKey = "ai.danmo.work"

var pluginNameRe = regexp.MustCompile(`^[a-z0-9][a-z0-9\-.]*[a-z0-9]$|^[a-z0-9]$`)

// ValidatePluginName checks the plugin name against Agent Plugins spec §5.5 constraints.
func ValidatePluginName(name string) error {
	if name == "" {
		return fmt.Errorf("name must not be empty")
	}
	if len(name) > 64 {
		return fmt.Errorf("name must be at most 64 characters")
	}
	if !pluginNameRe.MatchString(name) {
		return fmt.Errorf("name %q must contain only lowercase alphanumeric, hyphens, and periods, and must start/end with alphanumeric", name)
	}
	if strings.Contains(name, "--") || strings.Contains(name, "..") {
		return fmt.Errorf("name %q must not contain consecutive hyphens or periods", name)
	}
	return nil
}
