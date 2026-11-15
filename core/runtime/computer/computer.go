// Package computer implements the OS-agnostic desktop-control surface behind
// the `computer` builtin tool. The Manager owns coordinate mapping and delegates
// input and capture to an OS backend or a containerized X11 desktop.
package computer

import (
	"bytes"
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
	screenshot(ctx context.Context, windowID string, zoom *domain.ScreenRect) (domain.ComputerImage, error)
	mouseMove(ctx context.Context, x, y int, modifiers []domain.KeyModifier) error
	mouseClick(ctx context.Context, button domain.MouseButton, x, y, clicks int, modifiers []domain.KeyModifier) error
	mouseDrag(ctx context.Context, x0, y0, x1, y1 int) error
	mouseButton(ctx context.Context, button domain.MouseButton, pressed bool) error
	scroll(ctx context.Context, x, y int, dir domain.ScrollDirection, amount int) error
	typeText(ctx context.Context, text string) error
	key(ctx context.Context, key string, hold bool) error
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
	shots    map[string]domain.ComputerImage
	zoomKeys map[string]string
	lastData map[string][]byte
	lastTurn map[string]string
}

// New builds a Manager for the current OS and probes availability.
func New(cfg domain.ConfigComputerSection) *Manager {
	m := &Manager{host: newBackend(), shots: make(map[string]domain.ComputerImage), zoomKeys: make(map[string]string), lastData: make(map[string][]byte), lastTurn: make(map[string]string)}
	m.backend = m.host
	m.Configure(cfg)
	return m
}

// SetDesktopEnvironment wires the sandbox desktop bridge and re-probes only
// when configuration changes; per-action readiness never executes the container.
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

// SetSession binds a session to its own project/workdir for sandboxed exec.
func (m *Manager) SetSession(opts port.SandboxRunOptions, sessionID string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.desktop != nil {
		m.desktop.SetSession(sessionID, opts)
		if m.status.Backend == "sandbox-desktop" && !m.status.Available {
			m.status = m.desktop.probeWithSession(m.cfg, opts)
			m.status.Enabled = m.cfg.Enabled
		}
	}
}

type sessionKey struct{}

func withSession(ctx context.Context, sessionID string) context.Context {
	return context.WithValue(ctx, sessionKey{}, sessionID)
}

func sessionFrom(ctx context.Context) string {
	value, _ := ctx.Value(sessionKey{}).(string)
	return value
}

type turnKey struct{}

func withTurn(ctx context.Context, turnID string) context.Context {
	return context.WithValue(ctx, turnKey{}, turnID)
}

func turnFrom(ctx context.Context) string {
	value, _ := ctx.Value(turnKey{}).(string)
	return value
}

// effectiveTargetLocked resolves auto to the actually selected backend target.
func (m *Manager) effectiveTargetLocked() string {
	target := domain.NormalizeComputerTarget(m.cfg.Target)
	if target == domain.ComputerTargetAuto {
		if m.desktop != nil && m.env != nil && m.env.DesktopProfile() == domain.SandboxProfileDesktop {
			return domain.ComputerTargetSandbox
		}
		return domain.ComputerTargetHost
	}
	return target
}

// Configure replaces policy and re-probes the selected backend.
func (m *Manager) Configure(cfg domain.ConfigComputerSection) {
	m.mu.Lock()
	defer m.mu.Unlock()
	cfg.Display = strings.TrimSpace(cfg.Display)
	cfg.Target = domain.NormalizeComputerTarget(cfg.Target)
	m.cfg = cfg
	m.reselectLocked()
}

func (m *Manager) reselectLocked() {
	previous := m.backend
	target := m.effectiveTargetLocked()
	if target == domain.ComputerTargetSandbox && m.desktop != nil {
		m.backend = m.desktop
	} else {
		m.backend = m.host
	}
	m.status = m.backend.probe(m.cfg)
	m.status.Enabled = m.cfg.Enabled
	if previous != m.backend {
		m.shots = make(map[string]domain.ComputerImage)
		m.zoomKeys = make(map[string]string)
		m.lastData = make(map[string][]byte)
		m.lastTurn = make(map[string]string)
	} else if m.backend == m.desktop && m.desktop != nil && m.cfg.Display != "" && m.desktop.display != m.cfg.Display {
		m.status = m.backend.probe(m.cfg)
		m.status.Enabled = m.cfg.Enabled
	}
}

