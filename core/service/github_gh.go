package service

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

const (
	// GitHubExpertID is the builtin GitHub expert agent / skill id (gh CLI pack).
	GitHubExpertID = "github"
	ghBinName      = "gh"
)

var ghHomeBinDir = userHomeDanmoBin

// ResolveGhBin returns the path to the local GitHub CLI.
// Order: WORK_GH_BIN → ~/.danmo-work/bin/gh → PATH.
func ResolveGhBin() string {
	if p := strings.TrimSpace(os.Getenv("WORK_GH_BIN")); p != "" {
		if st, err := os.Stat(p); err == nil && !st.IsDir() {
			return p
		}
	}
	homeBin := filepath.Join(ghHomeBinDir(), ghExecutableName())
	if st, err := os.Stat(homeBin); err == nil && !st.IsDir() {
		return homeBin
	}
	if p, err := exec.LookPath(ghBinName); err == nil {
		return p
	}
	return ""
}

func ghExecutableName() string {
	if runtime.GOOS == "windows" {
		return ghBinName + ".exe"
	}
	return ghBinName
}

// GitHubGhHint prepends context for the github expert child turn.
func GitHubGhHint(binPath string) string {
	if strings.TrimSpace(binPath) == "" {
		return "[github-gh: missing] gh CLI not found (PATH / WORK_GH_BIN / ~/.danmo-work/bin). Do not invent GitHub results; report install + gh auth login."
	}
	return "[github-gh: ready] bin=" + binPath + " — use exec_shell → gh; verify auth with gh auth status before mutating."
}
