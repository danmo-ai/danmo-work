package prompt

import "embed"

//go:embed agents/*.md
var AgentTemplates embed.FS

//go:embed skills/*
var SkillTemplates embed.FS

//go:embed knowledge/novel-craft/*.md
var NovelCraftKnowledge embed.FS
