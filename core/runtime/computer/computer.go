// Package computer implements the OS-agnostic desktop-control surface behind
// the `computer` builtin tool. The Manager holds configuration and delegates to
// a per-OS backend (X11 via xdotool, macOS via osascript/screencapture, Windows
// via user32/PowerShell) or a sandbox desktop backend when
// runtime.sandbox.profile=desktop.
package computer

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"danmo-work/core/domain"
	"danmo-work/core/port"
)

var _ port.Computer = (*Manager)(nil)

// backend is the OS-specific implementation selected at construction time.
type backend interface {
	probe(cfg domain.ConfigComputerSection) domain.ComputerStatus
	listWindows(ctx context.Context, query string) ([]domain.WindowInfo, error)
	focusWindow(ctx context.Context, id string) error
	windowBounds(ctx context.Context, id string) (domain.WindowInfo, error)
	screenshot(ctx context.Context, windowID string) (domain.ComputerImage, error)
	mouseMove(ctx context.Context, x, y int) error
	mouseClick(ctx context.Context, button domain.MouseButton, x, y, clicks int) error
	mouseDrag(ctx context.Context, x0, y0, x1, y1 int) error
	scroll(ctx context.Context, x, y int, dir domain.ScrollDirection, amount int) error
	typeText(ctx context.Context, text string) error
	key(ctx context.Context, key string) error
	cursorPosition(ctx context.Context) (int, int, error)
}

// Manager is the port.Computer implementation used by bootstrap.
type Manager struct {
	mu       sync.Mutex
	cfg      domain.ConfigComputerSection
	status   domain.ComputerStatus
	host     backend
	desktop  *desktopBackend
	backend  backend
	env      port.DesktopEnvironment
	// lastShot maps model image coords → screen coords after downscale.
	lastImgW, lastImgH   int
	lastDispW, lastDispH int
}

// New builds a Manager for the current OS and probes availability.
func New(cfg domain.ConfigComputerSection) *Manager {
	m := &Manager{host: newBackend()}
	m.backend = m.host
	m.Configure(cfg)
	return m
}

// SetDesktopEnvironment wires the sandbox desktop bridge. When the active
// sandbox profile is "desktop", computer actions target the container DISPLAY.
func (m *Manager) SetDesktopEnvironment(env port.DesktopEnvironment) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.env = env
	if env != nil {
		m.desktop = newDesktopBackend(env)
	} else {
		m.desktop = nil
	}
	m.reselectLocked()
}

// SetSession updates project/workdir for sandboxed desktop exec (from tool input).
func (m *Manager) SetSession(opts port.SandboxRunOptions) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.desktop != nil {
		m.desktop.SetSession(opts)
	}
}

// Configure replaces policy and re-probes the backend.
func (m *Manager) Configure(cfg domain.ConfigComputerSection) {
	m.mu.Lock()
	defer m.mu.Unlock()
	cfg.Display = strings.TrimSpace(cfg.Display)
	m.cfg = cfg
	m.reselectLocked()
}

func (m *Manager) reselectLocked() {
	if m.desktop != nil && m.env != nil && m.env.DesktopProfile() == domain.SandboxProfileDesktop {
		m.backend = m.desktop
	} else {
		m.backend = m.host
	}
	m.status = m.backend.probe(m.cfg)
	m.status.Enabled = m.cfg.Enabled
}

// Status returns the last probed capability surface.
func (m *Manager) Status() domain.ComputerStatus {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.reselectLocked()
	return m.status
}

func (m *Manager) ready() error {
	m.mu.Lock()
	m.reselectLocked()
	st := m.status
	enabled := m.cfg.Enabled
	m.mu.Unlock()
	if !enabled {
		return fmt.Errorf("desktop control is disabled; set runtime.computer.enabled=true in config to allow the computer tool")
	}
	if !st.Available {
		reason := st.DegradedReason
		if reason == "" {
			reason = "no usable desktop backend on this host"
		}
		return fmt.Errorf("desktop control unavailable: %s", reason)
	}
	return nil
}

func (m *Manager) maxShotEdge() int {
	m.mu.Lock()
	edge := m.cfg.MaxShotEdge
	env := m.env
	m.mu.Unlock()
	if edge != 0 {
		return edge // negative disables downscale
	}
	if env != nil {
		if n := env.DesktopMaxShotEdge(); n > 0 {
			return n
		}
	}
	return domain.DefaultDesktopMaxShotEdge
}

