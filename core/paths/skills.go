package paths

import (
	"os"
	"path/filepath"
	"strings"
)

const (
	PhDanmoWorkHome = "{danmo_work_home}"
	PhAgentsHome    = "{agents_home}"
	PhProject       = "{project}"
	PhWorkHome      = "{work_home}"
)

// SkillRootMapping maps an on-disk skill root to the placeholder prefix shown
// in the skills library UI and in <available_skills><path>.
type SkillRootMapping struct {
	AbsRoot           string
	PlaceholderPrefix string
}

// WorkHomeFromDataDir returns the parent of dataDir, matching PluginManager
// layout ({dataDir}/../plugins). Empty dataDir falls back to Home().
func WorkHomeFromDataDir(dataDir string) string {
	if dataDir == "" {
		return Home()
	}
	return filepath.Clean(filepath.Join(dataDir, ".."))
}

// PluginsDir is {dataDir}/../plugins.
func PluginsDir(dataDir string) string {
	return filepath.Join(WorkHomeFromDataDir(dataDir), "plugins")
}

// PluginSkillDirs scans {dataDir}/../plugins/*/skills for installed plugin skill roots.
func PluginSkillDirs(dataDir string) []string {
	root := PluginsDir(dataDir)
	entries, err := os.ReadDir(root)
	if err != nil {
		return nil
	}
	var out []string
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		sd := filepath.Join(root, e.Name(), "skills")
		st, err := os.Stat(sd)
		if err != nil || !st.IsDir() {
			continue
		}
		out = append(out, sd)
	}
	return out
}

// PathUnderRoot reports whether p is root or a descendant of root.
func PathUnderRoot(p, root string) bool {
	if p == "" || root == "" {
		return false
	}
	p = filepath.Clean(p)
	root = filepath.Clean(root)
	if p == root {
		return true
	}
	return strings.HasPrefix(p, root+string(filepath.Separator))
}

// SkillRootMappings returns skill directory roots and their placeholder prefixes.
// pluginSkillDirs are absolute plugin skills/ directories (e.g. .../plugins/name/skills).
func SkillRootMappings(dataDir, agentsHome, projectDir string, pluginSkillDirs []string) []SkillRootMapping {
	var out []SkillRootMapping
	if dataDir != "" {
		out = append(out, SkillRootMapping{
			AbsRoot:           filepath.Clean(filepath.Join(dataDir, "skills")),
			PlaceholderPrefix: PhDanmoWorkHome + "/skills",
		})
	}
	if agentsHome != "" {
		out = append(out, SkillRootMapping{
			AbsRoot:           filepath.Clean(filepath.Join(agentsHome, "skills")),
			PlaceholderPrefix: PhAgentsHome + "/skills",
		})
	}
	if projectDir != "" {
		out = append(out, SkillRootMapping{
			AbsRoot:           filepath.Clean(filepath.Join(projectDir, DirName, "skills")),
			PlaceholderPrefix: PhProject + "/" + DirName + "/skills",
		})
		out = append(out, SkillRootMapping{
			AbsRoot:           filepath.Clean(filepath.Join(projectDir, ".agents", "skills")),
			PlaceholderPrefix: PhProject + "/.agents/skills",
		})
	}
	workHome := WorkHomeFromDataDir(dataDir)
	for _, dir := range pluginSkillDirs {
		abs := filepath.Clean(dir)
		if absPath, err := filepath.Abs(dir); err == nil {
			abs = absPath
		}
		rel, err := filepath.Rel(workHome, abs)
		if err != nil || rel == "." || strings.HasPrefix(rel, "..") {
			continue
		}
		out = append(out, SkillRootMapping{
			AbsRoot:           abs,
			PlaceholderPrefix: PhWorkHome + "/" + filepath.ToSlash(rel),
		})
	}
	return out
}

// FormatSkillPath converts an absolute skill directory to placeholder form using
// the longest matching AbsRoot. Unmatched paths are returned cleaned as-is.
func FormatSkillPath(skillDir string, mappings []SkillRootMapping) string {
	if skillDir == "" {
		return ""
	}
	skillDir = filepath.Clean(skillDir)
	bestIdx := -1
	bestLen := -1
	for i, m := range mappings {
		root := filepath.Clean(m.AbsRoot)
		if !PathUnderRoot(skillDir, root) {
			continue
		}
		if len(root) > bestLen {
			bestLen = len(root)
			bestIdx = i
		}
	}
	if bestIdx < 0 {
		return skillDir
	}
	m := mappings[bestIdx]
	rel, err := filepath.Rel(filepath.Clean(m.AbsRoot), skillDir)
	if err != nil {
		return skillDir
	}
	rel = filepath.ToSlash(rel)
	if rel == "." {
		return m.PlaceholderPrefix
	}
	return m.PlaceholderPrefix + "/" + rel
}

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

// HomeSkillDirs returns Home-tier skill roots, low → high priority:
// ~/.agents/skills, $WORK_HOME/skills, {dataDir}/skills (deduped).
func HomeSkillDirs(dataDir string) []string {
	seen := make(map[string]struct{}, 3)
	var out []string
	add := func(d string) {
		d = filepath.Clean(d)
		if d == "" || d == "." {
			return
		}
		if _, ok := seen[d]; ok {
			return
		}
		seen[d] = struct{}{}
		out = append(out, d)
	}
	add(AgentsSkillDir())
	add(filepath.Join(WorkHomeFromDataDir(dataDir), "skills"))
	if dataDir != "" {
		add(filepath.Join(dataDir, "skills"))
	}
	return out
}

// JoinSkillID joins a scan root with a relative skill id (slash-separated).
// Returns empty when id is empty or contains "..".
func JoinSkillID(root, id string) string {
	id = strings.Trim(filepath.ToSlash(id), "/")
	if root == "" || id == "" || strings.Contains(id, "..") {
		return ""
	}
	parts := strings.Split(id, "/")
	return filepath.Join(append([]string{root}, parts...)...)
}

// HasSKILLMD reports whether dir contains a SKILL.md file.
func HasSKILLMD(dir string) bool {
	if dir == "" {
		return false
	}
	st, err := os.Stat(filepath.Join(dir, "SKILL.md"))
	return err == nil && !st.IsDir()
}

// SkillLookupRoots returns skill scan roots in lookup order (first match wins):
// project (high) → plugin → Home (low). Within each tier, later entries win.
func SkillLookupRoots(home, plugin, project []string) []string {
	var out []string
	appendRev := func(dirs []string) {
		for i := len(dirs) - 1; i >= 0; i-- {
			d := strings.TrimSpace(dirs[i])
			if d != "" {
				out = append(out, d)
			}
		}
	}
	appendRev(project)
	appendRev(plugin)
	appendRev(home)
	return out
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
