package runtime

import (
	"strings"
	"testing"

	"danmo-work/core/domain"
)

func TestBuildRuntimeEnvironmentGitBash(t *testing.T) {
	out := buildRuntimeEnvironment(domain.SandboxStatus{
		Backend:   domain.SandboxBackendWinToken,
		Shell:     "bash (Git for Windows)",
		ShellPath: `C:\Program Files\Git\bin\bash.exe`,
	}, domain.EnvironmentStatus{})
	for _, want := range []string{
		"Shell: bash (Git for Windows)",
		`Shell path: C:\Program Files\Git\bin\bash.exe`,
		"Sandbox backend: win-token",
		"Prefer POSIX shell syntax",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("missing %q in:\n%s", want, out)
		}
	}
}

func TestBuildRuntimeEnvironmentWSL2(t *testing.T) {
	out := buildRuntimeEnvironment(domain.SandboxStatus{
		Backend: domain.SandboxBackendWSL2,
		Shell:   "bash (WSL2)",
	}, domain.EnvironmentStatus{})
	if !strings.Contains(out, "Shell: bash (WSL2)") {
		t.Fatal(out)
	}
	if !strings.Contains(out, "via: wsl -e bash -lc") {
		t.Fatal(out)
	}
	if strings.Contains(out, "Shell path:") {
		t.Fatalf("unexpected shell path:\n%s", out)
	}
}

func TestBuildRuntimeEnvironmentCmdFallback(t *testing.T) {
	out := buildRuntimeEnvironment(domain.SandboxStatus{
		Backend: domain.SandboxBackendWinToken,
		Shell:   "cmd",
	}, domain.EnvironmentStatus{})
	if !strings.Contains(out, "neither bundled/system Coreutils nor Git Bash detected") {
		t.Fatal(out)
	}
}

func TestBuildRuntimeEnvironmentCmdCoreutils(t *testing.T) {
	out := buildRuntimeEnvironment(domain.SandboxStatus{
		Backend:      domain.SandboxBackendWinToken,
		Shell:        "cmd (Coreutils)",
		CoreutilsBin: `C:\Users\x\.danmo-work\bin\coreutils\bin`,
	}, domain.EnvironmentStatus{})
	for _, want := range []string{
		"Shell: cmd (Coreutils)",
		"Microsoft Coreutils on PATH",
		`Coreutils bin: C:\Users\x\.danmo-work\bin\coreutils\bin`,
		"NUL instead of /dev/null",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("missing %q in:\n%s", want, out)
		}
	}
}
