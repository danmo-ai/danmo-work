package plugins

import "embed"

// FS holds first-party builtin plugins shipped with the binary.
// Each top-level directory is one plugin (plugin.json + components).
// Subagent experts live here; shared skills stay in core/resource/home.
// Primary lead agent (team) remains in home.
//
//go:embed all:github all:danmo-make all:novel all:browser all:operator all:implementer all:explorer all:reviewer all:researcher all:document all:data
var FS embed.FS
