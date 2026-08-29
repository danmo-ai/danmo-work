package sandbox

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"danmo-work/core/adapter/container"
	"danmo-work/core/domain"
	"danmo-work/core/port"
	"danmo-work/core/runtime/egress"
)

const (
	defaultContainerImage = "localhost/danmo-work-env:bundled"
	// containerPref v3: workspace + read-only $WORK_HOME at the same host paths.
	containerPref = "danmo-wp3-"
	// containerGitCredDir is the in-container path for the git credential bind.
	containerGitCredDir = "/root/.danmo-git-cred"
	// containerGitCredFile is the in-container path of the derived credentials file.
	containerGitCredFile = containerGitCredDir + "/.git-credentials"
)

// containerBackend runs exec_shell in a per-project OCI container (podman,
// docker, or apple-container). The image is loaded from a user-downloaded tar
// — never a registry pull.
type containerBackend struct {
	mu           sync.Mutex
	engine       domain.EnvironmentEngine
	runtime      container.Runtime
	image        string
	tarPath      string
	tarOverride  string
	mount        string
	resources    domain.EnvironmentResources
	imageReady   bool
	tarMissing   bool
	daemonDown   bool
	daemonMsg    string
	active       map[string]struct{}
	credProvider port.GitCredentialProvider
}

func newContainerBackend(engine domain.EnvironmentEngine, cfg domain.ConfigSandboxSection) (*containerBackend, error) {
	rt, err := container.Detect(engine)
	if err != nil {
		return nil, err
	}
	b := &containerBackend{
		engine:  engine,
		runtime: rt,
		active:  make(map[string]struct{}),
	}
	b.Configure(cfg)
	return b, nil
}

// SetGitCredentials attaches the credential provider used when creating
// project containers. Containers created before credentials existed still see
// updates: the provider dir is always bind-mounted and its file content
// changes are visible live through the mount.
func (b *containerBackend) SetGitCredentials(p port.GitCredentialProvider) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.credProvider = p
}

// Configure refreshes cfg-derived fields (image/tar/mount/resources) and
// re-evaluates tar-missing degradation. The runtime and container state survive.
func (b *containerBackend) Configure(cfg domain.ConfigSandboxSection) {
	b.mu.Lock()
	defer b.mu.Unlock()
	image := strings.TrimSpace(cfg.Image)
	if image == "" {
		image = defaultContainerImage
	}
	if image != b.image {
		b.image = image
		b.imageReady = false
	}
	b.tarOverride = cfg.TarPath
	b.tarPath = container.ResolveTarPath(cfg.TarPath)
	b.mount = cfg.WorkspaceMount
	b.resources = cfg.Resources
	b.tarMissing = b.tarPath == ""
	if !b.tarMissing {
		// Engine errors are retried on the next Run.
		b.daemonDown = false
		b.daemonMsg = ""
	}
}

func (b *containerBackend) Name() domain.SandboxBackend {
	return domain.SandboxBackend(string(b.engine))
}

func (b *containerBackend) RuntimeName() string { return b.runtime.Name() }

func (b *containerBackend) ImageReady() bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.imageReady
}

func (b *containerBackend) ActiveProjects() []string {
	b.mu.Lock()
	defer b.mu.Unlock()
	out := make([]string, 0, len(b.active))
	for id := range b.active {
		out = append(out, id)
	}
	return out
}

func (b *containerBackend) Degraded() bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.tarMissing || b.daemonDown
}

func (b *containerBackend) DegradedReason() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	switch {
	case b.daemonDown:
		return b.daemonMsg
	case b.tarMissing:
		return "env tar not found — download danmo-work-env-linux-*.tar.gz from GitHub Releases into ~/.danmo-work/env/ (or set WORK_ENV_TAR / make build-env-tar)"
	default:
		return ""
	}
}

// NotifyTarInstalled re-resolves the tar path after a Settings download.
func (b *containerBackend) NotifyTarInstalled() {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.tarPath = container.ResolveTarPath(b.tarOverride)
	b.tarMissing = b.tarPath == ""
	if !b.tarMissing {
		b.daemonDown = false
		b.daemonMsg = ""
		b.imageReady = false
	}
}

func (b *containerBackend) Run(ctx context.Context, opts port.SandboxRunOptions, cfg domain.ConfigSandboxSection) ([]byte, error) {
	if err := b.ensureImage(ctx); err != nil {
		return nil, err
	}
	name, mount, err := b.ensureProjectContainer(ctx, opts, cfg)
	if err != nil {
		return nil, err
	}
	execCtx, cancel := context.WithTimeout(ctx, opts.Timeout)
	defer cancel()
	proxyEnv := egress.BuildProxyEnv(nil, egress.ProxyEnvOpts{
		ProxyAddr:    opts.AllowlistProxy,
		Engine:       b.runtime.Name(),
		ForContainer: true,
	})
	proxyEnv = ensureWorkHomeEnv(proxyEnv)
	out, err := b.runtime.Exec(execCtx, name, mount, opts.Command, proxyEnv)
	if err != nil && isFatalEngineErr(err) {
		b.mu.Lock()
		b.daemonDown = true
		b.daemonMsg = err.Error()
		b.mu.Unlock()
	} else if err == nil {
		b.mu.Lock()
		if b.daemonDown {
			b.daemonDown = false
			b.daemonMsg = ""
		}
		b.mu.Unlock()
	}
	return out, err
}

