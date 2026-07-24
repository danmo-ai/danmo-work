// Package paths provides the unified user-home data layout for Danmo Work.
//
//	~/.danmo-work/
//	  config.yaml   — runtime config
//	  work.db       — SQLite database
//	  data/         — projects, turn logs, checkpoints
//	  bin/          — desktop sidecar binary (optional)
//	  bin/coreutils/— Windows Microsoft Coreutils multi-call + applet hardlinks (optional)
//	  backend.log   — desktop sidecar log (optional)
package paths

import (
	"io"
	"os"
	"path/filepath"
)

const DirName = ".danmo-work"

// Home returns ~/.danmo-work (absolute). Creates the directory if needed.
func Home() string {
	h, err := os.UserHomeDir()
	if err != nil || h == "" {
		h = "."
	}
	dir := filepath.Join(h, DirName)
	_ = os.MkdirAll(dir, 0o755)
	_ = os.MkdirAll(filepath.Join(dir, "data"), 0o755)
	return dir
}

// ConfigFile is ~/.danmo-work/config.yaml
func ConfigFile() string { return filepath.Join(Home(), "config.yaml") }

// DataDir is ~/.danmo-work/data
func DataDir() string { return filepath.Join(Home(), "data") }

// DatabaseFile is ~/.danmo-work/work.db
func DatabaseFile() string { return filepath.Join(Home(), "work.db") }

// ResolveAgainstHome joins a relative path to ~/.danmo-work; absolute paths pass through.
func ResolveAgainstHome(p string) string {
	if p == "" {
		return ""
	}
	if filepath.IsAbs(p) {
		return filepath.Clean(p)
	}
	return filepath.Join(Home(), p)
}

// MigrateLegacyOnce copies data from older locations into ~/.danmo-work when the
// new home is empty. Safe to call repeatedly — never overwrites existing files.
func MigrateLegacyOnce() {
	home := Home()
	dbDst := filepath.Join(home, "work.db")
	cfgDst := filepath.Join(home, "config.yaml")

	if _, err := os.Stat(dbDst); os.IsNotExist(err) {
		for _, src := range legacyDBCandidates() {
			if copyFileIfExists(src, dbDst) {
				break
			}
		}
	}

	if _, err := os.Stat(cfgDst); os.IsNotExist(err) {
		for _, src := range legacyConfigCandidates() {
			if copyFileIfExists(src, cfgDst) {
				break
			}
		}
	}
}

func legacyDBCandidates() []string {
	var out []string
	if h, err := os.UserHomeDir(); err == nil {
		out = append(out,
			// Pre-Danmo layout
			filepath.Join(h, ".dq-teams", "teams.db"),
			filepath.Join(h, ".dq-teams", "work.db"),
			filepath.Join(h, "Library", "Application Support", "com.danqing.teams", "teams.db"),
			filepath.Join(h, ".local", "share", "com.danqing.teams", "teams.db"),
		)
	}
	if cwd, err := os.Getwd(); err == nil {
		out = append(out,
			filepath.Join(cwd, "data", "work.db"),
			filepath.Join(cwd, "data", "teams.db"),
		)
	}
	return out
}

func legacyConfigCandidates() []string {
	var out []string
	if h, err := os.UserHomeDir(); err == nil {
		out = append(out,
			filepath.Join(h, ".dq-teams", "config.yaml"),
			filepath.Join(h, "Library", "Application Support", "com.danqing.teams", "danqing-teams.yaml"),
			filepath.Join(h, ".local", "share", "com.danqing.teams", "danqing-teams.yaml"),
		)
	}
	if cwd, err := os.Getwd(); err == nil {
		out = append(out,
			filepath.Join(cwd, ".danmo-work", "config.yaml"),
			filepath.Join(cwd, ".dq-teams", "config.yaml"),
		)
	}
	return out
}

func copyFileIfExists(src, dst string) bool {
	in, err := os.Open(src)
	if err != nil {
		return false
	}
	defer in.Close()
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return false
	}
	out, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_EXCL, 0o644)
	if err != nil {
		return false
	}
	defer out.Close()
	if _, err := io.Copy(out, in); err != nil {
		_ = os.Remove(dst)
		return false
	}
	return true
}
