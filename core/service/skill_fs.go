package service

import (
	"danmo-work/core/domain"
	"danmo-work/core/paths"
)

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
