package prompt

import "embed"

//go:embed builtin/*
var BuiltinFS embed.FS

//go:embed knowledge/novel-craft/*.md
var NovelCraftKnowledge embed.FS
