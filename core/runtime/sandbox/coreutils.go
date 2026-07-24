package sandbox

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
)

// Env vars for Windows Coreutils (Microsoft / uutils multi-call binary).
const (
	envCoreutilsBin = "WORK_COREUTILS_BIN" // directory with ls.exe / cat.exe hardlinks
	envCoreutilsExe = "WORK_COREUTILS_EXE" // absolute path to coreutils.exe (optional)
)

// coreutilsApplets are hardlink names created next to a bundled coreutils.exe.
// Matches the Microsoft Coreutils for Windows multi-call surface (preview).
var coreutilsApplets = []string{
	"arch", "b2sum", "base32", "base64", "basename", "basenc", "cat", "cksum",
	"comm", "cp", "csplit", "cut", "date", "df", "dirname", "du", "echo", "env",
	"expr", "factor", "false", "find", "fmt", "fold", "grep", "head", "hostname",
	"join", "link", "ln", "ls", "md5sum", "mkdir", "mktemp", "mv", "nl", "nproc",
	"numfmt", "od", "paste", "pathchk", "pr", "printenv", "printf", "ptx", "pwd",
	"readlink", "realpath", "rm", "rmdir", "seq", "sha1sum", "sha224sum", "sha256sum",
	"sha384sum", "sha512sum", "shuf", "sleep", "sort", "split", "stat", "sum",
	"tac", "tail", "tee", "test", "touch", "tr", "true", "truncate", "tsort",
	"unexpand", "uniq", "unlink", "uptime", "wc", "xargs", "yes",
	"coreutils-manager",
}

var (
	coreutilsOnce sync.Once
	coreutilsBin  string
)

// findCoreutilsBin returns a directory that contains Coreutils applets (e.g. ls.exe).
// Empty when unavailable. Cached after first probe; call resetCoreutilsCache in tests.
func findCoreutilsBin() string {
	if runtime.GOOS != "windows" {
		return ""
	}
	coreutilsOnce.Do(func() {
		coreutilsBin = probeCoreutilsBin()
	})
	return coreutilsBin
}

func resetCoreutilsCache() {
	coreutilsOnce = sync.Once{}
	coreutilsBin = ""
}

// coreutilsCandidateBins is overridable in tests.
var coreutilsCandidateBins = defaultCoreutilsCandidateBins

func defaultCoreutilsCandidateBins() []string {
	var out []string
	add := func(p string) {
		if p != "" {
			out = append(out, p)
		}
	}
	if v := strings.TrimSpace(os.Getenv(envCoreutilsBin)); v != "" {
		add(v)
	}
	if home, err := os.UserHomeDir(); err == nil && home != "" {
		add(filepath.Join(home, ".danmo-work", "bin", "coreutils", "bin"))
	}
	if pf := os.Getenv("ProgramFiles"); pf != "" {
		add(filepath.Join(pf, "coreutils", "bin"))
	}
	if local := os.Getenv("LOCALAPPDATA"); local != "" {
		add(filepath.Join(local, "Programs", "coreutils", "bin"))
	}
	if exe, err := os.Executable(); err == nil {
		dir := filepath.Dir(exe)
		add(filepath.Join(dir, "coreutils", "bin"))
		add(filepath.Join(dir, "resources", "coreutils", "bin"))
		// Tauri resource layout next to sidecar / app exe.
		add(filepath.Join(dir, "..", "resources", "coreutils", "bin"))
	}
	return out
}

func probeCoreutilsBin() string {
	for _, dir := range coreutilsCandidateBins() {
		if isCoreutilsBinDir(dir) {
			return dir
		}
	}
	// Try to materialize from a bundled/system coreutils.exe once.
	if exe := findCoreutilsExe(); exe != "" {
		if bin, err := prepareCoreutilsFromExe(exe); err == nil && isCoreutilsBinDir(bin) {
			return bin
		}
	}
	return ""
}

func isCoreutilsBinDir(dir string) bool {
	if dir == "" {
		return false
	}
	// ls.exe is the smoke check; also accept coreutils.exe in parent.
	if fileExists(filepath.Join(dir, "ls.exe")) || fileExists(filepath.Join(dir, "cat.exe")) {
		return true
	}
	return false
}

// coreutilsExeCandidates is overridable in tests.
var coreutilsExeCandidates = defaultCoreutilsExeCandidates

