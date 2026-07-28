package execution

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
)

const (
	defaultImage  = "localhost/danmo-work-env:bundled"
	defaultMount  = "/workspace"
	containerPref = "danmo-work-proj-"
)

// Manager routes exec_shell to LocalOS sandbox or per-project OCI containers.
type Manager struct {
	mu         sync.Mutex
	envCfg     domain.ConfigEnvironmentSection
	sandboxCfg domain.ConfigSandboxSection
	sandbox    port.Sandbox
	runtime    container.Runtime
	imageReady bool
	tarPath    string
	degraded   bool
	degradeMsg string
	active     map[string]struct{} // projectIDs with containers
}

// New wraps an existing Sandbox for local mode and optional container backend.
func New(envCfg domain.ConfigEnvironmentSection, sandboxCfg domain.ConfigSandboxSection, sb port.Sandbox) *Manager {
	m := &Manager{
		sandbox: sb,
		active:  make(map[string]struct{}),
	}
	m.Configure(envCfg, sandboxCfg)
	return m
}

func (m *Manager) Configure(envCfg domain.ConfigEnvironmentSection, sandboxCfg domain.ConfigSandboxSection) {
	m.mu.Lock()
	defer m.mu.Unlock()
	envCfg = normalizeEnv(envCfg)
	m.envCfg = envCfg
	m.sandboxCfg = sandboxCfg
	m.imageReady = false
	m.degraded = false
	m.degradeMsg = ""
	m.tarPath = container.ResolveTarPath(envCfg.TarPath)
	if envCfg.Backend != domain.EnvironmentBackendContainer {
		m.runtime = nil
		return
	}
	rt, err := container.Detect(envCfg.Engine)
	if err != nil {
		m.runtime = nil
		m.degraded = true
		m.degradeMsg = "container backend requested but no engine: " + err.Error()
		return
	}
	m.runtime = rt
	if m.tarPath == "" {
		m.degraded = true
		m.degradeMsg = "container backend requested but env tar not found — download danmo-work-env-linux-*.tar from GitHub Releases into ~/.danmo-work/env/ (or set WORK_ENV_TAR / make build-env-tar)"
	}
}

func normalizeEnv(cfg domain.ConfigEnvironmentSection) domain.ConfigEnvironmentSection {
	switch strings.ToLower(strings.TrimSpace(string(cfg.Backend))) {
	case "container", "oci":
		cfg.Backend = domain.EnvironmentBackendContainer
	default:
		cfg.Backend = domain.EnvironmentBackendLocal
	}
	switch strings.ToLower(strings.TrimSpace(string(cfg.Engine))) {
	case "podman":
		cfg.Engine = domain.EnvironmentEnginePodman
	case "docker":
		cfg.Engine = domain.EnvironmentEngineDocker
	case "apple-container", "apple", "container":
		cfg.Engine = domain.EnvironmentEngineAppleContainer
	default:
		cfg.Engine = domain.EnvironmentEngineAuto
	}
	if strings.TrimSpace(cfg.Image) == "" {
		cfg.Image = defaultImage
	}
	if strings.TrimSpace(cfg.WorkspaceMount) == "" {
		cfg.WorkspaceMount = defaultMount
	}
	return cfg
}

func (m *Manager) Status() domain.EnvironmentStatus {
	m.mu.Lock()
	defer m.mu.Unlock()
	st := domain.EnvironmentStatus{
		Backend:        m.envCfg.Backend,
		Image:          m.envCfg.Image,
		ImageLoaded:    m.imageReady,
		TarPath:        m.tarPath,
		WorkspaceMount: m.envCfg.WorkspaceMount,
		Resources:      m.envCfg.Resources,
		Degraded:       m.degraded,
		DegradedReason: m.degradeMsg,
	}
	if m.runtime != nil {
		st.Engine = m.runtime.Name()
	}
	if m.degraded && m.envCfg.Backend == domain.EnvironmentBackendContainer {
		st.Backend = domain.EnvironmentBackendLocal // effective
	}
	for id := range m.active {
		st.ActiveProjects = append(st.ActiveProjects, id)
	}
	return st
}

