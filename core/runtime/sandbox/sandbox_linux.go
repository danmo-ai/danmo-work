//go:build linux

package sandbox

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"

	"danmo-work/core/domain"
	"danmo-work/core/port"
)

// probeOSBackends lists the OS-level backends relevant on this platform
// (container engines are appended by the factory).
func probeOSBackends() []domain.SandboxBackendInfo {
	hasBwrap := lookPath("bwrap")
	hasLL := landlockAvailable()
	auto := ""
	switch {
	case hasLL:
		auto = string(domain.SandboxBackendLandlock)
	case hasBwrap:
		auto = string(domain.SandboxBackendBwrap)
	}
	infos := []domain.SandboxBackendInfo{
		{
			Name:          domain.SandboxBackendLandlock,
			Available:     hasLL,
			Capabilities:  []string{"fs-isolation"},
			AutoPreferred: hasLL && auto == string(domain.SandboxBackendLandlock),
		},
		{
			Name:         domain.SandboxBackendBwrap,
			Available:    hasBwrap,
			Capabilities: []string{"fs-isolation", "network-control", "seccomp-via-bwrap"},
		},
	}
	if !hasLL {
		infos[0].Reason = "landlock unavailable (kernel < 5.13 or not enabled)"
	}
	if !hasBwrap {
		infos[1].Reason = "bwrap not in PATH"
	}
	return infos
}

// selectOSBackend picks an OS-level backend for the normalized force name.
// Container engines are handled by the factory before this is called.
func selectOSBackend(force string, cfg domain.ConfigSandboxSection, allowlistProxyActive bool) (port.SandboxBackend, domain.SandboxBackend, bool, string, []string) {
	netDeny := needNetDeny(cfg, allowlistProxyActive)
	switch force {
	case string(domain.SandboxBackendHostWeak):
		return hostBackend{}, domain.SandboxBackendHostWeak, true, "forced host-weak backend", []string{"host"}
	case string(domain.SandboxBackendBwrap):
		if lookPath("bwrap") {
			return bwrapBackend{}, domain.SandboxBackendBwrap, false, "", []string{"bwrap", "fs-isolation", "network-control"}
		}
		return hostBackend{}, domain.SandboxBackendHostWeak, true, "bwrap forced but not installed", []string{"host"}
	case string(domain.SandboxBackendLandlock):
		if landlockAvailable() {
			caps := []string{"landlock", "fs-isolation"}
			degraded, reason := false, ""
			if netDeny {
				degraded, reason = true, "landlock backend does not isolate network; install bubblewrap for --unshare-net"
			}
			return landlockBackend{}, domain.SandboxBackendLandlock, degraded, reason, caps
		}
		return hostBackend{}, domain.SandboxBackendHostWeak, true, "landlock forced but unavailable", []string{"host"}
	case string(domain.SandboxBackendSeatbelt), string(domain.SandboxBackendWinToken), string(domain.SandboxBackendWSL2):
		return hostBackend{}, domain.SandboxBackendHostWeak, true, force + " backend is not available on linux", []string{"host"}
	}

	// Auto: prefer landlock when network allows; prefer bwrap when network deny.
	hasBwrap := lookPath("bwrap")
	hasLL := landlockAvailable()

	if netDeny && hasBwrap {
		return bwrapBackend{}, domain.SandboxBackendBwrap, false, "", []string{"bwrap", "fs-isolation", "network-control", "seccomp-via-bwrap"}
	}
	if hasLL {
		caps := []string{"landlock", "fs-isolation"}
		degraded, reason := false, ""
		if netDeny {
			if hasBwrap {
				return bwrapBackend{}, domain.SandboxBackendBwrap, false, "", []string{"bwrap", "fs-isolation", "network-control"}
			}
			degraded, reason = true, "network deny requested but bubblewrap unavailable; FS-only landlock"
		}
		return landlockBackend{}, domain.SandboxBackendLandlock, degraded, reason, caps
	}
	if hasBwrap {
		return bwrapBackend{}, domain.SandboxBackendBwrap, false, "", []string{"bwrap", "fs-isolation", "network-control"}
	}
	return hostBackend{}, domain.SandboxBackendHostWeak, true, "neither landlock nor bubblewrap available", []string{"host"}
}

type bwrapBackend struct{}

func (bwrapBackend) Name() domain.SandboxBackend { return domain.SandboxBackendBwrap }

func (bwrapBackend) Close() error { return nil }

