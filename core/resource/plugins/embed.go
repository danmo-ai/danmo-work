package plugins

import "embed"

// FS holds first-party builtin plugins shipped with the binary.
// Each top-level directory is one plugin (plugin.json + components).
//
//go:embed all:github all:danmo-make all:novel all:browser
var FS embed.FS
