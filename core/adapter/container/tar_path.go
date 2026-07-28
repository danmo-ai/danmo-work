package container

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

// ResolveTarPath finds the bundled env image tar. Order:
//  1. explicit override
//  2. WORK_ENV_TAR
//  3. ~/.danmo-work/env/danmo-work-env.tar (and arch-specific names)
//  4. next to executable: env/danmo-work-env*.tar
//  5. $DQ_ROOT/out/env or ./out/env (dev)
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
	candidates := []string{}
	if home, err := os.UserHomeDir(); err == nil {
		base := filepath.Join(home, ".danmo-work", "env")
		candidates = append(candidates, archTarNames(base)...)
	}
	if exe, err := os.Executable(); err == nil {
		dir := filepath.Join(filepath.Dir(exe), "env")
		candidates = append(candidates, archTarNames(dir)...)
	}
	if root := strings.TrimSpace(os.Getenv("DQ_ROOT")); root != "" {
		candidates = append(candidates, archTarNames(filepath.Join(root, "out", "env"))...)
	}
	candidates = append(candidates, archTarNames(filepath.Join("out", "env"))...)
	for _, p := range candidates {
		if fileExists(p) {
			return p
		}
	}
	return ""
}

func archTarNames(dir string) []string {
	arch := runtime.GOARCH
	return []string{
		filepath.Join(dir, "danmo-work-env.tar"),
		filepath.Join(dir, "danmo-work-env-linux-"+arch+".tar"),
		filepath.Join(dir, "danmo-work-env-linux-amd64.tar"),
		filepath.Join(dir, "danmo-work-env-linux-arm64.tar"),
	}
}

func fileExists(p string) bool {
	st, err := os.Stat(p)
	return err == nil && !st.IsDir()
}
