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
	"danmo-work/core/paths"
	"danmo-work/core/port"

	"github.com/chromedp/chromedp"
)

var _ port.Browser = (*Manager)(nil)

const sessionIdleTTL = 15 * time.Minute

// Manager launches or attaches to a CDP browser for one-shot HTML renders
// and sticky interactive pages (browser_* tools).
type Manager struct {
	mu     sync.Mutex
	cfg    domain.ConfigBrowserSection
	status domain.BrowserStatus

	// active holds cancel funcs for in-flight one-shot Launch sessions (for Close).
	active map[uint64]context.CancelFunc
	nextID uint64

	// Sticky interactive sessions.
	pages       map[string]*pageSession
	allocCtx    context.Context
	cancelAlloc context.CancelFunc
	allocMode   string // attach | launch | ""
	stopSweep   chan struct{}
}

// New creates a browser manager and probes availability.
func New(cfg domain.ConfigBrowserSection) *Manager {
	m := &Manager{
		active:    make(map[uint64]context.CancelFunc),
		pages:     make(map[string]*pageSession),
		stopSweep: make(chan struct{}),
	}
	m.Configure(cfg)
	go m.idleSweepLoop()
	return m
}

func normalizeBrowserConfig(cfg domain.ConfigBrowserSection) domain.ConfigBrowserSection {
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

// Close cancels sticky pages, shared allocator, and in-flight one-shot renders.
func (m *Manager) Close(ctx context.Context) error {
	_ = m.CloseAll(ctx)
	m.mu.Lock()
	cancels := make([]context.CancelFunc, 0, len(m.active))
	for id, cancel := range m.active {
		cancels = append(cancels, cancel)
		delete(m.active, id)
	}
	select {
	case <-m.stopSweep:
	default:
		close(m.stopSweep)
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

// AcquirePage returns the sticky page for sessionID, creating it if needed.
func (m *Manager) AcquirePage(ctx context.Context, sessionID string) (port.BrowserPage, error) {
	sessionID = strings.TrimSpace(sessionID)
	if sessionID == "" {
		return nil, fmt.Errorf("session id is required")
	}
	_ = ctx

	m.mu.Lock()
	defer m.mu.Unlock()

	if !m.cfg.Enabled {
		return nil, fmt.Errorf("browser rendering is disabled")
	}
	if !m.status.Available {
		reason := m.status.DegradedReason
		if reason == "" {
			reason = "no browser engine available"
		}
		return nil, fmt.Errorf("browser unavailable: %s", reason)
	}

	if p, ok := m.pages[sessionID]; ok && p != nil && p.cancel != nil {
		p.touch()
		return p, nil
	}

	if err := m.ensureAllocatorLocked(); err != nil {
		return nil, err
	}

	tabCtx, cancelTab := chromedp.NewContext(m.allocCtx)
	// Warm the target so the first navigate is reliable.
	if err := chromedp.Run(tabCtx); err != nil {
		cancelTab()
		return nil, fmt.Errorf("browser tab start failed: %w", err)
	}

	p := &pageSession{
		id:       sessionID,
		tabCtx:   tabCtx,
		cancel:   cancelTab,
		lastUsed: time.Now(),
		mgr:      m,
	}
	m.pages[sessionID] = p
	return p, nil
}

// ClosePage tears down the sticky page for sessionID (idempotent).
func (m *Manager) ClosePage(ctx context.Context, sessionID string) error {
	_ = ctx
	sessionID = strings.TrimSpace(sessionID)
	if sessionID == "" {
		return nil
	}
	m.mu.Lock()
	p := m.pages[sessionID]
	delete(m.pages, sessionID)
	m.maybeStopAllocatorLocked()
	m.mu.Unlock()
	if p == nil {
		return nil
	}
	p.mu.Lock()
	cancel := p.cancel
	p.cancel = nil
	p.mu.Unlock()
	if cancel != nil {
		cancel()
	}
	return nil
}

// CloseAll tears down every sticky page and the shared allocator.
func (m *Manager) CloseAll(ctx context.Context) error {
	_ = ctx
	m.mu.Lock()
	pages := make([]*pageSession, 0, len(m.pages))
	for id, p := range m.pages {
		pages = append(pages, p)
		delete(m.pages, id)
	}
	cancelAlloc := m.cancelAlloc
	m.cancelAlloc = nil
	m.allocCtx = nil
	m.allocMode = ""
	m.mu.Unlock()

	for _, p := range pages {
		if p == nil {
			continue
		}
		p.mu.Lock()
		cancel := p.cancel
		p.cancel = nil
		p.mu.Unlock()
		if cancel != nil {
			cancel()
		}
	}
	if cancelAlloc != nil {
		cancelAlloc()
	}
	return nil
}

func (m *Manager) removePage(sessionID string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.pages, sessionID)
	m.maybeStopAllocatorLocked()
}

func (m *Manager) ensureAllocatorLocked() error {
	if m.allocCtx != nil && m.cancelAlloc != nil {
		return nil
	}
	st := m.status
	cfg := m.cfg
	parent := context.Background()

	var allocCtx context.Context
	var cancelAlloc context.CancelFunc
	if st.Mode == "attach" {
		allocCtx, cancelAlloc = chromedp.NewRemoteAllocator(parent, cfg.CDPURL)
		m.allocMode = "attach"
	} else {
		profileDir := filepath.Join(paths.Home(), "browser", "profile")
		if err := os.MkdirAll(profileDir, 0o755); err != nil {
			return fmt.Errorf("create browser profile dir: %w", err)
		}
		execOpts := append(chromedp.DefaultExecAllocatorOptions[:],
			chromedp.ExecPath(st.Path),
			chromedp.UserDataDir(profileDir),
			chromedp.Flag("headless", true),
			chromedp.Flag("disable-gpu", true),
			chromedp.Flag("no-first-run", true),
			chromedp.Flag("no-default-browser-check", true),
			chromedp.Flag("disable-background-networking", true),
			chromedp.Flag("remote-debugging-address", "127.0.0.1"),
		)
		allocCtx, cancelAlloc = chromedp.NewExecAllocator(parent, execOpts...)
		m.allocMode = "launch"
	}
	m.allocCtx = allocCtx
	m.cancelAlloc = cancelAlloc
	return nil
}

func (m *Manager) maybeStopAllocatorLocked() {
	if len(m.pages) > 0 {
		return
	}
	if m.cancelAlloc != nil {
		m.cancelAlloc()
		m.cancelAlloc = nil
		m.allocCtx = nil
		m.allocMode = ""
	}
}

func (m *Manager) idleSweepLoop() {
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()
	for {
		select {
		case <-m.stopSweep:
			return
		case <-ticker.C:
			m.sweepIdlePages()
		}
	}
}

func (m *Manager) sweepIdlePages() {
	cutoff := time.Now().Add(-sessionIdleTTL)
	m.mu.Lock()
	var stale []string
	for id, p := range m.pages {
		if p == nil || p.lastUsed.Before(cutoff) {
			stale = append(stale, id)
		}
	}
	m.mu.Unlock()
	for _, id := range stale {
		_ = m.ClosePage(context.Background(), id)
	}
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
