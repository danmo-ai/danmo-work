package paths

import (
	"os"
	"path/filepath"
)

// GlobalSkillDirs returns global skill roots, low priority → high priority.
func GlobalSkillDirs() []string {
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		home = "."
	}
	return []string{
		filepath.Join(home, ".agents", "skills"),
		filepath.Join(home, DirName, "skills"),
	}
}

// GlobalSkillRoots returns global skill root dirs using the configured dataDir as danmo-work home.
func GlobalSkillRoots(dataDir string) []string {
	return []string{
		filepath.Join(dataDir, "skills"),
	}
}

// AgentsSkillDir returns ~/.agents/skills.
func AgentsSkillDir() string {
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		home = "."
	}
	return filepath.Join(home, ".agents", "skills")
}

// ProjectSkillDirs returns project-level orphan skill roots under workDir,
// low priority → high priority. These are always ambient (unbound).
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

// AllSkillDirs returns global then project skill roots (low → high priority).
func AllSkillDirs(workDir string) []string {
	return append(GlobalSkillDirs(), ProjectSkillDirs(workDir)...)
}
