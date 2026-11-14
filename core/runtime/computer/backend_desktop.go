package computer

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"image"
	"strings"
	"sync"
	"time"

	"danmo-work/core/domain"
	"danmo-work/core/port"
)

// desktopBackend is an X11 implementation with commands executed in a sandbox
// container. Input semantics are shared with the host X11 backend; only the
// executor, screenshot transport, and availability probe differ.
type desktopBackend struct {
	mu       sync.RWMutex
	desktop  port.DesktopEnvironment
	display  string
	sessions map[string]port.SandboxRunOptions
	probeFn  func(domain.ConfigComputerSection) domain.ComputerStatus
}

func newDesktopBackend(desktop port.DesktopEnvironment) *desktopBackend {
	return &desktopBackend{desktop: desktop, display: ":1", sessions: make(map[string]port.SandboxRunOptions)}
}

func (b *desktopBackend) SetSession(sessionID string, opts port.SandboxRunOptions) {
	if sessionID == "" {
		sessionID = "default"
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	b.sessions[sessionID] = opts
}

func (b *desktopBackend) session(ctx context.Context) port.SandboxRunOptions {
	b.mu.RLock()
	opts := b.sessions[sessionFrom(ctx)]
	if opts.ProjectID == "" && len(b.sessions) == 1 {
		for _, candidate := range b.sessions {
			opts = candidate
		}
	}
	b.mu.RUnlock()
	if opts.ProjectID == "" {
		opts.ProjectID = "default"
	}
	if opts.WorkDir == "" {
		opts.WorkDir = "."
	}
	return opts
}

func (b *desktopBackend) run(ctx context.Context, name string, args ...string) (string, error) {
	return b.runSession(ctx, b.session(ctx), name, args...)
}

func (b *desktopBackend) runSession(ctx context.Context, opts port.SandboxRunOptions, name string, args ...string) (string, error) {
	quoted := make([]string, 0, 1+len(args))
	quoted = append(quoted, shellQuote(name))
	for _, a := range args {
		quoted = append(quoted, shellQuote(a))
	}
	out, err := b.desktop.ExecDesktop(ctx, opts, strings.Join(quoted, " "), []string{"DISPLAY=" + b.display})
	if err != nil {
		msg := strings.TrimSpace(string(out))
		if msg == "" {
			msg = err.Error()
		}
		return "", fmt.Errorf("%s failed: %s", name, msg)
	}
	return strings.TrimSpace(string(out)), nil
}

func shellQuote(s string) string {
	if s == "" {
		return "''"
	}
	if !strings.ContainsAny(s, " \t\n'\"\\$`") {
		return s
	}
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}

func (b *desktopBackend) probe(cfg domain.ConfigComputerSection) domain.ComputerStatus {
	return b.probeWithSession(cfg, b.session(context.Background()))
}

func (b *desktopBackend) probeWithSession(cfg domain.ConfigComputerSection, opts port.SandboxRunOptions) domain.ComputerStatus {
	if b.probeFn != nil {
		return b.probeFn(cfg)
	}
	st := domain.ComputerStatus{Platform: "linux", Backend: "sandbox-desktop", Display: b.display}
	if b.desktop == nil || b.desktop.DesktopProfile() != domain.SandboxProfileDesktop {
		st.DegradedReason = "sandbox desktop profile not active"
		return st
	}
	if d := strings.TrimSpace(cfg.Display); d != "" {
		b.display = d
		st.Display = d
	}
	if opts.ProjectID == "" {
		opts.ProjectID = "default"
	}
	if opts.WorkDir == "" {
		opts.WorkDir = "."
	}
	opts.Timeout = 5 * time.Second
	if _, err := b.desktop.EnsureDesktop(context.Background(), opts); err != nil {
		st.DegradedReason = "sandbox desktop is not running: " + err.Error()
		return st
	}
	out, err := b.desktop.ExecDesktop(context.Background(), opts, "command -v xdotool && command -v import && command -v xdpyinfo", []string{"DISPLAY=" + b.display})
	if err != nil {
		st.DegradedReason = "desktop tools missing in container (xdotool, imagemagick, xdpyinfo): " + strings.TrimSpace(string(out))
		return st
	}
	st.Available = true
	return st
}

func (b *desktopBackend) screenshot(ctx context.Context, windowID string, zoom *domain.ScreenRect) (domain.ComputerImage, error) {
	opts := b.session(ctx)
	origin, screenW, screenH, err := captureGeometry(ctx, b, windowID)
	if err != nil {
		return domain.ComputerImage{}, err
	}
	if zoom != nil {
		if zoom.X < origin.X || zoom.Y < origin.Y || zoom.X+zoom.Width > origin.X+screenW || zoom.Y+zoom.Height > origin.Y+screenH {
			return domain.ComputerImage{}, fmt.Errorf("zoom rectangle is outside the selected screen region")
		}
		origin = image.Point{X: zoom.X, Y: zoom.Y}
		screenW, screenH = zoom.Width, zoom.Height
	}
	var suffix [6]byte
	if _, err := rand.Read(suffix[:]); err != nil {
		return domain.ComputerImage{}, err
	}
	path := "/tmp/dq-computer-" + hex.EncodeToString(suffix[:]) + ".png"
	args := []string{"-silent", "-window", "root", "-crop", fmt.Sprintf("%dx%d+%d+%d", screenW, screenH, origin.X, origin.Y), "+repage", path}
	script := "import " + joinShellArgs(args) + " && (base64 -w0 " + shellQuote(path) + " 2>/dev/null || base64 " + shellQuote(path) + " | tr -d '\\n'); code=$?; rm -f " + shellQuote(path) + "; exit $code"
	out, err := b.desktop.ExecDesktop(ctx, opts, script, []string{"DISPLAY=" + b.display})
	if err != nil {
		return domain.ComputerImage{}, fmt.Errorf("capture sandbox screen: %s", strings.TrimSpace(string(out)))
	}
	img, err := decodeShot(out, origin)
	if err != nil {
		return domain.ComputerImage{}, err
	}
	img.DisplayWidth, img.DisplayHeight = screenW, screenH
	return img, nil
}
