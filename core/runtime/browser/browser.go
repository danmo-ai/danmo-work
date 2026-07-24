package browser

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"danmo-work/core/domain"
	"danmo-work/core/port"
)

var _ port.Browser = (*Manager)(nil)

// Manager launches or attaches to a CDP browser for one-shot HTML renders.
type Manager struct {
	mu     sync.Mutex
	cfg    domain.ConfigBrowserSection
	status domain.BrowserStatus

	// active holds cancel funcs for in-flight Launch sessions (for Close).
	active map[uint64]context.CancelFunc
	nextID uint64
}

// New creates a browser manager and probes availability.
func New(cfg domain.ConfigBrowserSection) *Manager {
	m := &Manager{
		active: make(map[uint64]context.CancelFunc),
	}
	m.Configure(cfg)
	return m
}

func normalizeBrowserConfig(cfg domain.ConfigBrowserSection) domain.ConfigBrowserSection {
	// Enabled defaults to true when unset via viper; zero value false only if explicitly disabled.
	cfg.ExecutablePath = strings.TrimSpace(cfg.ExecutablePath)
	cfg.CDPURL = strings.TrimSpace(cfg.CDPURL)
	return cfg
}

// Configure replaces policy and re-probes the engine.
func (m *Manager) Configure(cfg domain.ConfigBrowserSection) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.cfg = normalizeBrowserConfig(cfg)
	m.status = probeStatus(m.cfg)
}

// Status returns the last probed capability surface.
func (m *Manager) Status() domain.BrowserStatus {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.status
}

// Close cancels any in-flight Launch sessions. Attach clients are cancelled
// the same way (CDP only); remote browsers are not killed.
func (m *Manager) Close(ctx context.Context) error {
	_ = ctx
	m.mu.Lock()
	cancels := make([]context.CancelFunc, 0, len(m.active))
	for id, cancel := range m.active {
		cancels = append(cancels, cancel)
		delete(m.active, id)
	}
	m.mu.Unlock()
	for _, cancel := range cancels {
		cancel()
	}
	return nil
}

func (m *Manager) track(cancel context.CancelFunc) (id uint64, untrack func()) {
	m.mu.Lock()
	m.nextID++
	id = m.nextID
	m.active[id] = cancel
	m.mu.Unlock()
	return id, func() {
		m.mu.Lock()
		delete(m.active, id)
		m.mu.Unlock()
	}
}

// RenderHTML navigates to opts.URL and returns the serialized DOM HTML.
func (m *Manager) RenderHTML(ctx context.Context, opts port.BrowserRenderOptions) (string, string, error) {
	if opts.URL == "" {
		return "", "", fmt.Errorf("url is required")
	}
	timeout := opts.Timeout
	if timeout <= 0 {
		timeout = 30 * time.Second
	}

	m.mu.Lock()
	cfg := m.cfg
	st := m.status
	m.mu.Unlock()

	if !cfg.Enabled {
		return "", "", fmt.Errorf("browser rendering is disabled")
	}
	if !st.Available {
		reason := st.DegradedReason
		if reason == "" {
			reason = "no browser engine available"
		}
		return "", "", fmt.Errorf("browser unavailable: %s", reason)
	}

	userDataDir := ""
	if st.Mode != "attach" {
		dir, err := os.MkdirTemp("", "danmo-work-browser-*")
		if err != nil {
			return "", "", fmt.Errorf("create browser user-data-dir: %w", err)
		}
		userDataDir = dir
		defer func() { _ = os.RemoveAll(userDataDir) }()
	}

	return m.renderOnce(ctx, cfg, st, opts, timeout, userDataDir)
}

func probeStatus(cfg domain.ConfigBrowserSection) domain.BrowserStatus {
	st := domain.BrowserStatus{
		Enabled: cfg.Enabled,
		Mode:    "none",
		Engine:  "none",
	}
	if !cfg.Enabled {
		st.DegradedReason = "browser disabled in config"
		return st
	}
	if cfg.CDPURL != "" {
		st.Available = true
		st.Mode = "attach"
		st.Engine = "cdp"
		st.Path = cfg.CDPURL
		return st
	}
	path, engine := resolveExecutable(cfg.ExecutablePath)
	if path == "" {
		st.DegradedReason = "no Chrome/Edge/Chromium found; install a browser or set runtime.browser.executable_path / cdp_url"
		return st
	}
	st.Available = true
	st.Mode = "launch"
	st.Engine = engine
	st.Path = path
	return st
}

func resolveExecutable(configured string) (path, engine string) {
	if configured != "" {
		if info, err := os.Stat(configured); err == nil && !info.IsDir() {
			return configured, engineFromPath(configured)
		}
		if p, err := lookPath(configured); err == nil {
			return p, engineFromPath(p)
		}
	}
	return discoverBrowser()
}

func engineFromPath(p string) string {
	lower := strings.ToLower(filepath.Base(p))
	switch {
	case strings.Contains(lower, "edge"):
		return "edge"
	case strings.Contains(lower, "chromium"):
		return "chromium"
	case strings.Contains(lower, "chrome"):
		return "chrome"
	case strings.Contains(lower, "lightpanda"):
		return "chromium"
	default:
		return "chrome"
	}
}