// StatusWithTar fills tar download/install fields (version from server build).
func (m *Manager) StatusWithTar(version string) domain.EnvironmentStatus {
	st := m.Status()
	info := container.InspectTar(version)
	st.TarPresent = info.Present
	st.TarBytes = info.Bytes
	st.TarArch = info.Arch
	st.DownloadURL = info.DownloadURL
	st.AssetName = info.AssetName
	if info.Path != "" {
		st.TarPath = info.Path
	}
	if info.Present && m.tarPath == "" {
		m.mu.Lock()
		m.tarPath = info.Path
		if m.envCfg.Backend == domain.EnvironmentBackendContainer && m.runtime != nil {
			m.degraded = false
			m.degradeMsg = ""
		}
		m.mu.Unlock()
		st = m.Status()
		st.TarPresent = info.Present
		st.TarBytes = info.Bytes
		st.TarArch = info.Arch
		st.DownloadURL = info.DownloadURL
		st.AssetName = info.AssetName
		st.TarPath = info.Path
	}
	return st
}

// NotifyTarInstalled re-resolves tar path after Settings download.
func (m *Manager) NotifyTarInstalled() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.tarPath = container.ResolveTarPath(m.envCfg.TarPath)
	if m.tarPath != "" && m.envCfg.Backend == domain.EnvironmentBackendContainer && m.runtime != nil {
		m.degraded = false
		m.degradeMsg = ""
		m.imageReady = false
	}
}