func (m *Manager) rememberShot(img domain.ComputerImage) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.lastImgW, m.lastImgH = img.Width, img.Height
	m.lastDispW, m.lastDispH = img.DisplayWidth, img.DisplayHeight
	if m.lastDispW == 0 {
		m.lastDispW = img.Width
	}
	if m.lastDispH == 0 {
		m.lastDispH = img.Height
	}
}

func (m *Manager) mapXY(x, y int) (int, int) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return mapShotCoord(x, y, m.lastImgW, m.lastImgH, m.lastDispW, m.lastDispH)
}

func (m *Manager) ListWindows(ctx context.Context, query string) ([]domain.WindowInfo, error) {
	if err := m.ready(); err != nil {
		return nil, err
	}
	m.mu.Lock()
	b := m.backend
	m.mu.Unlock()
	return b.listWindows(ctx, query)
}

func (m *Manager) FocusWindow(ctx context.Context, id string) error {
	if err := m.ready(); err != nil {
		return err
	}
	m.mu.Lock()
	b := m.backend
	m.mu.Unlock()
	return b.focusWindow(ctx, id)
}

func (m *Manager) WindowBounds(ctx context.Context, id string) (domain.WindowInfo, error) {
	if err := m.ready(); err != nil {
		return domain.WindowInfo{}, err
	}
	m.mu.Lock()
	b := m.backend
	m.mu.Unlock()
	return b.windowBounds(ctx, id)
}

func (m *Manager) Screenshot(ctx context.Context, windowID string) (domain.ComputerImage, error) {
	if err := m.ready(); err != nil {
		return domain.ComputerImage{}, err
	}
	m.mu.Lock()
	b := m.backend
	m.mu.Unlock()
	img, err := b.screenshot(ctx, windowID)
	if err != nil {
		return domain.ComputerImage{}, err
	}
	img = shrinkComputerImage(img, m.maxShotEdge())
	m.rememberShot(img)
	return img, nil
}

func (m *Manager) MouseMove(ctx context.Context, x, y int) error {
	if err := m.ready(); err != nil {
		return err
	}
	x, y = m.mapXY(x, y)
	m.mu.Lock()
	b := m.backend
	m.mu.Unlock()
	return b.mouseMove(ctx, x, y)
}

func (m *Manager) MouseClick(ctx context.Context, button domain.MouseButton, x, y, clicks int) error {
	if err := m.ready(); err != nil {
		return err
	}
	if clicks < 1 {
		clicks = 1
	}
	if x >= 0 && y >= 0 {
		x, y = m.mapXY(x, y)
	}
	m.mu.Lock()
	b := m.backend
	m.mu.Unlock()
	return b.mouseClick(ctx, button, x, y, clicks)
}

func (m *Manager) MouseDrag(ctx context.Context, x0, y0, x1, y1 int) error {
	if err := m.ready(); err != nil {
		return err
	}
	x0, y0 = m.mapXY(x0, y0)
	x1, y1 = m.mapXY(x1, y1)
	m.mu.Lock()
	b := m.backend
	m.mu.Unlock()
	return b.mouseDrag(ctx, x0, y0, x1, y1)
}

func (m *Manager) Scroll(ctx context.Context, x, y int, dir domain.ScrollDirection, amount int) error {
	if err := m.ready(); err != nil {
		return err
	}
	if amount < 1 {
		amount = 1
	}
	if x >= 0 && y >= 0 {
		x, y = m.mapXY(x, y)
	}
	m.mu.Lock()
	b := m.backend
	m.mu.Unlock()
	return b.scroll(ctx, x, y, dir, amount)
}

func (m *Manager) TypeText(ctx context.Context, text string) error {
	if err := m.ready(); err != nil {
		return err
	}
	m.mu.Lock()
	b := m.backend
	m.mu.Unlock()
	return b.typeText(ctx, text)
}

func (m *Manager) Key(ctx context.Context, key string) error {
	if err := m.ready(); err != nil {
		return err
	}
	m.mu.Lock()
	b := m.backend
	m.mu.Unlock()
	return b.key(ctx, key)
}

func (m *Manager) CursorPosition(ctx context.Context) (int, int, error) {
	if err := m.ready(); err != nil {
		return 0, 0, err
	}
	m.mu.Lock()
	b := m.backend
	m.mu.Unlock()
	return b.cursorPosition(ctx)
}
