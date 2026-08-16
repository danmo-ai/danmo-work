package container

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"

	"danmo-work/core/domain"
)

const (
	appleHostDNSName = "host.container.internal"
	appleHostDNSIP   = "203.0.113.113" // TEST-NET-3; redirected to host localhost by Apple DNS
)

// AppleRuntime wraps Apple's `container` CLI (macOS / Apple silicon).
// https://github.com/apple/container
type AppleRuntime struct {
	bin string

	mu         sync.Mutex
	sysReady   bool
	hostDNSOK  bool
	hostDNSTry bool
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
	return strings.Contains(s, "image load") ||
		strings.Contains(s, "system start") ||
		strings.Contains(s, "containerization") ||
		(strings.Contains(s, "create") && strings.Contains(s, "exec") && strings.Contains(s, "image"))
}

// ensureSystem starts Apple container services if needed (idempotent).
func (e *AppleRuntime) ensureSystem(ctx context.Context) error {
	e.mu.Lock()
	if e.sysReady {
		e.mu.Unlock()
		return nil
	}
	e.mu.Unlock()

	if out, err := e.run(ctx, "system", "status"); err == nil {
		low := strings.ToLower(out)
		if strings.Contains(low, "running") || strings.Contains(low, "ok") || strings.Contains(low, "apiserver") {
			e.mu.Lock()
			e.sysReady = true
			e.mu.Unlock()
			return nil
		}
	}
	// Non-interactive kernel install when possible.
	startCtx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()
	if _, err := e.run(startCtx, "system", "start", "--enable-kernel-install"); err != nil {
		// Retry without flag (older CLIs).
		if _, err2 := e.run(startCtx, "system", "start"); err2 != nil {
			return fmt.Errorf("apple-container: system start failed: %w", err)
		}
	}
	e.mu.Lock()
	e.sysReady = true
	e.mu.Unlock()
	return nil
}

// EnsureHostDNS best-effort creates host.container.internal → host localhost
// (needed for allowlist proxy). May require sudo; failures are non-fatal.
func (e *AppleRuntime) EnsureHostDNS(ctx context.Context) {
	e.mu.Lock()
	if e.hostDNSTry {
		e.mu.Unlock()
		return
	}
	e.hostDNSTry = true
	e.mu.Unlock()

	_ = e.ensureSystem(ctx)
	// Already present?
	if out, err := e.run(ctx, "system", "dns", "list"); err == nil && strings.Contains(out, appleHostDNSName) {
		e.mu.Lock()
		e.hostDNSOK = true
		e.mu.Unlock()
		return
	}
	// Try without sudo first, then sudo (may prompt / fail in headless).
	args := []string{"system", "dns", "create", appleHostDNSName, "--localhost", appleHostDNSIP}
	if _, err := e.run(ctx, args...); err != nil {
		sudoArgs := append([]string{e.bin}, args...)
		cmd := exec.CommandContext(ctx, "sudo", "-n")
		cmd.Args = append(cmd.Args, sudoArgs...)
		_ = cmd.Run()
	}
	if out, err := e.run(ctx, "system", "dns", "list"); err == nil && strings.Contains(out, appleHostDNSName) {
		e.mu.Lock()
		e.hostDNSOK = true
		e.mu.Unlock()
	}
}

// RewriteProxyForApple maps 127.0.0.1/localhost proxy host to host.container.internal.
func RewriteProxyForApple(proxyAddr string) string {
	p := strings.TrimSpace(proxyAddr)
	p = strings.TrimPrefix(p, "http://")
	p = strings.TrimPrefix(p, "https://")
	if p == "" {
		return ""
	}
	host, port, ok := strings.Cut(p, ":")
	if !ok {
		return "http://" + appleHostDNSName
	}
	if host == "127.0.0.1" || host == "localhost" || host == "::1" {
		return "http://" + appleHostDNSName + ":" + port
	}
	return "http://" + p
}

func (e *AppleRuntime) ImageExists(ctx context.Context, image string) (bool, error) {
	if err := e.ensureSystem(ctx); err != nil {
		return false, err
	}
	if _, err := e.run(ctx, "image", "inspect", image); err == nil {
		return true, nil
	}
	// Also try short name without localhost/
	alt := strings.TrimPrefix(image, "localhost/")
	if alt != image {
		if _, err := e.run(ctx, "image", "inspect", alt); err == nil {
			return true, nil
		}
	}
	out, err := e.run(ctx, "image", "list", "--format", "json")
	if err != nil {
		out, err = e.run(ctx, "image", "list")
		if err != nil {
			return false, nil
		}
		return strings.Contains(out, image) || strings.Contains(out, alt), nil
	}
	return strings.Contains(out, image) || strings.Contains(out, alt), nil
}

