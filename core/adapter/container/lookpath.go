package container

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"

	"danmo-work/core/paths"
)

// lookCLI resolves a container engine binary. It first ensures common GUI-app
// PATH gaps are filled, then uses exec.LookPath, then checks well-known
// absolute install locations (Docker Desktop, Homebrew, Podman Desktop, …).
func lookCLI(name string) (string, error) {
	paths.EnsureHostPATH()
	if p, err := exec.LookPath(name); err == nil && p != "" {
		return p, nil
	}
	for _, candidate := range cliCandidatePaths(name) {
		if isExecutableFile(candidate) {
			return candidate, nil
		}
	}
	return "", exec.ErrNotFound
}

// cliCandidatePaths returns absolute fallbacks when PATH lookup fails.
// Overridable in tests.
var cliCandidatePaths = defaultCLICandidatePaths

func defaultCLICandidatePaths(name string) []string {
	home, _ := os.UserHomeDir()
	var dirs []string
	switch runtime.GOOS {
	case "darwin":
		dirs = []string{
			"/opt/homebrew/bin",
			"/usr/local/bin",
			"/opt/podman/bin",
			"/Applications/Docker.app/Contents/Resources/bin",
			filepath.Join(home, ".docker", "bin"),
			filepath.Join(home, ".orbstack", "bin"),
			filepath.Join(home, ".local", "bin"),
		}
		if name == "docker" {
			// OrbStack ships docker under xbin in some builds.
			dirs = append(dirs,
				"/Applications/OrbStack.app/Contents/MacOS/xbin",
				filepath.Join(home, ".orbstack", "bin"),
			)
		}
		if name == "container" {
			dirs = append(dirs, "/usr/local/bin", "/opt/homebrew/bin")
		}
	case "linux":
		dirs = []string{
			"/usr/local/bin",
			"/usr/bin",
			"/opt/podman/bin",
			filepath.Join(home, ".docker", "bin"),
			filepath.Join(home, ".local", "bin"),
		}
	default:
		return nil
	}
	out := make([]string, 0, len(dirs))
	seen := map[string]struct{}{}
	for _, d := range dirs {
		if d == "" {
			continue
		}
		p := filepath.Join(d, name)
		if _, ok := seen[p]; ok {
			continue
		}
		seen[p] = struct{}{}
		out = append(out, p)
	}
	return out
}

func isExecutableFile(path string) bool {
	st, err := os.Stat(path)
	if err != nil || st.IsDir() {
		return false
	}
	// On Unix, require at least one execute bit; Windows Stat mode is less reliable.
	if runtime.GOOS == "windows" {
		return true
	}
	return st.Mode()&0o111 != 0
}
