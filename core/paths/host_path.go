package paths

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
)

var ensureHostPATHOnce sync.Once

// EnsureHostPATH prepends common third-party bin directories that are missing
// from PATH. macOS GUI apps launched from Finder/Dock/Tauri often inherit only
// /usr/bin:/bin:/usr/sbin:/sbin, so Homebrew / Docker Desktop / Podman CLIs
// are invisible to exec.LookPath until this runs.
func EnsureHostPATH() {
	ensureHostPATHOnce.Do(applyHostPATH)
}

func applyHostPATH() {
	if newPATH, ok := mergeHostPATH(os.Getenv("PATH"), hostExtraBinDirs()); ok {
		_ = os.Setenv("PATH", newPATH)
	}
}

// mergeHostPATH prepends existing dirs from extra that are not already in pathEnv.
// Returns the new PATH and whether it changed.
func mergeHostPATH(pathEnv string, extra []string) (string, bool) {
	sep := string(os.PathListSeparator)
	var parts []string
	if strings.TrimSpace(pathEnv) != "" {
		parts = strings.Split(pathEnv, sep)
	}
	have := make(map[string]struct{}, len(parts)+len(extra))
	for _, p := range parts {
		if p == "" {
			continue
		}
		have[p] = struct{}{}
	}
	var prepend []string
	for _, d := range extra {
		d = strings.TrimSpace(d)
		if d == "" {
			continue
		}
		if _, ok := have[d]; ok {
			continue
		}
		st, err := os.Stat(d)
		if err != nil || !st.IsDir() {
			continue
		}
		prepend = append(prepend, d)
		have[d] = struct{}{}
	}
	if len(prepend) == 0 {
		return pathEnv, false
	}
	return strings.Join(append(prepend, parts...), sep), true
}

// hostExtraBinDirs lists well-known CLI locations for the current OS/user.
func hostExtraBinDirs() []string {
	home, _ := os.UserHomeDir()
	switch runtime.GOOS {
	case "darwin":
		dirs := []string{
			"/opt/homebrew/bin",
			"/opt/homebrew/sbin",
			"/usr/local/bin",
			"/usr/local/sbin",
			"/opt/local/bin", // MacPorts
			"/opt/podman/bin",
			"/Applications/Docker.app/Contents/Resources/bin",
			filepath.Join(home, ".docker", "bin"),
			filepath.Join(home, ".orbstack", "bin"),
			filepath.Join(home, ".local", "bin"),
			filepath.Join(home, "bin"),
		}
		if home != "" {
			dirs = append(dirs, filepath.Join(home, DirName, "bin"))
		}
		return dirs
	case "linux":
		dirs := []string{
			"/usr/local/bin",
			"/opt/podman/bin",
			filepath.Join(home, ".docker", "bin"),
			filepath.Join(home, ".local", "bin"),
			filepath.Join(home, "bin"),
		}
		if home != "" {
			dirs = append(dirs, filepath.Join(home, DirName, "bin"))
		}
		return dirs
	default:
		return nil
	}
}
