//go:build darwin

package sandbox

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"danmo-work/core/domain"
	"danmo-work/core/port"
)

func selectBackend(cfg domain.ConfigSandboxSection, _ bool) (domain.SandboxBackend, runner, bool, string, []string) {
	force := strings.ToLower(strings.TrimSpace(cfg.Backend))
	switch force {
	case "host-weak", "host":
		return domain.SandboxBackendHostWeak, hostRunner{}, true, "forced host-weak backend", []string{"host"}
	case "seatbelt", "":
		// continue
	default:
		return domain.SandboxBackendHostWeak, hostRunner{}, true, "unknown backend " + force + "; using host-weak", []string{"host"}
	}

	if _, err := exec.LookPath("sandbox-exec"); err != nil {
		return domain.SandboxBackendHostWeak, hostRunner{}, true, "sandbox-exec not found", []string{"host"}
	}
	return domain.SandboxBackendSeatbelt, seatbeltRunner{}, false, "", []string{"seatbelt", "fs-isolation", "network-control"}
}

type seatbeltRunner struct{}

func (seatbeltRunner) name() domain.SandboxBackend { return domain.SandboxBackendSeatbelt }

func (seatbeltRunner) run(ctx context.Context, opts port.SandboxRunOptions, cfg domain.ConfigSandboxSection) ([]byte, error) {
	workdir, err := filepath.Abs(opts.WorkDir)
	if err != nil {
		return nil, fmt.Errorf("sandbox: workdir: %w", err)
	}
	profile := buildSeatbeltProfile(workdir, cfg, opts)
	tmp, err := os.CreateTemp("", "dq-seatbelt-*.sb")
	if err != nil {
		return nil, fmt.Errorf("sandbox: seatbelt profile: %w", err)
	}
	profilePath := tmp.Name()
	defer os.Remove(profilePath)
	if _, err := tmp.WriteString(profile); err != nil {
		tmp.Close()
		return nil, err
	}
	if err := tmp.Close(); err != nil {
		return nil, err
	}

	ctx, cancel := context.WithTimeout(ctx, opts.Timeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, "sandbox-exec", "-f", profilePath, "sh", "-c", opts.Command)
	cmd.Dir = workdir
	cmd.Env = opts.Env
	out, err := cmd.CombinedOutput()
	if ctx.Err() == context.DeadlineExceeded {
		return out, fmt.Errorf("sandbox: command timed out after %s", opts.Timeout)
	}
	return out, err
}

func buildSeatbeltProfile(workdir string, cfg domain.ConfigSandboxSection, opts port.SandboxRunOptions) string {
	// Prefer allow-default + selective deny — matches practical Codex/Claude profiles
	// and avoids brittle filter enumerations that break across macOS versions.
	var b strings.Builder
	b.WriteString("(version 1)\n")
	b.WriteString("(allow default)\n")
	b.WriteString("(deny file-write*)\n")
	b.WriteString("(allow file-write-data (literal \"/dev/null\"))\n")
	b.WriteString("(allow file-ioctl (literal \"/dev/null\"))\n")

	for _, p := range []string{"/tmp", "/private/tmp", "/var/folders", "/private/var/folders"} {
		fmt.Fprintf(&b, "(allow file-write* (subpath %q))\n", p)
	}

	switch cfg.Mode {
	case domain.SandboxModeReadOnly:
		// workspace is read-only; only temp writes allowed
	default:
		fmt.Fprintf(&b, "(allow file-write* (subpath %q))\n", workdir)
	}

	if !networkAllowed(cfg, opts) {
		b.WriteString("(deny network*)\n")
	}
	return b.String()
}
