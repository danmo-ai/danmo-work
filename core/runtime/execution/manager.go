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
	"danmo-work/core/runtime/egress"
)

const (
	defaultImage = "localhost/danmo-work-env:bundled"
	// containerPref v2: workspace bind uses host abs path (not /workspace).
	containerPref = "danmo-wp-"
)

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

// Manager routes exec_shell to LocalOS sandbox or per-project OCI containers.
type Manager struct {
	mu           sync.Mutex
	envCfg       domain.ConfigEnvironmentSection
	sandboxCfg   domain.ConfigSandboxSection
	sandbox      port.Sandbox
	runtime      container.Runtime
	imageReady   bool
	tarPath      string
	degraded     bool
	degradeMsg   string
	active       map[string]struct{} // projectIDs with containers
	detectEngine domain.EnvironmentEngine
	detectOK     bool
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
	prev := m.envCfg
	m.sandboxCfg = sandboxCfg

	engineChanged := prev.Engine != envCfg.Engine || prev.Backend != envCfg.Backend
	imageChanged := prev.Image != envCfg.Image
	tarOverrideChanged := prev.TarPath != envCfg.TarPath

	m.envCfg = envCfg
	newTar := container.ResolveTarPath(envCfg.TarPath)
	tarPathChanged := newTar != m.tarPath || tarOverrideChanged
	m.tarPath = newTar

	if envCfg.Backend != domain.EnvironmentBackendContainer {
		m.runtime = nil
		m.detectOK = false
		m.degraded = false
		m.degradeMsg = ""
		m.imageReady = false
		return
	}

	needDetect := !m.detectOK || engineChanged || m.runtime == nil
	if needDetect {
		rt, err := container.Detect(envCfg.Engine)
		if err != nil {
			m.runtime = nil
			m.detectOK = false
			m.degraded = true
			m.degradeMsg = "container backend requested but no engine: " + err.Error()
			return
		}
		m.runtime = rt
		m.detectEngine = envCfg.Engine
		m.detectOK = true
	}

	if m.tarPath == "" {
		m.degraded = true
		m.degradeMsg = "container backend requested but env tar not found — download danmo-work-env-linux-*.tar from GitHub Releases into ~/.danmo-work/env/ (or set WORK_ENV_TAR / make build-env-tar)"
	} else if m.degraded && strings.Contains(m.degradeMsg, "env tar not found") {
		m.degraded = false
		m.degradeMsg = ""
	}

	if imageChanged || tarPathChanged || engineChanged {
		m.imageReady = false
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
	if strings.TrimSpace(st.WorkspaceMount) == "" {
		st.WorkspaceMount = "same-as-host"
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
	st.TarVariants = tarVariantsFrom(version)
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
		st.TarVariants = tarVariantsFrom(version)
	}
	return st
}

func tarVariantsFrom(version string) []domain.EnvironmentTarVariant {
	list := container.ListTarVariants(version)
	out := make([]domain.EnvironmentTarVariant, 0, len(list))
	for _, t := range list {
		out = append(out, domain.EnvironmentTarVariant{
			Arch:        t.Arch,
			Present:     t.Present,
			Path:        t.Path,
			Bytes:       t.Bytes,
			DownloadURL: t.DownloadURL,
			AssetName:   t.AssetName,
			Recommended: t.Recommended,
		})
	}
	return out
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
	if sb != nil {
		return sb.Run(ctx, opts.SandboxRunOptions)
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
	proxyEnv := egress.BuildProxyEnv(nil, egress.ProxyEnvOpts{
		ProxyAddr:    opts.AllowlistProxy,
		Engine:       engineName,
		ForContainer: true,
	})
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
	cfgMount := m.envCfg.WorkspaceMount
	resources := m.envCfg.Resources
	engineName := ""
	if m.runtime != nil {
		engineName = m.runtime.Name()
	}
	netMode := egress.ContainerNetworkMode(m.sandboxCfg.Network, opts.AllowNetwork, engineName)
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
	mount = resolveWorkspaceMount(cfgMount, workDir)

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
		Env: egress.BuildProxyEnv(nil, egress.ProxyEnvOpts{
			ProxyAddr:    opts.AllowlistProxy,
			Engine:       engineName,
			ForContainer: true,
		}),
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

// Close stops and removes all tracked project containers.
func (m *Manager) Close() error {
	m.mu.Lock()
	ids := make([]string, 0, len(m.active))
	for id := range m.active {
		ids = append(ids, id)
	}
	m.mu.Unlock()
	var first error
	for _, id := range ids {
		if err := m.Teardown(context.Background(), id); err != nil && first == nil {
			first = err
		}
	}
	return first
}
