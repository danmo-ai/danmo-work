package service

import (
	"context"

	"danmo-work/core/domain"
	"danmo-work/core/paths"
)

const SkillSourceBound = "bound"
const SkillSourceFilesystem = "filesystem"
const SkillSourceBoth = "both"
const SkillSourceOrphan = "orphan"

type AvailableSkill struct {
	domain.Skill
	Source string `json:"source"`
}

// ScanAllSkills loads skills from Home, plugin, then project directories.
// ID is the path relative to each scan root ({xxx/xxx}/SKILL.md). Same ID keeps
// the higher-priority copy: project → plugin → Home.
func ScanAllSkills(dataDir, projectDir string, pluginDirs ...string) []domain.Skill {
	if len(pluginDirs) == 0 {
		pluginDirs = paths.PluginSkillDirs(dataDir)
	}
	var dirs []string
	dirs = append(dirs, paths.HomeSkillDirs(dataDir)...)
	dirs = append(dirs, pluginDirs...)
	dirs = append(dirs, paths.ProjectSkillDirs(projectDir)...)
	skills, _ := ScanSkillDirs(dirs)
	return skills
}

func ScanSkillDirs(dirs []string) ([]domain.Skill, map[string][]domain.SkillFile) {
	imp := NewSkillImporter()
	byID := make(map[string]domain.Skill)
	dirByID := make(map[string]string)
	filesByID := make(map[string][]domain.SkillFile)
	var order []string

	for _, dir := range dirs {
		skills, files, err := imp.ImportAll(dir)
		if err != nil {
			continue
		}
		filesForDir := groupSkillFiles(files)
		for _, sk := range skills {
			if _, exists := byID[sk.ID]; !exists {
				order = append(order, sk.ID)
			}
			if sk.Dir == "" {
				sk.Dir = paths.JoinSkillID(dir, sk.ID)
			}
			byID[sk.ID] = sk
			dirByID[sk.ID] = dir
			if sf, ok := filesForDir[sk.ID]; ok {
				filesByID[sk.ID] = sf
			} else {
				filesByID[sk.ID] = nil
			}
		}
	}

	out := make([]domain.Skill, 0, len(order))
	for _, id := range order {
		out = append(out, byID[id])
	}
	return out, filesByID
}

func groupSkillFiles(files []domain.SkillFile) map[string][]domain.SkillFile {
	out := make(map[string][]domain.SkillFile)
	for _, f := range files {
		out[f.SkillID] = append(out[f.SkillID], f)
	}
	return out
}

func MergeSkillsByID(layers ...[]domain.Skill) []domain.Skill {
	byID := make(map[string]domain.Skill)
	var order []string
	for _, layer := range layers {
		for _, sk := range layer {
			if _, exists := byID[sk.ID]; !exists {
				order = append(order, sk.ID)
			}
			byID[sk.ID] = sk
		}
	}
	out := make([]domain.Skill, 0, len(order))
	for _, id := range order {
		out = append(out, byID[id])
	}
	return out
}

func BoundSkills(all []domain.Skill, agent domain.Agent) []domain.Skill {
	if len(agent.SkillIDs) == 0 {
		return nil
	}
	wanted := make(map[string]struct{}, len(agent.SkillIDs))
	for _, id := range agent.SkillIDs {
		wanted[id] = struct{}{}
	}
	var result []domain.Skill
	for _, sk := range all {
		if _, ok := wanted[sk.ID]; ok {
			result = append(result, sk)
		}
	}
	return result
}

// OrphanSkills returns global skills NOT bound to the agent.
func OrphanSkills(all []domain.Skill, agent domain.Agent) []domain.Skill {
	if len(agent.SkillIDs) == 0 {
		return all
	}
	bound := make(map[string]struct{}, len(agent.SkillIDs))
	for _, id := range agent.SkillIDs {
		bound[id] = struct{}{}
	}
	var result []domain.Skill
	for _, sk := range all {
		if _, ok := bound[sk.ID]; !ok {
			result = append(result, sk)
		}
	}
	return result
}

func ListAvailableSkillsForAgent(ctx context.Context, skills *SkillManager, agent domain.Agent, workDir string) ([]AvailableSkill, error) {
	_ = ctx
	var pluginDirs []string
	var dataDir string
	if skills != nil {
		pluginDirs = skills.PluginSkillDirs()
		dataDir = skills.DataDir()
	}
	all := ScanAllSkills(dataDir, workDir, pluginDirs...)

	bound := BoundSkills(all, agent)
	boundIDs := make(map[string]struct{}, len(bound))
	for _, sk := range bound {
		boundIDs[sk.ID] = struct{}{}
	}

	var orphan []domain.Skill
	if agent.Mode != domain.AgentModeSubagent {
		orphan = OrphanSkills(all, agent)
	}

	merged := MergeSkillsByID(bound, orphan)
	out := make([]AvailableSkill, 0, len(merged))
	for _, sk := range merged {
		_, inBound := boundIDs[sk.ID]
		source := SkillSourceBound
		if !inBound {
			source = SkillSourceOrphan
		}
		sk.Body = ""
		out = append(out, AvailableSkill{Skill: sk, Source: source})
	}
	return out, nil
}
