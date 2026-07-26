package service

import (
	"context"

	"danmo-work/core/domain"
	"danmo-work/core/paths"
)

// SkillSourceBound is a skill from the agent's configured skillIds (DB).
const SkillSourceBound = "bound"

// SkillSourceFilesystem is a skill discovered via user/project directory scan.
const SkillSourceFilesystem = "filesystem"

// SkillSourceBoth is present in both agent bindings and filesystem scan.
const SkillSourceBoth = "both"

// AvailableSkill is a skill usable for an agent turn, with provenance for UI.
type AvailableSkill struct {
	domain.Skill
	Source string `json:"source"`
}

// ScanFilesystemSkills loads Agentskills-compliant skills from user and project
// directories. Missing directories and invalid SKILL.md entries are skipped.
// Later directories override earlier ones by skill ID (see paths.AllSkillDirs order).
// Does not write to the database.
func ScanFilesystemSkills(workDir string) ([]domain.Skill, map[string][]domain.SkillFile) {
	return ScanSkillDirs(paths.AllSkillDirs(workDir))
}

// ScanSkillDirs imports skills from each directory in order; later wins on ID collision.
func ScanSkillDirs(dirs []string) ([]domain.Skill, map[string][]domain.SkillFile) {
	imp := NewSkillImporter()
	byID := make(map[string]domain.Skill)
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
			byID[sk.ID] = sk
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

// MergeSkillsByID merges skill layers; later layers override earlier ones by ID.
// Relative order of first appearance is preserved for non-overridden IDs; overrides
// keep the position of the earlier entry.
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

// BoundDBSkills returns skills from the library that are listed on the agent.
func BoundDBSkills(all []domain.Skill, agent domain.Agent) []domain.Skill {
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

// ListAvailableSkillsForAgent merges agent-bound DB skills with filesystem
// skills for workDir — same composition as runtime resolveAgentSkills.
// Filesystem (Ambient) skills are included only when agent.InheritsAmbient().
// Body is cleared on returned skills (picker/metadata only).
func ListAvailableSkillsForAgent(ctx context.Context, skills *SkillManager, agent domain.Agent, workDir string) ([]AvailableSkill, error) {
	_ = ctx
	all, err := skills.List(ctx)
	if err != nil {
		return nil, err
	}
	bound := BoundDBSkills(all, agent)
	var fsSkills []domain.Skill
	if agent.InheritsAmbient() {
		fsSkills, _ = ScanFilesystemSkills(workDir)
	}

	boundIDs := make(map[string]struct{}, len(bound))
	for _, sk := range bound {
		boundIDs[sk.ID] = struct{}{}
	}
	fsIDs := make(map[string]struct{}, len(fsSkills))
	for _, sk := range fsSkills {
		fsIDs[sk.ID] = struct{}{}
	}

	merged := MergeSkillsByID(bound, fsSkills)
	out := make([]AvailableSkill, 0, len(merged))
	for _, sk := range merged {
		_, inBound := boundIDs[sk.ID]
		_, inFS := fsIDs[sk.ID]
		source := SkillSourceBound
		if inBound && inFS {
			source = SkillSourceBoth
		} else if inFS {
			source = SkillSourceFilesystem
		}
		sk.Body = ""
		out = append(out, AvailableSkill{Skill: sk, Source: source})
	}
	return out, nil
}
