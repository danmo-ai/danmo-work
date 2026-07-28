package container

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"danmo-work/core/domain"
)

// AppleRuntime wraps Apple's `container` CLI (macOS / Apple silicon).
// https://github.com/apple/container — OCI-compatible, image load via
// `container image load --input`, no registry pull in our path.
type AppleRuntime struct {
	bin string
}

func newAppleRuntime(bin string) *AppleRuntime {
	return &AppleRuntime{bin: bin}
}

func (e *AppleRuntime) Name() string { return string(domain.EnvironmentEngineAppleContainer) }

func (e *AppleRuntime) run(ctx context.Context, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, e.bin, args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	out := strings.TrimSpace(stdout.String())
	if err != nil {
		msg := strings.TrimSpace(stderr.String())
		if msg == "" {
			msg = out
		}
		if msg == "" {
			msg = err.Error()
		}
		return out, fmt.Errorf("%s %s: %s", e.bin, strings.Join(args, " "), msg)
	}
	return out, nil
}

func isAppleContainerCLI(bin string) bool {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	out, err := exec.CommandContext(ctx, bin, "--help").CombinedOutput()
	if err != nil && len(out) == 0 {
		return false
	}
	s := strings.ToLower(string(out))
	// Distinguish from unrelated "container" binaries.
	return strings.Contains(s, "image load") ||
		strings.Contains(s, "system start") ||
		strings.Contains(s, "containerization") ||
		(strings.Contains(s, "create") && strings.Contains(s, "exec") && strings.Contains(s, "image"))
}

func (e *AppleRuntime) ImageExists(ctx context.Context, image string) (bool, error) {
	// `container images` / `container image list` — try inspect-style first.
	_, err := e.run(ctx, "images", "inspect", image)
	if err == nil {
		return true, nil
	}
	out, err2 := e.run(ctx, "images", "list")
	if err2 != nil {
		out, err2 = e.run(ctx, "image", "list")
	}
	if err2 != nil {
		return false, nil
	}
	return strings.Contains(out, image) || strings.Contains(out, strings.TrimPrefix(image, "localhost/")), nil
}

func (e *AppleRuntime) LoadTar(ctx context.Context, tarPath string) error {
	_, err := e.run(ctx, "image", "load", "--input", tarPath)
	return err
}

func (e *AppleRuntime) EnsureTag(ctx context.Context, image string) error {
	ok, err := e.ImageExists(ctx, image)
	if err != nil {
		return err
	}
	if ok {
		return nil
	}
	return fmt.Errorf("container: apple-container image %q not present after load", image)
}

func (e *AppleRuntime) ContainerInspectState(ctx context.Context, name string) (string, error) {
	out, err := e.run(ctx, "inspect", name)
	if err != nil {
		return "", nil
	}
	low := strings.ToLower(out)
	switch {
	case strings.Contains(low, `"status": "running"`) || strings.Contains(low, "running"):
		return "running", nil
	case strings.Contains(low, "exited") || strings.Contains(low, "stopped"):
		return "exited", nil
	case strings.Contains(low, "created"):
		return "created", nil
	default:
		if strings.TrimSpace(out) != "" {
			return "created", nil
		}
		return "", nil
	}
}

func (e *AppleRuntime) CreateDetached(ctx context.Context, opts CreateOpts) error {
	args := []string{
		"create",
		"--name", opts.Name,
		"--workdir", opts.Mount,
		"-v", opts.WorkDir + ":" + opts.Mount,
	}
	if opts.Network != "" && opts.Network != "host" {
		// Apple networking differs; "none" may map to --network none when supported.
		args = append(args, "--network", opts.Network)
	}
	args = append(args, resourceFlagsApple(opts.Resources)...)
	for _, ev := range opts.Env {
		args = append(args, "-e", ev)
	}
	args = append(args, opts.Image, "sleep", "infinity")
	_, err := e.run(ctx, args...)
	return err
}

func resourceFlagsApple(r domain.EnvironmentResources) []string {
	var args []string
	if c := strings.TrimSpace(r.CPUs); c != "" {
		args = append(args, "--cpus", c)
	}
	if m := strings.TrimSpace(r.Memory); m != "" {
		args = append(args, "--memory", m)
	}
	return args
}

func (e *AppleRuntime) Start(ctx context.Context, name string) error {
	_, err := e.run(ctx, "start", name)
	return err
}

func (e *AppleRuntime) Stop(ctx context.Context, name string) error {
	ctx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	_, err := e.run(ctx, "stop", name)
	return err
}

func (e *AppleRuntime) Rm(ctx context.Context, name string) error {
	_, err := e.run(ctx, "rm", name)
	if err != nil {
		_, err = e.run(ctx, "delete", name)
	}
	return err
}

func (e *AppleRuntime) Exec(ctx context.Context, name, workdir, command string, env []string) ([]byte, error) {
	args := []string{"exec", "--workdir", workdir}
	for _, ev := range env {
		args = append(args, "-e", ev)
	}
	args = append(args, name, "sh", "-c", command)
	cmd := exec.CommandContext(ctx, e.bin, args...)
	return cmd.CombinedOutput()
}
