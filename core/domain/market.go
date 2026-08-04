package domain

// MarketSource is a configured content source for the expert/skill/connector market.
type MarketSource struct {
	ID          string `json:"id" mapstructure:"id" yaml:"id"`
	Name        string `json:"name" mapstructure:"name" yaml:"name"`
	Kind        string `json:"kind" mapstructure:"kind" yaml:"kind"` // git | clawhub | techleads
	Platform    string `json:"platform,omitempty" mapstructure:"platform" yaml:"platform,omitempty"`
	Repo        string `json:"repo,omitempty" mapstructure:"repo" yaml:"repo,omitempty"`
	Ref         string `json:"ref,omitempty" mapstructure:"ref" yaml:"ref,omitempty"`
	CatalogPath string `json:"catalogPath,omitempty" mapstructure:"catalog_path" yaml:"catalog_path,omitempty"`
	Token       string `json:"token,omitempty" mapstructure:"token" yaml:"token,omitempty"`
	Enabled     bool   `json:"enabled" mapstructure:"enabled" yaml:"enabled"`
	Priority    int    `json:"priority" mapstructure:"priority" yaml:"priority"`
}

// ConfigMarketSection is persisted under market in config.yaml.
type ConfigMarketSection struct {
	CacheTTLHours int            `json:"cacheTtlHours" mapstructure:"cache_ttl_hours" yaml:"cache_ttl_hours"`
	Sources       []MarketSource `json:"sources" mapstructure:"sources" yaml:"sources"`
}

// MarketItemKind classifies a catalog entry.
type MarketItemKind string

const (
	MarketKindSkill     MarketItemKind = "skill"
	MarketKindExpert    MarketItemKind = "expert"
	MarketKindConnector MarketItemKind = "connector"
	MarketKindBundle    MarketItemKind = "bundle"
)

// MarketItem is one installable entry from a source catalog.
type MarketItem struct {
	Kind          MarketItemKind `json:"kind"`
	ID            string         `json:"id"`
	Name          string         `json:"name"`
	Description   string         `json:"description,omitempty"`
	Keywords      []string       `json:"keywords,omitempty"`
	Category      string         `json:"category,omitempty"`
	Version       string         `json:"version,omitempty"`
	License       string         `json:"license,omitempty"`
	Author        string         `json:"author,omitempty"`
	Path          string         `json:"path"`
	SkillDeps     []string       `json:"skillDeps,omitempty"`
	ConnectorDeps []string       `json:"connectorDeps,omitempty"` // expert/bundle → connectors
	// Deps maps platform → relative install script path inside the connector package
	// (e.g. "darwin":"deps/darwin.sh"). Empty means use package convention deps/.
	Deps map[string]string `json:"deps,omitempty"`
	// UninstallDeps maps platform → relative uninstall/cleanup script (optional).
	UninstallDeps map[string]string `json:"uninstallDeps,omitempty"`
	UpdatedAt     string            `json:"updatedAt,omitempty"`
	Compatibility string            `json:"compatibility,omitempty"`
}

// MarketCatalog is the parsed catalog/index.json for one source.
type MarketCatalog struct {
	SchemaVersion int          `json:"schemaVersion"`
	Items         []MarketItem `json:"items"`
	SourceID      string       `json:"sourceId,omitempty"`
}

// MarketListing is a catalog item enriched for multi-source browsing.
type MarketListing struct {
	MarketItem
	SourceID   string `json:"sourceId"`
	SourceName string `json:"sourceName,omitempty"`
	Installed  bool   `json:"installed,omitempty"`
}

// MarketCatalogResponse is the API payload for GET /market/catalog.
type MarketCatalogResponse struct {
	Items    []MarketListing `json:"items"`
	Warnings []string        `json:"warnings,omitempty"`
}

// InstallMarketRequest installs a catalog item from a source.
type InstallMarketRequest struct {
	SourceID  string `json:"sourceId"`
	Kind      string `json:"kind"`
	ID        string `json:"id"`
	Ref       string `json:"ref,omitempty"`
	Overwrite bool   `json:"overwrite,omitempty"`
}

// ConnectorDepsRun records one deps/uninstall script execution during market install/uninstall.
type ConnectorDepsRun struct {
	ConnectorID string `json:"connectorId"`
	Script      string `json:"script,omitempty"`
	Log         string `json:"log,omitempty"`
	Phase       string `json:"phase"` // install | uninstall
}

// InstallMarketResult summarizes what was written locally.
type InstallMarketResult struct {
	Kind       string             `json:"kind"`
	ID         string             `json:"id"`
	SourceID   string             `json:"sourceId"`
	Ref        string             `json:"ref,omitempty"`
	Version    string             `json:"version,omitempty"`
	Installed  []string           `json:"installed,omitempty"` // ids written (skill/expert/connector)
	DepsRuns   []ConnectorDepsRun `json:"depsRuns,omitempty"`  // per-connector install script logs
	DepsScript string             `json:"depsScript,omitempty"` // last install script (compat)
	DepsLog    string             `json:"depsLog,omitempty"`    // last install log (compat)
	Skipped    []string           `json:"skipped,omitempty"`
}

// UninstallMarketRequest removes a market-installed skill, expert, or connector.
type UninstallMarketRequest struct {
	Kind       string `json:"kind"`
	ID         string `json:"id"`
	RunCleanup bool   `json:"runCleanup,omitempty"` // connector: run optional uninstall deps script
	SourceID   string `json:"sourceId,omitempty"`   // preferred market source for package fetch
	Ref        string `json:"ref,omitempty"`
}

// UninstallMarketResult summarizes connector cleanup + removal.
type UninstallMarketResult struct {
	Kind          string             `json:"kind"`
	ID            string             `json:"id"`
	CleanupRuns   []ConnectorDepsRun `json:"cleanupRuns,omitempty"`
	CleanupScript string             `json:"cleanupScript,omitempty"`
	CleanupLog    string             `json:"cleanupLog,omitempty"`
}
