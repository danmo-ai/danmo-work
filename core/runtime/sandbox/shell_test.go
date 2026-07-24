package sandbox

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"danmo-work/core/domain"
)

func TestResolveShellUnix(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("unix-only")
	}
	sh := resolveShell(domain.ConfigSandboxSection{}, domain.SandboxBackendSeatbelt)
	if sh.kind != "sh" || sh.label != "sh" {
		t.Fatalf("got %+v", sh)
	}
}

func TestResolveShellWSL2(t *testing.T) {
	sh := resolveShell(domain.ConfigSandboxSection{Shell: "cmd"}, domain.SandboxBackendWSL2)
	if sh.kind != "wsl-bash" || sh.label != "bash (WSL2)" {
		t.Fatalf("got %+v", sh)
	}
	if sh.path != "" {
		t.Fatalf("unexpected path %q", sh.path)
	}
}

func TestResolveShellGitBashAuto(t *testing.T) {
	dir := t.TempDir()
	bash := filepath.Join(dir, "bash.exe")
	if err := os.WriteFile(bash, []byte("fake"), 0o755); err != nil {
		t.Fatal(err)
	}
	prev := gitBashCandidatePaths
	gitBashCandidatePaths = func() []string { return []string{bash} }
	t.Cleanup(func() { gitBashCandidatePaths = prev })

	prevCU := coreutilsCandidateBins
	coreutilsCandidateBins = func() []string { return nil }
	prevExe := coreutilsExeCandidates
	coreutilsExeCandidates = func() []string { return nil }
	resetCoreutilsCache()
	t.Cleanup(func() {
		coreutilsCandidateBins = prevCU
		coreutilsExeCandidates = prevExe
		resetCoreutilsCache()
	})

	// Force Windows branch by only testing path find + resolve logic pieces.
	found := findGitBash()
	if found != bash {
		t.Fatalf("findGitBash=%q want %q", found, bash)
	}

	if runtime.GOOS != "windows" {
		// On non-Windows, resolveShell ignores Git Bash for host backends.
		sh := resolveShell(domain.ConfigSandboxSection{Shell: "auto"}, domain.SandboxBackendHostWeak)
		if sh.kind != "sh" {
			t.Fatalf("non-windows host shell=%+v", sh)
		}
		return
	}

	sh := resolveShell(domain.ConfigSandboxSection{Shell: "auto"}, domain.SandboxBackendWinToken)
	if sh.kind != "git-bash" || sh.path != bash || sh.label != "bash (Git for Windows)" {
		t.Fatalf("got %+v", sh)
	}
}

func TestResolveShellAutoPrefersCoreutilsOverGitBash(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("windows-only")
	}
	dir := t.TempDir()
	bash := filepath.Join(dir, "bash.exe")
	_ = os.WriteFile(bash, []byte("fake"), 0o755)
	cuBin := filepath.Join(dir, "cubin")
	_ = os.MkdirAll(cuBin, 0o755)
	_ = os.WriteFile(filepath.Join(cuBin, "ls.exe"), []byte("fake"), 0o755)

	prev := gitBashCandidatePaths
	gitBashCandidatePaths = func() []string { return []string{bash} }
	prevCU := coreutilsCandidateBins
	coreutilsCandidateBins = func() []string { return []string{cuBin} }
	prevExe := coreutilsExeCandidates
	coreutilsExeCandidates = func() []string { return nil }
	resetCoreutilsCache()
	t.Cleanup(func() {
		gitBashCandidatePaths = prev
		coreutilsCandidateBins = prevCU
		coreutilsExeCandidates = prevExe
		resetCoreutilsCache()
	})

	sh := resolveShell(domain.ConfigSandboxSection{Shell: "auto"}, domain.SandboxBackendWinToken)
	if sh.kind != "cmd" || sh.label != "cmd (Coreutils)" || sh.coreutilsBin != cuBin {
		t.Fatalf("got %+v", sh)
	}
}

func TestResolveShellForceCmd(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("windows-only")
	}
	dir := t.TempDir()
	bash := filepath.Join(dir, "bash.exe")
	_ = os.WriteFile(bash, []byte("fake"), 0o755)
	prev := gitBashCandidatePaths
	gitBashCandidatePaths = func() []string { return []string{bash} }
	t.Cleanup(func() { gitBashCandidatePaths = prev })

	sh := resolveShell(domain.ConfigSandboxSection{Shell: "cmd"}, domain.SandboxBackendWinToken)
	if sh.kind != "cmd" || sh.path != "" {
		t.Fatalf("got %+v", sh)
	}
}

func TestResolveShellRequireBashMissing(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("windows-only")
	}
	prev := gitBashCandidatePaths
	gitBashCandidatePaths = func() []string { return nil }
	prevCU := coreutilsCandidateBins
	coreutilsCandidateBins = func() []string { return nil }
	prevExe := coreutilsExeCandidates
	coreutilsExeCandidates = func() []string { return nil }
	resetCoreutilsCache()
	t.Cleanup(func() {
		gitBashCandidatePaths = prev
		coreutilsCandidateBins = prevCU
		coreutilsExeCandidates = prevExe
		resetCoreutilsCache()
	})

	sh := resolveShell(domain.ConfigSandboxSection{Shell: "bash"}, domain.SandboxBackendWinToken)
	if sh.err == nil {
		t.Fatal("expected error when bash required but missing")
	}
}

func TestResolveShellAutoFallsBackCmd(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("windows-only")
	}
	prev := gitBashCandidatePaths
	gitBashCandidatePaths = func() []string { return []string{filepath.Join(t.TempDir(), "missing", "bash.exe")} }
	prevCU := coreutilsCandidateBins
	coreutilsCandidateBins = func() []string { return nil }
	prevExe := coreutilsExeCandidates
	coreutilsExeCandidates = func() []string { return nil }
	resetCoreutilsCache()
	t.Cleanup(func() {
		gitBashCandidatePaths = prev
		coreutilsCandidateBins = prevCU
		coreutilsExeCandidates = prevExe
		resetCoreutilsCache()
	})

	sh := resolveShell(domain.ConfigSandboxSection{Shell: "auto"}, domain.SandboxBackendWinToken)
	if sh.kind != "cmd" || sh.err != nil {
		t.Fatalf("got %+v", sh)
	}
}

func TestNormalizeShellPref(t *testing.T) {
	cases := map[string]string{
		"":      domain.SandboxShellAuto,
		"AUTO":  domain.SandboxShellAuto,
		"bash":  domain.SandboxShellBash,
		"cmd":   domain.SandboxShellCmd,
		"other": domain.SandboxShellAuto,
	}
	for in, want := range cases {
		if got := normalizeShellPref(in); got != want {
			t.Fatalf("%q -> %q want %q", in, got, want)
		}
	}
}

func TestManagerStatusIncludesShell(t *testing.T) {
	m := New(domain.ConfigSandboxSection{Enabled: true})
	st := m.Status()
	if st.Shell == "" {
		t.Fatal("expected shell label on status")
	}
}
