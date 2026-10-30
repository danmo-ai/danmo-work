package container

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"danmo-work/core/domain"
)

// CLIRuntime wraps docker/podman-compatible CLIs.
type CLIRuntime struct {
	bin  string
	name string // podman | docker
}

func newCLIRuntime(bin, name string) *CLIRuntime {
	return &CLIRuntime{bin: bin, name: name}
}

func (e *CLIRuntime) Name() string { return e.name }

func (e *CLIRuntime) run(ctx context.Context, args ...string) (string, error) {
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

func (e *CLIRuntime) ImageExists(ctx context.Context, image string) (bool, error) {
	_, err := e.run(ctx, "image", "inspect", image)
	if err != nil {
		return false, nil
	}
	return true, nil
}

func (e *CLIRuntime) LoadTar(ctx context.Context, tarPath string) error {
	p, tmp, err := prepLoadTar(tarPath)
	if err != nil {
		return err
	}
	if tmp != "" {
		defer os.Remove(tmp)
	}
	_, err = e.run(ctx, "load", "-i", p)
	return err
}

func (e *CLIRuntime) EnsureTag(ctx context.Context, image string) error {
	ok, err := e.ImageExists(ctx, image)
	if err != nil {
		return err
	}
	if ok {
		return nil
	}
	// Tars built by scripts/build_env_tar.sh may carry an arch-suffixed tag
	// (e.g. localhost/danmo-work-env:bundled-arm64). Retag the first match.
	out, err := e.run(ctx, "image", "list", "--format", "{{.Repository}}:{{.Tag}}")
	if err == nil {
		for _, ref := range strings.Fields(out) {
			ref = strings.Trim(ref, "'\"")
			if ref == "" || ref == "<none>:<none>" {
				continue
			}
			// Prefer refs that match the requested image family (bundled vs desktop).
			if strings.Contains(image, ":desktop") && !strings.Contains(ref, "desktop") {
				continue
			}
			if !strings.Contains(image, ":desktop") && strings.Contains(ref, "desktop") {
				continue
			}
			if !strings.Contains(ref, "danmo-work-env") {
				continue
			}
			if _, tagErr := e.run(ctx, "tag", ref, image); tagErr == nil {
				if ok, _ := e.ImageExists(ctx, image); ok {
					return nil
				}
			}
		}
	}
	// Fallback: retag by image ID (untagged images after load).
	if ids, err := e.run(ctx, "image", "list", "--quiet"); err == nil {
		for _, id := range strings.Fields(ids) {
			if _, tagErr := e.run(ctx, "tag", id, image); tagErr == nil {
				if ok, _ := e.ImageExists(ctx, image); ok {
					return nil
				}
			}
		}
	}
	return fmt.Errorf("container: image %q not present after load (build/save with that tag)", image)
}

func (e *CLIRuntime) ContainerInspectState(ctx context.Context, name string) (string, error) {
	out, err := e.run(ctx, "inspect", "-f", "{{.State.Status}}", name)
	if err != nil {
		return "", nil
	}
	return strings.TrimSpace(out), nil
}

func (e *CLIRuntime) CreateDetached(ctx context.Context, opts CreateOpts) error {
	args := []string{
		"create",
		"--name", opts.Name,
		"--workdir", opts.Mount,
		"-v", opts.WorkDir + ":" + opts.Mount + ":rw",
	}
	if opts.Network != "" {
		args = append(args, "--network", opts.Network)
	}
	for _, b := range opts.Binds {
		mode := "rw"
		if b.ReadOnly {
			mode = "ro"
		}
		args = append(args, "-v", b.Host+":"+b.Container+":"+mode)
	}
	args = append(args, resourceFlagsDocker(opts.Resources)...)
	if shm := strings.TrimSpace(opts.ShmSize); shm != "" {
		args = append(args, "--shm-size", shm)
	}
	for _, p := range opts.Ports {
		args = append(args, "-p", formatPortPublish(p))
	}
	for _, ev := range opts.Env {
		args = append(args, "-e", ev)
	}
	if opts.Entrypoint != nil && len(opts.Entrypoint) > 0 {
		args = append(args, "--entrypoint", opts.Entrypoint[0])
		// docker --entrypoint only takes the binary; remaining entrypoint args go after image.
	}
	args = append(args, opts.Image)
	switch {
	case opts.KeepImageCmd:
		// leave image ENTRYPOINT/CMD as-is
	case len(opts.Cmd) > 0:
		args = append(args, opts.Cmd...)
	default:
		args = append(args, "sleep", "infinity")
	}
	_, err := e.run(ctx, args...)
	return err
}

func formatPortPublish(p PortPublish) string {
	hostIP := strings.TrimSpace(p.HostIP)
	if hostIP == "" {
		hostIP = "127.0.0.1"
	}
	proto := strings.TrimSpace(p.Proto)
	if proto == "" {
		proto = "tcp"
	}
	if p.HostPort > 0 {
		return fmt.Sprintf("%s:%d:%d/%s", hostIP, p.HostPort, p.ContainerPort, proto)
	}
	return fmt.Sprintf("%s::%d/%s", hostIP, p.ContainerPort, proto)
}

func (e *CLIRuntime) InspectPublishedPorts(ctx context.Context, name string) (PublishedPorts, error) {
	out, err := e.run(ctx, "inspect", "-f", "{{json .NetworkSettings.Ports}}", name)
	if err != nil {
		return nil, err
	}
	return parseDockerPortsJSON(out)
}

func resourceFlagsDocker(r domain.EnvironmentResources) []string {
	var args []string
	if c := strings.TrimSpace(r.CPUs); c != "" {
		args = append(args, "--cpus", c)
	}
	if m := strings.TrimSpace(r.Memory); m != "" {
		args = append(args, "--memory", m)
	}
	if r.Pids > 0 {
		args = append(args, "--pids-limit", fmt.Sprintf("%d", r.Pids))
	}
	return args
}

func (e *CLIRuntime) Start(ctx context.Context, name string) error {
	_, err := e.run(ctx, "start", name)
	return err
}

func (e *CLIRuntime) Stop(ctx context.Context, name string) error {
	ctx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	_, err := e.run(ctx, "stop", "-t", "5", name)
	return err
}

func (e *CLIRuntime) Rm(ctx context.Context, name string) error {
	_, err := e.run(ctx, "rm", "-f", name)
	return err
}

func (e *CLIRuntime) Exec(ctx context.Context, name, workdir, command string, env []string) ([]byte, error) {
	args := []string{"exec", "-w", workdir}
	for _, ev := range env {
		args = append(args, "-e", ev)
	}
	args = append(args, name, "sh", "-c", command)
	cmd := exec.CommandContext(ctx, e.bin, args...)
	return cmd.CombinedOutput()
}
