package home

import "embed"

// all:knowledge includes _meta.json (underscore files are otherwise omitted).
//
//go:embed agents skills all:knowledge manifest.yaml
var FS embed.FS