func isFatalEngineErr(err error) bool {
	if err == nil {
		return false
	}
	s := strings.ToLower(err.Error())
	return strings.Contains(s, "cannot connect") || strings.Contains(s, "permission denied")
}

func (b *containerBackend) ensureImage(ctx context.Context) error {
	b.mu.Lock()
	rt := b.runtime
	image := b.image
	tarPath := b.tarPath
	ready := b.imageReady
	b.mu.Unlock()
	if ready {
		return nil
	}
	ok, _ := rt.ImageExists(ctx, image)
	if !ok {
		if tarPath == "" {
			return fmt.Errorf("execution: image %q missing and no tar to load", image)
		}
		if err := rt.LoadTar(ctx, tarPath); err != nil {
			return fmt.Errorf("execution: load tar: %w", err)
		}
		if err := rt.EnsureTag(ctx, image); err != nil {
			return err
		}
	}
	b.mu.Lock()
	b.imageReady = true
	b.mu.Unlock()
	return nil
}

func (b *containerBackend) ensureProjectContainer(ctx context.Context, opts port.SandboxRunOptions, cfg domain.ConfigSandboxSection) (name, mount string, err error) {
	projectID := strings.TrimSpace(opts.ProjectID)
	if projectID == "" {
		projectID = "default"
	}
	name = containerPref + sanitizeID(projectID)
	workDir, err := filepath.Abs(opts.WorkDir)
	if err != nil {
		return "", "", err
	}
	if st, err := os.Stat(workDir); err != nil || !st.IsDir() {
		return "", "", fmt.Errorf("execution: workdir %q not a directory", workDir)
	}
	mount = resolveWorkspaceMount(b.mount, workDir)
	netMode := egress.ContainerNetworkMode(cfg.Network, opts.AllowNetwork, b.runtime.Name())

	state, _ := b.runtime.ContainerInspectState(ctx, name)
	switch state {
	case "running":
		b.mu.Lock()
		b.active[projectID] = struct{}{}
		b.mu.Unlock()
		return name, mount, nil
	case "created", "exited", "paused":
		if err := b.runtime.Start(ctx, name); err != nil {
			_ = b.runtime.Rm(ctx, name)
		} else {
			b.mu.Lock()
			b.active[projectID] = struct{}{}
			b.mu.Unlock()
			return name, mount, nil
		}
	}

	env := egress.BuildProxyEnv(nil, egress.ProxyEnvOpts{
		ProxyAddr:    opts.AllowlistProxy,
		Engine:       b.runtime.Name(),
		ForContainer: true,
	})
	env = ensureWorkHomeEnv(env)
	var binds []container.Bind
	if bind, ok := workHomeBind(workDir); ok {
		binds = append(binds, bind)
	}
	b.mu.Lock()
	credProvider := b.credProvider
	b.mu.Unlock()
	if credProvider != nil {
		if dir := credProvider.CredentialDir(); dir != "" {
			binds = append(binds, container.Bind{
				Host:      dir,
				Container: containerGitCredDir,
				ReadOnly:  true,
			})
			env = append(env,
				"GIT_CONFIG_COUNT=2",
				"GIT_CONFIG_KEY_0=credential.helper",
				"GIT_CONFIG_VALUE_0=store --file="+containerGitCredFile,
			)
		}
	}

	create := container.CreateOpts{
		Name:      name,
		Image:     b.image,
		WorkDir:   workDir,
		Mount:     mount,
		Network:   netMode,
		Env:       env,
		Binds:     binds,
		Resources: b.resources,
	}
	if err := b.runtime.CreateDetached(ctx, create); err != nil {
		return "", "", err
	}
	if err := b.runtime.Start(ctx, name); err != nil {
		return "", "", err
	}
	b.mu.Lock()
	b.active[projectID] = struct{}{}
	b.mu.Unlock()
	return name, mount, nil
}

// Teardown stops and removes the per-project container.
func (b *containerBackend) Teardown(ctx context.Context, projectID string) error {
	id := strings.TrimSpace(projectID)
	if id == "" {
		id = "default"
	}
	name := containerPref + sanitizeID(id)
	_ = b.runtime.Stop(ctx, name)
	err := b.runtime.Rm(ctx, name)
	b.mu.Lock()
	delete(b.active, id)
	b.mu.Unlock()
	return err
}

// Close stops and removes all tracked project containers.
func (b *containerBackend) Close() error {
	b.mu.Lock()
	ids := make([]string, 0, len(b.active))
	for id := range b.active {
		ids = append(ids, id)
	}
	b.mu.Unlock()
	var first error
	for _, id := range ids {
		if err := b.Teardown(context.Background(), id); err != nil && first == nil {
			first = err
		}
	}
	return first
}

// resolveWorkspaceMount picks the in-container path for the project bind.
// Empty / "same" / "host" → identical to the host absolute workDir so file tools
// and exec_shell share the same absolute paths.
func resolveWorkspaceMount(cfgMount, workDirAbs string) string {
	m := strings.TrimSpace(cfgMount)
	switch strings.ToLower(m) {
	case "", "same", "host", "$workdir", "${workdir}":
		return workDirAbs
	default:
		return m
	}
}

func sanitizeID(id string) string {
	var b strings.Builder
	for _, r := range id {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '-', r == '_':
			b.WriteRune(r)
		default:
			b.WriteByte('-')
		}
	}
	s := b.String()
	if s == "" {
		return "default"
	}
	if len(s) > 80 {
		s = s[:80]
	}
	return s
}
