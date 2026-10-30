package sandbox

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"danmo-work/core/adapter/container"
	"danmo-work/core/domain"
	"danmo-work/core/port"
	"danmo-work/core/runtime/egress"
)

const (
	defaultContainerImage = domain.EnvImageTagCLI
	// containerPref v3: workspace + read-only $WORK_HOME at the same host paths.
	containerPref = "danmo-wp3-"
	// containerPrefDesktop avoids colliding with cli containers for the same project.
	containerPrefDesktop = "danmo-wp3d-"
	// containerGitCredDir is the in-container path for the git credential bind.
	containerGitCredDir = "/root/.danmo-git-cred"
	// containerGitCredFile is the in-container path of the derived credentials file.
	containerGitCredFile = containerGitCredDir + "/.git-credentials"

	desktopDisplay     = ":1"
	desktopCDPPort     = 9222
	desktopNoVNCPort   = 6080
	desktopShmSize     = "1g"
	desktopReadyWait   = 45 * time.Second
)

// containerBackend runs exec_shell in a per-project OCI container (podman,
// docker, or apple-container). The image is loaded from a user-downloaded tar
// — never a registry pull.
type containerBackend struct {
	mu           sync.Mutex
	engine       domain.EnvironmentEngine
	runtime      container.Runtime
	profile        string
	image          string
	tarPath        string
	tarOverride    string
	mount          string
	resources      domain.EnvironmentResources
	desktopWidth   int
	desktopHeight  int
	imageReady     bool
	tarMissing     bool
	daemonDown     bool
	daemonMsg      string
	active         map[string]struct{}
	endpoints      map[string]domain.DesktopEndpoints // projectID → published ports
	credProvider   port.GitCredentialProvider
}

func newContainerBackend(engine domain.EnvironmentEngine, cfg domain.ConfigSandboxSection) (*containerBackend, error) {
	rt, err := container.Detect(engine)
	if err != nil {
		return nil, err
	}
	b := &containerBackend{
		engine:    engine,
		runtime:   rt,
		active:    make(map[string]struct{}),
		endpoints: make(map[string]domain.DesktopEndpoints),
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

// Configure refreshes cfg-derived fields (profile/image/tar/mount/resources) and
// re-evaluates tar-missing degradation. The runtime and container state survive
// unless the profile/image change requires a different container family.
func (b *containerBackend) Configure(cfg domain.ConfigSandboxSection) {
	b.mu.Lock()
	defer b.mu.Unlock()
	profile := domain.NormalizeSandboxProfile(cfg.Profile)
	image := strings.TrimSpace(cfg.Image)
	if image == "" {
		image = container.DefaultImageTag(profile)
	}
	profileChanged := b.profile != "" && b.profile != profile
	imageChanged := b.image != "" && image != b.image
	if profileChanged || imageChanged {
		// Drop cached readiness; callers must Teardown old containers separately
		// (Manager.Configure recreates the backend via factory cache carefully).
		b.imageReady = false
		b.endpoints = make(map[string]domain.DesktopEndpoints)
	}
	b.profile = profile
	b.image = image
	b.tarOverride = cfg.TarPath
	b.tarPath = container.ResolveTarPathProfile(cfg.TarPath, profile)
	b.mount = cfg.WorkspaceMount
	b.resources = cfg.Resources
	b.desktopWidth = cfg.DesktopWidth
	b.desktopHeight = cfg.DesktopHeight
	if b.desktopWidth <= 0 {
		b.desktopWidth = domain.DefaultDesktopWidth
	}
	if b.desktopHeight <= 0 {
		b.desktopHeight = domain.DefaultDesktopHeight
	}
	b.tarMissing = b.tarPath == ""
	if !b.tarMissing {
		b.daemonDown = false
		b.daemonMsg = ""
	}
}

func (b *containerBackend) Name() domain.SandboxBackend {
	return domain.SandboxBackend(string(b.engine))
}

func (b *containerBackend) RuntimeName() string { return b.runtime.Name() }

func (b *containerBackend) Profile() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.profile
}

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

func (b *containerBackend) DesktopEndpoints(projectID string) (domain.DesktopEndpoints, bool) {
	b.mu.Lock()
	defer b.mu.Unlock()
	id := strings.TrimSpace(projectID)
	if id == "" {
		id = "default"
	}
	ep, ok := b.endpoints[id]
	return ep, ok
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
		if b.profile == domain.SandboxProfileDesktop {
			return "desktop env tar not found — download danmo-work-env-desktop-linux-*.tar.gz from GitHub Releases into ~/.danmo-work/env/ (or set WORK_ENV_DESKTOP_TAR / DQ_ENV_PROFILE=desktop make build-env-tar)"
		}
		return "env tar not found — download danmo-work-env-linux-*.tar.gz from GitHub Releases into ~/.danmo-work/env/ (or set WORK_ENV_TAR / make build-env-tar)"
	default:
		return ""
	}
}

