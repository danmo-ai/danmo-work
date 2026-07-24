package paths

import (
	"os"
	"path/filepath"
)

// UserSkillDirs returns user-level custom skill roots, low priority → high priority.
// Later entries override earlier ones when skill IDs collide.
func UserSkillDirs() []string {
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		home = "."
	}
	return []string{
		filepath.Join(home, ".agents", "skills"),
		filepath.Join(home, DirName, "skills"),
	}
}

// ProjectSkillDirs returns project-level custom skill roots under workDir,
// low priority → high priority (.agents then .danmo-work).
func ProjectSkillDirs(workDir string) []string {
	if workDir == "" {
		return nil
	}
	root := filepath.Clean(workDir)
	return []string{
		filepath.Join(root, ".agents", "skills"),
		filepath.Join(root, DirName, "skills"),
	}
}

// AllSkillDirs returns user then project skill roots (low → high priority overall).
func AllSkillDirs(workDir string) []string {
	return append(UserSkillDirs(), ProjectSkillDirs(workDir)...)
}
