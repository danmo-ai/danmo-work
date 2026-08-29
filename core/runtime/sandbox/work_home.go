package sandbox

import (
	"path/filepath"
	"strings"

	"danmo-work/core/adapter/container"
	"danmo-work/core/paths"
)

func workHomePath() string {
	home, err := filepath.Abs(paths.Home())
	if err != nil || home == "" {
		return ""
	}
	return home
}

// ensureWorkHomeEnv sets WORK_HOME to the resolved data home so plugin scripts
// are reachable as ${WORK_HOME}/plugins/... in every sandbox backend.
func ensureWorkHomeEnv(env []string) []string {
	home := workHomePath()
	if home == "" {
		return env
	}
	out := make([]string, 0, len(env)+1)
	for _, e := range env {
		if strings.HasPrefix(e, "WORK_HOME=") {
			continue
		}
		out = append(out, e)
	}
	return append(out, "WORK_HOME="+home)
}

// workHomeBind is a read-only same-path mount of $WORK_HOME when it is not
// already the workspace root (typical: project under data/, plugins beside it).
func workHomeBind(workDirAbs string) (container.Bind, bool) {
	home := workHomePath()
	if home == "" || home == workDirAbs {
		return container.Bind{}, false
	}
	return container.Bind{Host: home, Container: home, ReadOnly: true}, true
}