// NotifyTarInstalled re-resolves the tar path after a Settings download.
func (b *containerBackend) NotifyTarInstalled() {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.tarPath = container.ResolveTarPathProfile(b.tarOverride, b.profile)
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
	if domain.NormalizeSandboxProfile(cfg.Profile) == domain.SandboxProfileDesktop || b.profile == domain.SandboxProfileDesktop {
		proxyEnv = append(proxyEnv, "DISPLAY="+desktopDisplay)
	}
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

// ExecDesktop runs a command inside the project's desktop container with DISPLAY set.
func (b *containerBackend) ExecDesktop(ctx context.Context, opts port.SandboxRunOptions, command string, env []string) ([]byte, error) {
	if err := b.ensureImage(ctx); err != nil {
		return nil, err
	}
	b.mu.Lock()
	deskCfg := domain.ConfigSandboxSection{
		Profile:        domain.SandboxProfileDesktop,
		Network:        domain.SandboxNetworkAllow, // desktop needs net for Chromium/CDP internals
		WorkspaceMount: b.mount,
		Resources:      b.resources,
		Image:          b.image,
		TarPath:        b.tarOverride,
		DesktopWidth:   b.desktopWidth,
		DesktopHeight:  b.desktopHeight,
	}
	b.mu.Unlock()
	name, mount, err := b.ensureProjectContainer(ctx, opts, deskCfg)
	if err != nil {
		return nil, err
	}
	execEnv := append([]string{}, env...)
	execEnv = append(execEnv, "DISPLAY="+desktopDisplay)
	execEnv = ensureWorkHomeEnv(execEnv)
	timeout := opts.Timeout
	if timeout <= 0 {
		timeout = 30 * time.Second
	}
	execCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	return b.runtime.Exec(execCtx, name, mount, command, execEnv)
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
	profile := domain.NormalizeSandboxProfile(cfg.Profile)
	if profile == "" {
		b.mu.Lock()
		profile = b.profile
		b.mu.Unlock()
	}
	pref := containerPref
	if profile == domain.SandboxProfileDesktop {
		pref = containerPrefDesktop
	}
	name = pref + sanitizeID(projectID)
	workDir, err := filepath.Abs(opts.WorkDir)
	if err != nil {
		return "", "", err
	}
	if st, err := os.Stat(workDir); err != nil || !st.IsDir() {
		return "", "", fmt.Errorf("execution: workdir %q not a directory", workDir)
	}
	mount = resolveWorkspaceMount(b.mount, workDir)
	netMode := egress.ContainerNetworkMode(cfg.Network, opts.AllowNetwork, b.runtime.Name())
	// Desktop Chromium needs outbound DNS/HTTP unless the user explicitly denies.
	if profile == domain.SandboxProfileDesktop && netMode == "none" && !opts.AllowNetwork {
		// Keep deny when configured; agent must approve network for browsing.
	}

	state, _ := b.runtime.ContainerInspectState(ctx, name)
	switch state {
	case "running":
		b.mu.Lock()
		b.active[projectID] = struct{}{}
		b.mu.Unlock()
		_ = b.refreshEndpoints(ctx, projectID, name, profile)
		return name, mount, nil
	case "created", "exited", "paused":
		if err := b.runtime.Start(ctx, name); err != nil {
			_ = b.runtime.Rm(ctx, name)
		} else {
			b.mu.Lock()
			b.active[projectID] = struct{}{}
			b.mu.Unlock()
			_ = b.refreshEndpoints(ctx, projectID, name, profile)
			if profile == domain.SandboxProfileDesktop {
				_ = b.waitDesktopReady(ctx, name, mount)
			}
			return name, mount, nil
		}
	}

	env := egress.BuildProxyEnv(nil, egress.ProxyEnvOpts{
		ProxyAddr:    opts.AllowlistProxy,
		Engine:       b.runtime.Name(),
		ForContainer: true,
	})
	env = ensureWorkHomeEnv(env)
	if profile == domain.SandboxProfileDesktop {
		w, h := cfg.DesktopWidth, cfg.DesktopHeight
		if w <= 0 {
			w = domain.DefaultDesktopWidth
		}
		if h <= 0 {
			h = domain.DefaultDesktopHeight
		}
		env = append(env,
			"DISPLAY="+desktopDisplay,
			fmt.Sprintf("DISPLAY_WIDTH=%d", w),
			fmt.Sprintf("DISPLAY_HEIGHT=%d", h),
		)
	}
	var binds []container.Bind
	if bind, ok := workHomeBind(workDir); ok {
		binds = append(binds, bind)
	}
	b.mu.Lock()
	credProvider := b.credProvider
	image := b.image
	resources := b.resources
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
		Image:     image,
		WorkDir:   workDir,
		Mount:     mount,
		Network:   netMode,
		Env:       env,
		Binds:     binds,
		Resources: resources,
	}
	if profile == domain.SandboxProfileDesktop {
		create.ShmSize = desktopShmSize
		// Fixed host ports match container defaults so browser_* / Settings can
		// always use http://127.0.0.1:9222 (same as a host Chrome with CDP).
		create.Ports = []container.PortPublish{
			{HostIP: "127.0.0.1", HostPort: desktopCDPPort, ContainerPort: desktopCDPPort},
			{HostIP: "127.0.0.1", HostPort: desktopNoVNCPort, ContainerPort: desktopNoVNCPort},
		}
		// Use image ENTRYPOINT (desktop-entrypoint.sh); do not override with sleep.
		create.KeepImageCmd = true
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
	_ = b.refreshEndpoints(ctx, projectID, name, profile)
	if profile == domain.SandboxProfileDesktop {
		_ = b.waitDesktopReady(ctx, name, mount)
	}
	return name, mount, nil
}

func (b *containerBackend) refreshEndpoints(ctx context.Context, projectID, name, profile string) error {
	if profile != domain.SandboxProfileDesktop {
		return nil
	}
	// Defaults match container Chromium/noVNC and the fixed HostPort publish.
	ep := domain.DesktopEndpoints{
		Display:   desktopDisplay,
		Container: name,
		CDPPort:   desktopCDPPort,
		CDPURL:    fmt.Sprintf("http://127.0.0.1:%d", desktopCDPPort),
		NoVNCPort: desktopNoVNCPort,
		NoVNCURL:  fmt.Sprintf("http://127.0.0.1:%d/vnc.html", desktopNoVNCPort),
	}
	if ports, err := b.runtime.InspectPublishedPorts(ctx, name); err == nil {
		if hp, ok := ports[desktopCDPPort]; ok && hp > 0 {
			ep.CDPPort = hp
			ep.CDPURL = fmt.Sprintf("http://127.0.0.1:%d", hp)
		}
		if hp, ok := ports[desktopNoVNCPort]; ok && hp > 0 {
			ep.NoVNCPort = hp
			ep.NoVNCURL = fmt.Sprintf("http://127.0.0.1:%d/vnc.html", hp)
		}
	}
	b.mu.Lock()
	b.endpoints[projectID] = ep
	b.mu.Unlock()
	return nil
}

func (b *containerBackend) waitDesktopReady(ctx context.Context, name, mount string) error {
	deadline := time.Now().Add(desktopReadyWait)
	displayOK := false
	for time.Now().Before(deadline) {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		if !displayOK {
			out, err := b.runtime.Exec(ctx, name, mount, "xdpyinfo >/dev/null 2>&1 && echo OK", []string{"DISPLAY=" + desktopDisplay})
			if err == nil && strings.Contains(string(out), "OK") {
				displayOK = true
			}
		}
		if displayOK {
			// Chromium CDP must answer before browser_* attach.
			out, err := b.runtime.Exec(ctx, name, mount,
				fmt.Sprintf(`curl -fsS --max-time 1 http://127.0.0.1:%d/json/version >/dev/null && echo CDP_OK`, desktopCDPPort),
				[]string{"DISPLAY=" + desktopDisplay})
			if err == nil && strings.Contains(string(out), "CDP_OK") {
				return nil
			}
		}
		time.Sleep(250 * time.Millisecond)
	}
	if !displayOK {
		return fmt.Errorf("desktop: display %s not ready within %s", desktopDisplay, desktopReadyWait)
	}
	return fmt.Errorf("desktop: Chromium CDP :%d not ready within %s", desktopCDPPort, desktopReadyWait)
}

// Teardown stops and removes the per-project container.
func (b *containerBackend) Teardown(ctx context.Context, projectID string) error {
	id := strings.TrimSpace(projectID)
	if id == "" {
		id = "default"
	}
	b.mu.Lock()
	profile := b.profile
	b.mu.Unlock()
	pref := containerPref
	if profile == domain.SandboxProfileDesktop {
		pref = containerPrefDesktop
	}
	name := pref + sanitizeID(id)
	_ = b.runtime.Stop(ctx, name)
	err := b.runtime.Rm(ctx, name)
	// Also remove the sibling profile container if present.
	other := containerPref + sanitizeID(id)
	if profile == domain.SandboxProfileDesktop {
		other = containerPref + sanitizeID(id)
	} else {
		other = containerPrefDesktop + sanitizeID(id)
	}
	_ = b.runtime.Stop(ctx, other)
	_ = b.runtime.Rm(ctx, other)
	b.mu.Lock()
	delete(b.active, id)
	delete(b.endpoints, id)
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