func (m *Manager) Run(ctx context.Context, opts port.ExecRunOptions) ([]byte, error) {
	if opts.Timeout <= 0 {
		opts.Timeout = 30 * time.Second
	}
	m.mu.Lock()
	useContainer := m.envCfg.Backend == domain.EnvironmentBackendContainer && m.runtime != nil && !m.degraded
	sb := m.sandbox
	m.mu.Unlock()

	if !useContainer {
		return m.runLocal(ctx, opts, sb)
	}
	out, err := m.runContainer(ctx, opts)
	if err != nil && isFatalEngineErr(err) {
		m.mu.Lock()
		m.degraded = true
		m.degradeMsg = err.Error()
		m.mu.Unlock()
		return m.runLocal(ctx, opts, sb)
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

func (m *Manager) runLocal(ctx context.Context, opts port.ExecRunOptions, sb port.Sandbox) ([]byte, error) {
	sopts := port.SandboxRunOptions{
		Command:        opts.Command,
		WorkDir:        opts.WorkDir,
		Timeout:        opts.Timeout,
		Env:            opts.Env,
		AllowNetwork:   opts.AllowNetwork,
		AllowlistProxy: opts.AllowlistProxy,
	}
	if sb != nil {
		return sb.Run(ctx, sopts)
	}
	return nil, fmt.Errorf("execution: no sandbox and container unavailable")
}

func (m *Manager) runContainer(ctx context.Context, opts port.ExecRunOptions) ([]byte, error) {
	if err := m.ensureImage(ctx); err != nil {
		return nil, err
	}
	name, mount, err := m.ensureProjectContainer(ctx, opts)
	if err != nil {
		return nil, err
	}
	execCtx, cancel := context.WithTimeout(ctx, opts.Timeout)
	defer cancel()
	m.mu.Lock()
	engineName := ""
	if m.runtime != nil {
		engineName = m.runtime.Name()
	}
	m.mu.Unlock()
	proxyEnv := proxyEnvFor(opts, engineName)
	return m.engineExec(execCtx, name, mount, opts.Command, proxyEnv)
}

func (m *Manager) engineExec(ctx context.Context, name, mount, command string, env []string) ([]byte, error) {
	m.mu.Lock()
	rt := m.runtime
	m.mu.Unlock()
	if rt == nil {
		return nil, fmt.Errorf("execution: engine missing")
	}
	return rt.Exec(ctx, name, mount, command, env)
}

func (m *Manager) ensureImage(ctx context.Context) error {
	m.mu.Lock()
	rt := m.runtime
	image := m.envCfg.Image
	tarPath := m.tarPath
	ready := m.imageReady
	m.mu.Unlock()
	if rt == nil {
		return fmt.Errorf("execution: no container engine")
	}
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
	m.mu.Lock()
	m.imageReady = true
	m.mu.Unlock()
	return nil
}

func (m *Manager) ensureProjectContainer(ctx context.Context, opts port.ExecRunOptions) (name, mount string, err error) {
	m.mu.Lock()
	image := m.envCfg.Image
	mount = m.envCfg.WorkspaceMount
	resources := m.envCfg.Resources
	engineName := ""
	if m.runtime != nil {
		engineName = m.runtime.Name()
	}
	netMode := containerNetwork(m.sandboxCfg, opts, engineName)
	rt := m.runtime
	m.mu.Unlock()

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

	state, _ := rt.ContainerInspectState(ctx, name)
	switch state {
	case "running":
		m.mu.Lock()
		m.active[projectID] = struct{}{}
		m.mu.Unlock()
		return name, mount, nil
	case "created", "exited", "paused":
		if err := rt.Start(ctx, name); err != nil {
			_ = rt.Rm(ctx, name)
		} else {
			m.mu.Lock()
			m.active[projectID] = struct{}{}
			m.mu.Unlock()
			return name, mount, nil
		}
	}

	create := container.CreateOpts{
		Name:      name,
		Image:     image,
		WorkDir:   workDir,
		Mount:     mount,
		Network:   netMode,
		Env:       proxyEnvFor(opts, engineName),
		Resources: resources,
	}
	if err := rt.CreateDetached(ctx, create); err != nil {
		return "", "", err
	}
	if err := rt.Start(ctx, name); err != nil {
		return "", "", err
	}
	m.mu.Lock()
	m.active[projectID] = struct{}{}
	m.mu.Unlock()
	return name, mount, nil
}

func containerNetwork(cfg domain.ConfigSandboxSection, opts port.ExecRunOptions, engine string) string {
	if opts.AllowNetwork {
		return ""
	}
	apple := engine == string(domain.EnvironmentEngineAppleContainer)
	switch cfg.Network {
	case domain.SandboxNetworkAllow:
		return ""
	case domain.SandboxNetworkAllowlist:
		if apple {
			// No host network on Apple Container — default vmnet + host.container.internal DNS.
			return ""
		}
		return "host"
	default:
		return "none"
	}
}

func proxyEnvFor(opts port.ExecRunOptions, engine string) []string {
	var out []string
	if p := strings.TrimSpace(opts.AllowlistProxy); p != "" {
		u := p
		if engine == string(domain.EnvironmentEngineAppleContainer) {
			u = container.RewriteProxyForApple(p)
		} else if !strings.HasPrefix(u, "http://") && !strings.HasPrefix(u, "https://") {
			u = "http://" + u
		}
		out = append(out,
			"HTTP_PROXY="+u,
			"HTTPS_PROXY="+u,
			"ALL_PROXY="+u,
			"NO_PROXY=localhost,127.0.0.1,host.container.internal",
		)
	}
	return out
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

// Teardown stops and removes the per-project container.
func (m *Manager) Teardown(ctx context.Context, projectID string) error {
	m.mu.Lock()
	rt := m.runtime
	m.mu.Unlock()
	if rt == nil {
		return nil
	}
	id := strings.TrimSpace(projectID)
	if id == "" {
		id = "default"
	}
	name := containerPref + sanitizeID(id)
	_ = rt.Stop(ctx, name)
	err := rt.Rm(ctx, name)
	m.mu.Lock()
	delete(m.active, id)
	m.mu.Unlock()
	return err
}