func (e *AppleRuntime) LoadTar(ctx context.Context, tarPath string) error {
	if err := e.ensureSystem(ctx); err != nil {
		return err
	}
	p, tmp, err := prepLoadTar(tarPath)
	if err != nil {
		return err
	}
	if tmp != "" {
		defer os.Remove(tmp)
	}
	_, err = e.run(ctx, "image", "load", "--input", p)
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
	// After load, tag may differ — list and retag first match containing danmo-work-env.
	out, err := e.run(ctx, "image", "list", "--format", "json")
	if err != nil {
		out, _ = e.run(ctx, "image", "list", "--quiet")
		for _, id := range strings.Fields(out) {
			if id == "" {
				continue
			}
			if _, tagErr := e.run(ctx, "image", "tag", id, image); tagErr == nil {
				if ok, _ := e.ImageExists(ctx, image); ok {
					return nil
				}
			}
		}
		return fmt.Errorf("apple-container: image %q not present after load", image)
	}
	var entries []map[string]any
	if json.Unmarshal([]byte(out), &entries) == nil {
		for _, ent := range entries {
			ref := firstString(ent, "reference", "name", "id", "ID")
			if ref == "" {
				continue
			}
			if strings.Contains(ref, "danmo-work-env") || strings.HasPrefix(ref, "sha256:") {
				if _, tagErr := e.run(ctx, "image", "tag", ref, image); tagErr == nil {
					if ok, _ := e.ImageExists(ctx, image); ok {
						return nil
					}
				}
			}
		}
	}
	return fmt.Errorf("apple-container: image %q not present after load", image)
}

func firstString(m map[string]any, keys ...string) string {
	for _, k := range keys {
		if v, ok := m[k]; ok {
			switch t := v.(type) {
			case string:
				if t != "" {
					return t
				}
			}
		}
	}
	return ""
}

func (e *AppleRuntime) ContainerInspectState(ctx context.Context, name string) (string, error) {
	if err := e.ensureSystem(ctx); err != nil {
		return "", err
	}
	out, err := e.run(ctx, "inspect", name)
	if err != nil {
		// Fallback: list --all --format json
		return e.stateFromList(ctx, name)
	}
	return parseAppleStatus(out), nil
}

func (e *AppleRuntime) stateFromList(ctx context.Context, name string) (string, error) {
	out, err := e.run(ctx, "list", "--all", "--format", "json")
	if err != nil {
		return "", nil
	}
	var entries []map[string]any
	if json.Unmarshal([]byte(out), &entries) != nil {
		return "", nil
	}
	for _, ent := range entries {
		id := firstString(ent, "id", "ID", "name", "Name")
		if id != name && !strings.Contains(id, name) {
			continue
		}
		st := firstString(ent, "status", "Status", "state", "State")
		return normalizeAppleState(st), nil
	}
	return "", nil
}

func parseAppleStatus(raw string) string {
	low := strings.ToLower(raw)
	// JSON status field
	var obj any
	if json.Unmarshal([]byte(raw), &obj) == nil {
		switch t := obj.(type) {
		case map[string]any:
			if s := digStatus(t); s != "" {
				return normalizeAppleState(s)
			}
		case []any:
			if len(t) > 0 {
				if m, ok := t[0].(map[string]any); ok {
					if s := digStatus(m); s != "" {
						return normalizeAppleState(s)
					}
				}
			}
		}
	}
	switch {
	case strings.Contains(low, "running"):
		return "running"
	case strings.Contains(low, "exited"), strings.Contains(low, "stopped"):
		return "exited"
	case strings.Contains(low, "created"):
		return "created"
	default:
		if strings.TrimSpace(raw) != "" {
			return "created"
		}
		return ""
	}
}

func digStatus(m map[string]any) string {
	if s := firstString(m, "status", "Status", "state", "State"); s != "" {
		return s
	}
	if st, ok := m["status"].(map[string]any); ok {
		return firstString(st, "status", "state", "Status")
	}
	if st, ok := m["State"].(map[string]any); ok {
		return firstString(st, "Status", "status")
	}
	return ""
}

func normalizeAppleState(s string) string {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "running", "run":
		return "running"
	case "exited", "stopped", "stop":
		return "exited"
	case "created", "create":
		return "created"
	case "paused":
		return "paused"
	default:
		return strings.ToLower(strings.TrimSpace(s))
	}
}

func (e *AppleRuntime) CreateDetached(ctx context.Context, opts CreateOpts) error {
	if err := e.ensureSystem(ctx); err != nil {
		return err
	}
	if opts.Network == "" || opts.Network == "default" {
		// allowlist uses default network + host.container.internal proxy
		e.EnsureHostDNS(ctx)
	}
	args := []string{
		"create",
		"--name", opts.Name,
		"--workdir", opts.Mount,
		"-v", opts.WorkDir + ":" + opts.Mount,
	}
	for _, b := range opts.Binds {
		mode := "rw"
		if b.ReadOnly {
			mode = "ro"
		}
		args = append(args, "-v", b.Host+":"+b.Container+":"+mode)
	}
	switch opts.Network {
	case "", "default":
		// default vmnet
	case "none":
		args = append(args, "--network", "none")
	case "host":
		// Apple has no host net — use default + host DNS for loopback services.
		e.EnsureHostDNS(ctx)
	default:
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
	if err := e.ensureSystem(ctx); err != nil {
		return err
	}
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
	_, err := e.run(ctx, "delete", "-f", name)
	if err != nil {
		_, err = e.run(ctx, "rm", "-f", name)
	}
	if err != nil {
		_, err = e.run(ctx, "delete", name)
	}
	return err
}

func (e *AppleRuntime) Exec(ctx context.Context, name, workdir, command string, env []string) ([]byte, error) {
	if err := e.ensureSystem(ctx); err != nil {
		return nil, err
	}
	args := []string{"exec", "--workdir", workdir}
	for _, ev := range env {
		args = append(args, "-e", ev)
	}
	args = append(args, name, "sh", "-c", command)
	cmd := exec.CommandContext(ctx, e.bin, args...)
	return cmd.CombinedOutput()
}
