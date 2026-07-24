//go:build windows

package sandbox

import (
	"context"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"unsafe"

	"danmo-work/core/domain"
	"danmo-work/core/port"

	"golang.org/x/sys/windows"
)

var (
	modadvapi32               = windows.NewLazySystemDLL("advapi32.dll")
	procCreateRestrictedToken = modadvapi32.NewProc("CreateRestrictedToken")
)

const disableMaxPrivilege = 0x1

func selectBackend(cfg domain.ConfigSandboxSection) (domain.SandboxBackend, runner, bool, string, []string) {
	force := strings.ToLower(strings.TrimSpace(cfg.Backend))
	switch force {
	case "host-weak", "host":
		return domain.SandboxBackendHostWeak, hostRunner{}, true, "forced host-weak backend", []string{"host"}
	case "wsl2", "wsl":
		if wslAvailable() {
			return domain.SandboxBackendWSL2, wslRunner{}, false, "", []string{"wsl2", "linux-userspace"}
		}
		return domain.SandboxBackendHostWeak, hostRunner{}, true, "wsl2 forced but wsl.exe not found", []string{"host"}
	case "win-token", "token", "":
		// auto / forced token below
	default:
		return domain.SandboxBackendHostWeak, hostRunner{}, true, "unknown backend " + force + "; using host-weak", []string{"host"}
	}

	if tokenSandboxAvailable() {
		caps := []string{"win-token", "privilege-restricted"}
		degraded, reason := false, ""
		if cfg.Network == domain.SandboxNetworkDeny {
			degraded = true
			reason = "win-token unelevated mode does not enforce kernel network deny; set runtime.sandbox.backend=wsl2 for stronger isolation"
		}
		return domain.SandboxBackendWinToken, winTokenRunner{}, degraded, reason, caps
	}
	if wslAvailable() {
		return domain.SandboxBackendWSL2, wslRunner{}, true, "restricted token unavailable; falling back to WSL2", []string{"wsl2", "linux-userspace"}
	}
	return domain.SandboxBackendHostWeak, hostRunner{}, true, "neither win-token nor WSL2 available", []string{"host"}
}

func wslAvailable() bool {
	return lookPath("wsl") || lookPath("wsl.exe")
}

func tokenSandboxAvailable() bool {
	var tok windows.Token
	err := windows.OpenProcessToken(windows.CurrentProcess(), windows.TOKEN_DUPLICATE|windows.TOKEN_QUERY, &tok)
	if err != nil {
		return false
	}
	defer tok.Close()
	restricted, err := createRestrictedToken(tok)
	if err != nil {
		return false
	}
	restricted.Close()
	return true
}

func createRestrictedToken(existing windows.Token) (windows.Token, error) {
	var newHandle windows.Handle
	r1, _, err := procCreateRestrictedToken.Call(
		uintptr(existing),
		uintptr(disableMaxPrivilege),
		0, 0, // DisableSidCount, SidsToDisable
		0, 0, // DeletePrivilegeCount, PrivilegesToDelete
		0, 0, // RestrictedSidCount, SidsToRestrict
		uintptr(unsafe.Pointer(&newHandle)),
	)
	if r1 == 0 {
		if err != nil {
			return 0, err
		}
		return 0, fmt.Errorf("CreateRestrictedToken failed")
	}
	return windows.Token(newHandle), nil
}

type winTokenRunner struct{}

func (winTokenRunner) name() domain.SandboxBackend { return domain.SandboxBackendWinToken }

func (winTokenRunner) run(ctx context.Context, opts port.SandboxRunOptions, cfg domain.ConfigSandboxSection) ([]byte, error) {
	workdir, err := filepath.Abs(opts.WorkDir)
	if err != nil {
		return nil, fmt.Errorf("sandbox: workdir: %w", err)
	}

	var primary windows.Token
	if err := windows.OpenProcessToken(windows.CurrentProcess(),
		windows.TOKEN_DUPLICATE|windows.TOKEN_QUERY|windows.TOKEN_ASSIGN_PRIMARY|windows.TOKEN_ADJUST_DEFAULT|windows.TOKEN_ADJUST_SESSIONID,
		&primary); err != nil {
		return runHost(ctx, opts, cfg, domain.SandboxBackendHostWeak)
	}
	defer primary.Close()

	restricted, err := createRestrictedToken(primary)
	if err != nil {
		return runHost(ctx, opts, cfg, domain.SandboxBackendHostWeak)
	}
	defer restricted.Close()

	ctx, cancel := context.WithTimeout(ctx, opts.Timeout)
	defer cancel()
	sh := resolveShell(cfg, domain.SandboxBackendWinToken)
	cmd, err := shellCommandFor(ctx, opts.Command, sh)
	if err != nil {
		return nil, err
	}
	cmd.Dir = workdir
	cmd.Env = opts.Env
	if sh.kind == "cmd" {
		cmd.Env = prependCoreutilsPATH(cmd.Env)
	}
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Token: syscall.Token(restricted),
	}
	out, err := cmd.CombinedOutput()
	if ctx.Err() == context.DeadlineExceeded {
		return out, fmt.Errorf("sandbox: command timed out after %s", opts.Timeout)
	}
	return out, err
}

type wslRunner struct{}

func (wslRunner) name() domain.SandboxBackend { return domain.SandboxBackendWSL2 }

func (wslRunner) run(ctx context.Context, opts port.SandboxRunOptions, _ domain.ConfigSandboxSection) ([]byte, error) {
	workdir, err := filepath.Abs(opts.WorkDir)
	if err != nil {
		return nil, fmt.Errorf("sandbox: workdir: %w", err)
	}
	wslDir := toWSLPath(workdir)
	script := fmt.Sprintf("cd %s && %s", shellQuote(wslDir), opts.Command)

	ctx, cancel := context.WithTimeout(ctx, opts.Timeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, "wsl", "-e", "bash", "-lc", script)
	cmd.Env = opts.Env
	out, err := cmd.CombinedOutput()
	if ctx.Err() == context.DeadlineExceeded {
		return out, fmt.Errorf("sandbox: command timed out after %s", opts.Timeout)
	}
	return out, err
}

func toWSLPath(windowsPath string) string {
	out, err := exec.Command("wsl", "wslpath", "-a", windowsPath).CombinedOutput()
	if err == nil {
		return strings.TrimSpace(string(out))
	}
	p := filepath.ToSlash(windowsPath)
	if len(p) >= 2 && p[1] == ':' {
		drive := strings.ToLower(string(p[0]))
		return "/mnt/" + drive + p[2:]
	}
	return p
}

func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'"'"'`) + "'"
}
