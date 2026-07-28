package container

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

// Engine is a thin CLI wrapper around podman or docker (no registry pull APIs).
type Engine struct {
	bin string // "podman" or "docker"
}

// Detect returns the preferred container CLI (podman, then docker).
func Detect() (*Engine, error) {
	for _, name := range []string{"podman", "docker"} {
		if p, err := exec.LookPath(name); err == nil && p != "" {
			return &Engine{bin: name}, nil
		}
	}
	return nil, fmt.Errorf("container: neither podman nor docker found in PATH")
}

func (e *Engine) Name() string { return e.bin }

func (e *Engine) run(ctx context.Context, args ...string) (string, error) {
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

// ImageExists reports whether a local image tag is present (never pulls).
func (e *Engine) ImageExists(ctx context.Context, image string) (bool, error) {
	_, err := e.run(ctx, "image", "inspect", image)
	if err != nil {
		// inspect failure usually means missing — treat as not found
		return false, nil
	}
	return true, nil
}

// LoadTar loads an OCI/docker image archive. Never contacts a registry.
func (e *Engine) LoadTar(ctx context.Context, tarPath string) error {
	_, err := e.run(ctx, "load", "-i", tarPath)
	return err
}

// Tag ensures image is available under wantTag. After load, the archive may
// already carry the tag; if not, retag the first loaded id when needed.
func (e *Engine) EnsureTag(ctx context.Context, image string) error {
	ok, err := e.ImageExists(ctx, image)
	if err != nil {
		return err
	}
	if ok {
		return nil
	}
	return fmt.Errorf("container: image %q not present after load (build/save with that tag)", image)
}

// ContainerInspectState returns status string (running, exited, …) or "" if missing.
func (e *Engine) ContainerInspectState(ctx context.Context, name string) (string, error) {
	out, err := e.run(ctx, "inspect", "-f", "{{.State.Status}}", name)
	if err != nil {
		return "", nil // missing
	}
	return strings.TrimSpace(out), nil
}

// CreateDetached creates a long-lived container (sleep infinity).
func (e *Engine) CreateDetached(ctx context.Context, name, image, workDir, mount, network string, env []string) error {
	args := []string{
		"create",
		"--name", name,
		"--workdir", mount,
		"-v", workDir + ":" + mount + ":rw",
	}
	if network != "" {
		args = append(args, "--network", network)
	}
	for _, ev := range env {
		args = append(args, "-e", ev)
	}
	args = append(args, image, "sleep", "infinity")
	_, err := e.run(ctx, args...)
	return err
}

func (e *Engine) Start(ctx context.Context, name string) error {
	_, err := e.run(ctx, "start", name)
	return err
}

func (e *Engine) Stop(ctx context.Context, name string) error {
	ctx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	_, err := e.run(ctx, "stop", "-t", "5", name)
	return err
}

func (e *Engine) Rm(ctx context.Context, name string) error {
	_, err := e.run(ctx, "rm", "-f", name)
	return err
}

// Exec runs a command inside the container and returns combined output.
func (e *Engine) Exec(ctx context.Context, name, workdir, command string, env []string) ([]byte, error) {
	args := []string{"exec", "-w", workdir}
	for _, ev := range env {
		args = append(args, "-e", ev)
	}
	args = append(args, name, "sh", "-c", command)
	cmd := exec.CommandContext(ctx, e.bin, args...)
	return cmd.CombinedOutput()
}