// Status returns the last cached capability probe.
func (m *Manager) Status() domain.ComputerStatus {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.status
}

func (m *Manager) ready() (backend, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if !m.cfg.Enabled {
		return nil, fmt.Errorf("desktop control is disabled; set runtime.computer.enabled=true in config to allow the computer tool")
	}
	if !m.status.Available {
		reason := m.status.DegradedReason
		if reason == "" {
			reason = "no usable desktop backend for the selected target"
		}
		return nil, fmt.Errorf("desktop control unavailable: %s", reason)
	}
	return m.backend, nil
}

func (m *Manager) maxShotEdge() int {
	m.mu.Lock()
	edge := m.cfg.MaxShotEdge
	env := m.env
	target := m.effectiveTargetLocked()
	m.mu.Unlock()
	if edge != 0 {
		return edge // negative disables downscale
	}
	if target != domain.ComputerTargetHost && env != nil {
		if n := env.DesktopMaxShotEdge(); n > 0 {
			return n
		}
	}
	return domain.DefaultComputerMaxShotEdge
}

func scopeFor(sessionID, windowID string, zoom *domain.ScreenRect) string {
	if zoom == nil {
		return sessionID + "\x00" + windowID
	}
	return fmt.Sprintf("%s\x00%s\x00%d,%d,%d,%d", sessionID, windowID, zoom.X, zoom.Y, zoom.Width, zoom.Height)
}

func (m *Manager) rememberShot(img domain.ComputerImage) {
	m.mu.Lock()
	defer m.mu.Unlock()
	base := strings.SplitN(img.Scope, "\x00", 3)
	if len(base) < 2 {
		return
	}
	parent := base[0] + "\x00" + base[1]
	if previous := m.zoomKeys[parent]; previous != "" && previous != img.Scope {
		delete(m.shots, previous)
		delete(m.lastData, previous)
		delete(m.lastTurn, previous)
	}
	if len(base) == 3 {
		m.zoomKeys[parent] = img.Scope
	} else {
		delete(m.zoomKeys, parent)
	}
	m.shots[img.Scope] = img
}

func (m *Manager) mapXY(sessionID, windowID string, x, y int) (int, int, error) {
	m.mu.Lock()
	// A zoom supersedes its parent capture for that session/window. Full-screen
	// and window captures still have independent scopes and cannot cross-map.
	parent := scopeFor(sessionID, windowID, nil)
	img, ok := m.shots[parent]
	if zoomScope := m.zoomKeys[parent]; zoomScope != "" {
		img, ok = m.shots[zoomScope]
	}
	m.mu.Unlock()
	if !ok {
		return 0, 0, fmt.Errorf("no screenshot is available for this target; take a screenshot first")
	}
	return mapImagePoint(img, x, y)
}

func (m *Manager) ListWindows(ctx context.Context, query string) ([]domain.WindowInfo, error) {
	return m.ListWindowsForSession(ctx, "", query)
}

func (m *Manager) ListWindowsForSession(ctx context.Context, sessionID, query string) ([]domain.WindowInfo, error) {
	b, err := m.ready()
	if err != nil {
		return nil, err
	}
	return b.listWindows(withSession(ctx, sessionID), query)
}

func (m *Manager) FocusWindow(ctx context.Context, id string) error {
	return m.FocusWindowForSession(ctx, "", id)
}

func (m *Manager) FocusWindowForSession(ctx context.Context, sessionID, id string) error {
	b, err := m.ready()
	if err != nil {
		return err
	}
	return b.focusWindow(withSession(ctx, sessionID), id)
}