func defaultCoreutilsExeCandidates() []string {
	var out []string
	add := func(p string) {
		if p != "" {
			out = append(out, p)
		}
	}
	if v := strings.TrimSpace(os.Getenv(envCoreutilsExe)); v != "" {
		add(v)
	}
	if home, err := os.UserHomeDir(); err == nil && home != "" {
		add(filepath.Join(home, ".danmo-work", "bin", "coreutils", "coreutils.exe"))
	}
	if pf := os.Getenv("ProgramFiles"); pf != "" {
		add(filepath.Join(pf, "coreutils", "coreutils.exe"))
	}
	if exe, err := os.Executable(); err == nil {
		dir := filepath.Dir(exe)
		add(filepath.Join(dir, "coreutils", "coreutils.exe"))
		add(filepath.Join(dir, "resources", "coreutils", "coreutils.exe"))
		add(filepath.Join(dir, "..", "resources", "coreutils", "coreutils.exe"))
	}
	return out
}

func findCoreutilsExe() string {
	for _, p := range coreutilsExeCandidates() {
		if fileExists(p) {
			return p
		}
	}
	return ""
}

// prepareCoreutilsFromExe installs hardlinks under ~/.danmo-work/bin/coreutils/bin
// (or next to the given exe when already under that tree).
func prepareCoreutilsFromExe(exePath string) (string, error) {
	exePath = filepath.Clean(exePath)
	appDir := filepath.Dir(exePath)
	// Prefer stable user-local install so hardlinks survive app updates.
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		home = appDir
	}
	dstRoot := filepath.Join(home, ".danmo-work", "bin", "coreutils")
	dstExe := filepath.Join(dstRoot, "coreutils.exe")
	binDir := filepath.Join(dstRoot, "bin")

	if err := os.MkdirAll(binDir, 0o755); err != nil {
		return "", err
	}
	if err := syncFile(exePath, dstExe); err != nil {
		return "", err
	}
	if err := ensureCoreutilsHardlinks(dstExe, binDir); err != nil {
		return "", err
	}
	return binDir, nil
}

func syncFile(src, dst string) error {
	if src == dst {
		return nil
	}
	si, err := os.Stat(src)
	if err != nil {
		return err
	}
	if di, err := os.Stat(dst); err == nil {
		if di.Size() == si.Size() && !si.ModTime().After(di.ModTime()) {
			return nil
		}
	}
	in, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	tmp := dst + ".tmp"
	if err := os.WriteFile(tmp, in, 0o755); err != nil {
		return err
	}
	_ = os.Remove(dst)
	return os.Rename(tmp, dst)
}

func ensureCoreutilsHardlinks(coreutilsExe, binDir string) error {
	if err := os.MkdirAll(binDir, 0o755); err != nil {
		return err
	}
	for _, name := range coreutilsApplets {
		link := filepath.Join(binDir, name+".exe")
		if fileExists(link) {
			continue
		}
		if err := os.Link(coreutilsExe, link); err != nil {
			// Cross-volume or FS without hardlinks: copy (larger, still works).
			if copyErr := copyFileBytes(coreutilsExe, link); copyErr != nil {
				return copyErr
			}
		}
	}
	return nil
}

func copyFileBytes(src, dst string) error {
	b, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	return os.WriteFile(dst, b, 0o755)
}

// prependCoreutilsPATH injects the Coreutils bin directory at the front of PATH.
func prependCoreutilsPATH(env []string) []string {
	bin := findCoreutilsBin()
	if bin == "" {
		return env
	}
	return prependPathEnv(env, bin)
}

func prependPathEnv(env []string, dir string) []string {
	if dir == "" {
		return env
	}
	dir = filepath.Clean(dir)
	out := make([]string, 0, len(env)+1)
	found := false
	for _, e := range env {
		key, val, ok := strings.Cut(e, "=")
		if !ok || !strings.EqualFold(key, "PATH") {
			out = append(out, e)
			continue
		}
		found = true
		parts := strings.Split(val, string(os.PathListSeparator))
		if len(parts) > 0 && filepath.Clean(parts[0]) == dir {
			out = append(out, e)
			continue
		}
		out = append(out, key+"="+dir+string(os.PathListSeparator)+val)
	}
	if !found {
		out = append(out, "PATH="+dir)
	}
	return out
}
