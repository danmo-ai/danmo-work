package sandbox

import (
	"path/filepath"
	"runtime"
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

// ensureUTF8Env forces UTF-8 for Python and (on non-Windows) libc locale so
// exec_shell cannot silently write GBK/GB18030 via system defaults.
// Existing LANG/LC_ALL values are preserved when already set.
func ensureUTF8Env(env []string) []string {
	set := map[string]string{
		"PYTHONUTF8":       "1",
		"PYTHONIOENCODING": "utf-8",
	}
	if runtime.GOOS != "windows" {
		set["LC_ALL"] = "C.UTF-8"
		set["LANG"] = "C.UTF-8"
	}
	have := make(map[string]bool, len(env))
	out := make([]string, 0, len(env)+len(set))
	for _, e := range env {
		key, _, ok := strings.Cut(e, "=")
		if !ok {
			out = append(out, e)
			continue
		}
		have[key] = true
		if key == "PYTHONUTF8" || key == "PYTHONIOENCODING" {
			// Always override Python charset knobs.
			continue
		}
		out = append(out, e)
	}
	for k, v := range set {
		if k == "LANG" || k == "LC_ALL" {
			if have[k] {
				continue
			}
		}
		out = append(out, k+"="+v)
	}
	return out
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
