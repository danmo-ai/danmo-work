package container

import (
	"os"
	"path/filepath"
	"strings"
)

// ResolveTarPath finds the optional env image tar (user-downloaded Release
// asset or local make build-env-tar). Not shipped inside app packages. Order:
//  1. explicit override
//  2. WORK_ENV_TAR
//  3. ~/.danmo-work/env/ host-arch specific, then alias
//  4. $DQ_ROOT/out/env or ./out/env (dev builds only, host-arch first)
func ResolveTarPath(override string) string {
	if p := strings.TrimSpace(override); p != "" {
		if fileExists(p) {
			return p
		}
		return ""
	}
	if p := strings.TrimSpace(os.Getenv("WORK_ENV_TAR")); p != "" && fileExists(p) {
		return p
	}
	arch := LinuxArch()
	candidates := []string{}
	if home, err := os.UserHomeDir(); err == nil {
		base := filepath.Join(home, ".danmo-work", "env")
		candidates = append(candidates, archTarNames(base, arch)...)
	}
	if root := strings.TrimSpace(os.Getenv("DQ_ROOT")); root != "" {
		candidates = append(candidates, archTarNames(filepath.Join(root, "out", "env"), arch)...)
	}
	candidates = append(candidates, archTarNames(filepath.Join("out", "env"), arch)...)
	for _, p := range candidates {
		if fileExists(p) {
			return p
		}
	}
	return ""
}

func archTarNames(dir, arch string) []string {
	return []string{
		filepath.Join(dir, "danmo-work-env-linux-"+arch+".tar.gz"),
		filepath.Join(dir, "danmo-work-env.tar.gz"),
		// legacy plain-tar installs (pre-gzip assets)
		filepath.Join(dir, "danmo-work-env-linux-"+arch+".tar"),
		filepath.Join(dir, "danmo-work-env.tar"),
	}
}

func fileExists(p string) bool {
	st, err := os.Stat(p)
	return err == nil && !st.IsDir()
}