func (m *Manager) WindowBounds(ctx context.Context, id string) (domain.WindowInfo, error) {
	b, err := m.ready()
	if err != nil {
		return domain.WindowInfo{}, err
	}
	return b.windowBounds(ctx, id)
}

func (m *Manager) Screenshot(ctx context.Context, sessionID, windowID string, zoom *domain.ScreenRect) (domain.ComputerImage, error) {
	b, err := m.ready()
	if err != nil {
		return domain.ComputerImage{}, err
	}
	img, err := b.screenshot(withSession(ctx, sessionID), windowID, zoom)
	if err != nil {
		return domain.ComputerImage{}, err
	}
	img.Scope = scopeFor(sessionID, windowID, zoom)
	img = shrinkComputerImage(img, m.maxShotEdge())
	turnID := turnFrom(ctx)
	m.mu.Lock()
	// Compare against the previous capture before overwriting it, and only
	// within the same turn: the first screenshot of a new turn (or subagent)
	// must always deliver real image bytes so see→act→verify never goes blind.
	img.Unchanged = m.lastTurn[img.Scope] == turnID && bytes.Equal(m.lastData[img.Scope], img.Data)
	m.lastTurn[img.Scope] = turnID
	m.lastData[img.Scope] = bytes.Clone(img.Data)
	m.mu.Unlock()
	m.rememberShot(img)
	return img, nil
}

func (m *Manager) MouseMove(ctx context.Context, sessionID, windowID string, x, y int, modifiers []domain.KeyModifier) error {
	b, err := m.ready()
	if err != nil {
		return err
	}
	x, y, err = m.mapXY(sessionID, windowID, x, y)
	if err != nil {
		return err
	}
	return b.mouseMove(withSession(ctx, sessionID), x, y, modifiers)
}

func (m *Manager) MouseClick(ctx context.Context, sessionID, windowID string, button domain.MouseButton, x, y, clicks int, modifiers []domain.KeyModifier) error {
	b, err := m.ready()
	if err != nil {
		return err
	}
	if clicks < 1 {
		clicks = 1
	}
	if x >= 0 && y >= 0 {
		x, y, err = m.mapXY(sessionID, windowID, x, y)
		if err != nil {
			return err
		}
	}
	return b.mouseClick(withSession(ctx, sessionID), button, x, y, clicks, modifiers)
}

func (m *Manager) MouseDrag(ctx context.Context, sessionID, windowID string, x0, y0, x1, y1 int) error {
	b, err := m.ready()
	if err != nil {
		return err
	}
	x0, y0, err = m.mapXY(sessionID, windowID, x0, y0)
	if err != nil {
		return err
	}
	x1, y1, err = m.mapXY(sessionID, windowID, x1, y1)
	if err != nil {
		return err
	}
	return b.mouseDrag(withSession(ctx, sessionID), x0, y0, x1, y1)
}

func (m *Manager) MouseButton(ctx context.Context, button domain.MouseButton, pressed bool) error {
	b, err := m.ready()
	if err != nil {
		return err
	}
	return b.mouseButton(ctx, button, pressed)
}

func (m *Manager) Scroll(ctx context.Context, sessionID, windowID string, x, y int, dir domain.ScrollDirection, amount int) error {
	b, err := m.ready()
	if err != nil {
		return err
	}
	if amount < 1 {
		amount = 1
	}
	if x >= 0 && y >= 0 {
		x, y, err = m.mapXY(sessionID, windowID, x, y)
		if err != nil {
			return err
		}
	}
	return b.scroll(withSession(ctx, sessionID), x, y, dir, amount)
}

func (m *Manager) TypeText(ctx context.Context, text string) error {
	b, err := m.ready()
	if err != nil {
		return err
	}
	return b.typeText(ctx, text)
}

func (m *Manager) Key(ctx context.Context, key string, hold bool) error {
	b, err := m.ready()
	if err != nil {
		return err
	}
	return b.key(ctx, key, hold)
}

func (m *Manager) CursorPosition(ctx context.Context) (int, int, error) {
	b, err := m.ready()
	if err != nil {
		return 0, 0, err
	}
	return b.cursorPosition(ctx)
}
