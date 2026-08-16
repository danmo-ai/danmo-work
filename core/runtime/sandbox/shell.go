package sandbox

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"

	"danmo-work/core/domain"
)

// resolvedShell is the interpreter used for host / win-token exec_shell invocations.
type resolvedShell struct {
	kind         string // sh | cmd | git-bash | wsl-bash
	label        string
	path         string // absolute bash.exe for git-bash
	coreutilsBin string // Windows Coreutils applet dir prepended to PATH when set
	err          error  // non-nil when preference cannot be satisfied
}

// gitBashCandidatePaths is overridable in tests.
var gitBashCandidatePaths = defaultGitBashCandidatePaths

func defaultGitBashCandidatePaths() []string {
	var out []string
	add := func(parts ...string) {
		p := filepath.Join(parts...)
		if p != "" {
			out = append(out, p)
		}
	}
	if pf := os.Getenv("ProgramFiles"); pf != "" {
		add(pf, "Git", "bin", "bash.exe")
		add(pf, "Git", "usr", "bin", "bash.exe")
	}
	if pf86 := os.Getenv("ProgramFiles(x86)"); pf86 != "" {
		add(pf86, "Git", "bin", "bash.exe")
		add(pf86, "Git", "usr", "bin", "bash.exe")
	}
	if local := os.Getenv("LOCALAPPDATA"); local != "" {
		add(local, "Programs", "Git", "bin", "bash.exe")
		add(local, "Programs", "Git", "usr", "bin", "bash.exe")
	}
	if home, err := os.UserHomeDir(); err == nil && home != "" {
		add(home, "scoop", "apps", "git", "current", "bin", "bash.exe")
		add(home, "scoop", "apps", "git", "current", "usr", "bin", "bash.exe")
	}
	return out
}

func findGitBash() string {
	for _, p := range gitBashCandidatePaths() {
		if fileExists(p) {
			return p
		}
	}
	return ""
}

func fileExists(path string) bool {
	st, err := os.Stat(path)
	return err == nil && !st.IsDir()
}

// resolveShell picks the host shell for the given sandbox backend. The shell
// interpreter is an internal detail of each backend (no user setting):
// WSL2 always runs bash inside Linux; on Windows host / win-token paths,
// auto prefers bundled or system Microsoft Coreutils with cmd.exe (POSIX
// utilities on PATH) over Git Bash, falling back to plain cmd; Unix uses sh.
func resolveShell(backend domain.SandboxBackend) resolvedShell {
	if backend == domain.SandboxBackendWSL2 {
		return resolvedShell{
			kind:  "wsl-bash",
			label: "bash (WSL2)",
		}
	}
	if runtime.GOOS != "windows" {
		return resolvedShell{kind: "sh", label: "sh"}
	}

	bashPath := findGitBash()
	cuBin := findCoreutilsBin()

	cmdWithCoreutils := func() resolvedShell {
		label := "cmd"
		if cuBin != "" {
			label = "cmd (Coreutils)"
		}
		return resolvedShell{kind: "cmd", label: label, coreutilsBin: cuBin}
	}

	// auto: Coreutils+cmd first, then Git Bash, else plain cmd
	if cuBin != "" {
		return cmdWithCoreutils()
	}
	if bashPath != "" {
		return resolvedShell{kind: "git-bash", label: "bash (Git for Windows)", path: bashPath}
	}
	return resolvedShell{kind: "cmd", label: "cmd"}
}

func applyShellStatus(st *domain.SandboxStatus, sh resolvedShell) {
	st.Shell = sh.label
	st.ShellPath = sh.path
	st.CoreutilsBin = sh.coreutilsBin
	if sh.coreutilsBin == "" && sh.kind == "cmd" {
		// Still surface discovered Coreutils even if label is plain cmd (should be rare).
		st.CoreutilsBin = findCoreutilsBin()
	}
	if sh.err != nil {
		st.Degraded = true
		if st.DegradedReason == "" {
			st.DegradedReason = sh.err.Error()
		}
	}
}

func shellCommandFor(ctx context.Context, command string, sh resolvedShell) (*exec.Cmd, error) {
	if sh.err != nil {
		return nil, sh.err
	}
	switch sh.kind {
	case "git-bash":
		return exec.CommandContext(ctx, sh.path, "-lc", command), nil
	case "cmd":
		return exec.CommandContext(ctx, "cmd", "/c", command), nil
	case "wsl-bash":
		// Caller should use wslRunner; defensive fallback.
		return exec.CommandContext(ctx, "wsl", "-e", "bash", "-lc", command), nil
	default:
		return exec.CommandContext(ctx, "sh", "-c", command), nil
	}
}

// HostShellCommand builds an *exec.Cmd for host execution using the same resolve
// rules as the sandbox manager (Coreutils+cmd or Git Bash on Windows when available).
func HostShellCommand(ctx context.Context, command string) (*exec.Cmd, error) {
	sh := resolveShell("")
	cmd, err := shellCommandFor(ctx, command, sh)
	if err != nil {
		return nil, err
	}
	if cmd.Env == nil {
		cmd.Env = os.Environ()
	}
	if sh.kind == "cmd" {
		cmd.Env = prependCoreutilsPATH(cmd.Env)
	}
	return cmd, nil
}