func (bwrapBackend) Run(ctx context.Context, opts port.SandboxRunOptions, cfg domain.ConfigSandboxSection) ([]byte, error) {
	workdir, err := filepath.Abs(opts.WorkDir)
	if err != nil {
		return nil, fmt.Errorf("sandbox: workdir: %w", err)
	}
	args := []string{
		"--die-with-parent",
		"--unshare-pid",
		"--unshare-ipc",
		"--unshare-uts",
		"--proc", "/proc",
		"--dev", "/dev",
		"--ro-bind", "/usr", "/usr",
		"--ro-bind", "/bin", "/bin",
		"--ro-bind", "/lib", "/lib",
		"--ro-bind-try", "/lib64", "/lib64",
		"--ro-bind-try", "/sbin", "/sbin",
		"--ro-bind-try", "/etc", "/etc",
		"--tmpfs", "/tmp",
	}
	if cfg.Mode == domain.SandboxModeReadOnly {
		args = append(args, "--ro-bind", workdir, workdir)
	} else {
		args = append(args, "--bind", workdir, workdir)
	}
	if !networkAllowed(cfg, opts) {
		args = append(args, "--unshare-net")
	}
	args = append(args, "--chdir", workdir, "sh", "-c", opts.Command)

	ctx, cancel := context.WithTimeout(ctx, opts.Timeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, "bwrap", args...)
	cmd.Env = opts.Env
	out, err := cmd.CombinedOutput()
	if ctx.Err() == context.DeadlineExceeded {
		return out, fmt.Errorf("sandbox: command timed out after %s", opts.Timeout)
	}
	return out, err
}

type landlockBackend struct{}

func (landlockBackend) Name() domain.SandboxBackend { return domain.SandboxBackendLandlock }

func (landlockBackend) Close() error { return nil }

func (landlockBackend) Run(ctx context.Context, opts port.SandboxRunOptions, cfg domain.ConfigSandboxSection) ([]byte, error) {
	workdir, err := filepath.Abs(opts.WorkDir)
	if err != nil {
		return nil, fmt.Errorf("sandbox: workdir: %w", err)
	}
	self, err := os.Executable()
	if err != nil {
		return nil, fmt.Errorf("sandbox: executable: %w", err)
	}

	ctx, cancel := context.WithTimeout(ctx, opts.Timeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, self, reexecArg, "--", "sh", "-c", opts.Command)
	cmd.Dir = workdir
	env := append([]string{}, opts.Env...)
	env = append(env,
		"WORK_SB_WORKDIR="+workdir,
		"WORK_SB_MODE="+string(cfg.Mode),
	)
	cmd.Env = env
	out, err := cmd.CombinedOutput()
	if ctx.Err() == context.DeadlineExceeded {
		return out, fmt.Errorf("sandbox: command timed out after %s", opts.Timeout)
	}
	return out, err
}

// MaybeReexec handles the landlock child entrypoint. Call from main() before
// normal startup. Returns true if this process was a sandbox child (never returns
// on success — execs into the target command).
func MaybeReexec() bool {
	if len(os.Args) < 2 || os.Args[1] != reexecArg {
		return false
	}
	workdir := os.Getenv("WORK_SB_WORKDIR")
	mode := domain.SandboxMode(os.Getenv("WORK_SB_MODE"))
	if workdir == "" {
		fmt.Fprintln(os.Stderr, "sandbox: missing WORK_SB_WORKDIR")
		os.Exit(2)
	}
	dash := -1
	for i, a := range os.Args {
		if a == "--" {
			dash = i
			break
		}
	}
	if dash < 0 || dash+1 >= len(os.Args) {
		fmt.Fprintln(os.Stderr, "sandbox: missing command after --")
		os.Exit(2)
	}
	if err := applyLandlock(workdir, mode); err != nil {
		fmt.Fprintf(os.Stderr, "sandbox: landlock: %v\n", err)
		os.Exit(2)
	}
	argv := os.Args[dash+1:]
	env := os.Environ()
	if err := syscall.Exec(argv[0], argv, env); err != nil {
		// argv[0] may be "sh" — resolve PATH
		path, lookErr := exec.LookPath(argv[0])
		if lookErr != nil {
			fmt.Fprintf(os.Stderr, "sandbox: exec: %v\n", err)
			os.Exit(2)
		}
		if err := syscall.Exec(path, argv, env); err != nil {
			fmt.Fprintf(os.Stderr, "sandbox: exec: %v\n", err)
			os.Exit(2)
		}
	}
	return true
}
