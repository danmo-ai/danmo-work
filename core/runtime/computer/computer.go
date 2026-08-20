// Package computer implements the OS-agnostic desktop-control surface behind
// the `computer` builtin tool. The Manager holds configuration and delegates to
// a per-OS backend (X11 via xdotool, macOS via osascript/screencapture, Windows
// via user32/PowerShell). When no backend is usable the Manager reports a
// degraded status and returns structured errors.
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
	mu      sync.Mutex
	cfg     domain.ConfigComputerSection
	status  domain.ComputerStatus
	backend backend
}

// New builds a Manager for the current OS and probes availability.
func New(cfg domain.ConfigComputerSection) *Manager {
	m := &Manager{backend: newBackend()}
	m.Configure(cfg)
	return m
}

// Configure replaces policy and re-probes the backend.
func (m *Manager) Configure(cfg domain.ConfigComputerSection) {
	m.mu.Lock()
	defer m.mu.Unlock()
	cfg.Display = strings.TrimSpace(cfg.Display)
	m.cfg = cfg
	m.status = m.backend.probe(cfg)
	m.status.Enabled = cfg.Enabled
}

// Status returns the last probed capability surface.
func (m *Manager) Status() domain.ComputerStatus {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.status
}

// ready validates that control is both enabled and available before an action.
func (m *Manager) ready() error {
	m.mu.Lock()
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

func (m *Manager) ListWindows(ctx context.Context, query string) ([]domain.WindowInfo, error) {
	if err := m.ready(); err != nil {
		return nil, err
	}
	return m.backend.listWindows(ctx, query)
}

func (m *Manager) FocusWindow(ctx context.Context, id string) error {
	if err := m.ready(); err != nil {
		return err
	}
	return m.backend.focusWindow(ctx, id)
}

func (m *Manager) WindowBounds(ctx context.Context, id string) (domain.WindowInfo, error) {
	if err := m.ready(); err != nil {
		return domain.WindowInfo{}, err
	}
	return m.backend.windowBounds(ctx, id)
}

func (m *Manager) Screenshot(ctx context.Context, windowID string) (domain.ComputerImage, error) {
	if err := m.ready(); err != nil {
		return domain.ComputerImage{}, err
	}
	return m.backend.screenshot(ctx, windowID)
}

func (m *Manager) MouseMove(ctx context.Context, x, y int) error {
	if err := m.ready(); err != nil {
		return err
	}
	return m.backend.mouseMove(ctx, x, y)
}

func (m *Manager) MouseClick(ctx context.Context, button domain.MouseButton, x, y, clicks int) error {
	if err := m.ready(); err != nil {
		return err
	}
	if clicks < 1 {
		clicks = 1
	}
	return m.backend.mouseClick(ctx, button, x, y, clicks)
}

func (m *Manager) MouseDrag(ctx context.Context, x0, y0, x1, y1 int) error {
	if err := m.ready(); err != nil {
		return err
	}
	return m.backend.mouseDrag(ctx, x0, y0, x1, y1)
}

func (m *Manager) Scroll(ctx context.Context, x, y int, dir domain.ScrollDirection, amount int) error {
	if err := m.ready(); err != nil {
		return err
	}
	if amount < 1 {
		amount = 1
	}
	return m.backend.scroll(ctx, x, y, dir, amount)
}

func (m *Manager) TypeText(ctx context.Context, text string) error {
	if err := m.ready(); err != nil {
		return err
	}
	return m.backend.typeText(ctx, text)
}

func (m *Manager) Key(ctx context.Context, key string) error {
	if err := m.ready(); err != nil {
		return err
	}
	return m.backend.key(ctx, key)
}

func (m *Manager) CursorPosition(ctx context.Context) (int, int, error) {
	if err := m.ready(); err != nil {
		return 0, 0, err
	}
	return m.backend.cursorPosition(ctx)
}
